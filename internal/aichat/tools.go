package aichat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
	searchdomain "github.com/Pherlsz/Gymkhana-Database/internal/search"
)

const maximumToolCellRunes = 500
const maximumModelSampleRows = 3
const maximumCompactEnumOptions = 12
const compactCatalogNotePrefix = "Cadastro lógico: "
const sampleNote = "total e match_count são a quantidade no cadastro inteiro. As linhas são só uma amostra para exemplos."
const advancedNote = "rows são o agrupamento, o padrão ou o conjunto, não uma amostra de cadastros. Numa contagem única, match_count é o total."

type SearchPort interface {
	Catalog(context.Context, auth.Session) (searchdomain.Catalog, error)
	Search(context.Context, auth.Session, searchdomain.Query) (searchdomain.Page, error)
}

type QueryPort interface {
	Catalog(context.Context, auth.Session, string) (querydomain.Catalog, error)
	Execute(context.Context, auth.Session, querydomain.QueryPlan, string, string) (querydomain.Execution, error)
	ExecuteV2(context.Context, auth.Session, querydomain.QueryPlan, string, string) (querydomain.Execution, error)
	CountMatches(context.Context, auth.Session, querydomain.QueryPlan, string) (int, error)
	ScanFields(context.Context, auth.Session, querydomain.QueryPlan, int, string) (querydomain.TextScan, error)
	ScanShape(context.Context, auth.Session, querydomain.QueryPlan, int, string) ([]querydomain.ShapeRow, bool, error)
	RunAdvanced(context.Context, auth.Session, querydomain.QueryPlan, string) (querydomain.AdvancedResult, error)
	Result(context.Context, auth.Session, querydomain.Identifier, int, int, string) (querydomain.ResultPage, error)
	ResultV2(context.Context, auth.Session, querydomain.Identifier, int, int, string) (querydomain.ResultPage, error)
}

type ResultReferencePort interface {
	ResultReference(context.Context, auth.Session, Identifier, string) (ResultReference, error)
}

type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ResultReferenceDraft struct {
	Kind               ResultReferenceKind
	QueryExecutionID   *Identifier
	LogicalRequest     []byte
	ContextFingerprint [sha256.Size]byte
	Label              string
	RowCount           int
	ColumnCount        int
	ExpiresAt          time.Time
}

type ToolOutput struct {
	Kind       ToolKind
	Payload    []byte
	RowCount   int
	FieldCount int
	ByteCount  int
	Reference  *ResultReferenceDraft
}

type ToolGateway struct {
	search SearchPort
	query  QueryPort
	refs   ResultReferencePort
	now    func() time.Time
}

func NewToolGateway(search SearchPort, query QueryPort, references ResultReferencePort, now func() time.Time) (*ToolGateway, error) {
	if search == nil || query == nil || references == nil {
		return nil, ErrInvalidSetup
	}
	if now == nil {
		now = time.Now
	}
	return &ToolGateway{search: search, query: query, refs: references, now: now}, nil
}

// queryToolSchema teaches the model the QueryPlan. v1 is a filtered list.
// v2 adds group, aggregate, pattern and set. Logical keys come from catalog.
const queryToolSchema = `{"type":"object","additionalProperties":false,"required":["plan"],"properties":{
"plan":{"type":"object","description":"QueryPlan. v1 lista com filtro. v2 agrupa, agrega, casa padrão ou combina conjuntos. Chaves lógicas da tool catalog. catalog_version = query.version.",
"required":["version","catalog_version","root_entity","projections"],
"properties":{
"version":{"type":"string","enum":["v1","v2"]},
"catalog_version":{"type":"string","description":"query.version do Cadastro lógico na política"},
"root_entity":{"type":"string","description":"Chave da entidade raiz (query.entities[].key). No conjunto, a entidade do primeiro plano."},
"projections":{"type":"array","minItems":0,"maxItems":20,"items":{"type":"string"},"description":"Deste passo: sempre o nome numa lista de pessoas, mais o que o passo pede para ver ou conferir. Campo com replaces: não projete o src. Vazio no v2 com aggregates ou set."},
"filter":{"type":"object","description":"Nó de filtro. kind=predicate: field, operator e values (strings). kind=group: conjunction AND|OR e children. Faixas distintas por letra: OR de grupos AND (Inicial + Número da casa). kind=not: children com um nó. kind=relation: relation (query.relations[].key) e children avaliados na entidade relacionada.",
"properties":{
"kind":{"type":"string","enum":["predicate","group","relation","not"]},
"conjunction":{"type":"string","enum":["AND","OR"]},
"field":{"type":"string"},
"other_field":{"type":"string","description":"Compara este campo com field, sem values."},
"operator":{"type":"string","enum":["eq","neq","contains","starts_with","gt","gte","lt","lte","between","in","is_null","not_null"]},
"values":{"type":"array","maxItems":50,"items":{"type":"string"}},
"relation":{"type":"string"},
"children":{"type":"array","items":{"type":"object"}}}},
"sort":{"type":"array","maxItems":3,"items":{"type":"object","required":["field","direction"],"properties":{"field":{"type":"string"},"direction":{"type":"string","enum":["asc","desc"]}}},"description":"O servidor ordena listas A-Z. Não envie sort numa lista de pessoas, documentos ou contas."},
"group_by":{"type":"array","maxItems":4,"items":{"type":"string"},"description":"v2. Campos do grupo."},
"aggregates":{"type":"array","maxItems":4,"items":{"type":"object","additionalProperties":false,"required":["key","function"],"properties":{"key":{"type":"string"},"function":{"type":"string","enum":["count","sum","average","minimum","maximum"]},"field":{"type":"string"},"distinct":{"type":"boolean"}}},"description":"v2. Não combine com projections."},
"patterns":{"type":"array","maxItems":4,"items":{"type":"object","additionalProperties":false,"required":["field","grammar","pattern"],"properties":{"field":{"type":"string"},"grammar":{"type":"string","enum":["literal_sequence","character_class","binary_digits","digits","letters","alphanumeric"]},"pattern":{"type":"string","maxLength":80},"anchored":{"type":"boolean"},"case_fold":{"type":"boolean"}}},"description":"v2. Padrão de caracteres num campo."},
"set":{"type":"object","description":"v2. operator union, intersection ou difference. inputs são outros sets, ou plan com root_entity, projections e filter."},
"combination":{"type":"object","description":"v2. Cruza planos e pode agregar depois. aggregates usa *:campo para somar a mesma projeção em cada entrada, ou chave:campo para uma entrada. having filtra o agregado."},
"derive":{"type":"array","maxItems":8,"description":"Coluna nova. op: keep, token, first_letter, fold, numbers, drop, pick, date_part, reverse. from é um campo do catalog ou o as de outra derivação. class: digits, letters, alnum. index da posição do token, negativo conta do fim. which: year, month, day, first, last.","items":{"type":"object","additionalProperties":false,"properties":{"as":{"type":"string"},"op":{"type":"string","enum":["keep","token","first_letter","fold","numbers","drop","pick","date_part","reverse"]},"from":{"type":"string"},"class":{"type":"string"},"index":{"type":"integer"},"pattern":{"type":"string"},"which":{"type":"string"}}}},
"matches":{"type":"array","maxItems":8,"description":"Casa o campo ou a coluna derivada. order keep devolve o trecho; any sem width exige o texto inteiro reordenado; any com width escolhe essa quantidade e o resto não impede. alphabet é o da pergunta, por exemplo 01 ou A-Z. equals lista os alvos. interpret byte enumera caracteres formáveis. show é o nome da coluna com os valores que couberam.","items":{"type":"object","additionalProperties":false,"properties":{"on":{"type":"string"},"grammar":{"type":"string","enum":["literal_sequence","character_class","binary_digits","digits","letters","alphanumeric"]},"pattern":{"type":"string"},"order":{"type":"string","enum":["keep","any"]},"width":{"type":"integer"},"alphabet":{"type":"string"},"anchored":{"type":"boolean"},"equals":{"type":"array","items":{"type":"string"}},"quantifier":{"type":"string","enum":["any","all"]},"interpret":{"type":"string"},"show":{"type":"string"}}}},
"sequence":{"type":"object","additionalProperties":false,"description":"Cadeia sobre as linhas já filtradas. by é a coluna do passo. step: next, increase, decrease ou either_monotonic. along repete a direção numa segunda coluna. partition parte a cadeia. tie: distinct, start ou sum. Sem tie, empate devolve todas. minimum é o tamanho mínimo.","properties":{"by":{"type":"string"},"alphabet":{"type":"string"},"step":{"type":"string","enum":["next","increase","decrease","either_monotonic"]},"along":{"type":"object","additionalProperties":false,"properties":{"field":{"type":"string"},"step":{"type":"string"}}},"one_per":{"type":"string"},"partition":{"type":"string"},"pick":{"type":"string"},"tie":{"type":"string"},"minimum":{"type":"integer"},"distinct":{"type":"string"}}},
"maximum_rows":{"type":"integer","minimum":1,"maximum":100}}},
"context_reference_id":{"type":"string","format":"uuid","description":"Refina um resultado de query anterior desta conversa (\"desses…\")"}}}`

func (gateway *ToolGateway) Schemas() []ToolSchema {
	return []ToolSchema{
		{Name: "catalog", Description: "Cadastro lógico extra. Só chame se faltar um campo no bloco da política.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`)},
		{Name: "query", Description: "Consulta o cadastro. catalog_version está na política. Inicial: profile.name_initial (A-Z). Número da casa: profile.address_house_number (inteiro; 3/3=3; s/n vazio). Não use between em profile.address_number. Faixas por letra: OR de AND. Se a tool recusar um ramo, não publique a grade. Projeções deste passo: sempre o nome numa lista de pessoas, mais o que o passo pede para ver ou conferir. Campo com replaces: não projete também o src. match_count é o total. Listas A-Z; omita sort. context_reference_id refina o resultado ativo.", InputSchema: json.RawMessage(queryToolSchema)},
		{Name: "result", Description: "Reabre uma referência de resultado da própria conversa com reautorização.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["reference_id"],"properties":{"reference_id":{"type":"string","format":"uuid"},"limit":{"type":"integer","minimum":1,"maximum":100},"offset":{"type":"integer","minimum":0,"maximum":10000}}}`)},
	}
}

func (gateway *ToolGateway) ReadResult(ctx context.Context, actor auth.Session, referenceID Identifier, limit, offset int, requestID string) (ToolOutput, error) {
	if gateway == nil || referenceID.IsZero() {
		return ToolOutput{}, ErrInvalidInput
	}
	arguments, err := json.Marshal(resultToolRequest{ReferenceID: referenceID.String(), Limit: limit, Offset: offset})
	if err != nil {
		return ToolOutput{}, fmt.Errorf("encode AI Chat result request: %w", err)
	}
	return gateway.result(ctx, actor, arguments, requestID)
}

func (gateway *ToolGateway) Execute(ctx context.Context, actor auth.Session, activeReferenceID *Identifier, runID Identifier, step int, call ToolCall, referenceExpiresAt time.Time, requestID string) (ToolOutput, error) {
	if gateway == nil || !validProviderCallID(call.ID) || runID.IsZero() || step < 1 || step > MaximumToolCalls ||
		referenceExpiresAt.IsZero() || !referenceExpiresAt.After(gateway.now().UTC()) {
		return ToolOutput{}, ErrInvalidInput
	}
	switch call.Name {
	case "catalog":
		return gateway.catalog(ctx, actor, call.Arguments, requestID)
	case "search":
		return gateway.searchData(ctx, actor, activeReferenceID, call.Arguments, referenceExpiresAt, requestID)
	case "query":
		return gateway.queryData(ctx, actor, activeReferenceID, runID, step, call.Arguments, referenceExpiresAt, requestID)
	case "sequencia":
		return gateway.sequenceData(ctx, actor, call.Arguments, requestID)
	case "tarefa":
		return gateway.taskData(ctx, actor, call.Arguments, requestID)
	case "result":
		return gateway.result(ctx, actor, call.Arguments, requestID)
	default:
		return ToolOutput{}, ErrInvalidInput
	}
}

type searchToolRequest struct {
	Q                  string                `json:"q"`
	Terms              []string              `json:"terms"`
	Modules            []searchdomain.Module `json:"modules,omitempty"`
	Fields             []string              `json:"fields,omitempty"`
	Limit              int32                 `json:"limit,omitempty"`
	Offset             int32                 `json:"offset,omitempty"`
	ContextReferenceID string                `json:"context_reference_id,omitempty"`
}

type storedSearchRequest struct {
	Q       string                 `json:"q,omitempty"`
	Terms   []string               `json:"terms"`
	Modules []searchdomain.Module  `json:"modules,omitempty"`
	Fields  []string               `json:"fields,omitempty"`
	Limit   int32                  `json:"limit"`
	Offset  int32                  `json:"offset"`
	Sort    searchdomain.SortField `json:"sort"`
	Order   searchdomain.SortOrder `json:"order"`
}

type queryToolRequest struct {
	Plan               querydomain.QueryPlan `json:"plan"`
	ContextReferenceID string                `json:"context_reference_id,omitempty"`
}

type storedQueryRequest struct {
	Plan querydomain.QueryPlan `json:"plan"`
}

type resultToolRequest struct {
	ReferenceID string `json:"reference_id"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
}

type searchResult struct {
	Module      searchdomain.Module `json:"module"`
	EntityKind  string              `json:"entity_kind"`
	EntityID    string              `json:"entity_id"`
	ProfileID   string              `json:"profile_id,omitempty"`
	TargetKind  string              `json:"target_kind"`
	TargetID    string              `json:"target_id"`
	EntityLabel string              `json:"entity_label"`
	FieldKey    string              `json:"field_key"`
	FieldLabel  string              `json:"field_label"`
	Preview     string              `json:"preview"`
	Score       int32               `json:"score"`
}

type queryCell struct {
	ColumnPosition int                   `json:"column_position"`
	Kind           querydomain.ValueKind `json:"kind"`
	IsNull         bool                  `json:"is_null"`
	Value          any                   `json:"value,omitempty"`
	Truncated      bool                  `json:"truncated,omitempty"`
}

type compactChatEntity struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type compactChatField struct {
	Key       string                         `json:"key"`
	Entity    string                         `json:"entity"`
	Label     string                         `json:"label"`
	Kind      querydomain.ValueKind          `json:"kind"`
	Operators []querydomain.Operator         `json:"ops,omitempty"`
	Options   []querydomain.OptionDefinition `json:"opts,omitempty"`
	Source    string                         `json:"src,omitempty"`
	Replaces  bool                           `json:"replaces,omitempty"`
}

type compactChatRelation struct {
	Key   string                          `json:"key"`
	From  string                          `json:"from"`
	To    string                          `json:"to"`
	Label string                          `json:"label"`
	Card  querydomain.RelationCardinality `json:"card,omitempty"`
}

type compactChatCatalog struct {
	Version   string                `json:"version"`
	Entities  []compactChatEntity   `json:"entities"`
	Fields    []compactChatField    `json:"fields"`
	Relations []compactChatRelation `json:"relations,omitempty"`
}

func compactQueryCatalog(catalog querydomain.Catalog) compactChatCatalog {
	fields := make([]compactChatField, 0, len(catalog.Fields))
	for _, field := range catalog.Fields {
		item := compactChatField{Key: field.Key, Entity: field.Entity, Label: field.Label, Kind: field.Kind, Operators: field.Operators, Source: field.Source, Replaces: field.Replaces}
		if len(field.Options) > 0 && len(field.Options) <= maximumCompactEnumOptions {
			item.Options = field.Options
		}
		fields = append(fields, item)
	}
	relations := make([]compactChatRelation, 0, len(catalog.Relations))
	for _, relation := range catalog.Relations {
		relations = append(relations, compactChatRelation{Key: relation.Key, From: relation.FromEntity, To: relation.ToEntity, Label: relation.Label, Card: relation.Cardinality})
	}
	entities := make([]compactChatEntity, 0, len(catalog.Entities))
	for _, entity := range catalog.Entities {
		entities = append(entities, compactChatEntity{Key: entity.Key, Label: entity.Label})
	}
	return compactChatCatalog{Version: catalog.Version, Entities: entities, Fields: fields, Relations: relations}
}

func compactCatalogNote(catalog querydomain.Catalog) string {
	if catalog.Version == "" {
		return ""
	}
	encoded, err := json.Marshal(compactQueryCatalog(catalog))
	if err != nil || len(encoded) == 0 {
		return ""
	}
	return compactCatalogNotePrefix + string(encoded)
}

func (gateway *ToolGateway) CompactCatalogNote(ctx context.Context, actor auth.Session, requestID string) (string, error) {
	if gateway == nil || gateway.query == nil {
		return "", ErrInvalidSetup
	}
	catalog, err := gateway.query.Catalog(ctx, actor, requestID)
	note := compactCatalogNote(catalog)
	if note == "" {
		return "", err
	}
	return note, nil
}

func (gateway *ToolGateway) catalog(ctx context.Context, actor auth.Session, raw json.RawMessage, requestID string) (ToolOutput, error) {
	var request struct{}
	if err := decodeToolArguments(raw, &request); err != nil {
		return ToolOutput{}, err
	}
	queryCatalog, err := gateway.query.Catalog(ctx, actor, requestID)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	payload, err := marshalBoundedToolPayload(compactQueryCatalog(queryCatalog))
	if err != nil {
		return ToolOutput{}, err
	}
	return ToolOutput{Kind: ToolCatalog, Payload: payload, ByteCount: len(payload)}, nil
}

func (gateway *ToolGateway) searchData(ctx context.Context, actor auth.Session, active *Identifier, raw json.RawMessage, expiresAt time.Time, requestID string) (ToolOutput, error) {
	var request searchToolRequest
	if err := decodeToolArguments(raw, &request); err != nil {
		return ToolOutput{}, err
	}
	query := searchdomain.Query{Q: request.Q, Terms: request.Terms, Modules: request.Modules, Fields: request.Fields, Limit: request.Limit, Offset: request.Offset,
		Sort: searchdomain.SortRelevance, Order: searchdomain.SortDescending}
	if request.ContextReferenceID != "" {
		base, err := gateway.explicitContext(ctx, actor, active, request.ContextReferenceID, ResultReferenceSearch, requestID)
		if err != nil {
			return ToolOutput{}, err
		}
		var previous storedSearchRequest
		if err := json.Unmarshal(base.LogicalRequest, &previous); err != nil {
			return ToolOutput{}, ErrStaleContext
		}
		if base.ExpiresAt.Before(expiresAt) {
			expiresAt = base.ExpiresAt
		}
		query.Terms = append(append([]string(nil), previous.Terms...), query.Terms...)
		query.Terms = uniqueStrings(query.Terms)
		if query.Q == "" {
			query.Q = previous.Q
		} else if previous.Q != "" {
			query.Q = strings.TrimSpace(previous.Q + " " + query.Q)
		}
		if len(query.Modules) == 0 {
			query.Modules = previous.Modules
		}
		if len(query.Fields) == 0 {
			query.Fields = previous.Fields
		}
	}
	if query.Limit == 0 || query.Limit > MaximumToolRows {
		query.Limit = MaximumToolRows
	}
	page, err := gateway.search.Search(ctx, actor, query)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	results := make([]searchResult, 0, len(page.Results))
	for _, value := range page.Results {
		results = append(results, searchResult{Module: value.Module, EntityKind: value.EntityKind, EntityID: value.EntityID,
			ProfileID: value.ProfileID, TargetKind: value.TargetKind, TargetID: value.TargetID, EntityLabel: value.EntityLabel,
			FieldKey: value.FieldKey, FieldLabel: value.FieldLabel, Preview: truncateRunes(value.Preview, maximumToolCellRunes), Score: value.Score})
	}
	payload, err := marshalBoundedToolPayload(struct {
		Note    string         `json:"note"`
		Results []searchResult `json:"results"`
		Total   int64          `json:"total"`
		Limit   int32          `json:"limit"`
		Offset  int32          `json:"offset"`
	}{Note: sampleNote, Results: results, Total: page.Total, Limit: page.Limit, Offset: page.Offset})
	if err != nil {
		return ToolOutput{}, err
	}
	stored := storedSearchRequest{Q: query.Q, Terms: query.Terms, Modules: query.Modules, Fields: query.Fields, Limit: page.Limit, Offset: page.Offset, Sort: page.Sort, Order: page.Order}
	logical, err := json.Marshal(stored)
	if err != nil {
		return ToolOutput{}, fmt.Errorf("encode AI Chat Search reference: %w", err)
	}
	label := query.Q
	if label == "" {
		label = strings.Join(query.Terms, " · ")
	}
	draft := resultDraft(ResultReferenceSearch, nil, logical, fmt.Sprintf("Busca: %s", label), len(results), 1, expiresAt)
	return ToolOutput{Kind: ToolSearch, Payload: payload, RowCount: len(results), FieldCount: 1, ByteCount: len(payload), Reference: &draft}, nil
}

func (gateway *ToolGateway) queryData(ctx context.Context, actor auth.Session, active *Identifier, runID Identifier, step int, raw json.RawMessage, referenceExpiresAt time.Time, requestID string) (ToolOutput, error) {
	var request queryToolRequest
	if err := decodeToolArguments(raw, &request); err != nil {
		return ToolOutput{}, err
	}
	if request.ContextReferenceID != "" {
		base, err := gateway.explicitContext(ctx, actor, active, request.ContextReferenceID, ResultReferenceQuery, requestID)
		if err != nil {
			return ToolOutput{}, err
		}
		var previous storedQueryRequest
		if err := json.Unmarshal(base.LogicalRequest, &previous); err != nil {
			return ToolOutput{}, ErrStaleContext
		}
		if base.ExpiresAt.Before(referenceExpiresAt) {
			referenceExpiresAt = base.ExpiresAt
		}
		if err := refineQueryPlan(&request.Plan, previous.Plan); err != nil {
			return ToolOutput{}, err
		}
	}
	if request.Plan.MaximumRows == 0 || request.Plan.MaximumRows > MaximumToolRows {
		request.Plan.MaximumRows = MaximumToolRows
	}
	stampChatListSort(&request.Plan)
	if catalog, err := gateway.query.Catalog(ctx, actor, requestID); err == nil {
		stampChatReplacedProjections(&request.Plan, catalog)
	}
	if querydomain.PlanNeedsShape(request.Plan) && !planMaterializesSQL(request.Plan) {
		return gateway.queryShape(ctx, actor, request.Plan, referenceExpiresAt, requestID)
	}
	idempotencyKey := fmt.Sprintf("chat.%s.%d", runID.String(), step)
	var execution querydomain.Execution
	var page querydomain.ResultPage
	var matchCount int
	var err error
	note := sampleNote
	if planIsAdvanced(request.Plan) {
		stampChatAdvanced(&request.Plan)
		execution, err = gateway.query.ExecuteV2(ctx, actor, request.Plan, idempotencyKey, requestID)
		if err != nil {
			return ToolOutput{}, normalizeToolError(err)
		}
		page, err = gateway.query.ResultV2(ctx, actor, execution.ID, MaximumToolRows, 0, requestID)
		if err != nil {
			return ToolOutput{}, normalizeToolError(err)
		}
		matchCount = aggregateMatchCount(page)
		note = advancedNote
	} else {
		execution, err = gateway.query.Execute(ctx, actor, request.Plan, idempotencyKey, requestID)
		if err != nil {
			return ToolOutput{}, normalizeToolError(err)
		}
		page, err = gateway.query.Result(ctx, actor, execution.ID, MaximumToolRows, 0, requestID)
		if err != nil {
			return ToolOutput{}, normalizeToolError(err)
		}
		matchCount, err = gateway.query.CountMatches(ctx, actor, request.Plan, requestID)
		if err != nil {
			return ToolOutput{}, normalizeToolError(err)
		}
	}
	payload, err := queryResultPayload(page, matchCount, note)
	if err != nil {
		return ToolOutput{}, err
	}
	logical, err := json.Marshal(storedQueryRequest{Plan: request.Plan})
	if err != nil {
		return ToolOutput{}, fmt.Errorf("encode AI Chat Query reference: %w", err)
	}
	executionID := Identifier(execution.ID)
	if execution.ExpiresAt.Before(referenceExpiresAt) {
		referenceExpiresAt = execution.ExpiresAt
	}
	draft := resultDraft(ResultReferenceQuery, &executionID, logical, "Consulta: "+execution.RootEntity, len(page.Rows), len(page.Columns), referenceExpiresAt)
	return ToolOutput{Kind: ToolQuery, Payload: payload, RowCount: len(page.Rows), FieldCount: len(page.Columns), ByteCount: len(payload), Reference: &draft}, nil
}

func planIsAdvanced(plan querydomain.QueryPlan) bool {
	return plan.Version == querydomain.PlanVersionV2 || len(plan.GroupBy) > 0 || len(plan.Aggregates) > 0 ||
		plan.Having != nil || len(plan.Patterns) > 0 || plan.Set != nil || plan.Combination != nil ||
		querydomain.PlanNeedsShape(plan)
}

func planMaterializesSQL(plan querydomain.QueryPlan) bool {
	return len(plan.GroupBy) > 0 || len(plan.Aggregates) > 0 || len(plan.Patterns) > 0 || plan.Set != nil || plan.Combination != nil
}

func advancedShapeEmpty(plan querydomain.QueryPlan) bool {
	return len(plan.GroupBy) == 0 && len(plan.Aggregates) == 0 && plan.Having == nil &&
		len(plan.Patterns) == 0 && plan.Set == nil && plan.Combination == nil
}

// stampChatAdvanced marks a chat plan as v2 and copies the catalog version into
// nested set plans. ponytail: length-64 is enough for the engine; ExecuteV2
// compiles against the live catalog. Upgrade path is the public v2 fingerprint.
func stampChatAdvanced(plan *querydomain.QueryPlan) {
	if plan == nil {
		return
	}
	plan.Version = querydomain.PlanVersionV2
	if plan.MaximumRows == 0 || plan.MaximumRows > MaximumToolRows {
		plan.MaximumRows = MaximumToolRows
	}
	stampChatSet(plan.Set, plan.CatalogVersion)
	stampChatListSort(plan)
}

func stampChatSet(set *querydomain.SetExpression, catalogVersion string) {
	if set == nil {
		return
	}
	if set.Plan != nil {
		set.Plan.Version = querydomain.PlanVersionV2
		if set.Plan.CatalogVersion == "" {
			set.Plan.CatalogVersion = catalogVersion
		}
		if set.Plan.MaximumRows == 0 || set.Plan.MaximumRows > MaximumToolRows {
			set.Plan.MaximumRows = MaximumToolRows
		}
		stampChatSet(set.Plan.Set, catalogVersion)
		stampChatListSort(set.Plan)
	}
	for index := range set.Inputs {
		stampChatSet(&set.Inputs[index], catalogVersion)
	}
}

func stampChatListSort(plan *querydomain.QueryPlan) {
	if plan == nil || plan.Sequence != nil {
		return
	}
	if len(plan.GroupBy) > 0 || len(plan.Aggregates) > 0 || plan.Combination != nil {
		return
	}
	if field := alphabeticalSortField(plan.RootEntity); field != "" {
		plan.Sort = []querydomain.Sort{{Field: field, Direction: querydomain.SortAscending}}
	}
	stampChatNameProjection(plan)
}

func stampChatNameProjection(plan *querydomain.QueryPlan) {
	if plan == nil || strings.TrimSpace(plan.RootEntity) != "profiles" {
		return
	}
	const name = "profile.full_name"
	for _, key := range plan.Projections {
		if key == name {
			return
		}
	}
	if len(plan.Projections) >= querydomain.MaximumProjections {
		return
	}
	plan.Projections = append([]string{name}, plan.Projections...)
}

func stampChatReplacedProjections(plan *querydomain.QueryPlan, catalog querydomain.Catalog) {
	if plan == nil || len(plan.Projections) == 0 {
		return
	}
	projected := make(map[string]struct{}, len(plan.Projections))
	for _, key := range plan.Projections {
		projected[key] = struct{}{}
	}
	drop := make(map[string]struct{})
	for _, field := range catalog.Fields {
		if !field.Replaces || field.Source == "" {
			continue
		}
		if _, ok := projected[field.Key]; !ok {
			continue
		}
		drop[field.Source] = struct{}{}
	}
	if len(drop) == 0 {
		return
	}
	kept := make([]string, 0, len(plan.Projections))
	for _, key := range plan.Projections {
		if _, ok := drop[key]; ok {
			continue
		}
		kept = append(kept, key)
	}
	plan.Projections = kept
}

func alphabeticalSortField(root string) string {
	switch strings.TrimSpace(root) {
	case "profiles":
		return "profile.full_name"
	case "documents":
		return "document.owner_name"
	case "bills":
		return "bill.owner_name"
	case "attachments":
		return "attachment.filename"
	default:
		return ""
	}
}

func aggregateMatchCount(page querydomain.ResultPage) int {
	if len(page.Rows) == 1 && len(page.Rows[0].Cells) == 1 {
		cell := page.Rows[0].Cells[0]
		if cell.IntegerValue != nil && *cell.IntegerValue >= 0 {
			return int(*cell.IntegerValue)
		}
	}
	return len(page.Rows)
}

func refineQueryPlan(current *querydomain.QueryPlan, previous querydomain.QueryPlan) error {
	previousAdvanced := planIsAdvanced(previous)
	if current == nil || previous.RootEntity == "" || (previous.Version != querydomain.PlanVersionV1 && !previousAdvanced) {
		return ErrStaleContext
	}
	if current.Version == "" {
		current.Version = previous.Version
	}
	if current.CatalogVersion == "" {
		current.CatalogVersion = previous.CatalogVersion
	}
	if current.RootEntity == "" {
		current.RootEntity = previous.RootEntity
	}
	if current.RootEntity != previous.RootEntity {
		return ErrStaleContext
	}
	currentAdvanced := planIsAdvanced(*current)
	if !currentAdvanced && previousAdvanced && !advancedShapeEmpty(previous) {
		return ErrStaleContext
	}
	if !currentAdvanced && previousAdvanced && advancedShapeEmpty(previous) && querydomain.PlanNeedsShape(previous) {
		current.Version = previous.Version
		if current.CatalogVersion == "" || current.CatalogVersion == previous.CatalogVersion {
			current.CatalogVersion = previous.CatalogVersion
		}
	}
	if !currentAdvanced && (current.Version != previous.Version || current.CatalogVersion != previous.CatalogVersion) {
		return ErrStaleContext
	}
	if currentAdvanced && advancedShapeEmpty(*current) {
		current.GroupBy = append([]string(nil), previous.GroupBy...)
		current.Aggregates = append([]querydomain.Aggregate(nil), previous.Aggregates...)
		current.Patterns = append([]querydomain.PatternPredicate(nil), previous.Patterns...)
		current.Having = previous.Having
		current.Set = previous.Set
		current.Combination = previous.Combination
	}
	if len(current.Derive) == 0 && len(current.Matches) == 0 && current.Sequence == nil {
		current.Derive = append([]querydomain.Derivation(nil), previous.Derive...)
		current.Matches = append([]querydomain.MatchSpec(nil), previous.Matches...)
		if previous.Sequence != nil {
			spec := *previous.Sequence
			current.Sequence = &spec
		}
	}
	if len(current.Projections) == 0 && advancedShapeEmpty(*current) {
		current.Projections = append([]string(nil), previous.Projections...)
	}
	if previous.Filter != nil {
		prior := *previous.Filter
		if current.Filter == nil {
			current.Filter = &prior
		} else {
			currentFilter := *current.Filter
			current.Filter = &querydomain.FilterNode{Kind: querydomain.FilterGroup, Conjunction: querydomain.ConjunctionAnd, Children: []querydomain.FilterNode{prior, currentFilter}}
		}
	}
	return nil
}

func (gateway *ToolGateway) result(ctx context.Context, actor auth.Session, raw json.RawMessage, requestID string) (ToolOutput, error) {
	var request resultToolRequest
	if err := decodeToolArguments(raw, &request); err != nil {
		return ToolOutput{}, err
	}
	id, err := ParseIdentifier(request.ReferenceID)
	if err != nil {
		return ToolOutput{}, ErrInvalidInput
	}
	reference, err := gateway.refs.ResultReference(ctx, actor, id, requestID)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	if request.Limit == 0 || request.Limit > MaximumToolRows {
		request.Limit = MaximumToolRows
	}
	if request.Offset < 0 || request.Offset > 10_000 {
		return ToolOutput{}, ErrInvalidInput
	}
	switch reference.Kind {
	case ResultReferenceSearch:
		var stored storedSearchRequest
		if err := json.Unmarshal(reference.LogicalRequest, &stored); err != nil {
			return ToolOutput{}, ErrStaleContext
		}
		page, err := gateway.search.Search(ctx, actor, searchdomain.Query{Terms: stored.Terms, Modules: stored.Modules, Fields: stored.Fields,
			Limit: int32(request.Limit), Offset: int32(request.Offset), Sort: stored.Sort, Order: stored.Order})
		if err != nil {
			return ToolOutput{}, normalizeToolError(err)
		}
		results := make([]searchResult, 0, len(page.Results))
		for _, value := range page.Results {
			results = append(results, searchResult{Module: value.Module, EntityKind: value.EntityKind, EntityID: value.EntityID,
				ProfileID: value.ProfileID, TargetKind: value.TargetKind, TargetID: value.TargetID, EntityLabel: value.EntityLabel,
				FieldKey: value.FieldKey, FieldLabel: value.FieldLabel, Preview: truncateRunes(value.Preview, maximumToolCellRunes), Score: value.Score})
		}
		payload, err := marshalBoundedToolPayload(struct {
			Note        string         `json:"note"`
			ReferenceID string         `json:"reference_id"`
			Results     []searchResult `json:"results"`
			Total       int64          `json:"total"`
		}{Note: sampleNote, ReferenceID: reference.ID.String(), Results: results, Total: page.Total})
		if err != nil {
			return ToolOutput{}, err
		}
		return ToolOutput{Kind: ToolResult, Payload: payload, RowCount: len(results), FieldCount: 1, ByteCount: len(payload)}, nil
	case ResultReferenceQuery:
		var stored storedQueryRequest
		if err := json.Unmarshal(reference.LogicalRequest, &stored); err != nil {
			return ToolOutput{}, ErrStaleContext
		}
		if reference.QueryExecutionID == nil || (querydomain.PlanNeedsShape(stored.Plan) && !planMaterializesSQL(stored.Plan)) {
			if !querydomain.PlanNeedsShape(stored.Plan) || planMaterializesSQL(stored.Plan) {
				return ToolOutput{}, ErrStaleContext
			}
			grid, err := gateway.readShapeGrid(ctx, actor, stored.Plan, request.Limit, request.Offset, requestID)
			if err != nil {
				return ToolOutput{}, err
			}
			payload, err := shapeToolPayload(grid.page())
			if err != nil {
				return ToolOutput{}, err
			}
			return ToolOutput{Kind: ToolResult, Payload: payload, RowCount: len(grid.Rows), FieldCount: len(grid.Columns), ByteCount: len(payload)}, nil
		}
		executionID := querydomain.Identifier(*reference.QueryExecutionID)
		var page querydomain.ResultPage
		var matchCount int
		note := sampleNote
		if planIsAdvanced(stored.Plan) {
			page, err = gateway.query.ResultV2(ctx, actor, executionID, request.Limit, request.Offset, requestID)
			if err != nil {
				return ToolOutput{}, normalizeToolError(err)
			}
			matchCount = aggregateMatchCount(page)
			note = advancedNote
		} else {
			page, err = gateway.query.Result(ctx, actor, executionID, request.Limit, request.Offset, requestID)
			if err != nil {
				return ToolOutput{}, normalizeToolError(err)
			}
			matchCount, err = gateway.query.CountMatches(ctx, actor, stored.Plan, requestID)
			if err != nil {
				return ToolOutput{}, normalizeToolError(err)
			}
		}
		payload, err := queryResultPayload(page, matchCount, note)
		if err != nil {
			return ToolOutput{}, err
		}
		return ToolOutput{Kind: ToolResult, Payload: payload, RowCount: len(page.Rows), FieldCount: len(page.Columns), ByteCount: len(payload)}, nil
	default:
		return ToolOutput{}, ErrStaleContext
	}
}

func (gateway *ToolGateway) explicitContext(ctx context.Context, actor auth.Session, active *Identifier, value string, kind ResultReferenceKind, requestID string) (ResultReference, error) {
	id, err := ParseIdentifier(value)
	if err != nil || active == nil || *active != id {
		return ResultReference{}, ErrStaleContext
	}
	reference, err := gateway.refs.ResultReference(ctx, actor, id, requestID)
	if err != nil {
		return ResultReference{}, normalizeToolError(err)
	}
	if reference.Kind != kind {
		return ResultReference{}, ErrStaleContext
	}
	return reference, nil
}

func queryResultPayload(page querydomain.ResultPage, matchCount int, note string) ([]byte, error) {
	source := page.Rows
	if len(source) > maximumModelSampleRows {
		source = source[:maximumModelSampleRows]
	}
	labels := make(map[int]string, len(page.Columns))
	for _, column := range page.Columns {
		label := column.Label
		if label == "" {
			label = column.FieldKey
		}
		labels[column.Position] = label
	}
	truncated := false
	rows := make([]struct {
		Title  string         `json:"titulo"`
		Values map[string]any `json:"vals"`
	}, 0, len(source))
	for _, value := range source {
		vals := make(map[string]any, len(value.Cells))
		for _, cell := range value.Cells {
			label := labels[cell.ColumnPosition]
			if label == "" {
				label = fmt.Sprintf("%d", cell.ColumnPosition)
			}
			item := safeQueryCell(cell)
			if item.Truncated {
				truncated = true
			}
			if item.IsNull {
				vals[label] = nil
				continue
			}
			vals[label] = item.Value
		}
		rows = append(rows, struct {
			Title  string         `json:"titulo"`
			Values map[string]any `json:"vals"`
		}{Title: truncateRunes(value.EntityLabel, maximumToolCellRunes), Values: vals})
	}
	return marshalBoundedToolPayload(struct {
		Note       string `json:"note"`
		MatchCount int    `json:"match_count"`
		Truncated  bool   `json:"truncated,omitempty"`
		Rows       any    `json:"rows"`
	}{Note: note, MatchCount: matchCount, Truncated: truncated, Rows: rows})
}

func safeQueryCell(value querydomain.ResultCell) queryCell {
	cell := queryCell{ColumnPosition: value.ColumnPosition, Kind: value.Kind, IsNull: value.IsNull}
	if value.IsNull {
		return cell
	}
	switch {
	case value.TextValue != nil:
		cell.Value, cell.Truncated = truncateWithFlag(*value.TextValue, maximumToolCellRunes)
	case value.IntegerValue != nil:
		cell.Value = *value.IntegerValue
	case value.DecimalValue != nil:
		cell.Value = *value.DecimalValue
	case value.BooleanValue != nil:
		cell.Value = *value.BooleanValue
	case value.CivilDateValue != nil:
		cell.Value = *value.CivilDateValue
	case value.TimestampValue != nil:
		cell.Value = value.TimestampValue.UTC().Format(time.RFC3339Nano)
	}
	return cell
}

func decodeToolArguments(raw json.RawMessage, destination any) error {
	if len(raw) == 0 || len(raw) > 32*1024 || !json.Valid(raw) {
		return ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return ErrInvalidInput
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrInvalidInput
	}
	return nil
}

func marshalBoundedToolPayload(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode AI Chat tool result: %w", err)
	}
	if len(encoded) > MaximumToolResultBytes {
		return nil, ErrQuotaExceeded
	}
	return encoded, nil
}

func resultDraft(kind ResultReferenceKind, executionID *Identifier, logical []byte, label string, rows, columns int, expiresAt time.Time) ResultReferenceDraft {
	payload := append([]byte(string(kind)+"\x00"), logical...)
	if executionID != nil {
		payload = append(payload, executionID[:]...)
	}
	return ResultReferenceDraft{Kind: kind, QueryExecutionID: executionID, LogicalRequest: logical,
		ContextFingerprint: sha256.Sum256(payload), Label: truncateRunes(label, 160), RowCount: rows, ColumnCount: columns, ExpiresAt: expiresAt}
}

func normalizeToolError(err error) error {
	if err == nil {
		return nil
	}
	var validation *querydomain.ValidationError
	if errors.As(err, &validation) {
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, searchdomain.ErrForbidden), errors.Is(err, querydomain.ErrForbidden):
		return ErrForbidden
	case errors.Is(err, ErrNotFound), errors.Is(err, querydomain.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, ErrRateLimited), errors.Is(err, searchdomain.ErrRateLimited), errors.Is(err, querydomain.ErrRateLimited):
		return ErrRateLimited
	case errors.Is(err, ErrTimeout), errors.Is(err, searchdomain.ErrQueryTimeout), errors.Is(err, querydomain.ErrTimeout):
		return ErrTimeout
	case errors.Is(err, ErrStaleContext), errors.Is(err, querydomain.ErrExpired), errors.Is(err, querydomain.ErrStaleCatalog):
		return ErrStaleContext
	case errors.Is(err, searchdomain.ErrCostLimit), errors.Is(err, searchdomain.ErrCardinalityLimit), errors.Is(err, querydomain.ErrCostLimit):
		return ErrQuotaExceeded
	case errors.Is(err, searchdomain.ErrInvalidQuery), errors.Is(err, querydomain.ErrInvalidPlan):
		// Keep the logical field/code detail so the orchestrator can hand it
		// back to the model as a correctable tool result.
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	case errors.Is(err, searchdomain.ErrUnsafeResult), errors.Is(err, querydomain.ErrUnsafeResult):
		return ErrUnsafeResult
	default:
		return ErrToolFailed
	}
}

// correctableToolPayload is the only tool failure the model may see and retry.
// Decode rejects (bare ErrInvalidInput, including unknown JSON fields) stay fatal.
// The payload is field paths and codes from the query catalog, never row values or source errors.
func correctableToolPayload(err error) (json.RawMessage, bool) {
	if err == nil || !errors.Is(err, ErrInvalidInput) {
		return nil, false
	}
	var validation *querydomain.ValidationError
	if errors.As(err, &validation) {
		return validationFeedback(validation), true
	}
	if errors.Is(err, querydomain.ErrInvalidPlan) || errors.Is(err, searchdomain.ErrInvalidQuery) {
		return json.RawMessage(`{"error":"invalid_input"}`), true
	}
	return nil, false
}

func validationFeedback(validation *querydomain.ValidationError) json.RawMessage {
	fields := make([]querydomain.FieldError, 0, len(validation.Fields))
	for _, field := range validation.Fields {
		if len(fields) == 20 || !safePlanToken(field.Field) || !safePlanToken(field.Code) {
			continue
		}
		fields = append(fields, field)
	}
	encoded, err := json.Marshal(struct {
		Error  string                   `json:"error"`
		Fields []querydomain.FieldError `json:"fields"`
	}{Error: "invalid_input", Fields: fields})
	if err != nil {
		return json.RawMessage(`{"error":"invalid_input"}`)
	}
	return encoded
}

func safePlanToken(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, runeValue := range value {
		if (runeValue < 'a' || runeValue > 'z') && (runeValue < '0' || runeValue > '9') && runeValue != '_' && runeValue != '.' {
			return false
		}
	}
	return true
}

func validProviderCallID(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) >= 1 && len(value) <= 128 && !strings.ContainsAny(value, "\r\n\x00")
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func truncateRunes(value string, maximum int) string {
	result, _ := truncateWithFlag(value, maximum)
	return result
}

func truncateWithFlag(value string, maximum int) (string, bool) {
	if utf8.RuneCountInString(value) <= maximum {
		return value, false
	}
	runes := []rune(value)
	return string(runes[:maximum]), true
}

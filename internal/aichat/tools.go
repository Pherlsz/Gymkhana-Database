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

type SearchPort interface {
	Catalog(context.Context, auth.Session) (searchdomain.Catalog, error)
	Search(context.Context, auth.Session, searchdomain.Query) (searchdomain.Page, error)
}

type QueryPort interface {
	Catalog(context.Context, auth.Session, string) (querydomain.Catalog, error)
	Execute(context.Context, auth.Session, querydomain.QueryPlan, string, string) (querydomain.Execution, error)
	Result(context.Context, auth.Session, querydomain.Identifier, int, int, string) (querydomain.ResultPage, error)
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

func (gateway *ToolGateway) Schemas() []ToolSchema {
	return []ToolSchema{
		{Name: "catalog", Description: "Lista apenas entidades, campos, operadores e relações lógicas autorizadas.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`)},
		{Name: "search", Description: "Busca termos literais nos campos lógicos autorizados. Não aceita SQL.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["terms"],"properties":{"terms":{"type":"array","minItems":1,"maxItems":5,"items":{"type":"string","maxLength":128}},"modules":{"type":"array","maxItems":5,"items":{"type":"string"}},"fields":{"type":"array","maxItems":40,"items":{"type":"string"}},"limit":{"type":"integer","minimum":1,"maximum":100},"offset":{"type":"integer","minimum":0,"maximum":10000},"context_reference_id":{"type":"string","format":"uuid"}}}`)},
		{Name: "query", Description: "Executa um QueryPlan v1 lógico, tipado e somente leitura.", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["plan"],"properties":{"plan":{"type":"object"},"context_reference_id":{"type":"string","format":"uuid"}}}`)},
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
	case "result":
		return gateway.result(ctx, actor, call.Arguments, requestID)
	default:
		return ToolOutput{}, ErrInvalidInput
	}
}

type searchToolRequest struct {
	Terms              []string              `json:"terms"`
	Modules            []searchdomain.Module `json:"modules,omitempty"`
	Fields             []string              `json:"fields,omitempty"`
	Limit              int32                 `json:"limit,omitempty"`
	Offset             int32                 `json:"offset,omitempty"`
	ContextReferenceID string                `json:"context_reference_id,omitempty"`
}

type storedSearchRequest struct {
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

type searchCatalogField struct {
	Key    string              `json:"key"`
	Module searchdomain.Module `json:"module"`
	Label  string              `json:"label"`
	Kind   string              `json:"kind"`
}

type searchCatalogModule struct {
	Key   searchdomain.Module `json:"key"`
	Label string              `json:"label"`
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

type queryRow struct {
	Position    int         `json:"position"`
	EntityKind  string      `json:"entity_kind"`
	EntityID    string      `json:"entity_id"`
	EntityLabel string      `json:"entity_label"`
	Cells       []queryCell `json:"cells"`
}

func (gateway *ToolGateway) catalog(ctx context.Context, actor auth.Session, raw json.RawMessage, requestID string) (ToolOutput, error) {
	var request struct{}
	if err := decodeToolArguments(raw, &request); err != nil {
		return ToolOutput{}, err
	}
	searchCatalog, err := gateway.search.Catalog(ctx, actor)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	queryCatalog, err := gateway.query.Catalog(ctx, actor, requestID)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	modules := make([]searchCatalogModule, 0, len(searchCatalog.Modules))
	for _, module := range searchCatalog.Modules {
		modules = append(modules, searchCatalogModule{Key: module.Key, Label: module.Label})
	}
	fields := make([]searchCatalogField, 0, len(searchCatalog.Fields))
	for _, field := range searchCatalog.Fields {
		fields = append(fields, searchCatalogField{Key: field.Key, Module: field.Module, Label: field.Label, Kind: field.Kind})
	}
	payload, err := marshalBoundedToolPayload(struct {
		Search struct {
			Modules []searchCatalogModule `json:"modules"`
			Fields  []searchCatalogField  `json:"fields"`
		} `json:"search"`
		Query querydomain.Catalog `json:"query"`
	}{Search: struct {
		Modules []searchCatalogModule `json:"modules"`
		Fields  []searchCatalogField  `json:"fields"`
	}{Modules: modules, Fields: fields}, Query: queryCatalog})
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
	query := searchdomain.Query{Terms: request.Terms, Modules: request.Modules, Fields: request.Fields, Limit: request.Limit, Offset: request.Offset,
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
		Results []searchResult `json:"results"`
		Total   int64          `json:"total"`
		Limit   int32          `json:"limit"`
		Offset  int32          `json:"offset"`
	}{Results: results, Total: page.Total, Limit: page.Limit, Offset: page.Offset})
	if err != nil {
		return ToolOutput{}, err
	}
	stored := storedSearchRequest{Terms: query.Terms, Modules: query.Modules, Fields: query.Fields, Limit: page.Limit, Offset: page.Offset, Sort: page.Sort, Order: page.Order}
	logical, err := json.Marshal(stored)
	if err != nil {
		return ToolOutput{}, fmt.Errorf("encode AI Chat Search reference: %w", err)
	}
	draft := resultDraft(ResultReferenceSearch, nil, logical, fmt.Sprintf("Busca: %s", strings.Join(query.Terms, " · ")), len(results), 1, expiresAt)
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
	execution, err := gateway.query.Execute(ctx, actor, request.Plan, fmt.Sprintf("chat.%s.%d", runID.String(), step), requestID)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	page, err := gateway.query.Result(ctx, actor, execution.ID, MaximumToolRows, 0, requestID)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	payload, rows, err := queryResultPayload(page)
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
	draft := resultDraft(ResultReferenceQuery, &executionID, logical, "Consulta: "+execution.RootEntity, len(rows), len(page.Columns), referenceExpiresAt)
	return ToolOutput{Kind: ToolQuery, Payload: payload, RowCount: len(rows), FieldCount: len(page.Columns), ByteCount: len(payload), Reference: &draft}, nil
}

func refineQueryPlan(current *querydomain.QueryPlan, previous querydomain.QueryPlan) error {
	if current == nil || previous.Version != querydomain.PlanVersionV1 || previous.RootEntity == "" {
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
	if current.Version != previous.Version || current.CatalogVersion != previous.CatalogVersion || current.RootEntity != previous.RootEntity {
		return ErrStaleContext
	}
	if len(current.Projections) == 0 {
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
			ReferenceID string         `json:"reference_id"`
			Results     []searchResult `json:"results"`
			Total       int64          `json:"total"`
		}{ReferenceID: reference.ID.String(), Results: results, Total: page.Total})
		if err != nil {
			return ToolOutput{}, err
		}
		return ToolOutput{Kind: ToolResult, Payload: payload, RowCount: len(results), FieldCount: 1, ByteCount: len(payload)}, nil
	case ResultReferenceQuery:
		if reference.QueryExecutionID == nil {
			return ToolOutput{}, ErrStaleContext
		}
		page, err := gateway.query.Result(ctx, actor, querydomain.Identifier(*reference.QueryExecutionID), request.Limit, request.Offset, requestID)
		if err != nil {
			return ToolOutput{}, normalizeToolError(err)
		}
		payload, rows, err := queryResultPayload(page)
		if err != nil {
			return ToolOutput{}, err
		}
		return ToolOutput{Kind: ToolResult, Payload: payload, RowCount: len(rows), FieldCount: len(page.Columns), ByteCount: len(payload)}, nil
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

func queryResultPayload(page querydomain.ResultPage) ([]byte, []queryRow, error) {
	rows := make([]queryRow, 0, len(page.Rows))
	for _, value := range page.Rows {
		converted := queryRow{Position: value.Position, EntityKind: value.EntityKind, EntityID: value.EntityID,
			EntityLabel: truncateRunes(value.EntityLabel, maximumToolCellRunes), Cells: make([]queryCell, 0, len(value.Cells))}
		for _, cell := range value.Cells {
			converted.Cells = append(converted.Cells, safeQueryCell(cell))
		}
		rows = append(rows, converted)
	}
	payload, err := marshalBoundedToolPayload(struct {
		ExecutionID string                     `json:"execution_id"`
		Columns     []querydomain.ResultColumn `json:"columns"`
		Rows        []queryRow                 `json:"rows"`
		Total       int                        `json:"total"`
	}{ExecutionID: page.Execution.ID.String(), Columns: page.Columns, Rows: rows, Total: page.Total})
	return payload, rows, err
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
	switch {
	case err == nil:
		return nil
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
		return ErrInvalidInput
	case errors.Is(err, searchdomain.ErrUnsafeResult), errors.Is(err, querydomain.ErrUnsafeResult):
		return ErrUnsafeResult
	default:
		return ErrToolFailed
	}
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

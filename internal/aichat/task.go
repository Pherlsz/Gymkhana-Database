package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

const maximumTaskStages = 8

type taskRequest struct {
	Stages []taskStage `json:"stages"`
}

type taskStage struct {
	Title           string                         `json:"titulo"`
	Kind            string                         `json:"tipo"`
	RootEntity      string                         `json:"root_entity,omitempty"`
	Projections     []string                       `json:"projections,omitempty"`
	Filter          *querydomain.FilterNode        `json:"filter,omitempty"`
	LetterField     string                         `json:"letter_field,omitempty"`
	NumberField     string                         `json:"number_field,omitempty"`
	NumberExtractor string                         `json:"number_extractor,omitempty"`
	CityField       string                         `json:"city_field,omitempty"`
	MinimumLength   int                            `json:"minimum_length,omitempty"`
	Patterns        []querydomain.PatternPredicate `json:"patterns,omitempty"`
	GroupBy         []string                       `json:"group_by,omitempty"`
	Aggregates      []querydomain.Aggregate        `json:"aggregates,omitempty"`
	Set             *querydomain.SetExpression     `json:"set,omitempty"`
	Note            string                         `json:"nota,omitempty"`
}

func (gateway *ToolGateway) taskData(ctx context.Context, actor auth.Session, raw json.RawMessage, requestID string) (ToolOutput, error) {
	var request taskRequest
	if err := decodeToolArguments(raw, &request); err != nil {
		return ToolOutput{}, err
	}
	if len(request.Stages) < 1 || len(request.Stages) > maximumTaskStages {
		return ToolOutput{}, taskFieldError("stages", "out_of_range")
	}
	catalog, err := gateway.query.Catalog(ctx, actor, requestID)
	if err != nil {
		return ToolOutput{}, normalizeToolError(err)
	}
	var summary strings.Builder
	evidence := 0
	for index, stage := range request.Stages {
		if index > 0 {
			summary.WriteString("\n\n")
		}
		block, rows, stageErr := gateway.taskStage(ctx, actor, index, stage, catalog.Version, requestID)
		if stageErr != nil {
			return ToolOutput{}, stageErr
		}
		summary.WriteString(block)
		evidence += rows
	}
	if evidence > MaximumToolRows {
		evidence = MaximumToolRows
	}
	payload, err := marshalBoundedToolPayload(struct {
		Note    string `json:"note"`
		Summary string `json:"resumo"`
	}{
		Note:    "A resposta é o resumo, um bloco por etapa. Não acrescente pessoas, documentos ou contas que não estejam nele.",
		Summary: summary.String(),
	})
	if err != nil {
		return ToolOutput{}, err
	}
	return ToolOutput{Kind: ToolQuery, Payload: payload, RowCount: evidence, FieldCount: 4, ByteCount: len(payload)}, nil
}

func (gateway *ToolGateway) taskStage(ctx context.Context, actor auth.Session, index int, stage taskStage, catalogVersion, requestID string) (string, int, error) {
	stage.Title = clipRunes(strings.TrimSpace(stage.Title), 120)
	stage.Kind = strings.TrimSpace(stage.Kind)
	stage.RootEntity = strings.TrimSpace(stage.RootEntity)
	path := fmt.Sprintf("stages.%d", index)
	if stage.Title == "" {
		return "", 0, taskFieldError(path+".titulo", "required")
	}
	if stage.Kind != "fora_da_base" && stage.RootEntity == "" {
		return "", 0, taskFieldError(path+".root_entity", "required")
	}
	switch stage.Kind {
	case "fora_da_base":
		note := clipRunes(strings.TrimSpace(stage.Note), 400)
		if note == "" {
			return "", 0, taskFieldError(path+".nota", "required")
		}
		return stageHeading(stage.Title, "Fora da base", "O cadastro não registra esta regra: "+note, nil), 0, nil
	case "cadeia":
		return gateway.chainStage(ctx, actor, stage, catalogVersion, requestID)
	case "consulta", "cruzada":
		if stage.Kind == "cruzada" && !filterHasRelation(stage.Filter) {
			return "", 0, taskFieldError(path+".filter", "required")
		}
		if len(stage.GroupBy) > 0 || len(stage.Aggregates) > 0 {
			return gateway.advancedStage(ctx, actor, stage.Title, querydomain.QueryPlan{
				RootEntity: stage.RootEntity, Filter: stage.Filter, GroupBy: stage.GroupBy, Aggregates: stage.Aggregates,
			}, requestID)
		}
		if len(stage.Projections) == 0 {
			return "", 0, taskFieldError(path+".projections", "required")
		}
		return gateway.countStage(ctx, actor, stage, catalogVersion, requestID)
	case "padrao":
		if len(stage.Patterns) == 0 {
			return "", 0, taskFieldError(path+".patterns", "required")
		}
		projections := stage.Projections
		if len(projections) == 0 {
			for _, pattern := range stage.Patterns {
				projections = append(projections, pattern.Field)
			}
		}
		return gateway.advancedStage(ctx, actor, stage.Title, querydomain.QueryPlan{
			RootEntity: stage.RootEntity, Filter: stage.Filter, Projections: projections, Patterns: stage.Patterns,
		}, requestID)
	case "conjunto":
		if stage.Set == nil {
			return "", 0, taskFieldError(path+".set", "required")
		}
		return gateway.advancedStage(ctx, actor, stage.Title, querydomain.QueryPlan{RootEntity: stage.RootEntity, Set: stage.Set}, requestID)
	default:
		return "", 0, taskFieldError(path+".tipo", "unsupported")
	}
}

func (gateway *ToolGateway) countStage(ctx context.Context, actor auth.Session, stage taskStage, catalogVersion, requestID string) (string, int, error) {
	plan := querydomain.QueryPlan{
		Version: querydomain.PlanVersionV1, CatalogVersion: catalogVersion, RootEntity: stage.RootEntity,
		Projections: stage.Projections, Filter: stage.Filter, MaximumRows: 1,
	}
	count, err := gateway.query.CountMatches(ctx, actor, plan, requestID)
	if err != nil {
		return "", 0, normalizeToolError(err)
	}
	if count == 0 {
		return stageHeading(stage.Title, "Não atende", "Há 0 registros no cadastro inteiro.", nil), 0, nil
	}
	scan, err := gateway.query.ScanFields(ctx, actor, plan, maximumModelSampleRows, requestID)
	if err != nil {
		return "", 0, normalizeToolError(err)
	}
	lines := scanLines(scan.Rows)
	return stageHeading(stage.Title, "Atende", fmt.Sprintf("Há %d registros no cadastro inteiro.", count), lines), len(lines), nil
}

func (gateway *ToolGateway) chainStage(ctx context.Context, actor auth.Session, stage taskStage, catalogVersion, requestID string) (string, int, error) {
	raw, err := json.Marshal(sequenceToolRequest{
		CatalogVersion: catalogVersion, RootEntity: stage.RootEntity, LetterField: stage.LetterField,
		NumberField: stage.NumberField, NumberExtractor: stage.NumberExtractor, CityField: stage.CityField,
		MinimumLength: stage.MinimumLength, Filter: stage.Filter,
	})
	if err != nil {
		return "", 0, fmt.Errorf("encode task chain: %w", err)
	}
	output, err := gateway.sequenceData(ctx, actor, raw, requestID)
	if err != nil {
		return "", 0, err
	}
	var parsed struct {
		Summary      string        `json:"resumo"`
		Length       int           `json:"length"`
		MeetsMinimum bool          `json:"meets_minimum"`
		ScanComplete bool          `json:"scan_complete"`
		Rows         []sequenceRow `json:"rows"`
	}
	if err := json.Unmarshal(output.Payload, &parsed); err != nil {
		return "", 0, fmt.Errorf("read task chain: %w", err)
	}
	verdict := "Não atende"
	if parsed.MeetsMinimum && parsed.ScanComplete {
		verdict = "Atende"
	} else if parsed.Length > 0 {
		verdict = "Parcial"
	}
	return stageHeading(stage.Title, verdict, parsed.Summary, chainLines(parsed.Rows)), len(parsed.Rows), nil
}

func (gateway *ToolGateway) advancedStage(ctx context.Context, actor auth.Session, title string, plan querydomain.QueryPlan, requestID string) (string, int, error) {
	result, err := gateway.query.RunAdvanced(ctx, actor, plan, requestID)
	if err != nil {
		return "", 0, normalizeToolError(err)
	}
	lines := advancedLines(result.Rows)
	verdict := "Não atende"
	if len(result.Rows) > 0 && !aggregateIsZero(result.Rows) {
		verdict = "Atende"
	}
	return stageHeading(title, verdict, fmt.Sprintf("%d linhas no cadastro inteiro.", len(result.Rows)), lines), len(lines), nil
}

func stageHeading(title, verdict, body string, lines []string) string {
	var builder strings.Builder
	builder.WriteString("## ")
	builder.WriteString(title)
	builder.WriteString("\n")
	builder.WriteString(verdict)
	builder.WriteString(". ")
	builder.WriteString(body)
	for _, line := range lines {
		builder.WriteString("\n- ")
		builder.WriteString(line)
	}
	return builder.String()
}

func chainLines(rows []sequenceRow) []string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		line := row.Letter + " " + row.Holder + ", número " + strconv.Itoa(row.Number)
		if row.City != "" {
			line += ", " + row.City
		}
		lines = append(lines, line)
	}
	return lines
}

func scanLines(rows []querydomain.TextScanRow) []string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		line := row.Label
		values := strings.Join(row.Values, ", ")
		if values != "" && values != line {
			if line != "" {
				line += " — "
			}
			line += values
		}
		if line != "" {
			lines = append(lines, clipRunes(line, 180))
		}
	}
	return lines
}

func advancedLines(rows []querydomain.ResultRow) []string {
	lines := make([]string, 0, len(rows))
	limit := maximumModelSampleRows
	if len(rows) < limit {
		limit = len(rows)
	}
	for _, row := range rows[:limit] {
		parts := make([]string, 0, len(row.Cells)+1)
		if row.EntityLabel != "" {
			parts = append(parts, row.EntityLabel)
		}
		for _, cell := range row.Cells {
			if text := cellPlain(cell); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			lines = append(lines, clipRunes(strings.Join(parts, ", "), 180))
		}
	}
	return lines
}

func cellPlain(cell querydomain.ResultCell) string {
	switch {
	case cell.TextValue != nil:
		return *cell.TextValue
	case cell.IntegerValue != nil:
		return strconv.FormatInt(*cell.IntegerValue, 10)
	case cell.DecimalValue != nil:
		return *cell.DecimalValue
	case cell.BooleanValue != nil:
		if *cell.BooleanValue {
			return "sim"
		}
		return "não"
	case cell.CivilDateValue != nil:
		return *cell.CivilDateValue
	default:
		return ""
	}
}

func aggregateIsZero(rows []querydomain.ResultRow) bool {
	if len(rows) != 1 {
		return false
	}
	sawNumber := false
	for _, cell := range rows[0].Cells {
		if cell.IntegerValue != nil {
			sawNumber = true
			if *cell.IntegerValue != 0 {
				return false
			}
		}
		if cell.TextValue != nil && *cell.TextValue != "" && *cell.TextValue != "0" {
			return false
		}
	}
	return sawNumber
}

func filterHasRelation(node *querydomain.FilterNode) bool {
	if node == nil {
		return false
	}
	if node.Kind == querydomain.FilterRelation && strings.TrimSpace(node.Relation) != "" {
		return true
	}
	for index := range node.Children {
		if filterHasRelation(&node.Children[index]) {
			return true
		}
	}
	return false
}

func taskFieldError(field, code string) error {
	return normalizeToolError(&querydomain.ValidationError{Fields: []querydomain.FieldError{{Field: field, Code: code}}})
}

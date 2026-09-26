package aichat

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

// ResultGrid is one page of an assistant result, ready for the open sheet.
type ResultGrid struct {
	Columns   []querydomain.ShapeColumn  `json:"columns"`
	Rows      []querydomain.ShapeGridRow `json:"rows"`
	Total     int                        `json:"total"`
	Limit     int                        `json:"limit"`
	Offset    int                        `json:"offset"`
	Summary   string                     `json:"summary,omitempty"`
	Truncated bool                       `json:"truncated,omitempty"`
}

func (grid ResultGrid) page() querydomain.ShapePage {
	return querydomain.ShapePage{
		Columns: grid.Columns, Rows: grid.Rows, Total: grid.Total,
		Summary: grid.Summary, Truncated: grid.Truncated,
	}
}

func referencePlanNeedsShape(logical []byte) bool {
	var stored storedQueryRequest
	if json.Unmarshal(logical, &stored) != nil {
		return false
	}
	return querydomain.PlanNeedsShape(stored.Plan)
}

func (gateway *ToolGateway) ReadPage(ctx context.Context, actor auth.Session, referenceID Identifier, limit, offset int, requestID string) (ResultGrid, error) {
	if gateway == nil || referenceID.IsZero() || limit < 1 || limit > MaximumResultPage || offset < 0 || offset > 10_000 {
		return ResultGrid{}, ErrInvalidInput
	}
	reference, err := gateway.refs.ResultReference(ctx, actor, referenceID, requestID)
	if err != nil {
		return ResultGrid{}, normalizeToolError(err)
	}
	if reference.Kind != ResultReferenceQuery {
		return ResultGrid{}, ErrInvalidInput
	}
	var stored storedQueryRequest
	if err := json.Unmarshal(reference.LogicalRequest, &stored); err != nil {
		return ResultGrid{}, ErrStaleContext
	}
	if planMaterializesSQL(stored.Plan) {
		return gateway.readSQLGrid(ctx, actor, reference, stored.Plan, limit, offset, requestID)
	}
	return gateway.readShapeGrid(ctx, actor, stored.Plan, limit, offset, requestID)
}

func (gateway *ToolGateway) queryShape(ctx context.Context, actor auth.Session, plan querydomain.QueryPlan, expiresAt time.Time, requestID string) (ToolOutput, error) {
	grid, err := gateway.readShapeGrid(ctx, actor, plan, maximumModelSampleRows, 0, requestID)
	if err != nil {
		return ToolOutput{}, err
	}
	payload, err := shapeToolPayload(grid.page())
	if err != nil {
		return ToolOutput{}, err
	}
	fields := len(grid.Columns)
	if fields > MaximumToolFields {
		fields = MaximumToolFields
	}
	output := ToolOutput{Kind: ToolQuery, Payload: payload, RowCount: len(grid.Rows), FieldCount: fields, ByteCount: len(payload)}
	if grid.Total == 0 {
		return output, nil
	}
	logical, err := json.Marshal(storedQueryRequest{Plan: plan})
	if err != nil {
		return ToolOutput{}, err
	}
	draft := resultDraft(ResultReferenceQuery, nil, logical, "Consulta: "+plan.RootEntity, len(grid.Rows), fields, expiresAt)
	output.Reference = &draft
	return output, nil
}

func (gateway *ToolGateway) readShapeGrid(ctx context.Context, actor auth.Session, plan querydomain.QueryPlan, limit, offset int, requestID string) (ResultGrid, error) {
	rows, truncated, err := gateway.query.ScanShape(ctx, actor, plan, querydomain.MaximumSequenceScan, requestID)
	if err != nil {
		return ResultGrid{}, normalizeToolError(err)
	}
	shaped := querydomain.ApplyShape(rows, plan, truncated).Page(limit, offset)
	gateway.labelColumns(ctx, actor, &shaped, requestID)
	return ResultGrid{
		Columns: shaped.Columns, Rows: shaped.Rows, Total: shaped.Total,
		Limit: limit, Offset: offset, Summary: shaped.Summary, Truncated: shaped.Truncated,
	}, nil
}

func (gateway *ToolGateway) readSQLGrid(ctx context.Context, actor auth.Session, reference ResultReference, plan querydomain.QueryPlan, limit, offset int, requestID string) (ResultGrid, error) {
	if reference.QueryExecutionID == nil {
		return ResultGrid{}, ErrStaleContext
	}
	pageLimit := limit
	if pageLimit > querydomain.MaximumPageSize {
		pageLimit = querydomain.MaximumPageSize
	}
	executionID := querydomain.Identifier(*reference.QueryExecutionID)
	var page querydomain.ResultPage
	var err error
	if planIsAdvanced(plan) {
		page, err = gateway.query.ResultV2(ctx, actor, executionID, pageLimit, offset, requestID)
	} else {
		page, err = gateway.query.Result(ctx, actor, executionID, pageLimit, offset, requestID)
	}
	if err != nil {
		return ResultGrid{}, normalizeToolError(err)
	}
	return gridFromResult(page, limit, offset), nil
}

func (gateway *ToolGateway) labelColumns(ctx context.Context, actor auth.Session, page *querydomain.ShapePage, requestID string) {
	catalog, err := gateway.query.Catalog(ctx, actor, requestID)
	if err != nil {
		return
	}
	labels := map[string]string{}
	for _, field := range catalog.Fields {
		if field.Label != "" {
			labels[field.Key] = field.Label
		}
	}
	for index := range page.Columns {
		if label, ok := labels[page.Columns[index].Key]; ok {
			page.Columns[index].Label = label
		}
	}
}

func gridFromResult(page querydomain.ResultPage, limit, offset int) ResultGrid {
	columns := make([]querydomain.ShapeColumn, 0, len(page.Columns))
	for _, column := range page.Columns {
		key := column.FieldKey
		if key == "" {
			key = column.AggregateKey
		}
		if key == "" {
			key = column.Label
		}
		columns = append(columns, querydomain.ShapeColumn{Key: key, Label: column.Label})
	}
	rows := make([]querydomain.ShapeGridRow, 0, len(page.Rows))
	for _, row := range page.Rows {
		cells := map[string]string{}
		for index, cell := range row.Cells {
			if index < len(columns) {
				cells[columns[index].Key] = displayCell(cell)
			}
		}
		rows = append(rows, querydomain.ShapeGridRow{
			ID: row.EntityID, EntityKind: row.EntityKind, EntityID: row.EntityID, Label: row.EntityLabel, Cells: cells,
		})
	}
	return ResultGrid{Columns: columns, Rows: rows, Total: page.Total, Limit: limit, Offset: offset}
}

func displayCell(cell querydomain.ResultCell) string {
	if cell.IsNull {
		return ""
	}
	switch {
	case cell.TextValue != nil:
		return *cell.TextValue
	case cell.IntegerValue != nil:
		return strconv.FormatInt(*cell.IntegerValue, 10)
	case cell.DecimalValue != nil:
		return *cell.DecimalValue
	case cell.BooleanValue != nil:
		return strconv.FormatBool(*cell.BooleanValue)
	case cell.CivilDateValue != nil:
		return *cell.CivilDateValue
	case cell.TimestampValue != nil:
		return cell.TimestampValue.UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

func shapeToolPayload(page querydomain.ShapePage) ([]byte, error) {
	source := page.Rows
	if len(source) > maximumModelSampleRows {
		source = source[:maximumModelSampleRows]
	}
	rows := make([]struct {
		Title  string            `json:"titulo"`
		Values map[string]string `json:"vals"`
	}, 0, len(source))
	for _, row := range source {
		vals := make(map[string]string, len(page.Columns))
		for _, column := range page.Columns {
			label := column.Label
			if label == "" {
				label = column.Key
			}
			vals[label] = truncateRunes(row.Cells[column.Key], maximumToolCellRunes)
		}
		rows = append(rows, struct {
			Title  string            `json:"titulo"`
			Values map[string]string `json:"vals"`
		}{Title: truncateRunes(row.Label, maximumToolCellRunes), Values: vals})
	}
	return marshalBoundedToolPayload(struct {
		Note       string `json:"note"`
		MatchCount int    `json:"match_count"`
		Summary    string `json:"summary,omitempty"`
		Truncated  bool   `json:"truncated,omitempty"`
		Rows       any    `json:"rows"`
	}{
		Note:       "match_count é o total. rows é amostra. Se summary falar em corte, diga o total. Se truncated, o resultado pode estar incompleto.",
		MatchCount: page.Total,
		Summary:    page.Summary,
		Truncated:  page.Truncated,
		Rows:       rows,
	})
}

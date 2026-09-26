package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

const (
	recorteSheetName          = "Recorte"
	recorteFallbackColumn     = "recorte"
	maximumExportCellBytes    = 32_768
	recorteFilenameTimeLayout = "20060102-150405"
)

// RecorteWorkbook is the on-grid assistant recorte as spreadsheet values.
// It never includes the rest of the table.
type RecorteWorkbook struct {
	Filename string
	Sheet    string
	Headers  []string
	Rows     [][]string
}

func (gateway *ToolGateway) ExportRecorte(ctx context.Context, actor auth.Session, referenceID Identifier, requestID string) (RecorteWorkbook, error) {
	grid, err := gateway.readFullGrid(ctx, actor, referenceID, requestID)
	if err != nil {
		return RecorteWorkbook{}, err
	}
	now := time.Now().UTC()
	if gateway != nil && gateway.now != nil {
		now = gateway.now().UTC()
	}
	return recorteWorkbook(grid, now), nil
}

func (gateway *ToolGateway) readFullGrid(ctx context.Context, actor auth.Session, referenceID Identifier, requestID string) (ResultGrid, error) {
	if gateway == nil || referenceID.IsZero() {
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
		return gateway.readSQLGridAll(ctx, actor, reference, stored.Plan, requestID)
	}
	return gateway.readShapeGrid(ctx, actor, stored.Plan, querydomain.MaximumSequenceScan, 0, requestID)
}

func (gateway *ToolGateway) readSQLGridAll(ctx context.Context, actor auth.Session, reference ResultReference, plan querydomain.QueryPlan, requestID string) (ResultGrid, error) {
	offset := 0
	var combined ResultGrid
	for {
		page, err := gateway.readSQLGrid(ctx, actor, reference, plan, querydomain.MaximumPageSize, offset, requestID)
		if err != nil {
			return ResultGrid{}, err
		}
		if combined.Columns == nil {
			combined.Columns = page.Columns
			combined.Summary = page.Summary
			combined.Truncated = page.Truncated
		}
		combined.Total = page.Total
		combined.Rows = append(combined.Rows, page.Rows...)
		if len(page.Rows) == 0 {
			break
		}
		offset += len(page.Rows)
		if offset >= page.Total || offset >= querydomain.MaximumRows || len(page.Rows) < page.Limit {
			break
		}
	}
	if combined.Columns == nil {
		combined.Columns = []querydomain.ShapeColumn{}
	}
	combined.Limit = len(combined.Rows)
	combined.Offset = 0
	return combined, nil
}

func recorteWorkbook(grid ResultGrid, now time.Time) RecorteWorkbook {
	headers, keys := recorteHeaders(grid.Columns)
	rows := make([][]string, 0, len(grid.Rows))
	for _, row := range grid.Rows {
		line := make([]string, len(keys))
		for index, key := range keys {
			if key == "" {
				line[index] = clipExportCell(row.Label)
				continue
			}
			line[index] = clipExportCell(row.Cells[key])
		}
		rows = append(rows, line)
	}
	return RecorteWorkbook{
		Filename: recorteFilename(now),
		Sheet:    recorteSheetName,
		Headers:  headers,
		Rows:     rows,
	}
}

func recorteHeaders(columns []querydomain.ShapeColumn) ([]string, []string) {
	if len(columns) == 0 {
		return []string{recorteFallbackColumn}, []string{""}
	}
	headers := make([]string, 0, len(columns))
	keys := make([]string, 0, len(columns))
	seen := map[string]int{}
	for _, column := range columns {
		label := strings.TrimSpace(column.Label)
		if label == "" {
			label = strings.TrimSpace(column.Key)
		}
		if label == "" {
			label = recorteFallbackColumn
		}
		headers = append(headers, uniqueExportHeader(label, seen))
		keys = append(keys, column.Key)
	}
	return headers, keys
}

func uniqueExportHeader(label string, seen map[string]int) string {
	base := clipExportCell(label)
	key := strings.ToLower(strings.TrimSpace(base))
	count := seen[key]
	seen[key] = count + 1
	if count == 0 {
		return base
	}
	return clipExportCell(fmt.Sprintf("%s (%d)", base, count+1))
}

func recorteFilename(now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return "recorte-" + now.UTC().Format(recorteFilenameTimeLayout) + ".xlsx"
}

func clipExportCell(value string) string {
	if len(value) <= maximumExportCellBytes {
		return value
	}
	value = value[:maximumExportCellBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

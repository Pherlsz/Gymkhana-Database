package aichat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

func TestRecorteWorkbookExportsEveryRowAndUniqueHeaders(t *testing.T) {
	now := time.Date(2026, time.September, 25, 3, 15, 4, 0, time.UTC)
	got := recorteWorkbook(ResultGrid{
		Columns: []querydomain.ShapeColumn{
			{Key: "full_name", Label: "Nome"},
			{Key: "also_name", Label: "Nome"},
			{Key: "note", Label: ""},
		},
		Rows: []querydomain.ShapeGridRow{
			{Label: "Ana", Cells: map[string]string{"full_name": "Ana", "also_name": "A.", "note": "=2+2"}},
			{Label: "Bia", Cells: map[string]string{"full_name": "Bia", "also_name": "B.", "note": "+cmd"}},
		},
	}, now)
	if got.Filename != "recorte-20260925-031504.xlsx" || got.Sheet != "Recorte" {
		t.Fatalf("workbook identity = %#v", got)
	}
	if len(got.Headers) != 3 || got.Headers[0] != "Nome" || got.Headers[1] != "Nome (2)" || got.Headers[2] != "note" {
		t.Fatalf("headers = %#v", got.Headers)
	}
	if len(got.Rows) != 2 || got.Rows[0][0] != "Ana" || got.Rows[1][0] != "Bia" || got.Rows[0][2] != "=2+2" {
		t.Fatalf("rows = %#v", got.Rows)
	}
}

func TestRecorteWorkbookEmptyColumnsStillHasAHeader(t *testing.T) {
	got := recorteWorkbook(ResultGrid{Rows: []querydomain.ShapeGridRow{{Label: "Ana"}}}, time.Now())
	if len(got.Headers) != 1 || got.Headers[0] != "recorte" || len(got.Rows) != 1 || got.Rows[0][0] != "Ana" {
		t.Fatalf("empty columns workbook = %#v", got)
	}
}

func TestExportRecorteIncludesEveryShapeRow(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	now := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)
	query := &fakeToolQuery{shape: []querydomain.ShapeRow{
		{EntityID: "p1", EntityKind: "profile", Label: "Ana", Fields: map[string]string{"profile.full_name": "Ana"}},
		{EntityID: "p2", EntityKind: "profile", Label: "Bia", Fields: map[string]string{"profile.full_name": "Bia"}},
	}}
	id := Identifier{41}
	logical := []byte(`{"plan":{"version":"v1","catalog_version":"` + strings.Repeat("a", 64) + `","root_entity":"profiles","projections":["profile.full_name"],"maximum_rows":10}}`)
	references := &fakeToolReferences{values: map[Identifier]ResultReference{
		id: {ID: id, Kind: ResultReferenceQuery, LogicalRequest: logical, ExpiresAt: now.Add(time.Hour)},
	}}
	gateway, err := NewToolGateway(&fakeToolSearch{}, query, references, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewToolGateway() error = %v", err)
	}
	page, err := gateway.ReadPage(context.Background(), actor, id, 1, 0, "request")
	if err != nil || page.Total != 2 || len(page.Rows) != 1 {
		t.Fatalf("paged recorte = %#v %v", page, err)
	}
	workbook, err := gateway.ExportRecorte(context.Background(), actor, id, "request")
	if err != nil || len(workbook.Rows) != 2 || workbook.Rows[0][0] != "Ana" || workbook.Rows[1][0] != "Bia" {
		t.Fatalf("export recorte = %#v %v", workbook, err)
	}
	if query.limit != querydomain.MaximumSequenceScan {
		t.Fatalf("export scan limit = %d", query.limit)
	}
}

func TestExportRecorteRejectsSearchReference(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	id := Identifier{42}
	gateway := testToolGateway(t, &fakeToolSearch{}, &fakeToolQuery{}, &fakeToolReferences{values: map[Identifier]ResultReference{
		id: {ID: id, Kind: ResultReferenceSearch, LogicalRequest: []byte(`{}`)},
	}})
	if _, err := gateway.ExportRecorte(context.Background(), actor, id, "request"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("search recorte export = %v", err)
	}
}

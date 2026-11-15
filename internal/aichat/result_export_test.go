package aichat

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
	searchdomain "github.com/Pherlsz/Gymkhana-Database/internal/search"
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

func searchReferenceGateway(t *testing.T, search *fakeToolSearch, logical string) (*ToolGateway, Identifier) {
	t.Helper()
	id := Identifier{42}
	return testToolGateway(t, search, &fakeToolQuery{}, &fakeToolReferences{values: map[Identifier]ResultReference{
		id: {ID: id, Kind: ResultReferenceSearch, LogicalRequest: []byte(logical)},
	}}), id
}

func TestReadPageLaysOutSearchReferenceHits(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	search := &fakeToolSearch{page: searchdomain.Page{Total: 3, Results: []searchdomain.Result{
		{Module: searchdomain.ModuleProfiles, EntityKind: "profile", EntityID: "p1", ProfileID: "p1", TargetKind: "profile", TargetID: "p1",
			EntityLabel: "Ana Gravataí", FieldKey: "profile.birth_city", FieldLabel: "Naturalidade", Preview: "Gravataí"},
		{Module: searchdomain.ModuleProfiles, EntityKind: "profile", EntityID: "p1", ProfileID: "p1", TargetKind: "profile", TargetID: "p1",
			EntityLabel: "Ana Gravataí", FieldKey: "profile.birth_city", FieldLabel: "Naturalidade", Preview: "Gravataí"},
		{Module: searchdomain.ModuleDocuments, EntityKind: "document", EntityID: "d1", ProfileID: "p2", TargetKind: "document", TargetID: "d1",
			EntityLabel: "RG 123", FieldKey: "document.notes", FieldLabel: "Observações", Preview: "emitido em Gravataí"},
	}}}
	gateway, id := searchReferenceGateway(t, search, `{"q":"Gravataí","terms":["Gravataí"],"limit":100,"offset":0,"sort":"relevance","order":"desc"}`)
	page, err := gateway.ReadPage(context.Background(), actor, id, 500, 0, "request")
	if err != nil {
		t.Fatalf("ReadPage(search) error = %v", err)
	}
	if search.query.Q != "Gravataí" || len(search.query.Terms) != 1 || search.query.Limit != searchdomain.MaxPageSize || search.query.Offset != 0 {
		t.Fatalf("rerun search query = %#v", search.query)
	}
	if page.Total != 3 || len(page.Columns) != 3 || page.Columns[0].Label != "Nome" || len(page.Rows) != 3 {
		t.Fatalf("search grid = %#v", page)
	}
	if page.Rows[0].ID == page.Rows[1].ID {
		t.Fatalf("duplicate hits share row id %q", page.Rows[0].ID)
	}
	person, document := page.Rows[0], page.Rows[2]
	if person.EntityKind != "profile" || person.EntityID != "p1" || person.Cells["nome"] != "Ana Gravataí" || person.Cells["onde"] != "Naturalidade" {
		t.Fatalf("person row = %#v", person)
	}
	if document.EntityKind != "document" || document.EntityID != "d1" || document.Cells["trecho"] != "emitido em Gravataí" {
		t.Fatalf("document row = %#v", document)
	}
}

func TestReadPageEmptySearchReferenceKeepsColumns(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	gateway, id := searchReferenceGateway(t, &fakeToolSearch{}, `{"terms":["ninguém"]}`)
	page, err := gateway.ReadPage(context.Background(), actor, id, 500, 0, "request")
	if err != nil || page.Total != 0 || len(page.Rows) != 0 || page.Rows == nil || len(page.Columns) != 3 {
		t.Fatalf("empty search grid = %#v %v", page, err)
	}
}

func TestReadPageSearchReferenceWithBrokenLogicalIsStale(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	gateway, id := searchReferenceGateway(t, &fakeToolSearch{}, `not json`)
	if _, err := gateway.ReadPage(context.Background(), actor, id, 10, 0, "request"); !errors.Is(err, ErrStaleContext) {
		t.Fatalf("broken search reference = %v", err)
	}
}

func TestExportRecorteIncludesSearchReferenceHits(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	search := &fakeToolSearch{page: searchdomain.Page{Total: 1, Results: []searchdomain.Result{
		{TargetKind: "bill", TargetID: "b1", EntityLabel: "Conta 9", FieldLabel: "Referência", Preview: "9"},
	}}}
	gateway, id := searchReferenceGateway(t, search, `{"terms":["9"]}`)
	workbook, err := gateway.ExportRecorte(context.Background(), actor, id, "request")
	if err != nil || len(workbook.Headers) != 3 || len(workbook.Rows) != 1 || workbook.Rows[0][0] != "Conta 9" || workbook.Rows[0][1] != "Referência" {
		t.Fatalf("search export = %#v %v", workbook, err)
	}
}

func TestReadPagePagesTheAssistantExecutionForAPlainPlan(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	now := time.Date(2026, time.November, 15, 3, 0, 0, 0, time.UTC)
	name := "Ana"
	query := &fakeToolQuery{
		page: querydomain.ResultPage{
			Columns: []querydomain.ResultColumn{{Position: 0, FieldKey: "profile.full_name", Label: "Nome", Kind: querydomain.ValueText}},
			Rows: []querydomain.ResultRow{{Position: 0, EntityKind: "profile", EntityID: "p1", EntityLabel: "Ana",
				Cells: []querydomain.ResultCell{{ColumnPosition: 0, Kind: querydomain.ValueText, TextValue: &name}}}},
			Total: 1, Limit: 500,
		},
		shape:     []querydomain.ShapeRow{{EntityID: "a1", EntityKind: "profile", Label: "Adao", Fields: map[string]string{"profile.full_name": "Adao"}}},
		truncated: true,
	}
	executionID := Identifier{50}
	id := Identifier{51}
	logical := []byte(`{"plan":{"version":"v1","catalog_version":"` + strings.Repeat("a", 64) + `","root_entity":"profiles","projections":["profile.full_name"],` +
		`"filter":{"kind":"predicate","field":"profile.cpf","operator":"contains","values":["73"]},"maximum_rows":10}}`)
	references := &fakeToolReferences{values: map[Identifier]ResultReference{
		id: {ID: id, Kind: ResultReferenceQuery, QueryExecutionID: &executionID, LogicalRequest: logical, ExpiresAt: now.Add(time.Hour)},
	}}
	gateway, err := NewToolGateway(&fakeToolSearch{}, query, references, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewToolGateway() error = %v", err)
	}
	page, err := gateway.ReadPage(context.Background(), actor, id, 500, 0, "request")
	if err != nil || page.Total != 1 || len(page.Rows) != 1 || page.Rows[0].Cells["profile.full_name"] != "Ana" || page.Truncated {
		t.Fatalf("plain plan page = %#v %v", page, err)
	}
	if query.limit != querydomain.MaximumPageSize {
		t.Fatalf("execution page limit = %d", query.limit)
	}
	workbook, err := gateway.ExportRecorte(context.Background(), actor, id, "request")
	if err != nil || len(workbook.Rows) != 1 || workbook.Rows[0][0] != "Ana" {
		t.Fatalf("plain plan export = %#v %v", workbook, err)
	}
}

func shapeSheetGateway(t *testing.T, query *fakeToolQuery, maximumRows int) (*ToolGateway, Identifier) {
	t.Helper()
	id := Identifier{52}
	logical := []byte(`{"plan":{"version":"v1","catalog_version":"` + strings.Repeat("a", 64) + `","root_entity":"profiles","projections":["profile.full_name"],` +
		`"derive":[{"as":"inicial","op":"first_letter","from":"profile.full_name"}],"maximum_rows":` + strconv.Itoa(maximumRows) + `}}`)
	return testToolGateway(t, &fakeToolSearch{}, query, &fakeToolReferences{values: map[Identifier]ResultReference{
		id: {ID: id, Kind: ResultReferenceQuery, LogicalRequest: logical},
	}}), id
}

func threeShapeRows() []querydomain.ShapeRow {
	return []querydomain.ShapeRow{
		{EntityID: "p1", EntityKind: "profile", Label: "Ana", Fields: map[string]string{"profile.full_name": "Ana"}},
		{EntityID: "p2", EntityKind: "profile", Label: "Bia", Fields: map[string]string{"profile.full_name": "Bia"}},
		{EntityID: "p3", EntityKind: "profile", Label: "Caio", Fields: map[string]string{"profile.full_name": "Caio"}},
	}
}

func TestReadPageShapeSheetStopsAtMaximumRowsWithoutIncompleteNote(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	gateway, id := shapeSheetGateway(t, &fakeToolQuery{shape: threeShapeRows(), truncated: true}, 2)
	page, err := gateway.ReadPage(context.Background(), actor, id, 500, 0, "request")
	if err != nil || page.Total != 2 || len(page.Rows) != 2 || page.Rows[1].Label != "Bia" {
		t.Fatalf("capped shape sheet = %#v %v", page, err)
	}
	if page.Truncated || strings.Contains(page.Summary, querydomain.IncompleteScanNote) {
		t.Fatalf("capped shape sheet kept the incomplete note: %#v", page)
	}
}

func TestReadPageShapeSheetKeepsIncompleteNoteBelowMaximumRows(t *testing.T) {
	actor, _ := chatTestActor(t, "member")
	gateway, id := shapeSheetGateway(t, &fakeToolQuery{shape: threeShapeRows(), truncated: true}, 10)
	page, err := gateway.ReadPage(context.Background(), actor, id, 500, 0, "request")
	if err != nil || page.Total != 3 || !page.Truncated || !strings.Contains(page.Summary, querydomain.IncompleteScanNote) {
		t.Fatalf("short shape sheet = %#v %v", page, err)
	}
}

package operations

import (
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/importcatalog"
)

func TestWorkbookLayoutDetectsTitleThenHeader(t *testing.T) {
	sheet := WorkbookSheet{Rows: []WorkbookRow{
		{Number: 1, Cells: []WorkbookCell{{Column: 0, Value: "PLANILHA GERAL 2016"}}},
		{Number: 2, Cells: []WorkbookCell{
			{Column: 0, Value: "Nome"},
			{Column: 1, Value: "CPF"},
			{Column: 2, Value: "E-mail"},
			{Column: 3, Value: "Celular"},
		}},
		{Number: 3, Cells: []WorkbookCell{
			{Column: 0, Value: "Ada Lovelace"},
			{Column: 1, Value: "52998224725"},
			{Column: 2, Value: "ada@example.test"},
		}},
	}}
	layout, err := workbookLayout(sheet)
	if err != nil {
		t.Fatal(err)
	}
	if layout.HeaderRow != 2 || layout.ColumnCount != 4 || layout.Headers[0] != "Nome" {
		t.Fatalf("layout = %#v", layout)
	}
	if !isDataRow(3, layout.HeaderRow) || isDataRow(2, layout.HeaderRow) {
		t.Fatalf("data-row split for header %d", layout.HeaderRow)
	}
}

func TestWorkbookLayoutTreatsGERALILikeHeaderlessAsData(t *testing.T) {
	sheet := WorkbookSheet{Rows: []WorkbookRow{
		{Number: 1, Cells: []WorkbookCell{
			{Column: 0, Value: "Fixture Batch Silva"},
			{Column: 1, Value: "F"},
			{Column: 2, Value: "39"},
			{Column: 3, Value: "15/03/1985"},
			{Column: 4, Value: "BRA"},
			{Column: 5, Value: "1099290030"},
			{Column: 6, Value: "93541134780"},
			{Column: 7, Value: "Rua Exemplo 100"},
			{Column: 8, Value: "Bairro Teste"},
			{Column: 9, Value: "Cidade Fixture"},
			{Column: 10, Value: "RS"},
			{Column: 11, Value: "90000000"},
			{Column: 12, Value: "batch@example.test"},
			{Column: 13, Value: "51900000001"},
			{Column: 15, Value: "5130000001"},
		}},
		{Number: 2, Cells: []WorkbookCell{
			{Column: 0, Value: "Fixture Batch Souza"},
			{Column: 1, Value: "M"},
			{Column: 6, Value: "22222222222"},
			{Column: 10, Value: "RS"},
			{Column: 12, Value: "souza@example.test"},
		}},
	}}
	layout, err := workbookLayout(sheet)
	if err != nil {
		t.Fatal(err)
	}
	if layout.HeaderRow != 0 {
		t.Fatalf("headerless row treated as header: %#v", layout)
	}
	if layout.Headers[0] != "Coluna 1" || layout.ColumnCount != 16 {
		t.Fatalf("synthetic headers = %#v", layout)
	}
	if !isDataRow(1, layout.HeaderRow) {
		t.Fatal("headerless first row must be data")
	}
}

func TestWorkbookLayoutDropsTrailingEmptyColumnsAndUniquifiesDuplicates(t *testing.T) {
	sheet := WorkbookSheet{Rows: []WorkbookRow{
		{Number: 1, Cells: []WorkbookCell{
			{Column: 0, Value: "TELEFONE"},
			{Column: 1, Value: "TELEFONE"},
			{Column: 2, Value: ""},
		}},
		{Number: 2, Cells: []WorkbookCell{
			{Column: 0, Value: "5130000001"},
			{Column: 1, Value: "51900000001"},
		}},
	}}
	layout, err := workbookLayout(sheet)
	if err != nil {
		t.Fatal(err)
	}
	if layout.ColumnCount != 2 {
		t.Fatalf("trailing empty column kept: %#v", layout)
	}
	if layout.Headers[0] != "TELEFONE" || layout.Headers[1] != "TELEFONE_2" {
		t.Fatalf("duplicate headers = %#v", layout.Headers)
	}
}

func TestDetectHasHeaderPortsLegacyCsvRule(t *testing.T) {
	if !detectHasHeader([]string{"nome", "cpf", "email"}, []string{"Ada", "52998224725", "ada@example.test"}) {
		t.Fatal("expected header row")
	}
	if detectHasHeader(
		[]string{"Fixture Batch Silva", "F", "15/03/1985", "RS", "batch@example.test"},
		[]string{"Fixture Batch Souza", "M", "01/01/1990", "RS", "souza@example.test"},
	) {
		t.Fatal("expected headerless data")
	}
}

func TestWorkbookLayoutSkipsExercitoIndexColumnA(t *testing.T) {
	sheet := WorkbookSheet{Rows: []WorkbookRow{
		{Number: 1, Cells: []WorkbookCell{
			{Column: 1, Value: "Nome"},
			{Column: 2, Value: "CPF"},
			{Column: 3, Value: "E-mail"},
		}},
		{Number: 2, Cells: []WorkbookCell{
			{Column: 0, Value: "1"},
			{Column: 1, Value: "Fixture Batch Silva"},
			{Column: 2, Value: "11144477735"},
			{Column: 3, Value: "a@example.test"},
		}},
		{Number: 3, Cells: []WorkbookCell{
			{Column: 0, Value: "2"},
			{Column: 1, Value: "Fixture Batch Souza"},
			{Column: 2, Value: "52998224725"},
			{Column: 3, Value: "b@example.test"},
		}},
	}}
	layout, err := workbookLayout(sheet)
	if err != nil {
		t.Fatal(err)
	}
	if layout.HeaderRow != 1 || layout.ColumnCount != 4 {
		t.Fatalf("layout = %#v", layout)
	}
	if layout.Headers[0] != "Coluna 1" || layout.Headers[1] != "Nome" {
		t.Fatalf("offset headers = %#v", layout.Headers)
	}
	targets := importcatalog.FillUnmapped(importcatalog.ApplyHeaders(importcatalog.ModuleProfiles, layout.Headers), [][]string{
		{"1", "Fixture Batch Silva", "11144477735", "a@example.test"},
		{"2", "Fixture Batch Souza", "52998224725", "b@example.test"},
		{"3", "Fixture Batch Costa", "93541134780", "c@example.test"},
	})
	if targets[0] != importcatalog.DiscardSentinel || targets[1] != "full_name" {
		t.Fatalf("exercito mapping = %#v", targets)
	}
}

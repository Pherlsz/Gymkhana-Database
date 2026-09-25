package operations

import (
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/importcatalog"
)

func TestValidateMappingRejectsDuplicateTargetsAndIncompleteMoneyPair(t *testing.T) {
	profiles, ok := moduleCatalog(auth.RoleExternal, ModuleProfiles)
	if !ok {
		t.Fatal("profiles catalog is unavailable")
	}
	if err := ValidateMapping(profiles, []MappingInput{
		{SourceColumn: 0, TargetField: "full_name"},
		{SourceColumn: 1, TargetField: "full_name"},
	}); !errors.Is(err, ErrInvalidMapping) {
		t.Fatalf("duplicate mapping error = %v", err)
	}
	if err := ValidateMapping(profiles, []MappingInput{
		{SourceColumn: 0, TargetField: "full_name"},
		{SourceColumn: 1, TargetField: "profiles.full_name;drop table profiles"},
	}); !errors.Is(err, ErrInvalidMapping) {
		t.Fatalf("physical/SQL mapping error = %v", err)
	}
	profiles.Fields = append(profiles.Fields, Field{ID: CustomFieldPrefix + "member_code", Label: "Código", Kind: FieldText, Importable: true})
	if err := ValidateMapping(profiles, []MappingInput{
		{SourceColumn: 0, TargetField: "full_name"},
		{SourceColumn: 1, TargetField: CustomFieldPrefix + "member_code"},
	}); err != nil {
		t.Fatalf("logical custom mapping error = %v", err)
	}

	bills, ok := moduleCatalog(auth.RoleExternal, ModuleBills)
	if !ok {
		t.Fatal("bills catalog is unavailable")
	}
	if err := ValidateMapping(bills, []MappingInput{
		{SourceColumn: 0, TargetField: "owner_profile_id"},
		{SourceColumn: 1, TargetField: "bill_type_id"},
		{SourceColumn: 2, TargetField: "amount"},
	}); !errors.Is(err, ErrInvalidMapping) {
		t.Fatalf("incomplete money mapping error = %v", err)
	}
}

func TestMappingFromColumnsSkipsDiscardAndEmpty(t *testing.T) {
	got := mappingFromColumns([]Column{
		{SourceColumn: 0, TargetField: "full_name"},
		{SourceColumn: 1, TargetField: importcatalog.DiscardSentinel},
		{SourceColumn: 2, TargetField: ""},
		{SourceColumn: 3, TargetField: "cpf"},
	})
	if len(got) != 2 || got[0].TargetField != "full_name" || got[1].TargetField != "cpf" || got[1].SourceColumn != 3 {
		t.Fatalf("mappingFromColumns() = %#v", got)
	}
}

func TestWorkbookHeadersRejectBlankSheet(t *testing.T) {
	sheet := WorkbookSheet{Rows: []WorkbookRow{{Number: 1, Cells: []WorkbookCell{{Column: 0, Value: ""}}}}}
	if _, _, err := workbookHeaders(sheet); !errors.Is(err, ErrUnsupportedWorkbook) {
		t.Fatalf("workbookHeaders(%#v) error = %v", sheet, err)
	}
}

func TestWorkbookHeadersUniquifiesCaseInsensitiveDuplicates(t *testing.T) {
	sheet := WorkbookSheet{Rows: []WorkbookRow{{Number: 1, Cells: []WorkbookCell{{Column: 0, Value: "Name"}, {Column: 1, Value: "name"}}}}}
	headers, count, err := workbookHeaders(sheet)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || headers[0] != "Name" || headers[1] != "name_2" {
		t.Fatalf("headers = %#v count=%d", headers, count)
	}
}

func TestCustomValueInputPreservesTextAndNormalizesTypedValues(t *testing.T) {
	fieldID, err := customdata.NewIdentifier()
	if err != nil {
		t.Fatalf("NewIdentifier() error = %v", err)
	}
	definition := customdata.FieldDefinition{
		ID: fieldID,
		Values: customdata.FieldDefinitionValues{
			TargetKind: customdata.TargetProfile, TechnicalKey: "member_code", Label: "Código",
			Kind: customdata.FieldText, Active: true,
		},
		Version: 1, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	value, err := customValueInput(definition, "00123")
	if err != nil || value.Text != "00123" {
		t.Fatalf("customValueInput(text) = %#v, error=%v", value, err)
	}

	definition.Values.Kind = customdata.FieldBoolean
	value, err = customValueInput(definition, "TRUE")
	if err != nil || value.Boolean == nil || !*value.Boolean {
		t.Fatalf("customValueInput(boolean) = %#v, error=%v", value, err)
	}

	definition.Values.Kind = customdata.FieldCivilDate
	if _, err := customValueInput(definition, "17/07/2026"); err == nil {
		t.Fatal("customValueInput accepted a non-canonical civil date")
	}
}

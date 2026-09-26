package bill

import (
	"errors"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

func TestNormalizeTypeCanonicalizesConfiguration(t *testing.T) {
	values, err := NormalizeType(TypeValues{
		TechnicalKey: "  ENERGY_BILL ",
		Label:        "  Conta   de Energia ",
		Active:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if values.TechnicalKey != "energy_bill" || values.Label != "Conta de Energia" || !values.Active {
		t.Fatalf("normalized type = %#v", values)
	}
}

func TestNormalizeBillPreservesPrintedDataAndCanonicalizesCivilMoney(t *testing.T) {
	owner, _ := profile.NewIdentifier()
	typeID, _ := NewIdentifier()
	definition := TypeDefinition{ID: typeID, Values: TypeValues{TechnicalKey: "energy", Label: "Energia", Active: true}}
	values, err := Normalize(Values{
		OwnerProfileID:    owner,
		TypeID:            typeID,
		PrintedHolderName: "  Ana   da Silva ",
		PrintedAddress:    " Rua   Original, 001 ",
		Reference:         "  000A-99  ",
		Competence:        "2026-07",
		Amount:            "000123.4",
		Currency:          "brl",
		Notes:             "  impresso no documento  ",
		Medium:            MediumPhysical,
	}, definition)
	if err != nil {
		t.Fatal(err)
	}
	if values.PrintedHolderName != "Ana da Silva" || values.PrintedAddress != "Rua Original, 001" {
		t.Fatalf("printed values = %#v", values)
	}
	if values.Reference != "000A-99" || values.Competence != "2026-07" || values.Amount != "123.40" || values.Currency != "BRL" {
		t.Fatalf("canonical values = %#v", values)
	}
	if values.Medium != MediumPhysical {
		t.Fatalf("medium = %q", values.Medium)
	}
}

func TestNormalizeBillRequiresPairedValidMoneyAndCivilCompetence(t *testing.T) {
	owner, _ := profile.NewIdentifier()
	typeID, _ := NewIdentifier()
	definition := TypeDefinition{ID: typeID}
	_, err := Normalize(Values{
		OwnerProfileID: owner,
		TypeID:         typeID,
		Competence:     "2026-13",
		Amount:         "12.345",
		Currency:       "real",
		Medium:         "INVALID",
	}, definition)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertBillFieldCode(t, validation, "competence", "invalid_format")
	assertBillFieldCode(t, validation, "amount", "invalid_format")
	assertBillFieldCode(t, validation, "currency", "invalid_format")
	assertBillFieldCode(t, validation, "medium", "invalid_value")
}

func TestNormalizeBillRejectsCurrencyWithoutAmount(t *testing.T) {
	owner, _ := profile.NewIdentifier()
	typeID, _ := NewIdentifier()
	_, err := Normalize(Values{OwnerProfileID: owner, TypeID: typeID, Currency: "BRL", Medium: MediumPhysical}, TypeDefinition{ID: typeID})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertBillFieldCode(t, validation, "currency", "unexpected")
}

func TestNormalizeRequiresMedium(t *testing.T) {
	owner, _ := profile.NewIdentifier()
	typeID, _ := NewIdentifier()
	definition := TypeDefinition{ID: typeID}
	_, err := NormalizeStored(Values{OwnerProfileID: owner, TypeID: typeID}, definition)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertBillFieldCode(t, validation, "medium", "invalid_value")
	_, err = Normalize(Values{OwnerProfileID: owner, TypeID: typeID}, definition)
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertBillFieldCode(t, validation, "medium", "invalid_value")
}

func TestCanonicalAmountRejectsNegativeAndOversizedValues(t *testing.T) {
	for _, value := range []string{"-1.00", "1,00", "12345678901234567.00", ""} {
		if normalized, ok := canonicalAmount(value); ok {
			t.Fatalf("canonicalAmount(%q) = %q, true", value, normalized)
		}
	}
	for input, expected := range map[string]string{"0": "0.00", "000.1": "0.10", "10.25": "10.25"} {
		normalized, ok := canonicalAmount(input)
		if !ok || normalized != expected {
			t.Fatalf("canonicalAmount(%q) = %q, %v; want %q", input, normalized, ok, expected)
		}
	}
}

func assertBillFieldCode(t *testing.T, validation *ValidationError, field, code string) {
	t.Helper()
	for _, problem := range validation.Fields {
		if problem.Field == field && problem.Code == code {
			return
		}
	}
	t.Fatalf("missing %s/%s in %#v", field, code, validation.Fields)
}

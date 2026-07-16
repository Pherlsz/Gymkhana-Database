package customdata

import (
	"errors"
	"testing"
)

func TestNormalizeFieldDefinitionPreservesTypedRules(t *testing.T) {
	targetID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}
	value, err := NormalizeFieldDefinition(FieldDefinitionValues{
		TargetKind:      TargetDocumentType,
		TargetID:        targetID,
		TechnicalKey:    "  Registration_Code ",
		Label:           " Código   de inscrição ",
		Kind:            FieldText,
		Required:        true,
		Active:          true,
		MinimumLength:   2,
		MaximumLength:   30,
		ValidationRegex: `^[A-Z0-9-]+$`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.TechnicalKey != "registration_code" || value.Label != "Código de inscrição" || value.TargetID != targetID {
		t.Fatalf("normalized = %#v", value)
	}
}

func TestNormalizeFieldDefinitionRejectsInvalidTargetAndRuleFamilies(t *testing.T) {
	_, err := NormalizeFieldDefinition(FieldDefinitionValues{
		TargetKind:     TargetProfile,
		TargetID:       mustIdentifier(t),
		TechnicalKey:   "amount",
		Label:          "Valor",
		Kind:           FieldInteger,
		MinimumLength:  1,
		MaximumDecimal: "10.00",
	})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldError(t, validation, "target_id", "unexpected")
	assertFieldError(t, validation, "minimum_length", "unexpected")
	assertFieldError(t, validation, "maximum_decimal", "unexpected")
}

func TestNormalizeDecimalValueCanonicalizesAndAppliesBounds(t *testing.T) {
	fieldID := mustIdentifier(t)
	definition := FieldDefinition{ID: fieldID, Values: FieldDefinitionValues{
		TargetKind: TargetProfile, TechnicalKey: "score", Label: "Pontuação", Kind: FieldDecimal,
		Required: true, Active: true, MinimumDecimal: "-10", MaximumDecimal: "100.5",
	}}
	value, err := NormalizeValue(ValueInput{FieldDefinitionID: fieldID, Decimal: "00042.5000"}, definition)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != FieldDecimal || value.Decimal != "42.5" {
		t.Fatalf("value = %#v", value)
	}
	_, err = NormalizeValue(ValueInput{FieldDefinitionID: fieldID, Decimal: "101"}, definition)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldError(t, validation, "decimal_value", "too_large")
}

func TestNormalizeTextEmailPhoneAndCivilValues(t *testing.T) {
	tests := []struct {
		name  string
		kind  FieldKind
		input ValueInput
		check func(t *testing.T, value ValueInput)
	}{
		{name: "leading zeros", kind: FieldText, input: ValueInput{Text: " 0012-A "}, check: func(t *testing.T, value ValueInput) {
			if value.Text != "0012-A" {
				t.Fatalf("text = %q", value.Text)
			}
		}},
		{name: "email", kind: FieldEmail, input: ValueInput{Text: " USER@EXAMPLE.COM "}, check: func(t *testing.T, value ValueInput) {
			if value.Text != "user@example.com" {
				t.Fatalf("email = %q", value.Text)
			}
		}},
		{name: "phone", kind: FieldPhone, input: ValueInput{Text: "+55 (51) 99999-0000"}, check: func(t *testing.T, value ValueInput) {
			if value.Text != "+5551999990000" {
				t.Fatalf("phone = %q", value.Text)
			}
		}},
		{name: "civil date", kind: FieldCivilDate, input: ValueInput{CivilDate: "2026-07-16"}, check: func(t *testing.T, value ValueInput) {
			if value.CivilDate != "2026-07-16" {
				t.Fatalf("date = %q", value.CivilDate)
			}
		}},
		{name: "civil month", kind: FieldCivilMonth, input: ValueInput{CivilMonth: "2026-07"}, check: func(t *testing.T, value ValueInput) {
			if value.CivilMonth != "2026-07" {
				t.Fatalf("month = %q", value.CivilMonth)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fieldID := mustIdentifier(t)
			definition := FieldDefinition{ID: fieldID, Values: FieldDefinitionValues{
				TargetKind: TargetProfile, TechnicalKey: "field", Label: "Campo", Kind: test.kind, Required: true, Active: true,
			}}
			test.input.FieldDefinitionID = fieldID
			value, err := NormalizeValue(test.input, definition)
			if err != nil {
				t.Fatal(err)
			}
			test.check(t, value)
		})
	}
}

func TestNormalizeSelectValueDeduplicatesAndEnforcesCardinality(t *testing.T) {
	fieldID := mustIdentifier(t)
	first := mustIdentifier(t)
	second := mustIdentifier(t)
	definition := FieldDefinition{ID: fieldID, Values: FieldDefinitionValues{
		TargetKind: TargetProfile, TechnicalKey: "team", Label: "Equipe", Kind: FieldMultiSelect, Required: true, Active: true,
	}}
	value, err := NormalizeValue(ValueInput{FieldDefinitionID: fieldID, OptionIDs: []Identifier{second, first, second}}, definition)
	if err != nil {
		t.Fatal(err)
	}
	if len(value.OptionIDs) != 2 || value.OptionIDs[0].String() > value.OptionIDs[1].String() {
		t.Fatalf("options = %#v", value.OptionIDs)
	}
	definition.Values.Kind = FieldSingleSelect
	_, err = NormalizeValue(ValueInput{FieldDefinitionID: fieldID, OptionIDs: []Identifier{first, second}}, definition)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldError(t, validation, "option_ids", "too_many")
}

func TestNormalizeValueRejectsInactiveOrMismatchedDefinition(t *testing.T) {
	fieldID := mustIdentifier(t)
	definition := FieldDefinition{ID: fieldID, Values: FieldDefinitionValues{
		TargetKind: TargetProfile, TechnicalKey: "note", Label: "Nota", Kind: FieldText, Active: false,
	}}
	_, err := NormalizeValue(ValueInput{FieldDefinitionID: mustIdentifier(t), Kind: FieldInteger, Text: "value"}, definition)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldError(t, validation, "field_definition_id", "invalid_value")
	assertFieldError(t, validation, "field_kind", "invalid_value")
	assertFieldError(t, validation, "field_definition_id", "inactive")
}

func TestNormalizeEntityTypeAllowsOnlyApprovedProfileCardinalities(t *testing.T) {
	value, err := NormalizeEntityType(EntityTypeValues{
		TechnicalKey: " awards ", Label: " Prêmios ", Active: true, ProfileCardinality: CardinalityManyPerProfile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.TechnicalKey != "awards" || value.Label != "Prêmios" {
		t.Fatalf("value = %#v", value)
	}
	_, err = NormalizeEntityType(EntityTypeValues{TechnicalKey: "award", Label: "Prêmio", ProfileCardinality: "GLOBAL"})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldError(t, validation, "profile_cardinality", "invalid_value")
}

func mustIdentifier(t *testing.T) Identifier {
	t.Helper()
	value, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func assertFieldError(t *testing.T, validation *ValidationError, field, code string) {
	t.Helper()
	for _, value := range validation.Fields {
		if value.Field == field && value.Code == code {
			return
		}
	}
	t.Fatalf("missing %s:%s in %#v", field, code, validation.Fields)
}

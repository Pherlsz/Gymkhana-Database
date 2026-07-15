package document

import (
	"errors"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

func TestNormalizeTypeCanonicalizesConfiguration(t *testing.T) {
	values, err := NormalizeType(TypeValues{
		TechnicalKey:     "  RG_GERAL ",
		Label:            "  Registro   Geral ",
		Active:           true,
		UniquenessPolicy: UniquenessPerProfile,
		ValidationRegex:  `^[A-Z0-9]{8,12}$`,
		DateRequired:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if values.TechnicalKey != "rg_geral" || values.Label != "Registro Geral" || values.ValidationRegex != `^[A-Z0-9]{8,12}$` {
		t.Fatalf("normalized type = %#v", values)
	}
}

func TestNormalizeTypeRejectsInvalidRules(t *testing.T) {
	_, err := NormalizeType(TypeValues{
		TechnicalKey:     "RG inválido",
		Label:            "",
		UniquenessPolicy: "UNKNOWN",
		ValidationRegex:  "[",
	})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldCode(t, validation, "technical_key", "invalid_format")
	assertFieldCode(t, validation, "label", "required")
	assertFieldCode(t, validation, "uniqueness_policy", "invalid_value")
	assertFieldCode(t, validation, "validation_regex", "invalid_format")
}

func TestNormalizeDocumentPreservesLeadingZerosAndAlphanumericContent(t *testing.T) {
	owner, _ := profile.NewIdentifier()
	typeID, _ := NewIdentifier()
	definition := TypeDefinition{ID: typeID, Values: TypeValues{
		TechnicalKey:     "registration",
		Label:            "Registration",
		Active:           true,
		UniquenessPolicy: UniquenessGlobalByType,
		ValidationRegex:  `^[A-Z0-9-]+$`,
		DateRequired:     true,
	}}
	values, err := Normalize(Values{
		OwnerProfileID: owner,
		TypeID:         typeID,
		Identifier:     "  00AB-009  ",
		DocumentDate:   "2026-07-15",
		Notes:          "  original value preserved  ",
	}, definition)
	if err != nil {
		t.Fatal(err)
	}
	if values.Identifier != "00AB-009" || values.DocumentDate != "2026-07-15" || values.RecordState != RecordCurrent {
		t.Fatalf("normalized document = %#v", values)
	}
}

func TestNormalizeDocumentValidatesTypeDateRegexAndState(t *testing.T) {
	owner, _ := profile.NewIdentifier()
	typeID, _ := NewIdentifier()
	otherTypeID, _ := NewIdentifier()
	definition := TypeDefinition{ID: typeID, Values: TypeValues{
		UniquenessPolicy: UniquenessNone,
		ValidationRegex:  `^[0-9]{4}$`,
		DateRequired:     true,
	}}
	_, err := Normalize(Values{
		OwnerProfileID: owner,
		TypeID:         otherTypeID,
		Identifier:     "AB12",
		DocumentDate:   "15/07/2026",
		RecordState:    "INVALID",
	}, definition)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldCode(t, validation, "document_type_id", "invalid_value")
	assertFieldCode(t, validation, "identifier_value", "invalid_format")
	assertFieldCode(t, validation, "document_date", "invalid_format")
	assertFieldCode(t, validation, "record_state", "invalid_value")
}

func TestSameRulesIgnoresDisplayOnlyChanges(t *testing.T) {
	left := TypeValues{TechnicalKey: "cpf", Label: "CPF", Active: true, UniquenessPolicy: UniquenessGlobalByType, ValidationRegex: `^[0-9]{11}$`, DateRequired: false}
	right := left
	right.Label = "Cadastro de Pessoa Física"
	right.Active = false
	if !SameRules(left, right) {
		t.Fatal("display and activation changes must not be treated as rule changes")
	}
	right.DateRequired = true
	if SameRules(left, right) {
		t.Fatal("date requirement must be treated as a rule change")
	}
}

func assertFieldCode(t *testing.T, validation *ValidationError, field, code string) {
	t.Helper()
	for _, problem := range validation.Fields {
		if problem.Field == field && problem.Code == code {
			return
		}
	}
	t.Fatalf("missing %s/%s in %#v", field, code, validation.Fields)
}

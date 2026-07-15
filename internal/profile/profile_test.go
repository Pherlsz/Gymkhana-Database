package profile

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeCanonicalProfile(t *testing.T) {
	values, err := Normalize(Values{
		FullName:      "  Ana   da Silva  ",
		SocialName:    "  Aninha ",
		CPF:           "529.982.247-25",
		Email:         "ANA.SILVA@EXAMPLE.COM",
		MobilePhone:   "(51) 99999-8888",
		LandlinePhone: "(51) 3333-4444",
		Address: Address{
			Street:       "  Rua   Principal ",
			Number:       " 123 ",
			Complement:   " Apto  4 ",
			Neighborhood: " Centro ",
			City:         " Porto   Alegre ",
			State:        " rs ",
			PostalCode:   "90000-000",
		},
		Notes: "  Observação com\nquebra de linha.  ",
	})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if values.FullName != "Ana da Silva" || values.SocialName != "Aninha" {
		t.Fatalf("names = %q, %q", values.FullName, values.SocialName)
	}
	if values.CPF != "52998224725" {
		t.Fatalf("CPF = %q", values.CPF)
	}
	if values.Email != "ana.silva@example.com" {
		t.Fatalf("Email = %q", values.Email)
	}
	if values.MobilePhone != "+5551999998888" || values.LandlinePhone != "+555133334444" {
		t.Fatalf("phones = %q, %q", values.MobilePhone, values.LandlinePhone)
	}
	if values.Address.Street != "Rua Principal" || values.Address.State != "RS" || values.Address.PostalCode != "90000000" {
		t.Fatalf("address = %#v", values.Address)
	}
	if values.Notes != "Observação com\nquebra de linha." {
		t.Fatalf("Notes = %q", values.Notes)
	}
}

func TestNormalizeAllowsEmptyOptionalFields(t *testing.T) {
	values, err := Normalize(Values{FullName: "Pessoa sem documentos"})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if values.CPF != "" || values.Email != "" || values.MobilePhone != "" || values.Address.State != "" {
		t.Fatalf("values = %#v", values)
	}
}

func TestNormalizeCollectsStableFieldErrors(t *testing.T) {
	_, err := Normalize(Values{
		FullName:      " ",
		CPF:           "111.111.111-11",
		Email:         "not an email",
		MobilePhone:   "(51) 3333-4444",
		LandlinePhone: "(51) 99999-8888",
		Address: Address{
			State:      "Rio Grande do Sul",
			PostalCode: "90A00-000",
		},
	})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Normalize() error = %T %v", err, err)
	}

	wantFields := map[string]bool{
		"full_name":           false,
		"cpf":                 false,
		"email":               false,
		"mobile_phone":        false,
		"landline_phone":      false,
		"address.state":       false,
		"address.postal_code": false,
	}
	for _, field := range validation.Fields {
		if _, exists := wantFields[field.Field]; exists {
			wantFields[field.Field] = true
		}
	}
	for field, found := range wantFields {
		if !found {
			t.Fatalf("missing field error %q in %#v", field, validation.Fields)
		}
	}
}

func TestNormalizeEnforcesLengthLimits(t *testing.T) {
	_, err := Normalize(Values{
		FullName: strings.Repeat("a", MaxNameLength+1),
		Notes:    strings.Repeat("n", MaxNotesLength+1),
	})
	var validation *ValidationError
	if !errors.As(err, &validation) || len(validation.Fields) != 2 {
		t.Fatalf("Normalize() error = %#v", err)
	}
}

func TestIdentifierRoundTrip(t *testing.T) {
	identifier, err := NewIdentifier()
	if err != nil {
		t.Fatalf("NewIdentifier() error = %v", err)
	}
	parsed, err := ParseIdentifier(identifier.String())
	if err != nil || parsed != identifier {
		t.Fatalf("ParseIdentifier() = %v, %v", parsed, err)
	}
	if _, err := ParseIdentifier("invalid"); !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("ParseIdentifier() error = %v", err)
	}
}

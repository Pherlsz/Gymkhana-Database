package importcatalog

import "testing"

func TestInferFromSamplesMapsGERALIHeaderlessLayout(t *testing.T) {
	row := []string{
		"Fixture Batch Silva",
		"F",
		"39",
		"15/03/1985",
		"BRA",
		"1099290030",
		"11144477735",
		"Rua Exemplo 100",
		"Bairro Teste",
		"Cidade Fixture",
		"RS",
		"90000000",
		"batch@example.test",
		"51900000001",
		"",
		"5130000001",
	}
	got := InferFromSamples([][]string{row}, len(row))
	want := map[int]string{
		0:  "full_name",
		1:  "gender",
		3:  "birth_date",
		4:  "nationality",
		5:  DocumentFieldPrefix + "rg",
		6:  "cpf",
		7:  "address_street",
		8:  "address_neighborhood",
		9:  "address_city",
		10: "address_state",
		11: "address_postal_code",
		12: "email",
		13: "mobile_phone",
		15: "landline_phone",
	}
	for index, field := range want {
		if got[index] != field {
			t.Fatalf("col %d = %q want %q\n%#v", index, got[index], field, got)
		}
	}
	if got[2] != "" || got[14] != "" {
		t.Fatalf("age/empty columns should stay unmapped: %#v", got)
	}
}

func TestInferFromSamplesPrefersCEPOverRGForEightDigits(t *testing.T) {
	got := InferFromSamples([][]string{{"93025690"}}, 1)
	if got[0] != "address_postal_code" {
		t.Fatalf("got %q", got[0])
	}
}

func TestFillUnmappedKeepsHeaderCatalog(t *testing.T) {
	current := []string{"full_name", "", ""}
	samples := [][]string{{"Ada Lovelace", "11144477735", "ada@example.test"}}
	got := FillUnmapped(current, samples)
	if got[0] != "full_name" || got[1] != "cpf" || got[2] != "email" {
		t.Fatalf("FillUnmapped() = %#v", got)
	}
}

func TestResolveDocumentTypeKeepsLabeledColumn(t *testing.T) {
	if got := ResolveDocumentType("ctps", "123.45678.90-1"); got != "ctps" {
		t.Fatalf("labeled ctps remapped to %q", got)
	}
	if got := ResolveDocumentType("generic", "OAB 12345/RS"); got != "oab" {
		t.Fatalf("generic OAB = %q", got)
	}
	if got := ResolveDocumentType("generic", "11144477735"); got != "" {
		t.Fatalf("unlabeled CPF must not promote: %q", got)
	}
	if got := ResolveDocumentType("pis", "123"); got != "pis" {
		t.Fatalf("pis must not become ctps: %q", got)
	}
}

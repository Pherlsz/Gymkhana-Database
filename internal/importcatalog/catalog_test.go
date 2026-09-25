package importcatalog

import "testing"

func TestSuggestColumnProfiles(t *testing.T) {
	cases := []struct {
		header  string
		want    string
		discard bool
	}{
		{"Nome", "full_name", false},
		{"E-mail", "email", false},
		{"Celular", "mobile_phone", false},
		{"Idade", DiscardSentinel, true},
		{"Signo", DiscardSentinel, true},
		{"Desconhecido", "", false},
		{"Sexo", "gender", false},
		{"Data de nascimento:", "birth_date", false},
		{"Município", "address_city", false},
		{"CPF/CGC", "cpf", false},
		{"arquivo", DiscardSentinel, true},
		{"RG", DocumentFieldPrefix + "rg", false},
		{"CNH", DocumentFieldPrefix + "cnh", false},
		{"PIS", DocumentFieldPrefix + "pis", false},
		{"Formação", DocumentFieldPrefix + "generic", false},
		{"rg_presenca", DiscardSentinel, true},
		{"Equipe", "team", false},
		{"Doador sangue", "blood_donor", false},
		{"SOMA_DIGITO", DiscardSentinel, true},
		{"Quem indicou?", DiscardSentinel, true},
		{"Nº", DiscardSentinel, true},
		{"CNH (número e data da primeira emissão):", DocumentFieldPrefix + "cnh", false},
	}
	for _, tc := range cases {
		got := SuggestColumn(ModuleProfiles, tc.header)
		if got.TargetField != tc.want || got.Discard != tc.discard {
			t.Fatalf("SuggestColumn(%q) = %#v, want target=%q discard=%v", tc.header, got, tc.want, tc.discard)
		}
	}
}

func TestApplyHeaders(t *testing.T) {
	targets := ApplyHeaders(ModuleProfiles, []string{"Nome", "Idade", "CPF"})
	if targets[0] != "full_name" || targets[1] != DiscardSentinel || targets[2] != "cpf" {
		t.Fatalf("ApplyHeaders() = %#v", targets)
	}
}

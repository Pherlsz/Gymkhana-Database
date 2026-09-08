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
		{"Idade", discardSentinel, true},
		{"Signo", discardSentinel, true},
		{"Desconhecido", "", false},
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
	if targets[0] != "full_name" || targets[1] != discardSentinel || targets[2] != "cpf" {
		t.Fatalf("ApplyHeaders() = %#v", targets)
	}
}

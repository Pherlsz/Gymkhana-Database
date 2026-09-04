package search

import (
	"testing"
)

func TestParseQueryBareWordsANDQuotedExcludeORAndField(t *testing.T) {
	parsed, err := ParseQuery(`Ana "da Silva" -equipe:azul cidade:Recife OU Pedro`)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if len(parsed.Branches) != 2 {
		t.Fatalf("branches = %d, want 2: %#v", len(parsed.Branches), parsed)
	}
	if got := parsed.Branches[0].Atoms; len(got) != 4 || got[0].Value != "Ana" || got[1].Compare != ComparePhrase || got[1].Value != "da Silva" || !got[2].Exclude || got[2].FieldToken != "equipe" || got[3].FieldToken != "cidade" || got[3].Value != "Recife" {
		t.Fatalf("first branch = %#v", got)
	}
	if got := parsed.Branches[1].Atoms; len(got) != 1 || got[0].Value != "Pedro" {
		t.Fatalf("second branch = %#v", got)
	}
}

func TestParseQueryPortugueseTokensTypeSugarPrefixAndModule(t *testing.T) {
	parsed, err := ParseQuery(`em:documentos tipo:RG cpf:529* identificador:8441*`)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if len(parsed.Modules) != 1 || parsed.Modules[0] != ModuleDocuments {
		t.Fatalf("modules = %#v", parsed.Modules)
	}
	atoms := parsed.Branches[0].Atoms
	if len(atoms) != 3 {
		t.Fatalf("atoms = %#v", atoms)
	}
	if atoms[0].FieldToken != "tipo" || atoms[0].Value != "RG" {
		t.Fatalf("tipo atom = %#v", atoms[0])
	}
	if atoms[1].TypeSugar != "cpf" || atoms[1].Compare != ComparePrefix || atoms[1].Value != "529" {
		t.Fatalf("cpf sugar = %#v", atoms[1])
	}
	if atoms[2].FieldToken != "identificador" || atoms[2].Compare != ComparePrefix || atoms[2].Value != "8441" {
		t.Fatalf("identificador = %#v", atoms[2])
	}
}

func TestParseQueryRangesComparatorsAndAccentFoldOnOR(t *testing.T) {
	parsed, err := ParseQuery(`valor:>=100 competencia:2024-01..2024-06 Ana OU Pedro`)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	atoms := parsed.Branches[0].Atoms
	if atoms[0].Compare != CompareGTE || atoms[0].Value != "100" || atoms[1].Compare != CompareBetween || atoms[1].Value != "2024-01" || atoms[1].Value2 != "2024-06" {
		t.Fatalf("atoms = %#v", atoms)
	}
	folded, err := ParseQuery("Ana ou Pedro")
	if err != nil || len(folded.Branches) != 2 {
		t.Fatalf("accented OU = %#v, err = %v", folded, err)
	}
}

func TestParseQueryRejectsPhysicalNamesUnknownModuleAndUnclosedQuote(t *testing.T) {
	if _, err := ParseQuery(`profiles.full_name:Ana`); err == nil || !hasCode(err, "physical_name") {
		t.Fatalf("physical name error = %v", err)
	}
	if _, err := ParseQuery(`em:physical_table Ana`); err == nil || !hasCode(err, "unknown_module") {
		t.Fatalf("unknown module error = %v", err)
	}
	if _, err := ParseQuery(`"Ana`); err == nil || !hasCode(err, "unclosed_quote") {
		t.Fatalf("unclosed quote error = %v", err)
	}
}

func TestParseQueryEnglishAliasesRemainValidForAssistente(t *testing.T) {
	parsed, err := ParseQuery(`in:profiles type:RG name:Ana`)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if parsed.Modules[0] != ModuleProfiles {
		t.Fatalf("modules = %#v", parsed.Modules)
	}
	if parsed.Branches[0].Atoms[0].FieldToken != "type" || parsed.Branches[0].Atoms[1].FieldToken != "name" {
		t.Fatalf("atoms = %#v", parsed.Branches[0].Atoms)
	}
}

func TestResolveUnknownFieldSuggestsCatalog(t *testing.T) {
	catalog := Catalog{Fields: []FieldDefinition{
		{Key: "profile.address_city", Module: ModuleProfiles, Label: "Cidade", Kind: "text"},
	}}
	parsed, err := ParseQuery(`cidde:Recife`)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	_, resolved := resolveParsed(parsed, catalog)
	if resolved == nil || !hasCode(resolved, "unknown_field") {
		t.Fatalf("resolve error = %v", resolved)
	}
}

func hasCode(err error, code string) bool {
	validation, ok := err.(*ValidationError)
	if !ok {
		return false
	}
	for _, field := range validation.Fields {
		if field.Code == code {
			return true
		}
	}
	return false
}

func TestParseQueryTypeSugarMapsToTechnicalKey(t *testing.T) {
	parsed, err := ParseQuery(`titulo:123 passaporte:AB123456`)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	atoms := parsed.Branches[0].Atoms
	if len(atoms) != 2 {
		t.Fatalf("atoms = %#v", atoms)
	}
	if atoms[0].TypeSugar != "voter_id" || atoms[1].TypeSugar != "passport" {
		t.Fatalf("type sugars = %#v", atoms)
	}
}

func TestParseQueryRejectsRetiredTypeSugars(t *testing.T) {
	parsed, err := ParseQuery(`reservista:123`)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if parsed.Branches[0].Atoms[0].TypeSugar != "" {
		t.Fatalf("reservista must not be a type sugar: %#v", parsed.Branches[0].Atoms[0])
	}
	catalog := Catalog{Fields: []FieldDefinition{
		{Key: "profile.full_name", Module: ModuleProfiles, Label: "Nome completo", Kind: "text"},
	}}
	if _, resolved := resolveParsed(parsed, catalog); resolved == nil || !hasCode(resolved, "unknown_field") {
		t.Fatalf("resolve error = %v", resolved)
	}
}

func TestFoldTokenStripsAccents(t *testing.T) {
	if got := foldToken("típo"); got != "tipo" {
		t.Fatalf("foldToken() = %q", got)
	}
}

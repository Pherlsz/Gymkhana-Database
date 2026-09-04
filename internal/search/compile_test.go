package search

import (
	"strings"
	"testing"
)

func TestTermSpecCanonicalizesFormattedCPF(t *testing.T) {
	spec := termSpecFor(resolvedAtom{
		Atom:      Atom{FieldToken: "identificador", Value: "529.982.247-25", TypeSugar: "cpf", Compare: CompareContains},
		FieldKeys: []string{"document.identifier", "profile.document_identifier"},
		Kind:      "identifier",
	})
	if spec.Term != "52998224725" {
		t.Fatalf("term = %q, want canonical CPF digits", spec.Term)
	}
	if !containsPattern(spec, "52998224725") {
		t.Fatalf("patterns = %#v, want canonical digits", spec.Patterns)
	}
}

func TestTermSpecPreservesOABLettersAndUF(t *testing.T) {
	spec := termSpecFor(resolvedAtom{
		Atom:      Atom{FieldToken: "identificador", Value: "12345/SP", TypeSugar: "oab", Compare: CompareContains},
		FieldKeys: []string{"document.identifier"},
		Kind:      "identifier",
	})
	joined := strings.Join(spec.Patterns, " ")
	if strings.Contains(joined, "12345/sp") == false && !strings.Contains(strings.ToLower(spec.Term), "12345/sp") {
		t.Fatalf("OAB spec lost UF: term=%q patterns=%#v", spec.Term, spec.Patterns)
	}
	if spec.Term == "12345" {
		t.Fatal("OAB must not collapse to digits-only")
	}
}

func TestTermSpecUnlabeledCPFAddsCanonicalAndKeepsText(t *testing.T) {
	spec := termSpecFor(resolvedAtom{
		Atom: Atom{Value: "529.982.247-25", Compare: CompareContains},
	})
	if !containsPattern(spec, "52998224725") {
		t.Fatalf("unlabeled CPF missing canonical pattern: %#v", spec.Patterns)
	}
	if spec.Term == "" {
		t.Fatal("unlabeled CPF dropped the search term")
	}
}

func TestTermSpecTipoUsesTechnicalKey(t *testing.T) {
	spec := termSpecFor(resolvedAtom{
		Atom:      Atom{FieldToken: "tipo", Value: "Título", Compare: CompareContains},
		FieldKeys: []string{"document.type", "bill.type"},
		Kind:      "text",
	})
	if spec.Term != "voter_id" && !containsPattern(spec, "voter_id") {
		t.Fatalf("tipo:Título = term %q patterns %#v, want voter_id", spec.Term, spec.Patterns)
	}
}

func TestExpandTypeSugarUsesTechnicalKey(t *testing.T) {
	expanded := expandTypeSugar(Atom{TypeSugar: "voter_id", Value: "123456780493", Compare: CompareContains})
	if len(expanded) != 2 || expanded[0].Value != "voter_id" || expanded[1].TypeSugar != "voter_id" {
		t.Fatalf("expandTypeSugar = %#v", expanded)
	}
}

func TestResolveCPFSugarBindsTipoToPresenceIdentifier(t *testing.T) {
	parsed, err := ParseQuery("cpf:529.982.247-25")
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	resolved, resolveErr := resolveParsed(parsed, Catalog{Fields: staticFields})
	if resolveErr != nil {
		t.Fatalf("resolveParsed() error = %v", resolveErr)
	}
	if len(resolved.Branches) != 1 || len(resolved.Branches[0]) != 2 {
		t.Fatalf("resolved = %#v", resolved)
	}
	tipo := resolved.Branches[0][0]
	if tipo.FieldToken != "tipo" {
		t.Fatalf("first atom = %#v, want tipo", tipo)
	}
	foundPresence := false
	for _, key := range tipo.FieldKeys {
		if key == "profile.document_identifier" {
			foundPresence = true
			break
		}
	}
	if !foundPresence {
		t.Fatalf("tipo field keys = %#v, want profile.document_identifier for presence-only CPF", tipo.FieldKeys)
	}
}

func TestCompileTituloKeepsKindAndDigits(t *testing.T) {
	parsed, err := ParseQuery("titulo:032658690493")
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if parsed.Branches[0].Atoms[0].TypeSugar != "voter_id" {
		t.Fatalf("sugar = %#v", parsed.Branches[0].Atoms[0])
	}
	resolved, resolveErr := resolveParsed(parsed, Catalog{Fields: staticFields})
	if resolveErr != nil {
		t.Fatalf("resolveParsed() error = %v", resolveErr)
	}
	includes, _, _ := compileBranch(resolved.Branches[0])
	if len(includes) != 2 {
		t.Fatalf("includes = %#v", includes)
	}
	if includes[0].Term != "voter_id" {
		t.Fatalf("tipo term = %q, want voter_id without underscore folding", includes[0].Term)
	}
	if !strings.Contains(includes[0].Pattern, `voter\_id`) {
		t.Fatalf("tipo pattern should LIKE-escape the technical key, got %#v", includes[0])
	}
	joined := strings.Join(includes[1].Patterns, " ")
	if !strings.Contains(joined, "032658690493") {
		t.Fatalf("compiled identifier patterns = %#v", includes[1])
	}
}

func TestCompileBranchKeepsSharedTermAcrossCanonicalPatterns(t *testing.T) {
	includes, _, terms := compileBranch([]resolvedAtom{{
		Atom:      Atom{FieldToken: "identificador", Value: "529.982.247-25", TypeSugar: "cpf", Compare: CompareContains},
		FieldKeys: []string{"document.identifier"},
		Kind:      "identifier",
	}})
	if len(includes) != 1 || len(includes[0].Patterns) < 1 {
		t.Fatalf("includes = %#v", includes)
	}
	if len(terms) != 1 {
		t.Fatalf("AND counting requires one term, got %#v", terms)
	}
}

func TestTextSearchPatternsFoldsAccentsForSQLTranslate(t *testing.T) {
	patterns := textSearchPatterns("Fênix 001")
	if !containsAny(patterns, "fenix 001") {
		t.Fatalf("accented terms must also emit a folded LIKE pattern, got %#v", patterns)
	}
	if !containsAny(patterns, "fênix 001") {
		t.Fatalf("raw lowercased pattern missing: %#v", patterns)
	}
}

func containsAny(patterns []string, needle string) bool {
	folded := strings.ToLower(needle)
	for _, pattern := range patterns {
		if strings.Contains(strings.ToLower(pattern), folded) {
			return true
		}
	}
	return false
}

func containsPattern(spec TermSpec, needle string) bool {
	folded := strings.ToLower(needle)
	if strings.Contains(strings.ToLower(spec.Pattern), folded) || strings.Contains(strings.ToLower(spec.Term), folded) {
		return true
	}
	for _, pattern := range spec.Patterns {
		if strings.Contains(strings.ToLower(pattern), folded) {
			return true
		}
	}
	return false
}

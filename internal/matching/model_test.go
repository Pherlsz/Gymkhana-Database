package matching

import (
	"testing"
)

func TestEvidenceCatalogAndScoreBandsAreClosedAndDeterministic(t *testing.T) {
	catalog := EvidenceCatalog()
	if len(catalog) != 9 {
		t.Fatalf("EvidenceCatalog() length = %d", len(catalog))
	}
	seen := make(map[EvidenceKind]struct{}, len(catalog))
	for _, definition := range catalog {
		if !definition.Kind.Valid() || definition.Label == "" {
			t.Fatalf("invalid evidence definition: %#v", definition)
		}
		if _, duplicate := seen[definition.Kind]; duplicate {
			t.Fatalf("duplicate evidence kind: %s", definition.Kind)
		}
		seen[definition.Kind] = struct{}{}
	}
	if EvidenceKind("RAW_SQL").Valid() {
		t.Fatal("unknown evidence kind is valid")
	}
	for score, expected := range map[int]ScoreBand{50: ScoreLow, 69: ScoreLow, 70: ScoreMedium, 89: ScoreMedium, 90: ScoreHigh, 100: ScoreHigh} {
		if actual := scoreBand(score); actual != expected {
			t.Fatalf("scoreBand(%d) = %s, want %s", score, actual, expected)
		}
	}
}

func TestPreviewInputAndIdempotencyValidationFailClosed(t *testing.T) {
	caseID, _ := NewIdentifier()
	survivorID, _ := NewIdentifier()
	sourceID, _ := NewIdentifier()
	valid := MergePreviewInput{
		CaseID: caseID, SurvivorID: survivorID, SourceID: sourceID,
		SurvivorVersion: 1, SourceVersion: 2,
		Choices: []FieldChoice{{FieldKey: "full_name", Source: FieldFromSurvivor}},
	}
	if err := validatePreviewInput(valid); err != nil {
		t.Fatalf("validatePreviewInput(valid) error = %v", err)
	}
	for _, mutate := range []func(*MergePreviewInput){
		func(value *MergePreviewInput) { value.CaseID = Identifier{} },
		func(value *MergePreviewInput) { value.SourceID = value.SurvivorID },
		func(value *MergePreviewInput) { value.SourceVersion = 0 },
		func(value *MergePreviewInput) { value.Choices[0].FieldKey = "profiles.full_name;drop" },
		func(value *MergePreviewInput) { value.Choices[0].Source = FieldSource("AUTO") },
		func(value *MergePreviewInput) { value.Choices = append(value.Choices, value.Choices[0]) },
	} {
		candidate := valid
		candidate.Choices = append([]FieldChoice(nil), valid.Choices...)
		mutate(&candidate)
		if err := validatePreviewInput(candidate); err == nil {
			t.Fatalf("invalid preview accepted: %#v", candidate)
		}
	}
	for _, key := range []string{"analysis-0001", "merge:0001", "a_b.c-123"} {
		if !validIdempotencyKey(key) {
			t.Fatalf("valid idempotency key rejected: %q", key)
		}
	}
	for _, key := range []string{"short", " leading-key", "key with spaces", "key/with/path"} {
		if validIdempotencyKey(key) {
			t.Fatalf("invalid idempotency key accepted: %q", key)
		}
	}
}

func TestCaseListOrderingIsClosedAndDeterministic(t *testing.T) {
	defaults, err := normalizeCaseListOptions(CaseListOptions{})
	if err != nil || defaults.Sort != CaseSortScore || defaults.Order != SortDescending ||
		len(defaults.States) != 1 || defaults.States[0] != CasePending {
		t.Fatalf("default CaseListOptions = %#v, error=%v", defaults, err)
	}
	for _, invalid := range []CaseListOptions{
		{Sort: CaseSort("profiles.full_name")},
		{Order: SortOrder("sideways")},
		{States: []CaseState{CasePending, CasePending}},
		{Bands: []ScoreBand{ScoreHigh, ScoreHigh}},
	} {
		if _, err := normalizeCaseListOptions(invalid); err == nil {
			t.Fatalf("invalid CaseListOptions accepted: %#v", invalid)
		}
	}
	for _, test := range []struct {
		sort     CaseSort
		order    SortOrder
		selected string
		outer    string
	}{
		{CaseSortScore, SortDescending, "score DESC, updated_at DESC, id DESC", "matching_case.score DESC, matching_case.updated_at DESC, matching_case.id DESC"},
		{CaseSortScore, SortAscending, "score ASC, updated_at DESC, id DESC", "matching_case.score ASC, matching_case.updated_at DESC, matching_case.id DESC"},
		{CaseSortUpdatedAt, SortAscending, "updated_at ASC, id ASC", "matching_case.updated_at ASC, matching_case.id ASC"},
		{CaseSortCreatedAt, SortDescending, "created_at DESC, id DESC", "matching_case.created_at DESC, matching_case.id DESC"},
	} {
		selected, outer := matchingCaseOrder(test.sort, test.order)
		if selected != test.selected || outer != test.outer {
			t.Fatalf("matchingCaseOrder(%q,%q) = %q / %q", test.sort, test.order, selected, outer)
		}
	}
}

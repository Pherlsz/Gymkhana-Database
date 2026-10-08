package search

import "testing"

func TestPartitionProfileSearchOrdersVisibleBeforeHidden(t *testing.T) {
	hits := []ProfileHit{
		{ProfileID: "hidden", FieldKey: "profile.notes", FieldLabel: "Observações", Display: "falou com Pedro ontem", Weight: 10},
		{ProfileID: "visible", FieldKey: "profile.full_name", FieldLabel: "Nome completo", Display: "Pedro Alves", Weight: 80},
	}
	ordered, evidence := PartitionProfileIDs([]string{"hidden", "visible"}, hits, []string{"full_name"}, false, nil, "Pedro")
	if len(ordered) != 2 || ordered[0] != "visible" || ordered[1] != "hidden" {
		t.Fatalf("ordered = %#v", ordered)
	}
	if _, ok := evidence["visible"]; ok {
		t.Fatal("visible name match must not carry hidden evidence")
	}
	got := evidence["hidden"]
	if got.Label != "Observações" || got.Snippet != "\u2026Pedro\u2026" || got.OtherHiddenMatches != 0 {
		t.Fatalf("evidence = %#v", got)
	}
}

func TestHiddenEvidenceUsesHighestWeightAndCountsTheRest(t *testing.T) {
	hits := []ProfileHit{
		{ProfileID: "p", FieldKey: "profile.notes", FieldLabel: "Observações", Display: "Pedro nas notas", Weight: 10},
		{ProfileID: "p", FieldKey: "profile.email", FieldLabel: "E-mail", Display: "pedro@example.com", Weight: 60},
		{ProfileID: "p", FieldKey: "profile.pet", FieldLabel: "Animal", Display: "Pedro o gato", Weight: 20},
	}
	ordered, evidence := PartitionProfileIDs([]string{"p"}, hits, []string{"full_name"}, false, nil, "Pedro")
	if len(ordered) != 1 || ordered[0] != "p" {
		t.Fatalf("ordered = %#v", ordered)
	}
	got := evidence["p"]
	if got.Label != "E-mail" || got.FieldKey != "profile.email" || got.OtherHiddenMatches != 2 || got.Snippet != "pedro\u2026" {
		t.Fatalf("evidence = %#v", got)
	}
}

func TestHiddenEvidenceOmitsFieldsTheCallerCannotSearch(t *testing.T) {
	hits := []ProfileHit{
		{ProfileID: "p", FieldKey: "profile.notes", FieldLabel: "Observações", Display: "Pedro", Weight: 10},
		{ProfileID: "p", FieldKey: "secret.field", FieldLabel: "Segredo", Display: "Pedro secreto", Weight: 100},
	}
	allowed := map[string]struct{}{"profile.notes": {}}
	_, evidence := PartitionProfileIDs([]string{"p"}, hits, []string{"full_name"}, false, allowed, "Pedro")
	got := evidence["p"]
	if got.Label != "Observações" || got.OtherHiddenMatches != 0 || got.FieldKey != "profile.notes" {
		t.Fatalf("evidence = %#v", got)
	}
}

func TestDefaultColumnsTreatNotesAsHiddenAndNameAsVisible(t *testing.T) {
	hits := []ProfileHit{
		{ProfileID: "notes", FieldKey: "profile.notes", FieldLabel: "Observações", Display: "só Pedro aqui", Weight: 10},
		{ProfileID: "name", FieldKey: "profile.full_name", FieldLabel: "Nome completo", Display: "Pedro", Weight: 80},
	}
	ordered, evidence := PartitionProfileIDs([]string{"notes", "name"}, hits, nil, true, nil, "Pedro")
	if ordered[0] != "name" || ordered[1] != "notes" {
		t.Fatalf("ordered = %#v", ordered)
	}
	if _, ok := evidence["name"]; ok {
		t.Fatal("default visible name must not emit evidence")
	}
	if evidence["notes"].Label != "Observações" {
		t.Fatalf("notes evidence = %#v", evidence["notes"])
	}
}

package queryengine

import (
	"strings"
	"testing"
)

func TestShapeBinaryCPFListsFormableCharacters(t *testing.T) {
	index := 0
	rows := []ShapeRow{{
		EntityID: "p1", EntityKind: "profile", Label: "Ana",
		Fields: map[string]string{"profile.full_name": "Ana", "profile.cpf": "011.000.019-35"},
	}}
	plan := QueryPlan{
		Projections: []string{"profile.full_name"},
		Derive:      []Derivation{{As: "bits", Op: "keep", From: "profile.cpf", Class: "digits"}},
		Matches: []MatchSpec{{
			On: "bits", Order: "any", Width: 8, Alphabet: "01", Interpret: "byte", Show: "caracteres",
		}},
	}
	_ = index
	page := ApplyShape(rows, plan, false)
	if page.Total != 1 {
		t.Fatalf("total = %d", page.Total)
	}
	shown := page.Rows[0].Cells["caracteres"]
	if !strings.Contains(shown, "a") || !strings.Contains(shown, "C") {
		t.Fatalf("characters = %q", shown)
	}
	for _, character := range strings.Split(shown, ", ") {
		if strings.Count(character, "") > 0 && bitsOf(character) > 3 {
			t.Fatalf("character %q needs more than three 1 bits", character)
		}
	}
}

func bitsOf(character string) int {
	if character == "" {
		return 0
	}
	value := int([]rune(character)[0])
	count := 0
	for value > 0 {
		count += value & 1
		value >>= 1
	}
	return count
}

func TestShapeFlowerNamesStayOnThePersonRow(t *testing.T) {
	rows := []ShapeRow{{
		EntityID: "p1", EntityKind: "profile", Label: "Rosa Lírio",
		Fields: map[string]string{"profile.full_name": "Rosa Maria Lírio"},
	}}
	plan := QueryPlan{
		Projections: []string{"profile.full_name"},
		Matches: []MatchSpec{{
			On: "profile.full_name", Order: "keep", Equals: []string{"Rosa", "Lírio", "Violeta"}, Show: "flor", Quantifier: "any",
		}},
	}
	page := ApplyShape(rows, plan, false)
	if page.Total != 1 || page.Rows[0].Cells["flor"] != "Rosa, Lírio" {
		t.Fatalf("flowers = %#v", page.Rows)
	}
}

func TestShapeKeepReturnsTheSpanAndAnyWithoutWidthUsesTheWholeText(t *testing.T) {
	rows := []ShapeRow{{
		EntityID: "d1", EntityKind: "document",
		Fields: map[string]string{"document.identifier": "RG-2024-77"},
	}}
	kept := ApplyShape(rows, QueryPlan{
		Projections: []string{"document.identifier"},
		Matches:     []MatchSpec{{On: "document.identifier", Order: "keep", Grammar: PatternDigits, Pattern: "2024", Show: "trecho"}},
	}, false)
	if kept.Rows[0].Cells["trecho"] != "2024" {
		t.Fatalf("span = %#v", kept.Rows)
	}
	reordered := ApplyShape([]ShapeRow{{
		EntityID: "p1", Fields: map[string]string{"phone": "19980321"},
	}}, QueryPlan{
		Projections: []string{"phone"},
		Matches:     []MatchSpec{{On: "phone", Order: "any", Equals: []string{"19980321"}, Show: "data"}},
	}, false)
	if reordered.Total != 1 {
		t.Fatalf("reorder = %#v", reordered)
	}
	leftover := ApplyShape([]ShapeRow{{
		EntityID: "p2", Fields: map[string]string{"phone": "199803219"},
	}}, QueryPlan{
		Projections: []string{"phone"},
		Matches:     []MatchSpec{{On: "phone", Order: "any", Equals: []string{"19980321"}}},
	}, false)
	if leftover.Total != 0 {
		t.Fatalf("leftover symbols still matched: %#v", leftover.Rows)
	}
}

func TestShapeSequencePartitionsAndReportsTies(t *testing.T) {
	rows := []ShapeRow{
		{EntityID: "a", Fields: map[string]string{"holder": "Ana", "letter": "A", "number": "10", "city": "Recife"}},
		{EntityID: "c", Fields: map[string]string{"holder": "Ana", "letter": "C", "number": "30", "city": "Olinda"}},
		{EntityID: "b", Fields: map[string]string{"holder": "Ana", "letter": "B", "number": "20", "city": "Natal"}},
		{EntityID: "e", Fields: map[string]string{"holder": "Bia", "letter": "A", "number": "1", "city": "Maceio"}},
		{EntityID: "f", Fields: map[string]string{"holder": "Bia", "letter": "B", "number": "2", "city": "Fortaleza"}},
	}
	page := ApplyShape(rows, QueryPlan{
		Projections: []string{"holder", "letter"},
		Sequence: &SequenceSpec{
			By: "letter", Alphabet: "A-Z", Step: "next",
			Along:     &SequenceAlong{Field: "number", Step: "either_monotonic"},
			Partition: "holder", Minimum: 3, Distinct: "city",
		},
	}, false)
	if page.Total != 3 || !strings.Contains(page.Summary, "Atende o mínimo de 3") {
		t.Fatalf("partition chain = %#v %s", page.Rows, page.Summary)
	}
	tied := ApplyShape([]ShapeRow{
		{EntityID: "1", Fields: map[string]string{"letter": "A", "number": "1"}},
		{EntityID: "2", Fields: map[string]string{"letter": "B", "number": "2"}},
		{EntityID: "3", Fields: map[string]string{"letter": "D", "number": "4"}},
		{EntityID: "4", Fields: map[string]string{"letter": "E", "number": "5"}},
	}, QueryPlan{
		Projections: []string{"letter"},
		Sequence: &SequenceSpec{
			By: "letter", Alphabet: "A-Z", Step: "next",
			Along: &SequenceAlong{Field: "number", Step: "increase"},
		},
	}, false)
	if tied.Total != 4 || !strings.Contains(tied.Summary, "Empate") {
		t.Fatalf("tie = total %d summary %s rows %#v", tied.Total, tied.Summary, tied.Rows)
	}
}

func TestShapeReverseAndSecondPage(t *testing.T) {
	rows := []ShapeRow{{
		EntityID: "p1", Fields: map[string]string{"profile.vehicle_plate": "ABA"},
	}, {
		EntityID: "p2", Fields: map[string]string{"profile.vehicle_plate": "ABC"},
	}}
	plan := QueryPlan{
		Projections: []string{"profile.vehicle_plate"},
		Derive:      []Derivation{{As: "invertida", Op: "reverse", From: "profile.vehicle_plate"}},
		Matches:     []MatchSpec{{On: "profile.vehicle_plate", Order: "keep", Equals: []string{"ABA"}, Show: "igual"}},
	}
	// Palindrome: reversed text equals the original. Filter in the test by comparing derived values.
	page := ApplyShape(rows, QueryPlan{
		Projections: []string{"profile.vehicle_plate"},
		Derive: []Derivation{
			{As: "invertida", Op: "reverse", From: "profile.vehicle_plate"},
		},
	}, false)
	if page.Rows[0].Cells["invertida"] != "ABA" || page.Rows[1].Cells["invertida"] != "CBA" {
		t.Fatalf("reverse = %#v", page.Rows)
	}
	paged := page.Page(1, 1)
	if paged.Total != 2 || len(paged.Rows) != 1 || paged.Rows[0].EntityID != "p2" {
		t.Fatalf("page = %#v", paged.Rows)
	}
	_ = plan
}

func TestShapeListsSortAlphabetically(t *testing.T) {
	page := ApplyShape([]ShapeRow{
		{EntityID: "z", Label: "Zilda", Fields: map[string]string{"profile.full_name": "Zilda"}},
		{EntityID: "o", Label: "Óscar", Fields: map[string]string{"profile.full_name": "Óscar"}},
		{EntityID: "a", Label: "Ana", Fields: map[string]string{"profile.full_name": "Ana"}},
	}, QueryPlan{Projections: []string{"profile.full_name"}}, false)
	if page.Total != 3 || page.Rows[0].Label != "Ana" || page.Rows[1].Label != "Óscar" || page.Rows[2].Label != "Zilda" {
		t.Fatalf("alphabetical = %#v", page.Rows)
	}
}

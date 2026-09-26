package queryengine

import "testing"

func TestOrderExpressionUsesPortugueseCollationForText(t *testing.T) {
	if got, want := orderExpression("q0.full_name", ValueText, "DESC"), "q0.full_name COLLATE gymkhana_pt_br DESC NULLS LAST"; got != want {
		t.Fatalf("text order = %q, want %q", got, want)
	}
	if got, want := orderExpression("q0.notes", ValueLongText, "ASC"), "q0.notes COLLATE gymkhana_pt_br ASC NULLS LAST"; got != want {
		t.Fatalf("long text order = %q, want %q", got, want)
	}
	if got, want := orderExpression("q0.amount", ValueDecimal, "ASC"), "q0.amount ASC NULLS LAST"; got != want {
		t.Fatalf("numeric order = %q, want %q", got, want)
	}
}

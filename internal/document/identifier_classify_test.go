package document

import "testing"

func TestClassifyIdentifierExtractsNumbersAndStates(t *testing.T) {
	cases := []struct {
		in     string
		action IdentifierAction
		number string
		state  string
	}{
		{in: "OAB/RS 114.458", action: IdentifierKeep, number: "114458", state: "RS"},
		{in: "PIS 120.5824.883-1", action: IdentifierKeep, number: "1205824883-1"},
		{in: "CREA 1234567890-0/RS", action: IdentifierKeep, number: "1234567890-0", state: "RS"},
		{in: "sim, carteirinha: 182265", action: IdentifierKeep, number: "182265"},
		{in: "Uniasselvi 4238539", action: IdentifierKeep, number: "4238539"},
		{in: "00AB-009", action: IdentifierKeep, number: "00AB-009"},
		{in: "Não", action: IdentifierDelete},
		{in: "não tenho", action: IdentifierDelete},
		{in: "Não possuo", action: IdentifierDelete},
		{in: "Perdida", action: IdentifierDelete},
		{in: "perdi", action: IdentifierDelete},
		{in: "Sim", action: IdentifierKeep},
		{in: "possuo", action: IdentifierKeep},
		{in: "", action: IdentifierKeep},
	}
	for _, test := range cases {
		got := ClassifyIdentifier(test.in)
		if got.Action != test.action || got.Number != test.number || got.State != test.state {
			t.Fatalf("%q: got %#v want action=%s number=%q state=%q", test.in, got, test.action, test.number, test.state)
		}
	}
}

func TestApplyIdentifierClassificationRejectsRefusals(t *testing.T) {
	_, _, err := ApplyIdentifierClassification("Não")
	if err == nil {
		t.Fatal("expected refusal")
	}
	number, classified, err := ApplyIdentifierClassification("OAB/RS 114.458")
	if err != nil || number != "114458" || classified.State != "RS" {
		t.Fatalf("apply = %q %#v %v", number, classified, err)
	}
}

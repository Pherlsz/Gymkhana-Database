package foundation

import (
	"encoding/json"
	"testing"

	"github.com/Pherlsz/Gymkhana-Core/civiltime"
	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

func TestReleasedCoreNormalizationContracts(t *testing.T) {
	if got, want := normalize.DisplayText("  Ana\tMaria  "), "Ana Maria"; got != want {
		t.Fatalf("DisplayText() = %q, want %q", got, want)
	}
	if got, want := normalize.SearchText("Ána-María"), "ana maria"; got != want {
		t.Fatalf("SearchText() = %q, want %q", got, want)
	}
	if got, want := normalize.Digits("code 001-020"), "001020"; got != want {
		t.Fatalf("Digits() = %q, want %q", got, want)
	}
}

func TestReleasedCoreDocumentContracts(t *testing.T) {
	canonical, err := normalize.CanonicalDocument(normalize.DocumentCPF, "529.982.247-25")
	if err != nil {
		t.Fatalf("CanonicalDocument() error = %v", err)
	}
	if got, want := canonical, "52998224725"; got != want {
		t.Fatalf("CanonicalDocument() = %q, want %q", got, want)
	}

	match, err := normalize.IdentifyDocument("529.982.247-25")
	if err != nil {
		t.Fatalf("IdentifyDocument() error = %v", err)
	}
	if match.Kind != normalize.DocumentCPF || match.Canonical != "52998224725" {
		t.Fatalf("IdentifyDocument() = %+v", match)
	}

	cep, err := normalize.CanonicalCEP("01310-100")
	if err != nil {
		t.Fatalf("CanonicalCEP() error = %v", err)
	}
	if got, want := cep, "01310100"; got != want {
		t.Fatalf("CanonicalCEP() = %q, want %q", got, want)
	}
}

func TestReleasedCoreCivilDateContracts(t *testing.T) {
	date, err := civiltime.ParseCivilDate("2024-02-29")
	if err != nil {
		t.Fatalf("ParseCivilDate() error = %v", err)
	}
	if got, want := date.String(), "2024-02-29"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}

	encoded, err := json.Marshal(date)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got, want := string(encoded), `"2024-02-29"`; got != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

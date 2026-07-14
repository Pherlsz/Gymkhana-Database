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

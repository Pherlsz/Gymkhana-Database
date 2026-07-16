package bill

import (
	"errors"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

func TestM4AcceptanceBillRequiresProfileAndType(t *testing.T) {
	typeID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}

	_, err = Normalize(Values{}, TypeDefinition{
		ID: typeID,
		Values: TypeValues{
			TechnicalKey: "energy",
			Label:        "Energy",
			Active:       true,
		},
	})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertBillFieldCode(t, validation, "owner_profile_id", "required")
	assertBillFieldCode(t, validation, "bill_type_id", "invalid_value")
}

func TestM4AcceptanceBillKeepsPrintedDataAndEveryHistoricalState(t *testing.T) {
	ownerID, err := profile.NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}
	typeID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}
	definition := TypeDefinition{
		ID: typeID,
		Values: TypeValues{
			TechnicalKey: "energy",
			Label:        "Energy",
			Active:       true,
		},
	}

	for _, state := range []RecordState{RecordCurrent, RecordReplaced, RecordExpired, RecordArchived} {
		t.Run(string(state), func(t *testing.T) {
			values, normalizeErr := Normalize(Values{
				OwnerProfileID:    ownerID,
				TypeID:            typeID,
				PrintedHolderName: "Original Printed Holder",
				PrintedAddress:    "Original Address, 001",
				Reference:         "000A-99",
				Competence:        "2026-07",
				Amount:            "000123.4",
				Currency:          "brl",
				RecordState:       state,
			}, definition)
			if normalizeErr != nil {
				t.Fatal(normalizeErr)
			}
			if values.RecordState != state || values.PrintedHolderName != "Original Printed Holder" || values.PrintedAddress != "Original Address, 001" {
				t.Fatalf("printed values = %#v", values)
			}
			if values.Reference != "000A-99" || values.Competence != "2026-07" || values.Amount != "123.40" || values.Currency != "BRL" {
				t.Fatalf("canonical values = %#v", values)
			}
		})
	}
}

func TestM4AcceptanceBillRejectsMissingCurrentUseHolder(t *testing.T) {
	store := &PostgresStore{queries: &fakeBillQueries{}}
	billID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.AssignCurrentUse(t.Context(), billID, profile.Identifier{})
	if !errors.Is(err, ErrReferenceNotFound) {
		t.Fatalf("assign error = %v", err)
	}
}

func TestM4AcceptanceBillRejectsNonPositiveOptimisticVersion(t *testing.T) {
	store := &PostgresStore{queries: &fakeBillQueries{}}
	billID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Update(t.Context(), billID, 0, Values{})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("update error = %v", err)
	}
}

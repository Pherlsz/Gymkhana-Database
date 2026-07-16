package document

import (
	"errors"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

func TestM4AcceptanceDocumentRequiresProfileTypeAndIdentifier(t *testing.T) {
	typeID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}

	_, err = Normalize(Values{}, TypeDefinition{
		ID: typeID,
		Values: TypeValues{
			TechnicalKey:     "rg",
			Label:            "RG",
			Active:           true,
			UniquenessPolicy: UniquenessNone,
		},
	})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldCode(t, validation, "owner_profile_id", "required")
	assertFieldCode(t, validation, "document_type_id", "invalid_value")
	assertFieldCode(t, validation, "identifier_value", "required")
}

func TestM4AcceptanceDocumentKeepsEveryHistoricalStateRepresentable(t *testing.T) {
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
			TechnicalKey:     "registration",
			Label:            "Registration",
			Active:           true,
			UniquenessPolicy: UniquenessPerProfile,
		},
	}

	for _, state := range []RecordState{RecordCurrent, RecordReplaced, RecordExpired, RecordArchived} {
		t.Run(string(state), func(t *testing.T) {
			values, normalizeErr := Normalize(Values{
				OwnerProfileID: ownerID,
				TypeID:         typeID,
				Identifier:     "00AB-009",
				RecordState:    state,
			}, definition)
			if normalizeErr != nil {
				t.Fatal(normalizeErr)
			}
			if values.RecordState != state || values.Identifier != "00AB-009" {
				t.Fatalf("normalized values = %#v", values)
			}
		})
	}
}

func TestM4AcceptanceDocumentRejectsMissingCurrentUseHolder(t *testing.T) {
	store := &PostgresStore{queries: &fakeDocumentQueries{}}
	documentID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.AssignCurrentUse(t.Context(), documentID, profile.Identifier{})
	if !errors.Is(err, ErrReferenceNotFound) {
		t.Fatalf("assign error = %v", err)
	}
}

func TestM4AcceptanceDocumentRejectsNonPositiveOptimisticVersion(t *testing.T) {
	store := &PostgresStore{queries: &fakeDocumentQueries{}}
	documentID, err := NewIdentifier()
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Update(t.Context(), documentID, 0, Values{})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("update error = %v", err)
	}
}

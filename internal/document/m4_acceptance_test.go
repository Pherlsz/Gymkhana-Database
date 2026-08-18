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
}

func TestNormalizeAllowsEmptyIdentifierAndRejectsRefusal(t *testing.T) {
	owner, _ := profile.NewIdentifier()
	typeID, _ := NewIdentifier()
	definition := TypeDefinition{ID: typeID, Values: TypeValues{UniquenessPolicy: UniquenessNone}}
	values, err := Normalize(Values{OwnerProfileID: owner, TypeID: typeID, Identifier: "Sim", Medium: MediumPhysical}, definition)
	if err != nil || values.Identifier != "" {
		t.Fatalf("empty possession identifier: %#v %v", values, err)
	}
	_, err = Normalize(Values{OwnerProfileID: owner, TypeID: typeID, Identifier: "Não", Medium: MediumPhysical}, definition)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	assertFieldCode(t, validation, "identifier_value", "refused")
}

func TestM4AcceptanceDocumentKeepsPhysicalAndDigitalMedia(t *testing.T) {
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

	for _, medium := range []Medium{MediumPhysical, MediumDigital} {
		t.Run(string(medium), func(t *testing.T) {
			values, normalizeErr := Normalize(Values{
				OwnerProfileID: ownerID,
				TypeID:         typeID,
				Identifier:     "00AB-009",
				Medium:         medium,
			}, definition)
			if normalizeErr != nil {
				t.Fatal(normalizeErr)
			}
			if values.Medium != medium || values.Identifier != "00AB-009" {
				t.Fatalf("normalized values = %#v", values)
			}
		})
	}
}

func TestM4AcceptanceDocumentRejectsCurrentUseOnDigital(t *testing.T) {
	if OperationalStatus(MediumDigital, true, "") != "" {
		t.Fatal("digital exemplar must not expose lending status")
	}
	if OperationalStatus(MediumPhysical, false, IdleCustodyOrganization) != StatusAvailable {
		t.Fatal("physical exemplar without current use must be available")
	}
	if OperationalStatus(MediumPhysical, true, IdleCustodyOrganization) != StatusInUse {
		t.Fatal("physical exemplar with current use must be in use")
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

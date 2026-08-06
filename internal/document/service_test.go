package document

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type fakeServiceStore struct {
	types           []TypeDefinition
	documents       []Document
	typeTotal       int64
	documentTotal   int64
	typeOptions     TypeListOptions
	documentOptions ListOptions
	createdType     TypeDefinition
	createdDocument Document
	currentUse      CurrentUse
	currentUseValue *CurrentUse
	audits          []AuditEvent
	err             error
}

func (store *fakeServiceStore) CreateType(_ context.Context, id Identifier, values TypeValues) (TypeDefinition, error) {
	if store.err != nil {
		return TypeDefinition{}, store.err
	}
	store.createdType = TypeDefinition{ID: id, Values: values, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	return store.createdType, nil
}
func (store *fakeServiceStore) GetType(context.Context, Identifier) (TypeDefinition, error) {
	if store.err != nil {
		return TypeDefinition{}, store.err
	}
	if store.createdType.ID.IsZero() {
		id, _ := NewIdentifier()
		store.createdType = TypeDefinition{ID: id, Values: TypeValues{TechnicalKey: "rg", Label: "RG", Active: true, UniquenessPolicy: UniquenessNone}, Version: 1}
	}
	return store.createdType, nil
}
func (store *fakeServiceStore) CountTypes(context.Context, TypeFilters) (int64, error) {
	return store.typeTotal, store.err
}
func (store *fakeServiceStore) ListTypes(_ context.Context, options TypeListOptions) ([]TypeDefinition, error) {
	store.typeOptions = options
	return store.types, store.err
}
func (store *fakeServiceStore) UpdateType(_ context.Context, id Identifier, version int64, values TypeValues) (TypeDefinition, error) {
	if store.err != nil {
		return TypeDefinition{}, store.err
	}
	return TypeDefinition{ID: id, Values: values, Version: version + 1}, nil
}
func (store *fakeServiceStore) DeleteType(context.Context, Identifier, int64) error { return store.err }
func (store *fakeServiceStore) Create(_ context.Context, id Identifier, values Values) (Document, error) {
	if store.err != nil {
		return Document{}, store.err
	}
	definition, _ := store.GetType(context.Background(), values.TypeID)
	store.createdDocument = Document{ID: id, Values: values, Type: definition, Status: StatusAvailable, Version: 1}
	return store.createdDocument, nil
}
func (store *fakeServiceStore) Get(context.Context, Identifier) (Document, error) {
	if store.err != nil {
		return Document{}, store.err
	}
	if store.createdDocument.ID.IsZero() {
		id, _ := NewIdentifier()
		definition, _ := store.GetType(context.Background(), Identifier{})
		store.createdDocument = Document{ID: id, Type: definition, Status: StatusAvailable, Version: 1}
	}
	return store.createdDocument, nil
}
func (store *fakeServiceStore) Count(context.Context, Filters) (int64, error) {
	return store.documentTotal, store.err
}
func (store *fakeServiceStore) List(_ context.Context, options ListOptions) ([]Document, error) {
	store.documentOptions = options
	return store.documents, store.err
}
func (store *fakeServiceStore) Update(_ context.Context, id Identifier, version int64, values Values) (Document, error) {
	if store.err != nil {
		return Document{}, store.err
	}
	definition, _ := store.GetType(context.Background(), values.TypeID)
	return Document{ID: id, Values: values, Type: definition, Version: version + 1}, nil
}
func (store *fakeServiceStore) Duplicate(_ context.Context, id, _ Identifier) (Document, error) {
	if store.err != nil {
		return Document{}, store.err
	}
	definition, _ := store.GetType(context.Background(), Identifier{})
	return Document{ID: id, Type: definition, Version: 1}, nil
}
func (store *fakeServiceStore) Delete(context.Context, Identifier, int64) error { return store.err }
func (store *fakeServiceStore) AssignCurrentUse(_ context.Context, _ Identifier, holder profile.Identifier) (CurrentUse, error) {
	if store.err != nil {
		return CurrentUse{}, store.err
	}
	store.currentUse = CurrentUse{HolderProfileID: holder, AssignedAt: time.Now().UTC()}
	return store.currentUse, nil
}
func (store *fakeServiceStore) ReturnCurrentUse(context.Context, Identifier) error { return store.err }
func (store *fakeServiceStore) GetCurrentUse(context.Context, Identifier) (*CurrentUse, error) {
	return store.currentUseValue, store.err
}
func (store *fakeServiceStore) RecordAuditEvent(_ context.Context, event AuditEvent) error {
	store.audits = append(store.audits, event)
	return nil
}

func documentActor(role auth.Role) auth.Session {
	id, _ := auth.NewIdentifier()
	return auth.Session{User: auth.User{ID: id, Login: "user", Role: role, Active: true}}
}

func TestServiceNormalizesDocumentAndTypeLists(t *testing.T) {
	owner, _ := profile.NewIdentifier()
	store := &fakeServiceStore{typeTotal: 2, documentTotal: 3}
	service, err := NewService(store, ServiceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ListTypes(context.Background(), documentActor(auth.RoleExternal), TypeListOptions{Limit: 250, SortField: TypeSortUpdatedAt, SortOrder: SortDescending, Filters: TypeFilters{Label: "  rg  "}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.List(context.Background(), documentActor(auth.RoleExternal), ListOptions{Limit: 500, SortField: SortUpdatedAt, SortOrder: SortDescending, Filters: Filters{OwnerProfileID: &owner, Identifier: "  00AB  ", Status: StatusAvailable}})
	if err != nil {
		t.Fatal(err)
	}
	if store.typeOptions.Filters.Label != "rg" || store.typeOptions.SortField != TypeSortUpdatedAt {
		t.Fatalf("type options = %#v", store.typeOptions)
	}
	if store.documentOptions.Filters.Identifier != "00ab" || store.documentOptions.SortField != SortUpdatedAt {
		t.Fatalf("document options = %#v", store.documentOptions)
	}
}

func TestServiceProtectsTypeAdministrationAndPermanentDelete(t *testing.T) {
	store := &fakeServiceStore{}
	service, _ := NewService(store, ServiceOptions{})
	_, err := service.CreateType(context.Background(), documentActor(auth.RoleExternal), TypeValues{TechnicalKey: "rg", Label: "RG", Active: true, UniquenessPolicy: UniquenessNone}, "req-denied")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("member create type error = %v", err)
	}
	admin := documentActor(auth.RoleAdmin)
	created, err := service.CreateType(context.Background(), admin, TypeValues{TechnicalKey: "rg", Label: "RG", Active: true, UniquenessPolicy: UniquenessNone}, "req-create")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(context.Background(), documentActor(auth.RoleExternal), created.ID, 1, DeleteConfirmation, "req-delete"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member delete error = %v", err)
	}
	if err := service.DeleteType(context.Background(), admin, created.ID, 1, "confirmar", "req-invalid"); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("invalid confirmation = %v", err)
	}
	if len(store.audits) < 3 || store.audits[1].Outcome != auth.AuditOutcomeSuccess {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceAuditsAssignAndReturnCurrentUse(t *testing.T) {
	store := &fakeServiceStore{}
	service, _ := NewService(store, ServiceOptions{})
	holder, _ := profile.NewIdentifier()
	docID, _ := NewIdentifier()
	if _, err := service.AssignCurrentUse(context.Background(), documentActor(auth.RoleExternal), docID, holder, "req-assign"); err != nil {
		t.Fatal(err)
	}
	use := store.currentUse
	store.currentUseValue = &use
	if err := service.ReturnCurrentUse(context.Background(), documentActor(auth.RoleExternal), docID, "req-return"); err != nil {
		t.Fatal(err)
	}
	if got := store.audits[len(store.audits)-1]; got.EventType != AuditEventUseReturned || got.Outcome != auth.AuditOutcomeSuccess || got.HolderProfileID == nil {
		t.Fatalf("last audit = %#v", got)
	}
}

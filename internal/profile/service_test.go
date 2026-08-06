package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type fakeServiceStore struct {
	profiles []Profile
	total    int64
	created  Profile
	updated  Profile
	copied   Profile
	got      Profile
	list     ListOptions
	filters  Filters
	audits   []AuditEvent
	err      error
}

func (store *fakeServiceStore) Create(_ context.Context, id Identifier, values Values) (Profile, error) {
	if store.err != nil {
		return Profile{}, store.err
	}
	store.created = Profile{ID: id, Values: values, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	return store.created, nil
}
func (store *fakeServiceStore) Get(context.Context, Identifier) (Profile, error) {
	return store.got, store.err
}
func (store *fakeServiceStore) Count(_ context.Context, filters Filters) (int64, error) {
	store.filters = filters
	return store.total, store.err
}
func (store *fakeServiceStore) List(_ context.Context, options ListOptions) ([]Profile, error) {
	store.list = options
	return store.profiles, store.err
}
func (store *fakeServiceStore) Update(_ context.Context, id Identifier, version int64, values Values) (Profile, error) {
	if store.err != nil {
		return Profile{}, store.err
	}
	store.updated = Profile{ID: id, Values: values, Version: version + 1}
	return store.updated, nil
}
func (store *fakeServiceStore) Duplicate(_ context.Context, id, _ Identifier) (Profile, error) {
	if store.err != nil {
		return Profile{}, store.err
	}
	store.copied = Profile{ID: id, Values: Values{FullName: "Cópia"}, Version: 1}
	return store.copied, nil
}
func (store *fakeServiceStore) Delete(context.Context, Identifier, int64) error { return store.err }
func (store *fakeServiceStore) RecordAuditEvent(_ context.Context, event AuditEvent) error {
	store.audits = append(store.audits, event)
	return nil
}

func profileActor(role auth.Role) auth.Session {
	id, _ := auth.NewIdentifier()
	return auth.Session{User: auth.User{ID: id, Email: "user", Role: role, Active: true}}
}

func TestServiceListsWithNormalizedOptions(t *testing.T) {
	store := &fakeServiceStore{total: 2, profiles: []Profile{{Values: Values{FullName: "Ana"}}}}
	service, err := NewService(store, ServiceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.List(context.Background(), profileActor(auth.RoleExternal), ListOptions{
		Limit: 200, SortField: SortUpdatedAt, SortOrder: SortDescending,
		Filters: Filters{FullName: "  ÁNA  ", CPF: "12.3", Email: " TEST@EXAMPLE.COM ", State: "rs"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Limit != 200 || store.list.SortField != SortUpdatedAt || store.list.SortOrder != SortDescending {
		t.Fatalf("page = %#v, options = %#v", page, store.list)
	}
	if store.filters.CPF != "123" || store.filters.Email != "test@example.com" || store.filters.State != "RS" {
		t.Fatalf("filters = %#v", store.filters)
	}
}

func TestServiceAuditsMutationsAndProtectsDelete(t *testing.T) {
	store := &fakeServiceStore{}
	service, _ := NewService(store, ServiceOptions{})
	member := profileActor(auth.RoleExternal)
	created, err := service.Create(context.Background(), member, Values{FullName: "Ana"}, "req-create")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == (Identifier{}) || len(store.audits) != 1 || store.audits[0].EventType != AuditEventCreated || store.audits[0].RequestID != "req-create" {
		t.Fatalf("created = %#v, audits = %#v", created, store.audits)
	}
	if err := service.Delete(context.Background(), member, created.ID, 1, DeleteConfirmation, "req-delete"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member delete error = %v", err)
	}
	admin := profileActor(auth.RoleAdmin)
	if err := service.Delete(context.Background(), admin, created.ID, 1, "confirmar", "req-invalid"); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("invalid confirmation error = %v", err)
	}
	if err := service.Delete(context.Background(), admin, created.ID, 1, DeleteConfirmation, "req-success"); err != nil {
		t.Fatal(err)
	}
	if got := store.audits[len(store.audits)-1]; got.EventType != AuditEventDeleted || got.Outcome != auth.AuditOutcomeSuccess {
		t.Fatalf("last audit = %#v", got)
	}
}

func TestServiceReportsValidationAsDeniedAudit(t *testing.T) {
	store := &fakeServiceStore{err: &ValidationError{Fields: []FieldError{{Field: "full_name", Code: "required"}}}}
	service, _ := NewService(store, ServiceOptions{})
	_, err := service.Create(context.Background(), profileActor(auth.RoleExternal), Values{}, "req")
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v", err)
	}
	if len(store.audits) != 1 || store.audits[0].Outcome != auth.AuditOutcomeDenied {
		t.Fatalf("audits = %#v", store.audits)
	}
}

package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type fakeStore struct {
	fields       []FieldDefinition
	results      []Result
	total        int64
	plan         Plan
	ids          []string
	suggestions  []SuggestHit
	reservations int
	reserveErr   error
	executeErr   error
	waitForDone  bool
}

func (store *fakeStore) ListDynamicFields(context.Context) ([]FieldDefinition, error) {
	return store.fields, nil
}

func (store *fakeStore) ReserveRateLimit(context.Context, auth.Identifier, time.Time, int) error {
	store.reservations++
	return store.reserveErr
}

func (store *fakeStore) Execute(ctx context.Context, plan Plan) ([]Result, int64, error) {
	store.plan = plan
	if store.waitForDone {
		<-ctx.Done()
		return nil, 0, ctx.Err()
	}
	return store.results, store.total, store.executeErr
}

func (store *fakeStore) MatchIDs(ctx context.Context, plan Plan, _ Module) ([]string, error) {
	store.plan = plan
	if store.waitForDone {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if store.executeErr != nil {
		return nil, store.executeErr
	}
	return store.ids, nil
}

func (store *fakeStore) MatchProfileHits(ctx context.Context, plan Plan) ([]ProfileHit, error) {
	store.plan = plan
	if store.waitForDone {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if store.executeErr != nil {
		return nil, store.executeErr
	}
	return nil, nil
}

func (store *fakeStore) Suggest(context.Context, SuggestQuery) ([]SuggestHit, error) {
	return store.suggestions, store.executeErr
}

func searchActor(t *testing.T, active bool) auth.Session {
	t.Helper()
	id, err := auth.NewIdentifier()
	if err != nil {
		t.Fatalf("auth.NewIdentifier() error = %v", err)
	}
	return auth.Session{User: auth.User{ID: id, Role: auth.RoleExternal, Active: active}}
}

func TestCatalogIsLogicalPermissionFilteredAndDynamic(t *testing.T) {
	store := &fakeStore{fields: []FieldDefinition{
		{Key: "custom.11111111-1111-1111-1111-111111111111", Module: ModuleCustomData, Group: ModuleProfiles, Label: "Equipe · Pessoa", Kind: "text"},
		{Key: "custom.not-a-uuid", Module: ModuleCustomData, Label: "Invalid", Kind: "text"},
		{Key: "physical.secret", Module: Module("physical_table"), Label: "Hidden", Kind: "text"},
	}}
	service, err := NewService(store, ServiceOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	catalog, err := service.Catalog(context.Background(), searchActor(t, true))
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if len(catalog.Modules) != 5 || catalog.Limits.MaximumTerms != MaxTerms {
		t.Fatalf("catalog = %#v", catalog)
	}
	foundDynamic := false
	for _, field := range catalog.Fields {
		if field.Key == "profile.cpf" {
			t.Fatal("catalog must not expose profile.cpf")
		}
		if field.Key == "physical.secret" || field.Key == "custom.not-a-uuid" {
			t.Fatal("catalog exposed a non-allowlisted logical field")
		}
		if field.Key == "custom.11111111-1111-1111-1111-111111111111" {
			foundDynamic = true
		}
	}
	if !foundDynamic {
		t.Fatal("catalog omitted an authorized dynamic field")
	}
	if _, err := service.Catalog(context.Background(), searchActor(t, false)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("inactive Catalog() error = %v", err)
	}
}

func TestSearchBuildsBoundedLiteralDeterministicPlan(t *testing.T) {
	store := &fakeStore{results: []Result{{
		Module: ModuleProfiles, EntityKind: "profile", EntityID: "profile-id", FieldKey: "profile.full_name",
		TargetKind: "profile", TargetID: "profile-id", EntityLabel: "Ana", FieldLabel: "Nome completo",
	}}, total: 1}
	now := time.Date(2026, 7, 17, 12, 34, 56, 0, time.UTC)
	service, err := NewService(store, ServiceOptions{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	page, err := service.Search(context.Background(), searchActor(t, true), Query{
		Terms:   []string{" 001 ", `50%_\`, "001"},
		Modules: []Module{ModuleProfiles, ModuleDocuments},
		Fields:  []string{"profile.full_name", "document.identifier"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if page.Total != 1 || page.Limit != 50 || page.Sort != SortRelevance || page.Order != SortDescending {
		t.Fatalf("page = %#v", page)
	}
	if store.reservations != 1 || len(store.plan.Terms) != 2 || store.plan.Terms[0] != "001" || len(store.plan.Includes) < 2 {
		t.Fatalf("plan = %#v, reservations = %d", store.plan, store.reservations)
	}
	if got, want := store.plan.Includes[1].Pattern, `%50\%\_\\%`; got != want {
		t.Fatalf("literal pattern = %q, want %q", got, want)
	}
	if store.plan.StatementTimeout != 2*time.Second {
		t.Fatalf("statement timeout = %s", store.plan.StatementTimeout)
	}
	if store.plan.CandidateLimit != MaxResultCardinality+1 {
		t.Fatalf("candidate limit = %d", store.plan.CandidateLimit)
	}
}

func TestSearchRejectsUnknownLogicalIdentifiersBeforeReservation(t *testing.T) {
	store := &fakeStore{}
	service, _ := NewService(store, ServiceOptions{})
	_, err := service.Search(context.Background(), searchActor(t, true), Query{
		Terms:  []string{"value"},
		Fields: []string{"profiles.full_name; DROP TABLE profiles"},
	})
	var validation *ValidationError
	if !errors.As(err, &validation) || store.reservations != 0 {
		t.Fatalf("Search() error = %v, reservations = %d", err, store.reservations)
	}
}

func TestSearchWithoutExplicitSelectionSearchesEveryAuthorizedField(t *testing.T) {
	store := &fakeStore{fields: oversizedFieldSet(t)}
	service, _ := NewService(store, ServiceOptions{})
	_, err := service.Search(context.Background(), searchActor(t, true), Query{Terms: []string{"value"}})
	if err != nil || store.reservations != 1 {
		t.Fatalf("Search() error = %v, reservations = %d", err, store.reservations)
	}
}

func TestSearchRejectsExplicitFieldSelectionsBeyondTheLimit(t *testing.T) {
	fields := oversizedFieldSet(t)
	keys := make([]string, 0, len(fields)+len(staticFields))
	for _, field := range fields {
		keys = append(keys, field.Key)
	}
	for _, field := range staticFields {
		keys = append(keys, field.Key)
	}
	if len(keys) <= MaxFields {
		t.Fatalf("setup produced %d fields, need more than %d", len(keys), MaxFields)
	}
	store := &fakeStore{fields: fields}
	service, _ := NewService(store, ServiceOptions{})
	_, err := service.Search(context.Background(), searchActor(t, true), Query{
		Terms:  []string{"value"},
		Fields: keys,
	})
	var validation *ValidationError
	if !errors.As(err, &validation) || store.reservations != 0 {
		t.Fatalf("Search() error = %v, reservations = %d", err, store.reservations)
	}
}

func oversizedFieldSet(t *testing.T) []FieldDefinition {
	t.Helper()
	dynamicCount := MaxFields - len(staticFields) + 1
	if dynamicCount < 1 {
		dynamicCount = 1
	}
	fields := make([]FieldDefinition, 0, dynamicCount)
	for range dynamicCount {
		identifier, err := auth.NewIdentifier()
		if err != nil {
			t.Fatalf("auth.NewIdentifier() error = %v", err)
		}
		fields = append(fields, FieldDefinition{
			Key: "custom." + identifier.String(), Module: ModuleCustomData, Group: ModuleCustomData, Label: "Campo", Kind: "text",
		})
	}
	return fields
}

func TestSearchRejectsStoreResultsOutsideAuthorizedCatalog(t *testing.T) {
	store := &fakeStore{results: []Result{{
		Module: ModuleProfiles, EntityKind: "physical_table", EntityID: "profile-id",
		FieldKey: "profile.full_name", FieldLabel: "Nome completo", EntityLabel: "Ana",
		TargetKind: "profile", TargetID: "profile-id",
	}}}
	service, _ := NewService(store, ServiceOptions{})
	_, err := service.Search(context.Background(), searchActor(t, true), Query{Terms: []string{"value"}})
	if !errors.Is(err, ErrUnsafeResult) {
		t.Fatalf("Search() error = %v, want ErrUnsafeResult", err)
	}
}

func TestSearchEnforcesCostRateAndTimeoutLimits(t *testing.T) {
	actor := searchActor(t, true)

	costStore := &fakeStore{}
	costService, _ := NewService(costStore, ServiceOptions{MaximumCost: 1})
	if _, err := costService.Search(context.Background(), actor, Query{Terms: []string{"a"}}); !errors.Is(err, ErrCostLimit) {
		t.Fatalf("cost-limited Search() error = %v", err)
	}
	if costStore.reservations != 0 {
		t.Fatal("cost-limited query consumed rate quota")
	}

	defaultCostStore := &fakeStore{}
	defaultCostService, _ := NewService(defaultCostStore, ServiceOptions{})
	if _, err := defaultCostService.Search(context.Background(), actor, Query{Terms: []string{"a", "b"}, Offset: MaxOffset}); !errors.Is(err, ErrCostLimit) {
		t.Fatalf("default cost-limited Search() error = %v", err)
	}

	cardinalityStore := &fakeStore{total: MaxResultCardinality + 1}
	cardinalityService, _ := NewService(cardinalityStore, ServiceOptions{})
	if _, err := cardinalityService.Search(context.Background(), actor, Query{Terms: []string{"a"}}); !errors.Is(err, ErrCardinalityLimit) {
		t.Fatalf("cardinality-limited Search() error = %v", err)
	}

	rateStore := &fakeStore{reserveErr: ErrRateLimited}
	rateService, _ := NewService(rateStore, ServiceOptions{})
	if _, err := rateService.Search(context.Background(), actor, Query{Terms: []string{"a"}}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate-limited Search() error = %v", err)
	}

	timeoutStore := &fakeStore{waitForDone: true}
	timeoutService, _ := NewService(timeoutStore, ServiceOptions{Timeout: time.Millisecond})
	if _, err := timeoutService.Search(context.Background(), actor, Query{Terms: []string{"a"}}); !errors.Is(err, ErrQueryTimeout) {
		t.Fatalf("timed-out Search() error = %v", err)
	}
}

func TestNewServiceRejectsUnsafeOptions(t *testing.T) {
	if _, err := NewService(nil, ServiceOptions{}); !errors.Is(err, ErrInvalidServiceSetup) {
		t.Fatalf("nil store error = %v", err)
	}
	if _, err := NewService(&fakeStore{}, ServiceOptions{Timeout: 11 * time.Second}); !errors.Is(err, ErrInvalidServiceSetup) {
		t.Fatalf("unsafe timeout error = %v", err)
	}
}

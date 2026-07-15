package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeProfileQueries struct {
	createdParams    dbgen.CreateProfileParams
	updatedParams    dbgen.UpdateProfileParams
	duplicatedParams dbgen.DuplicateProfileParams
	deletedParams    dbgen.DeleteProfileParams
	countedParams    dbgen.CountProfilesParams
	listedParams     dbgen.ListProfilesParams
	auditParams      dbgen.RecordProfileAuditEventParams
	profile          dbgen.Profile
	profiles         []dbgen.Profile
	count            int64
	createErr        error
	getErr           error
	countErr         error
	listErr          error
	updateErr        error
	duplicateErr     error
	deleteErr        error
	auditErr         error
}

func (queries *fakeProfileQueries) CreateProfile(_ context.Context, params dbgen.CreateProfileParams) (dbgen.Profile, error) {
	queries.createdParams = params
	return queries.profile, queries.createErr
}
func (queries *fakeProfileQueries) GetProfileByID(context.Context, pgtype.UUID) (dbgen.Profile, error) {
	return queries.profile, queries.getErr
}
func (queries *fakeProfileQueries) CountProfiles(_ context.Context, params dbgen.CountProfilesParams) (int64, error) {
	queries.countedParams = params
	return queries.count, queries.countErr
}
func (queries *fakeProfileQueries) ListProfiles(_ context.Context, params dbgen.ListProfilesParams) ([]dbgen.Profile, error) {
	queries.listedParams = params
	return queries.profiles, queries.listErr
}
func (queries *fakeProfileQueries) UpdateProfile(_ context.Context, params dbgen.UpdateProfileParams) (dbgen.Profile, error) {
	queries.updatedParams = params
	return queries.profile, queries.updateErr
}
func (queries *fakeProfileQueries) DuplicateProfile(_ context.Context, params dbgen.DuplicateProfileParams) (dbgen.Profile, error) {
	queries.duplicatedParams = params
	return queries.profile, queries.duplicateErr
}
func (queries *fakeProfileQueries) DeleteProfile(_ context.Context, params dbgen.DeleteProfileParams) (pgtype.UUID, error) {
	queries.deletedParams = params
	return params.ID, queries.deleteErr
}
func (queries *fakeProfileQueries) RecordProfileAuditEvent(_ context.Context, params dbgen.RecordProfileAuditEventParams) error {
	queries.auditParams = params
	return queries.auditErr
}

func TestPostgresStoreCreatesNormalizedProfile(t *testing.T) {
	id, _ := NewIdentifier()
	createdAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	queries := &fakeProfileQueries{profile: databaseProfile(id, createdAt)}
	store := &PostgresStore{queries: queries}

	value, err := store.Create(context.Background(), id, Values{
		FullName: "  Ana   da Silva ", CPF: "529.982.247-25", Email: "ANA@EXAMPLE.COM",
		MobilePhone: "(51) 99999-8888", Address: Address{State: "rs", PostalCode: "90000-000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if queries.createdParams.FullName != "Ana da Silva" || pointerValue(queries.createdParams.Email) != "ana@example.com" || value.ID != id {
		t.Fatalf("params = %#v, profile = %#v", queries.createdParams, value)
	}
}

func TestPostgresStorePassesFiltersSortingAndPagination(t *testing.T) {
	id, _ := NewIdentifier()
	queries := &fakeProfileQueries{count: 1, profiles: []dbgen.Profile{databaseProfile(id, time.Now())}}
	store := &PostgresStore{queries: queries}
	filters := Filters{FullName: "ana", CPF: "123", Email: "mail", City: "porto", State: "RS"}
	count, err := store.Count(context.Background(), filters)
	if err != nil || count != 1 {
		t.Fatalf("count = %d, error = %v", count, err)
	}
	values, err := store.List(context.Background(), ListOptions{Limit: 250, Offset: 10, SortField: SortUpdatedAt, SortOrder: SortDescending, Filters: filters})
	if err != nil || len(values) != 1 {
		t.Fatalf("values = %#v, error = %v", values, err)
	}
	if queries.countedParams.CpfFilter != "123" || queries.listedParams.PageLimit != 250 || queries.listedParams.SortField != "updated_at" || queries.listedParams.SortOrder != "desc" {
		t.Fatalf("count params = %#v, list params = %#v", queries.countedParams, queries.listedParams)
	}
}

func TestPostgresStoreClassifiesOptimisticWriteFailures(t *testing.T) {
	id, _ := NewIdentifier()
	queries := &fakeProfileQueries{profile: databaseProfile(id, time.Now()), updateErr: pgx.ErrNoRows}
	store := &PostgresStore{queries: queries}
	if _, err := store.Update(context.Background(), id, 1, Values{FullName: "Profile"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("update error = %v", err)
	}
	queries.getErr = pgx.ErrNoRows
	if _, err := store.Update(context.Background(), id, 1, Values{FullName: "Profile"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing update error = %v", err)
	}
}

func TestPostgresStoreRecordsProfileAudit(t *testing.T) {
	actor, _ := authIdentifier()
	profileID, _ := NewIdentifier()
	sourceID, _ := NewIdentifier()
	eventID, _ := NewIdentifier()
	queries := &fakeProfileQueries{}
	store := &PostgresStore{queries: queries}
	if err := store.RecordAuditEvent(context.Background(), AuditEvent{
		ID: eventID, ActorUserID: actor, ProfileID: profileID, SourceProfileID: &sourceID,
		EventType: AuditEventDuplicated, Outcome: "SUCCESS", RequestID: "request-id",
	}); err != nil {
		t.Fatal(err)
	}
	if queries.auditParams.ProfileID.Bytes != profileID || !queries.auditParams.SourceProfileID.Valid || queries.auditParams.RequestID != "request-id" {
		t.Fatalf("audit params = %#v", queries.auditParams)
	}
}

func authIdentifier() (auth.Identifier, error) {
	value, err := auth.NewIdentifier()
	return value, err
}

func databaseProfile(id Identifier, createdAt time.Time) dbgen.Profile {
	socialName := "Ana"
	cpf := "52998224725"
	email := "ana@example.com"
	mobile := "+5551999998888"
	landline := "+555133334444"
	street := "Rua Principal"
	number := "123"
	complement := "Apto 4"
	neighborhood := "Centro"
	city := "Porto Alegre"
	state := "RS"
	postalCode := "90000000"
	notes := "Observação"
	return dbgen.Profile{
		ID: databaseUUID(id), FullName: "Ana da Silva", SocialName: &socialName, Cpf: &cpf, Email: &email,
		MobilePhone: &mobile, LandlinePhone: &landline, AddressStreet: &street, AddressNumber: &number,
		AddressComplement: &complement, AddressNeighborhood: &neighborhood, AddressCity: &city,
		AddressState: &state, AddressPostalCode: &postalCode, Notes: &notes, Version: 1,
		CreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: createdAt.Add(time.Minute), Valid: true},
	}
}

func pointerValue(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

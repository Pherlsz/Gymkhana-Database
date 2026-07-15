package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeProfileQueries struct {
	createdParams    dbgen.CreateProfileParams
	updatedParams    dbgen.UpdateProfileParams
	duplicatedParams dbgen.DuplicateProfileParams
	deletedParams    dbgen.DeleteProfileParams
	listedParams     dbgen.ListProfilesParams
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
}

func (queries *fakeProfileQueries) CreateProfile(_ context.Context, params dbgen.CreateProfileParams) (dbgen.Profile, error) {
	queries.createdParams = params
	return queries.profile, queries.createErr
}

func (queries *fakeProfileQueries) GetProfileByID(context.Context, pgtype.UUID) (dbgen.Profile, error) {
	return queries.profile, queries.getErr
}

func (queries *fakeProfileQueries) CountProfiles(context.Context) (int64, error) {
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

func TestPostgresStoreCreatesNormalizedProfile(t *testing.T) {
	id, _ := NewIdentifier()
	createdAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	queries := &fakeProfileQueries{profile: databaseProfile(id, createdAt)}
	store := &PostgresStore{queries: queries}

	profile, err := store.Create(context.Background(), id, Values{
		FullName:    "  Ana   da Silva ",
		CPF:         "529.982.247-25",
		Email:       "ANA@EXAMPLE.COM",
		MobilePhone: "(51) 99999-8888",
		Address: Address{
			State:      "rs",
			PostalCode: "90000-000",
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if queries.createdParams.FullName != "Ana da Silva" || value(queries.createdParams.Email) != "ana@example.com" {
		t.Fatalf("params = %#v", queries.createdParams)
	}
	if value(queries.createdParams.Cpf) != "52998224725" || value(queries.createdParams.MobilePhone) != "+5551999998888" {
		t.Fatalf("params = %#v", queries.createdParams)
	}
	if value(queries.createdParams.AddressState) != "RS" || value(queries.createdParams.AddressPostalCode) != "90000000" {
		t.Fatalf("params = %#v", queries.createdParams)
	}
	if queries.createdParams.SocialName != nil || queries.createdParams.LandlinePhone != nil {
		t.Fatalf("optional values = %#v", queries.createdParams)
	}
	if profile.ID != id || profile.Version != 1 || profile.CreatedAt != createdAt {
		t.Fatalf("profile = %#v", profile)
	}
}

func TestPostgresStoreRejectsInvalidValuesBeforeWriting(t *testing.T) {
	id, _ := NewIdentifier()
	queries := &fakeProfileQueries{}
	store := &PostgresStore{queries: queries}

	_, err := store.Create(context.Background(), id, Values{FullName: ""})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("Create() error = %T %v", err, err)
	}
	if queries.createdParams.ID.Valid {
		t.Fatalf("unexpected database write = %#v", queries.createdParams)
	}
}

func TestPostgresStoreClassifiesOptimisticWriteFailures(t *testing.T) {
	id, _ := NewIdentifier()
	values := Values{FullName: "Profile"}

	t.Run("conflict", func(t *testing.T) {
		queries := &fakeProfileQueries{
			profile:   databaseProfile(id, time.Now()),
			updateErr: pgx.ErrNoRows,
		}
		store := &PostgresStore{queries: queries}
		_, err := store.Update(context.Background(), id, 1, values)
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("Update() error = %v, want %v", err, ErrConflict)
		}
	})

	t.Run("not found", func(t *testing.T) {
		queries := &fakeProfileQueries{updateErr: pgx.ErrNoRows, getErr: pgx.ErrNoRows}
		store := &PostgresStore{queries: queries}
		_, err := store.Update(context.Background(), id, 1, values)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Update() error = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("delete conflict", func(t *testing.T) {
		queries := &fakeProfileQueries{
			profile:   databaseProfile(id, time.Now()),
			deleteErr: pgx.ErrNoRows,
		}
		store := &PostgresStore{queries: queries}
		if err := store.Delete(context.Background(), id, 1); !errors.Is(err, ErrConflict) {
			t.Fatalf("Delete() error = %v, want %v", err, ErrConflict)
		}
	})
}

func TestPostgresStoreListsWithBoundedPagination(t *testing.T) {
	id, _ := NewIdentifier()
	queries := &fakeProfileQueries{profiles: []dbgen.Profile{databaseProfile(id, time.Now())}}
	store := &PostgresStore{queries: queries}

	profiles, err := store.List(context.Background(), 5000, -10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if queries.listedParams.PageLimit != 100 || queries.listedParams.PageOffset != 0 {
		t.Fatalf("params = %#v", queries.listedParams)
	}
	if len(profiles) != 1 || profiles[0].ID != id {
		t.Fatalf("profiles = %#v", profiles)
	}
}

func TestPostgresStoreDuplicatesAndDeletes(t *testing.T) {
	sourceID, _ := NewIdentifier()
	newID, _ := NewIdentifier()
	queries := &fakeProfileQueries{profile: databaseProfile(newID, time.Now())}
	store := &PostgresStore{queries: queries}

	duplicated, err := store.Duplicate(context.Background(), newID, sourceID)
	if err != nil {
		t.Fatalf("Duplicate() error = %v", err)
	}
	if duplicated.ID != newID || queries.duplicatedParams.NewID.Bytes != newID || queries.duplicatedParams.SourceID.Bytes != sourceID {
		t.Fatalf("duplicated = %#v, params = %#v", duplicated, queries.duplicatedParams)
	}
	if err := store.Delete(context.Background(), newID, 1); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if queries.deletedParams.ID.Bytes != newID || queries.deletedParams.Version != 1 {
		t.Fatalf("delete params = %#v", queries.deletedParams)
	}
}

func TestProfileFromDatabaseRejectsInvalidRequiredMetadata(t *testing.T) {
	id, _ := NewIdentifier()
	value := databaseProfile(id, time.Now())
	value.CreatedAt.Valid = false
	if _, err := profileFromDatabase(value); err == nil {
		t.Fatal("profileFromDatabase() error = nil")
	}

	value = databaseProfile(id, time.Now())
	value.ID.Valid = false
	if _, err := profileFromDatabase(value); err == nil {
		t.Fatal("profileFromDatabase() identifier error = nil")
	}
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
		ID:                  databaseUUID(id),
		FullName:            "Ana da Silva",
		SocialName:          &socialName,
		Cpf:                 &cpf,
		Email:               &email,
		MobilePhone:         &mobile,
		LandlinePhone:       &landline,
		AddressStreet:       &street,
		AddressNumber:       &number,
		AddressComplement:   &complement,
		AddressNeighborhood: &neighborhood,
		AddressCity:         &city,
		AddressState:        &state,
		AddressPostalCode:   &postalCode,
		Notes:               &notes,
		Version:             1,
		CreatedAt:           pgtype.Timestamptz{Time: createdAt, Valid: true},
		UpdatedAt:           pgtype.Timestamptz{Time: createdAt.Add(time.Minute), Valid: true},
	}
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

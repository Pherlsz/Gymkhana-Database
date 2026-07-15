package profile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound = errors.New("profile not found")
	ErrConflict = errors.New("profile changed concurrently")
)

type Store interface {
	Create(context.Context, Identifier, Values) (Profile, error)
	Get(context.Context, Identifier) (Profile, error)
	Count(context.Context, Filters) (int64, error)
	List(context.Context, ListOptions) ([]Profile, error)
	Update(context.Context, Identifier, int64, Values) (Profile, error)
	Duplicate(context.Context, Identifier, Identifier) (Profile, error)
	Delete(context.Context, Identifier, int64) error
}

type profileQueries interface {
	CreateProfile(context.Context, dbgen.CreateProfileParams) (dbgen.Profile, error)
	GetProfileByID(context.Context, pgtype.UUID) (dbgen.Profile, error)
	CountProfiles(context.Context, dbgen.CountProfilesParams) (int64, error)
	ListProfiles(context.Context, dbgen.ListProfilesParams) ([]dbgen.Profile, error)
	UpdateProfile(context.Context, dbgen.UpdateProfileParams) (dbgen.Profile, error)
	DuplicateProfile(context.Context, dbgen.DuplicateProfileParams) (dbgen.Profile, error)
	DeleteProfile(context.Context, dbgen.DeleteProfileParams) (pgtype.UUID, error)
	RecordProfileAuditEvent(context.Context, dbgen.RecordProfileAuditEventParams) error
}

type PostgresStore struct {
	queries profileQueries
}

func NewPostgresStore(database dbgen.DBTX) *PostgresStore {
	return &PostgresStore{queries: dbgen.New(database)}
}

func (store *PostgresStore) Create(ctx context.Context, id Identifier, values Values) (Profile, error) {
	normalized, err := Normalize(values)
	if err != nil {
		return Profile{}, err
	}
	value, err := store.queries.CreateProfile(ctx, createDatabaseParams(id, normalized))
	if err != nil {
		return Profile{}, fmt.Errorf("create profile: %w", err)
	}
	return profileFromDatabase(value)
}

func (store *PostgresStore) Get(ctx context.Context, id Identifier) (Profile, error) {
	value, err := store.queries.GetProfileByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return profileFromDatabase(value)
}

func (store *PostgresStore) Count(ctx context.Context, filters Filters) (int64, error) {
	count, err := store.queries.CountProfiles(ctx, dbgen.CountProfilesParams{
		FullNameFilter: filters.FullName,
		CpfFilter:      filters.CPF,
		EmailFilter:    filters.Email,
		CityFilter:     filters.City,
		StateFilter:    filters.State,
	})
	if err != nil {
		return 0, fmt.Errorf("count profiles: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) List(ctx context.Context, options ListOptions) ([]Profile, error) {
	values, err := store.queries.ListProfiles(ctx, dbgen.ListProfilesParams{
		FullNameFilter: options.Filters.FullName,
		CpfFilter:      options.Filters.CPF,
		EmailFilter:    options.Filters.Email,
		CityFilter:     options.Filters.City,
		StateFilter:    options.Filters.State,
		SortField:      string(options.SortField),
		SortOrder:      string(options.SortOrder),
		PageLimit:      options.Limit,
		PageOffset:     options.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	profiles := make([]Profile, 0, len(values))
	for _, value := range values {
		mapped, mapErr := profileFromDatabase(value)
		if mapErr != nil {
			return nil, mapErr
		}
		profiles = append(profiles, mapped)
	}
	return profiles, nil
}

func (store *PostgresStore) Update(ctx context.Context, id Identifier, version int64, values Values) (Profile, error) {
	if version <= 0 {
		return Profile{}, ErrConflict
	}
	normalized, err := Normalize(values)
	if err != nil {
		return Profile{}, err
	}
	params := updateDatabaseParams(id, version, normalized)
	value, err := store.queries.UpdateProfile(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, store.classifyMissingWrite(ctx, id)
	}
	if err != nil {
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return profileFromDatabase(value)
}

func (store *PostgresStore) Duplicate(ctx context.Context, newID, sourceID Identifier) (Profile, error) {
	value, err := store.queries.DuplicateProfile(ctx, dbgen.DuplicateProfileParams{
		NewID:    databaseUUID(newID),
		SourceID: databaseUUID(sourceID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("duplicate profile: %w", err)
	}
	return profileFromDatabase(value)
}

func (store *PostgresStore) Delete(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrConflict
	}
	_, err := store.queries.DeleteProfile(ctx, dbgen.DeleteProfileParams{
		ID:      databaseUUID(id),
		Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.classifyMissingWrite(ctx, id)
	}
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	return nil
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	params := dbgen.RecordProfileAuditEventParams{
		ID:              databaseUUID(event.ID),
		ActorUserID:     pgtype.UUID{Bytes: event.ActorUserID, Valid: true},
		ProfileID:       databaseUUID(event.ProfileID),
		SourceProfileID: optionalDatabaseUUID(event.SourceProfileID),
		EventType:       string(event.EventType),
		Outcome:         string(event.Outcome),
		RequestID:       event.RequestID,
	}
	if err := store.queries.RecordProfileAuditEvent(ctx, params); err != nil {
		return fmt.Errorf("record profile audit event: %w", err)
	}
	return nil
}

func (store *PostgresStore) classifyMissingWrite(ctx context.Context, id Identifier) error {
	_, err := store.queries.GetProfileByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify profile write: %w", err)
	}
	return ErrConflict
}

func createDatabaseParams(id Identifier, values Values) dbgen.CreateProfileParams {
	return dbgen.CreateProfileParams{
		ID:                  databaseUUID(id),
		FullName:            values.FullName,
		SocialName:          optionalString(values.SocialName),
		Cpf:                 optionalString(values.CPF),
		Email:               optionalString(values.Email),
		MobilePhone:         optionalString(values.MobilePhone),
		LandlinePhone:       optionalString(values.LandlinePhone),
		AddressStreet:       optionalString(values.Address.Street),
		AddressNumber:       optionalString(values.Address.Number),
		AddressComplement:   optionalString(values.Address.Complement),
		AddressNeighborhood: optionalString(values.Address.Neighborhood),
		AddressCity:         optionalString(values.Address.City),
		AddressState:        optionalString(values.Address.State),
		AddressPostalCode:   optionalString(values.Address.PostalCode),
		Notes:               optionalString(values.Notes),
	}
}

func updateDatabaseParams(id Identifier, version int64, values Values) dbgen.UpdateProfileParams {
	created := createDatabaseParams(id, values)
	return dbgen.UpdateProfileParams{
		FullName:            created.FullName,
		SocialName:          created.SocialName,
		Cpf:                 created.Cpf,
		Email:               created.Email,
		MobilePhone:         created.MobilePhone,
		LandlinePhone:       created.LandlinePhone,
		AddressStreet:       created.AddressStreet,
		AddressNumber:       created.AddressNumber,
		AddressComplement:   created.AddressComplement,
		AddressNeighborhood: created.AddressNeighborhood,
		AddressCity:         created.AddressCity,
		AddressState:        created.AddressState,
		AddressPostalCode:   created.AddressPostalCode,
		Notes:               created.Notes,
		ID:                  created.ID,
		Version:             version,
	}
}

func profileFromDatabase(value dbgen.Profile) (Profile, error) {
	id, err := identifierFromDatabase(value.ID)
	if err != nil {
		return Profile{}, err
	}
	createdAt, err := timeFromDatabase(value.CreatedAt, "created_at")
	if err != nil {
		return Profile{}, err
	}
	updatedAt, err := timeFromDatabase(value.UpdatedAt, "updated_at")
	if err != nil {
		return Profile{}, err
	}
	values, err := Normalize(Values{
		FullName:      value.FullName,
		SocialName:    stringValue(value.SocialName),
		CPF:           stringValue(value.Cpf),
		Email:         stringValue(value.Email),
		MobilePhone:   stringValue(value.MobilePhone),
		LandlinePhone: stringValue(value.LandlinePhone),
		Address: Address{
			Street:       stringValue(value.AddressStreet),
			Number:       stringValue(value.AddressNumber),
			Complement:   stringValue(value.AddressComplement),
			Neighborhood: stringValue(value.AddressNeighborhood),
			City:         stringValue(value.AddressCity),
			State:        stringValue(value.AddressState),
			PostalCode:   stringValue(value.AddressPostalCode),
		},
		Notes: stringValue(value.Notes),
	})
	if err != nil {
		return Profile{}, fmt.Errorf("map profile from database: %w", err)
	}
	return Profile{
		ID:        id,
		Values:    values,
		Version:   value.Version,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func databaseUUID(identifier Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: identifier, Valid: true}
}

func optionalDatabaseUUID(identifier *Identifier) pgtype.UUID {
	if identifier == nil {
		return pgtype.UUID{}
	}
	return databaseUUID(*identifier)
}

func identifierFromDatabase(value pgtype.UUID) (Identifier, error) {
	if !value.Valid {
		return Identifier{}, errors.New("profile database identifier is invalid")
	}
	return Identifier(value.Bytes), nil
}

func timeFromDatabase(value pgtype.Timestamptz, field string) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("profile database %s is invalid", field)
	}
	return value.Time.UTC(), nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ Store = (*PostgresStore)(nil)
var _ AuditStore = (*PostgresStore)(nil)

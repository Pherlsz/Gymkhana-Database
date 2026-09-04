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
	ListByExactFullName(context.Context, string) ([]Profile, error)
	DistinctCities(context.Context, Filters, int32) ([]string, error)
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
	ListProfilesByExactFullName(context.Context, string) ([]dbgen.Profile, error)
	ListDistinctCities(context.Context, dbgen.ListDistinctCitiesParams) ([]*string, error)
	UpsertCPFPresence(context.Context, dbgen.UpsertCPFPresenceParams) (dbgen.DocumentPresence, error)
	ClearCPFPresenceNumber(context.Context, pgtype.UUID) error
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
	if err := store.syncCPFPresence(ctx, id, normalized.CPF); err != nil {
		return Profile{}, err
	}
	mapped, err := profileFromDatabase(value)
	if err != nil {
		return Profile{}, err
	}
	mapped.Values.CPF = normalized.CPF
	return mapped, nil
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
		RestrictIds:    filters.RestrictIDs,
		IDFilter:       uuidList(filters.IDFilter),
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
		RestrictIds:    options.Filters.RestrictIDs,
		IDFilter:       uuidList(options.Filters.IDFilter),
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

func (store *PostgresStore) ListByExactFullName(ctx context.Context, fullName string) ([]Profile, error) {
	values, err := store.queries.ListProfilesByExactFullName(ctx, fullName)
	if err != nil {
		return nil, fmt.Errorf("list profiles by exact full name: %w", err)
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

func (store *PostgresStore) DistinctCities(ctx context.Context, filters Filters, limit int32) ([]string, error) {
	values, err := store.queries.ListDistinctCities(ctx, dbgen.ListDistinctCitiesParams{
		FullNameFilter: filters.FullName,
		CpfFilter:      filters.CPF,
		EmailFilter:    filters.Email,
		StateFilter:    filters.State,
		ValueLimit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list distinct cities: %w", err)
	}
	cities := make([]string, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		cities = append(cities, *value)
	}
	return cities, nil
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
	if err := store.syncCPFPresence(ctx, id, normalized.CPF); err != nil {
		return Profile{}, err
	}
	mapped, err := profileFromDatabase(value)
	if err != nil {
		return Profile{}, err
	}
	mapped.Values.CPF = normalized.CPF
	return mapped, nil
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
		BirthDate:           optionalDate(values.BirthDate),
		Gender:              optionalString(values.Gender),
		BloodType:           optionalString(values.BloodType),
		Nationality:         optionalString(values.Nationality),
		BirthCity:           optionalString(values.BirthCity),
		MaritalStatus:       optionalString(values.MaritalStatus),
		WeddingDate:         optionalDate(values.WeddingDate),
		FatherName:          optionalString(values.FatherName),
		FatherBirthDate:     optionalDate(values.FatherBirthDate),
		MotherName:          optionalString(values.MotherName),
		MotherBirthDate:     optionalDate(values.MotherBirthDate),
		HealthPlan:          optionalString(values.HealthPlan),
		BloodDonor:          values.BloodDonor,
		OrganDonor:          values.OrganDonor,
		Team:                optionalString(values.Team),
		Sector:              optionalString(values.Sector),
		Collections:         optionalString(values.Collections),
		VehicleModel:        optionalString(values.VehicleModel),
		VehicleColor:        optionalString(values.VehicleColor),
		VehiclePlate:        optionalString(values.VehiclePlate),
		VehicleYear:         values.VehicleYear,
		ClubMembership:      optionalString(values.ClubMembership),
		MembershipType:      optionalString(values.MembershipType),
		PlaceOfOrigin:       optionalString(values.PlaceOfOrigin),
		BirthCountry:        optionalString(values.BirthCountry),
		ParentsWeddingDate:  optionalDate(values.ParentsWedding),
		SupermarketClub:     optionalString(values.SupermarketClub),
		Pet:                 optionalString(values.Pet),
		TravelCountries:     optionalString(values.TravelCountries),
		CardBrand:           optionalString(values.CardBrand),
		CardBank:            optionalString(values.CardBank),
	}
}

func updateDatabaseParams(id Identifier, version int64, values Values) dbgen.UpdateProfileParams {
	created := createDatabaseParams(id, values)
	return dbgen.UpdateProfileParams{
		FullName:            created.FullName,
		SocialName:          created.SocialName,
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
		BirthDate:           created.BirthDate,
		Gender:              created.Gender,
		BloodType:           created.BloodType,
		Nationality:         created.Nationality,
		BirthCity:           created.BirthCity,
		MaritalStatus:       created.MaritalStatus,
		WeddingDate:         created.WeddingDate,
		FatherName:          created.FatherName,
		FatherBirthDate:     created.FatherBirthDate,
		MotherName:          created.MotherName,
		MotherBirthDate:     created.MotherBirthDate,
		HealthPlan:          created.HealthPlan,
		BloodDonor:          created.BloodDonor,
		OrganDonor:          created.OrganDonor,
		Team:                created.Team,
		Sector:              created.Sector,
		Collections:         created.Collections,
		VehicleModel:        created.VehicleModel,
		VehicleColor:        created.VehicleColor,
		VehiclePlate:        created.VehiclePlate,
		VehicleYear:         created.VehicleYear,
		ClubMembership:      created.ClubMembership,
		MembershipType:      created.MembershipType,
		PlaceOfOrigin:       created.PlaceOfOrigin,
		BirthCountry:        created.BirthCountry,
		ParentsWeddingDate:  created.ParentsWeddingDate,
		SupermarketClub:     created.SupermarketClub,
		Pet:                 created.Pet,
		TravelCountries:     created.TravelCountries,
		CardBrand:           created.CardBrand,
		CardBank:            created.CardBank,
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
	return Profile{
		ID:        id,
		Values:    valuesFromDatabase(value),
		Version:   value.Version,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func valuesFromDatabase(value dbgen.Profile) Values {
	return Values{
		FullName:      value.FullName,
		SocialName:    stringValue(value.SocialName),
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
		Notes:           stringValue(value.Notes),
		BirthDate:       dateValue(value.BirthDate),
		Gender:          stringValue(value.Gender),
		BloodType:       stringValue(value.BloodType),
		Nationality:     stringValue(value.Nationality),
		BirthCity:       stringValue(value.BirthCity),
		MaritalStatus:   stringValue(value.MaritalStatus),
		WeddingDate:     dateValue(value.WeddingDate),
		FatherName:      stringValue(value.FatherName),
		FatherBirthDate: dateValue(value.FatherBirthDate),
		MotherName:      stringValue(value.MotherName),
		MotherBirthDate: dateValue(value.MotherBirthDate),
		HealthPlan:      stringValue(value.HealthPlan),
		BloodDonor:      value.BloodDonor,
		OrganDonor:      value.OrganDonor,
		Team:            stringValue(value.Team),
		Sector:          stringValue(value.Sector),
		Collections:     stringValue(value.Collections),
		VehicleModel:    stringValue(value.VehicleModel),
		VehicleColor:    stringValue(value.VehicleColor),
		VehiclePlate:    stringValue(value.VehiclePlate),
		VehicleYear:     value.VehicleYear,
		ClubMembership:  stringValue(value.ClubMembership),
		MembershipType:  stringValue(value.MembershipType),
		PlaceOfOrigin:   stringValue(value.PlaceOfOrigin),
		BirthCountry:    stringValue(value.BirthCountry),
		ParentsWedding:  dateValue(value.ParentsWeddingDate),
		SupermarketClub: stringValue(value.SupermarketClub),
		Pet:             stringValue(value.Pet),
		TravelCountries: stringValue(value.TravelCountries),
		CardBrand:       stringValue(value.CardBrand),
		CardBank:        stringValue(value.CardBank),
	}
}

func databaseUUID(identifier Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: identifier, Valid: true}
}

func uuidList(ids []Identifier) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		out = append(out, databaseUUID(id))
	}
	return out
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

func optionalDate(value string) pgtype.Date {
	if value == "" {
		return pgtype.Date{}
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

func dateValue(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (store *PostgresStore) syncCPFPresence(ctx context.Context, profileID Identifier, cpf string) error {
	if cpf == "" {
		if err := store.queries.ClearCPFPresenceNumber(ctx, databaseUUID(profileID)); err != nil {
			return fmt.Errorf("clear cpf presence: %w", err)
		}
		return nil
	}
	presenceID, err := NewIdentifier()
	if err != nil {
		return fmt.Errorf("generate cpf presence identifier: %w", err)
	}
	if _, err := store.queries.UpsertCPFPresence(ctx, dbgen.UpsertCPFPresenceParams{
		ID: databaseUUID(presenceID), ProfileID: databaseUUID(profileID), IdentifierValue: optionalString(cpf),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("upsert cpf presence: %w", err)
	}
	return nil
}

var _ Store = (*PostgresStore)(nil)
var _ AuditStore = (*PostgresStore)(nil)

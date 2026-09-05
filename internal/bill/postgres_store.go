package bill

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound              = errors.New("bill not found")
	ErrTypeNotFound          = errors.New("bill type not found")
	ErrConflict              = errors.New("bill changed concurrently")
	ErrTypeConflict          = errors.New("bill type changed concurrently")
	ErrTypeInactive          = errors.New("bill type is inactive")
	ErrTypeInUse             = errors.New("bill type is in use")
	ErrTechnicalKeyImmutable = errors.New("bill type technical key is immutable")
	ErrTechnicalKeyConflict  = errors.New("bill type technical key already exists")
	ErrReferenceNotFound     = errors.New("bill profile reference not found")
	ErrCurrentUseUnsupported = errors.New("bill medium does not support current use")
	ErrCurrentUseExists      = errors.New("bill has current use")
	ErrCurrentUseNotFound    = errors.New("bill current use not found")
)

type SortField string
type SortOrder string
type TypeSortField string

const (
	SortReference  SortField = "reference_value"
	SortTypeLabel  SortField = "type_label"
	SortCompetence SortField = "competence"
	SortAmount     SortField = "amount"
	SortCreatedAt  SortField = "created_at"
	SortUpdatedAt  SortField = "updated_at"

	TypeSortLabel        TypeSortField = "label"
	TypeSortTechnicalKey TypeSortField = "technical_key"
	TypeSortCreatedAt    TypeSortField = "created_at"
	TypeSortUpdatedAt    TypeSortField = "updated_at"

	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

type TypeFilters struct {
	Label  string
	Active *bool
}

type TypeListOptions struct {
	Limit     int32
	Offset    int32
	SortField TypeSortField
	SortOrder SortOrder
	Filters   TypeFilters
}

type Filters struct {
	OwnerProfileID  *profile.Identifier
	TypeID          *Identifier
	Reference       string
	Competence      string
	Medium          Medium
	Status          Status
	HolderProfileID *profile.Identifier
	RestrictIDs     bool
	IDFilter        []Identifier
}

type ListOptions struct {
	Limit     int32
	Offset    int32
	SortField SortField
	SortOrder SortOrder
	Filters   Filters
}

type billQueries interface {
	CreateBillType(context.Context, dbgen.CreateBillTypeParams) (dbgen.BillType, error)
	GetBillTypeByID(context.Context, pgtype.UUID) (dbgen.BillType, error)
	CountBillTypes(context.Context, dbgen.CountBillTypesParams) (int64, error)
	ListBillTypes(context.Context, dbgen.ListBillTypesParams) ([]dbgen.BillType, error)
	CountBillsByType(context.Context) ([]dbgen.CountBillsByTypeRow, error)
	BillTypeHasBills(context.Context, pgtype.UUID) (bool, error)
	UpdateBillType(context.Context, dbgen.UpdateBillTypeParams) (dbgen.BillType, error)
	DeleteBillType(context.Context, dbgen.DeleteBillTypeParams) (pgtype.UUID, error)
	CreateBill(context.Context, dbgen.CreateBillParams) (dbgen.Bill, error)
	GetBillByID(context.Context, pgtype.UUID) (dbgen.GetBillByIDRow, error)
	CountBills(context.Context, dbgen.CountBillsParams) (int64, error)
	ListBills(context.Context, dbgen.ListBillsParams) ([]dbgen.ListBillsRow, error)
	UpdateBill(context.Context, dbgen.UpdateBillParams) (dbgen.Bill, error)
	DuplicateBill(context.Context, dbgen.DuplicateBillParams) (dbgen.Bill, error)
	DeleteBill(context.Context, dbgen.DeleteBillParams) (pgtype.UUID, error)
	AssignBillCurrentUse(context.Context, dbgen.AssignBillCurrentUseParams) (dbgen.BillCurrentUse, error)
	ReturnBillCurrentUse(context.Context, pgtype.UUID) (pgtype.UUID, error)
	GetBillCurrentUse(context.Context, pgtype.UUID) (dbgen.BillCurrentUse, error)
}

type PostgresStore struct {
	queries billQueries
}

func NewPostgresStore(database dbgen.DBTX) *PostgresStore {
	return &PostgresStore{queries: dbgen.New(database)}
}

func (store *PostgresStore) CreateType(ctx context.Context, id Identifier, values TypeValues) (TypeDefinition, error) {
	normalized, err := NormalizeType(values)
	if err != nil {
		return TypeDefinition{}, err
	}
	value, err := store.queries.CreateBillType(ctx, dbgen.CreateBillTypeParams{
		ID: databaseUUID(id), TechnicalKey: normalized.TechnicalKey, Label: normalized.Label,
		Active: normalized.Active,
	})
	if err != nil {
		return TypeDefinition{}, mapTypePersistenceError("create bill type", err)
	}
	return typeFromDatabase(value)
}

func (store *PostgresStore) GetType(ctx context.Context, id Identifier) (TypeDefinition, error) {
	value, err := store.queries.GetBillTypeByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return TypeDefinition{}, ErrTypeNotFound
	}
	if err != nil {
		return TypeDefinition{}, fmt.Errorf("get bill type: %w", err)
	}
	return typeFromDatabase(value)
}

func (store *PostgresStore) CountTypes(ctx context.Context, filters TypeFilters) (int64, error) {
	count, err := store.queries.CountBillTypes(ctx, dbgen.CountBillTypesParams{
		LabelFilter: normalize.SearchText(filters.Label), ActiveFilter: activeFilter(filters.Active),
	})
	if err != nil {
		return 0, fmt.Errorf("count bill types: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) ListTypes(ctx context.Context, options TypeListOptions) ([]TypeDefinition, error) {
	values, err := store.queries.ListBillTypes(ctx, dbgen.ListBillTypesParams{
		LabelFilter: normalize.SearchText(options.Filters.Label), ActiveFilter: activeFilter(options.Filters.Active),
		SortField: string(options.SortField), SortOrder: string(options.SortOrder),
		PageOffset: options.Offset, PageLimit: options.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list bill types: %w", err)
	}
	counts, err := store.queries.CountBillsByType(ctx)
	if err != nil {
		return nil, fmt.Errorf("count bills by type: %w", err)
	}
	countByType := make(map[Identifier]int64, len(counts))
	for _, row := range counts {
		id, idErr := identifierFromDatabase(row.BillTypeID, "bill type")
		if idErr != nil {
			return nil, idErr
		}
		countByType[id] = row.BillCount
	}
	result := make([]TypeDefinition, 0, len(values))
	for _, value := range values {
		mapped, mapErr := typeFromDatabase(value)
		if mapErr != nil {
			return nil, mapErr
		}
		mapped.ExemplarCount = countByType[mapped.ID]
		result = append(result, mapped)
	}
	return result, nil
}

func (store *PostgresStore) UpdateType(ctx context.Context, id Identifier, version int64, values TypeValues) (TypeDefinition, error) {
	if version <= 0 {
		return TypeDefinition{}, ErrTypeConflict
	}
	existing, err := store.GetType(ctx, id)
	if err != nil {
		return TypeDefinition{}, err
	}
	normalized, err := NormalizeType(values)
	if err != nil {
		return TypeDefinition{}, err
	}
	if normalized.TechnicalKey != existing.Values.TechnicalKey {
		return TypeDefinition{}, ErrTechnicalKeyImmutable
	}
	value, err := store.queries.UpdateBillType(ctx, dbgen.UpdateBillTypeParams{
		Label: normalized.Label, Active: normalized.Active,
		ID: databaseUUID(id), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return TypeDefinition{}, store.classifyMissingTypeWrite(ctx, id)
	}
	if err != nil {
		return TypeDefinition{}, mapTypePersistenceError("update bill type", err)
	}
	return typeFromDatabase(value)
}

func (store *PostgresStore) DeleteType(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrTypeConflict
	}
	hasBills, err := store.queries.BillTypeHasBills(ctx, databaseUUID(id))
	if err != nil {
		return fmt.Errorf("check bill type usage: %w", err)
	}
	if hasBills {
		return ErrTypeInUse
	}
	_, err = store.queries.DeleteBillType(ctx, dbgen.DeleteBillTypeParams{ID: databaseUUID(id), Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.classifyMissingTypeWrite(ctx, id)
	}
	if err != nil {
		return mapTypePersistenceError("delete bill type", err)
	}
	return nil
}

func (store *PostgresStore) Create(ctx context.Context, id Identifier, values Values) (Bill, error) {
	definition, err := store.GetType(ctx, values.TypeID)
	if err != nil {
		return Bill{}, err
	}
	if !definition.Values.Active {
		return Bill{}, ErrTypeInactive
	}
	normalized, err := Normalize(values, definition)
	if err != nil {
		return Bill{}, err
	}
	params, err := createDatabaseParams(id, normalized)
	if err != nil {
		return Bill{}, err
	}
	_, err = store.queries.CreateBill(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return Bill{}, ErrTypeInactive
	}
	if err != nil {
		return Bill{}, mapBillPersistenceError("create bill", err)
	}
	return store.Get(ctx, id)
}

func (store *PostgresStore) Get(ctx context.Context, id Identifier) (Bill, error) {
	value, err := store.queries.GetBillByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Bill{}, ErrNotFound
	}
	if err != nil {
		return Bill{}, fmt.Errorf("get bill: %w", err)
	}
	return billFromGetRow(value)
}

func (store *PostgresStore) Count(ctx context.Context, filters Filters) (int64, error) {
	count, err := store.queries.CountBills(ctx, dbgen.CountBillsParams{
		OwnerProfileIDFilter: optionalProfileUUID(filters.OwnerProfileID), BillTypeIDFilter: optionalDatabaseUUID(filters.TypeID),
		ReferenceFilter: normalize.SearchText(filters.Reference), CompetenceFilter: strings.TrimSpace(filters.Competence),
		MediumFilter: string(filters.Medium), StatusFilter: string(filters.Status),
		HolderProfileIDFilter: optionalProfileUUID(filters.HolderProfileID),
		RestrictIds:           filters.RestrictIDs,
		IDFilter:              billUUIDList(filters.IDFilter),
	})
	if err != nil {
		return 0, fmt.Errorf("count bills: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) List(ctx context.Context, options ListOptions) ([]Bill, error) {
	values, err := store.queries.ListBills(ctx, dbgen.ListBillsParams{
		OwnerProfileIDFilter: optionalProfileUUID(options.Filters.OwnerProfileID), BillTypeIDFilter: optionalDatabaseUUID(options.Filters.TypeID),
		ReferenceFilter: normalize.SearchText(options.Filters.Reference), CompetenceFilter: strings.TrimSpace(options.Filters.Competence),
		MediumFilter: string(options.Filters.Medium), StatusFilter: string(options.Filters.Status),
		HolderProfileIDFilter: optionalProfileUUID(options.Filters.HolderProfileID),
		RestrictIds:           options.Filters.RestrictIDs,
		IDFilter:              billUUIDList(options.Filters.IDFilter),
		SortField:             string(options.SortField), SortOrder: string(options.SortOrder), PageOffset: options.Offset, PageLimit: options.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list bills: %w", err)
	}
	result := make([]Bill, 0, len(values))
	for _, value := range values {
		mapped, mapErr := billFromListRow(value)
		if mapErr != nil {
			return nil, mapErr
		}
		result = append(result, mapped)
	}
	return result, nil
}

func (store *PostgresStore) Update(ctx context.Context, id Identifier, version int64, values Values) (Bill, error) {
	if version <= 0 {
		return Bill{}, ErrConflict
	}
	definition, err := store.GetType(ctx, values.TypeID)
	if err != nil {
		return Bill{}, err
	}
	if !definition.Values.Active {
		return Bill{}, ErrTypeInactive
	}
	normalized, err := Normalize(values, definition)
	if err != nil {
		return Bill{}, err
	}
	created, err := createDatabaseParams(id, normalized)
	if err != nil {
		return Bill{}, err
	}
	_, err = store.queries.UpdateBill(ctx, dbgen.UpdateBillParams{
		OwnerProfileID: created.OwnerProfileID, PrintedHolderName: created.PrintedHolderName,
		PrintedAddress: created.PrintedAddress, ReferenceValue: created.ReferenceValue,
		Competence: created.Competence, Amount: created.Amount, Currency: created.Currency,
		Notes: created.Notes, Medium: created.Medium, IdleCustody: created.IdleCustody, ID: created.ID, Version: version,
		BillTypeID: created.BillTypeID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Bill{}, store.classifyMissingWrite(ctx, id)
	}
	if err != nil {
		return Bill{}, mapBillPersistenceError("update bill", err)
	}
	return store.Get(ctx, id)
}

func (store *PostgresStore) Duplicate(ctx context.Context, newID, sourceID Identifier) (Bill, error) {
	_, err := store.queries.DuplicateBill(ctx, dbgen.DuplicateBillParams{NewID: databaseUUID(newID), SourceID: databaseUUID(sourceID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return Bill{}, ErrNotFound
	}
	if err != nil {
		return Bill{}, mapBillPersistenceError("duplicate bill", err)
	}
	return store.Get(ctx, newID)
}

func (store *PostgresStore) Delete(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrConflict
	}
	_, err := store.queries.DeleteBill(ctx, dbgen.DeleteBillParams{ID: databaseUUID(id), Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.classifyMissingWrite(ctx, id)
	}
	if err != nil {
		return mapBillPersistenceError("delete bill", err)
	}
	return nil
}

func (store *PostgresStore) AssignCurrentUse(ctx context.Context, id Identifier, holderProfileID profile.Identifier) (CurrentUse, error) {
	if holderProfileID == (profile.Identifier{}) {
		return CurrentUse{}, ErrReferenceNotFound
	}
	bill, err := store.Get(ctx, id)
	if err != nil {
		return CurrentUse{}, err
	}
	if !bill.Values.Medium.SupportsCurrentUse() {
		return CurrentUse{}, ErrCurrentUseUnsupported
	}
	value, err := store.queries.AssignBillCurrentUse(ctx, dbgen.AssignBillCurrentUseParams{
		HolderProfileID: profileUUID(holderProfileID), BillID: databaseUUID(id),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CurrentUse{}, ErrCurrentUseUnsupported
	}
	if err != nil {
		return CurrentUse{}, mapBillPersistenceError("assign bill current use", err)
	}
	return currentUseFromDatabase(value.HolderProfileID, nil, value.AssignedAt)
}

func (store *PostgresStore) ReturnCurrentUse(ctx context.Context, id Identifier) error {
	_, err := store.queries.ReturnBillCurrentUse(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCurrentUseNotFound
	}
	if err != nil {
		return fmt.Errorf("return bill current use: %w", err)
	}
	return nil
}

func (store *PostgresStore) GetCurrentUse(ctx context.Context, id Identifier) (*CurrentUse, error) {
	value, err := store.queries.GetBillCurrentUse(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bill current use: %w", err)
	}
	currentUse, err := currentUseFromDatabase(value.HolderProfileID, nil, value.AssignedAt)
	if err != nil {
		return nil, err
	}
	return &currentUse, nil
}

func (store *PostgresStore) classifyMissingWrite(ctx context.Context, id Identifier) error {
	_, err := store.queries.GetBillByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify bill write: %w", err)
	}
	return ErrConflict
}

func (store *PostgresStore) classifyMissingTypeWrite(ctx context.Context, id Identifier) error {
	_, err := store.queries.GetBillTypeByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTypeNotFound
	}
	if err != nil {
		return fmt.Errorf("classify bill type write: %w", err)
	}
	return ErrTypeConflict
}

func createDatabaseParams(id Identifier, values Values) (dbgen.CreateBillParams, error) {
	amount, err := optionalNumeric(values.Amount)
	if err != nil {
		return dbgen.CreateBillParams{}, fmt.Errorf("convert bill amount: %w", err)
	}
	return dbgen.CreateBillParams{
		ID: databaseUUID(id), OwnerProfileID: profileUUID(values.OwnerProfileID),
		PrintedHolderName: optionalString(values.PrintedHolderName), PrintedAddress: optionalString(values.PrintedAddress),
		ReferenceValue: optionalString(values.Reference), Competence: optionalString(values.Competence),
		Amount: amount, Currency: optionalString(values.Currency), Notes: optionalString(values.Notes),
		Medium: string(values.Medium), IdleCustody: optionalString(string(values.IdleCustody)), BillTypeID: databaseUUID(values.TypeID),
	}, nil
}

func typeFromDatabase(value dbgen.BillType) (TypeDefinition, error) {
	id, err := identifierFromDatabase(value.ID, "bill type")
	if err != nil {
		return TypeDefinition{}, err
	}
	createdAt, err := timeFromDatabase(value.CreatedAt, "bill type created_at")
	if err != nil {
		return TypeDefinition{}, err
	}
	updatedAt, err := timeFromDatabase(value.UpdatedAt, "bill type updated_at")
	if err != nil {
		return TypeDefinition{}, err
	}
	normalized, err := NormalizeType(TypeValues{
		TechnicalKey: value.TechnicalKey, Label: value.Label, Active: value.Active,
	})
	if err != nil {
		return TypeDefinition{}, fmt.Errorf("map bill type from database: %w", err)
	}
	return TypeDefinition{ID: id, Values: normalized, Version: value.Version, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func billFromDatabase(value dbgen.Bill, definition TypeDefinition) (Bill, error) {
	id, err := identifierFromDatabase(value.ID, "bill")
	if err != nil {
		return Bill{}, err
	}
	ownerID, err := profileIdentifierFromDatabase(value.OwnerProfileID, "bill owner")
	if err != nil {
		return Bill{}, err
	}
	createdAt, err := timeFromDatabase(value.CreatedAt, "bill created_at")
	if err != nil {
		return Bill{}, err
	}
	updatedAt, err := timeFromDatabase(value.UpdatedAt, "bill updated_at")
	if err != nil {
		return Bill{}, err
	}
	amount, err := numericValue(value.Amount)
	if err != nil {
		return Bill{}, fmt.Errorf("map bill amount from database: %w", err)
	}
	normalized, err := NormalizeStored(Values{
		OwnerProfileID: ownerID, TypeID: definition.ID, PrintedHolderName: stringValue(value.PrintedHolderName),
		PrintedAddress: stringValue(value.PrintedAddress), Reference: stringValue(value.ReferenceValue),
		Competence: stringValue(value.Competence), Amount: amount, Currency: stringValue(value.Currency),
		Notes: stringValue(value.Notes), Medium: Medium(value.Medium), IdleCustody: IdleCustody(stringValue(value.IdleCustody)),
	}, definition)
	if err != nil {
		return Bill{}, fmt.Errorf("map bill from database: %w", err)
	}
	return Bill{ID: id, Values: normalized, Type: definition, Status: OperationalStatus(normalized.Medium, false, normalized.IdleCustody), Version: value.Version, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func billFromGetRow(value dbgen.GetBillByIDRow) (Bill, error) {
	return billFromJoinedRow(value.ID, value.OwnerProfileID, value.OwnerFullName, value.BillTypeID, value.PrintedHolderName,
		value.PrintedAddress, value.ReferenceValue, value.Competence, value.Amount, value.Currency,
		value.Notes, value.Medium, value.IdleCustody, value.Version, value.CreatedAt, value.UpdatedAt,
		value.TypeTechnicalKey, value.TypeLabel, value.TypeActive,
		value.TypeVersion, value.TypeCreatedAt, value.TypeUpdatedAt,
		value.CurrentHolderProfileID, value.CurrentHolderFullName, value.CurrentAssignedAt)
}

func billFromListRow(value dbgen.ListBillsRow) (Bill, error) {
	return billFromJoinedRow(value.ID, value.OwnerProfileID, value.OwnerFullName, value.BillTypeID, value.PrintedHolderName,
		value.PrintedAddress, value.ReferenceValue, value.Competence, value.Amount, value.Currency,
		value.Notes, value.Medium, value.IdleCustody, value.Version, value.CreatedAt, value.UpdatedAt,
		value.TypeTechnicalKey, value.TypeLabel, value.TypeActive,
		value.TypeVersion, value.TypeCreatedAt, value.TypeUpdatedAt,
		value.CurrentHolderProfileID, value.CurrentHolderFullName, value.CurrentAssignedAt)
}

func billFromJoinedRow(idValue, ownerValue pgtype.UUID, ownerFullName string, typeValue pgtype.UUID, printedHolderName, printedAddress,
	referenceValue, competence *string, amount pgtype.Numeric, currency, notes *string,
	medium string, idleCustody *string, version int64, createdValue, updatedValue pgtype.Timestamptz,
	typeTechnicalKey, typeLabel string, typeActive bool, typeVersion int64,
	typeCreatedValue, typeUpdatedValue pgtype.Timestamptz, holderValue pgtype.UUID, holderFullName *string, assignedValue pgtype.Timestamptz,
) (Bill, error) {
	definition, err := typeFromDatabase(dbgen.BillType{
		ID: typeValue, TechnicalKey: typeTechnicalKey, Label: typeLabel, Active: typeActive,
		Version: typeVersion, CreatedAt: typeCreatedValue, UpdatedAt: typeUpdatedValue,
	})
	if err != nil {
		return Bill{}, err
	}
	bill, err := billFromDatabase(dbgen.Bill{
		ID: idValue, OwnerProfileID: ownerValue, BillTypeID: typeValue,
		PrintedHolderName: printedHolderName, PrintedAddress: printedAddress, ReferenceValue: referenceValue,
		Competence: competence, Amount: amount, Currency: currency, Notes: notes, Medium: medium,
		IdleCustody: idleCustody, Version: version, CreatedAt: createdValue, UpdatedAt: updatedValue,
	}, definition)
	if err != nil {
		return Bill{}, err
	}
	bill.OwnerFullName = ownerFullName
	if holderValue.Valid {
		currentUse, currentErr := currentUseFromDatabase(holderValue, holderFullName, assignedValue)
		if currentErr != nil {
			return Bill{}, currentErr
		}
		bill.CurrentUse = &currentUse
		bill.Status = OperationalStatus(bill.Values.Medium, true, bill.Values.IdleCustody)
	}
	return bill, nil
}

func currentUseFromDatabase(holder pgtype.UUID, holderFullName *string, assignedAt pgtype.Timestamptz) (CurrentUse, error) {
	holderID, err := profileIdentifierFromDatabase(holder, "bill current holder")
	if err != nil {
		return CurrentUse{}, err
	}
	assigned, err := timeFromDatabase(assignedAt, "bill current use assigned_at")
	if err != nil {
		return CurrentUse{}, err
	}
	return CurrentUse{HolderProfileID: holderID, HolderFullName: stringValue(holderFullName), AssignedAt: assigned}, nil
}

func mapTypePersistenceError(operation string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return ErrTechnicalKeyConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func mapBillPersistenceError(operation string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "23503":
			if strings.HasPrefix(operation, "delete") && strings.Contains(databaseError.ConstraintName, "current_uses") {
				return ErrCurrentUseExists
			}
			return ErrReferenceNotFound
		case "23514":
			return ErrCurrentUseUnsupported
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func databaseUUID(identifier Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: identifier, Valid: true}
}

func billUUIDList(ids []Identifier) []pgtype.UUID {
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

func profileUUID(identifier profile.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: identifier, Valid: true}
}

func optionalProfileUUID(identifier *profile.Identifier) pgtype.UUID {
	if identifier == nil {
		return pgtype.UUID{}
	}
	return profileUUID(*identifier)
}

func identifierFromDatabase(value pgtype.UUID, subject string) (Identifier, error) {
	if !value.Valid {
		return Identifier{}, fmt.Errorf("%s database identifier is invalid", subject)
	}
	return Identifier(value.Bytes), nil
}

func profileIdentifierFromDatabase(value pgtype.UUID, subject string) (profile.Identifier, error) {
	if !value.Valid {
		return profile.Identifier{}, fmt.Errorf("%s database identifier is invalid", subject)
	}
	return profile.Identifier(value.Bytes), nil
}

func optionalNumeric(value string) (pgtype.Numeric, error) {
	if value == "" {
		return pgtype.Numeric{}, nil
	}
	var numeric pgtype.Numeric
	if err := numeric.Scan(value); err != nil {
		return pgtype.Numeric{}, err
	}
	return numeric, nil
}

func numericValue(value pgtype.Numeric) (string, error) {
	if !value.Valid {
		return "", nil
	}
	databaseValue, err := value.Value()
	if err != nil {
		return "", err
	}
	text, ok := databaseValue.(string)
	if !ok {
		return "", fmt.Errorf("unexpected numeric database value %T", databaseValue)
	}
	normalized, ok := canonicalAmount(text)
	if !ok {
		return "", fmt.Errorf("invalid numeric database value %q", text)
	}
	return normalized, nil
}

func timeFromDatabase(value pgtype.Timestamptz, field string) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("%s is invalid", field)
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

func activeFilter(value *bool) string {
	if value == nil {
		return ""
	}
	return strconv.FormatBool(*value)
}

var _ Store = (*PostgresStore)(nil)

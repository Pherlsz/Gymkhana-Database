package document

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
	ErrNotFound              = errors.New("document not found")
	ErrTypeNotFound          = errors.New("document type not found")
	ErrConflict              = errors.New("document changed concurrently")
	ErrTypeConflict          = errors.New("document type changed concurrently")
	ErrTypeInactive          = errors.New("document type is inactive")
	ErrTypeInUse             = errors.New("document type is in use")
	ErrTechnicalKeyImmutable = errors.New("document type technical key is immutable")
	ErrTechnicalKeyConflict  = errors.New("document type technical key already exists")
	ErrUniquenessConflict    = errors.New("document uniqueness policy conflict")
	ErrReferenceNotFound     = errors.New("document profile reference not found")
	ErrCurrentUseExists      = errors.New("document has current use")
	ErrCurrentUseNotFound    = errors.New("document current use not found")
)

type SortField string
type SortOrder string

type TypeSortField string

const (
	SortIdentifier   SortField = "identifier_value"
	SortTypeLabel    SortField = "type_label"
	SortDocumentDate SortField = "document_date"
	SortCreatedAt    SortField = "created_at"
	SortUpdatedAt    SortField = "updated_at"

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
	Identifier      string
	RecordState     RecordState
	Status          Status
	HolderProfileID *profile.Identifier
}

type ListOptions struct {
	Limit     int32
	Offset    int32
	SortField SortField
	SortOrder SortOrder
	Filters   Filters
}

type Store interface {
	CreateType(context.Context, Identifier, TypeValues) (TypeDefinition, error)
	GetType(context.Context, Identifier) (TypeDefinition, error)
	CountTypes(context.Context, TypeFilters) (int64, error)
	ListTypes(context.Context, TypeListOptions) ([]TypeDefinition, error)
	UpdateType(context.Context, Identifier, int64, TypeValues) (TypeDefinition, error)
	DeleteType(context.Context, Identifier, int64) error
	Create(context.Context, Identifier, Values) (Document, error)
	Get(context.Context, Identifier) (Document, error)
	Count(context.Context, Filters) (int64, error)
	List(context.Context, ListOptions) ([]Document, error)
	Update(context.Context, Identifier, int64, Values) (Document, error)
	Duplicate(context.Context, Identifier, Identifier) (Document, error)
	Delete(context.Context, Identifier, int64) error
	AssignCurrentUse(context.Context, Identifier, profile.Identifier) (CurrentUse, error)
	ReturnCurrentUse(context.Context, Identifier) error
	GetCurrentUse(context.Context, Identifier) (*CurrentUse, error)
}

type documentQueries interface {
	CreateDocumentType(context.Context, dbgen.CreateDocumentTypeParams) (dbgen.DocumentType, error)
	GetDocumentTypeByID(context.Context, pgtype.UUID) (dbgen.DocumentType, error)
	CountDocumentTypes(context.Context, dbgen.CountDocumentTypesParams) (int64, error)
	ListDocumentTypes(context.Context, dbgen.ListDocumentTypesParams) ([]dbgen.DocumentType, error)
	DocumentTypeHasDocuments(context.Context, pgtype.UUID) (bool, error)
	UpdateDocumentType(context.Context, dbgen.UpdateDocumentTypeParams) (dbgen.DocumentType, error)
	DeleteDocumentType(context.Context, dbgen.DeleteDocumentTypeParams) (pgtype.UUID, error)
	CreateDocument(context.Context, dbgen.CreateDocumentParams) (dbgen.Document, error)
	GetDocumentByID(context.Context, pgtype.UUID) (dbgen.GetDocumentByIDRow, error)
	CountDocuments(context.Context, dbgen.CountDocumentsParams) (int64, error)
	ListDocuments(context.Context, dbgen.ListDocumentsParams) ([]dbgen.ListDocumentsRow, error)
	UpdateDocument(context.Context, dbgen.UpdateDocumentParams) (dbgen.Document, error)
	DuplicateDocument(context.Context, dbgen.DuplicateDocumentParams) (dbgen.Document, error)
	DeleteDocument(context.Context, dbgen.DeleteDocumentParams) (pgtype.UUID, error)
	AssignDocumentCurrentUse(context.Context, dbgen.AssignDocumentCurrentUseParams) (dbgen.DocumentCurrentUse, error)
	ReturnDocumentCurrentUse(context.Context, pgtype.UUID) (pgtype.UUID, error)
	GetDocumentCurrentUse(context.Context, pgtype.UUID) (dbgen.DocumentCurrentUse, error)
}

type PostgresStore struct {
	queries documentQueries
}

func NewPostgresStore(database dbgen.DBTX) *PostgresStore {
	return &PostgresStore{queries: dbgen.New(database)}
}

func (store *PostgresStore) CreateType(ctx context.Context, id Identifier, values TypeValues) (TypeDefinition, error) {
	normalized, err := NormalizeType(values)
	if err != nil {
		return TypeDefinition{}, err
	}
	value, err := store.queries.CreateDocumentType(ctx, dbgen.CreateDocumentTypeParams{
		ID: databaseUUID(id), TechnicalKey: normalized.TechnicalKey, Label: normalized.Label,
		Active: normalized.Active, UniquenessPolicy: string(normalized.UniquenessPolicy),
		ValidationRegex: optionalString(normalized.ValidationRegex), DateRequired: normalized.DateRequired,
	})
	if err != nil {
		return TypeDefinition{}, mapTypePersistenceError("create document type", err)
	}
	return typeFromDatabase(value)
}

func (store *PostgresStore) GetType(ctx context.Context, id Identifier) (TypeDefinition, error) {
	value, err := store.queries.GetDocumentTypeByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return TypeDefinition{}, ErrTypeNotFound
	}
	if err != nil {
		return TypeDefinition{}, fmt.Errorf("get document type: %w", err)
	}
	return typeFromDatabase(value)
}

func (store *PostgresStore) CountTypes(ctx context.Context, filters TypeFilters) (int64, error) {
	count, err := store.queries.CountDocumentTypes(ctx, dbgen.CountDocumentTypesParams{
		LabelFilter: normalize.SearchText(filters.Label), ActiveFilter: activeFilter(filters.Active),
	})
	if err != nil {
		return 0, fmt.Errorf("count document types: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) ListTypes(ctx context.Context, options TypeListOptions) ([]TypeDefinition, error) {
	values, err := store.queries.ListDocumentTypes(ctx, dbgen.ListDocumentTypesParams{
		LabelFilter: normalize.SearchText(options.Filters.Label), ActiveFilter: activeFilter(options.Filters.Active),
		SortField: string(options.SortField), SortOrder: string(options.SortOrder),
		PageOffset: options.Offset, PageLimit: options.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list document types: %w", err)
	}
	result := make([]TypeDefinition, 0, len(values))
	for _, value := range values {
		mapped, mapErr := typeFromDatabase(value)
		if mapErr != nil {
			return nil, mapErr
		}
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
	if !SameRules(existing.Values, normalized) {
		hasDocuments, checkErr := store.queries.DocumentTypeHasDocuments(ctx, databaseUUID(id))
		if checkErr != nil {
			return TypeDefinition{}, fmt.Errorf("check document type usage: %w", checkErr)
		}
		if hasDocuments {
			return TypeDefinition{}, ErrTypeInUse
		}
	}
	value, err := store.queries.UpdateDocumentType(ctx, dbgen.UpdateDocumentTypeParams{
		Label: normalized.Label, Active: normalized.Active, UniquenessPolicy: string(normalized.UniquenessPolicy),
		ValidationRegex: optionalString(normalized.ValidationRegex), DateRequired: normalized.DateRequired,
		ID: databaseUUID(id), Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return TypeDefinition{}, store.classifyMissingTypeWrite(ctx, id)
	}
	if err != nil {
		return TypeDefinition{}, mapTypePersistenceError("update document type", err)
	}
	return typeFromDatabase(value)
}

func (store *PostgresStore) DeleteType(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrTypeConflict
	}
	hasDocuments, err := store.queries.DocumentTypeHasDocuments(ctx, databaseUUID(id))
	if err != nil {
		return fmt.Errorf("check document type usage: %w", err)
	}
	if hasDocuments {
		return ErrTypeInUse
	}
	_, err = store.queries.DeleteDocumentType(ctx, dbgen.DeleteDocumentTypeParams{ID: databaseUUID(id), Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.classifyMissingTypeWrite(ctx, id)
	}
	if err != nil {
		return mapTypePersistenceError("delete document type", err)
	}
	return nil
}

func (store *PostgresStore) Create(ctx context.Context, id Identifier, values Values) (Document, error) {
	definition, err := store.GetType(ctx, values.TypeID)
	if err != nil {
		return Document{}, err
	}
	if !definition.Values.Active {
		return Document{}, ErrTypeInactive
	}
	normalized, err := Normalize(values, definition)
	if err != nil {
		return Document{}, err
	}
	value, err := store.queries.CreateDocument(ctx, createDatabaseParams(id, normalized))
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrTypeInactive
	}
	if err != nil {
		return Document{}, mapDocumentPersistenceError("create document", err)
	}
	return documentFromDatabase(value, definition)
}

func (store *PostgresStore) Get(ctx context.Context, id Identifier) (Document, error) {
	value, err := store.queries.GetDocumentByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, fmt.Errorf("get document: %w", err)
	}
	return documentFromGetRow(value)
}

func (store *PostgresStore) Count(ctx context.Context, filters Filters) (int64, error) {
	count, err := store.queries.CountDocuments(ctx, dbgen.CountDocumentsParams{
		OwnerProfileIDFilter: optionalProfileUUID(filters.OwnerProfileID), DocumentTypeIDFilter: optionalDatabaseUUID(filters.TypeID),
		IdentifierFilter: normalize.SearchText(filters.Identifier), RecordStateFilter: string(filters.RecordState),
		StatusFilter: string(filters.Status), HolderProfileIDFilter: optionalProfileUUID(filters.HolderProfileID),
	})
	if err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) List(ctx context.Context, options ListOptions) ([]Document, error) {
	values, err := store.queries.ListDocuments(ctx, dbgen.ListDocumentsParams{
		OwnerProfileIDFilter: optionalProfileUUID(options.Filters.OwnerProfileID), DocumentTypeIDFilter: optionalDatabaseUUID(options.Filters.TypeID),
		IdentifierFilter: normalize.SearchText(options.Filters.Identifier), RecordStateFilter: string(options.Filters.RecordState),
		StatusFilter: string(options.Filters.Status), HolderProfileIDFilter: optionalProfileUUID(options.Filters.HolderProfileID),
		SortField: string(options.SortField), SortOrder: string(options.SortOrder), PageOffset: options.Offset, PageLimit: options.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	result := make([]Document, 0, len(values))
	for _, value := range values {
		mapped, mapErr := documentFromListRow(value)
		if mapErr != nil {
			return nil, mapErr
		}
		result = append(result, mapped)
	}
	return result, nil
}

func (store *PostgresStore) Update(ctx context.Context, id Identifier, version int64, values Values) (Document, error) {
	if version <= 0 {
		return Document{}, ErrConflict
	}
	definition, err := store.GetType(ctx, values.TypeID)
	if err != nil {
		return Document{}, err
	}
	if !definition.Values.Active {
		return Document{}, ErrTypeInactive
	}
	normalized, err := Normalize(values, definition)
	if err != nil {
		return Document{}, err
	}
	params := createDatabaseParams(id, normalized)
	value, err := store.queries.UpdateDocument(ctx, dbgen.UpdateDocumentParams{
		OwnerProfileID: params.OwnerProfileID, IdentifierValue: params.IdentifierValue,
		DocumentDate: params.DocumentDate, Notes: params.Notes, RecordState: params.RecordState,
		ID: params.ID, Version: version, DocumentTypeID: params.DocumentTypeID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, store.classifyMissingWrite(ctx, id)
	}
	if err != nil {
		return Document{}, mapDocumentPersistenceError("update document", err)
	}
	return documentFromDatabase(value, definition)
}

func (store *PostgresStore) Duplicate(ctx context.Context, newID, sourceID Identifier) (Document, error) {
	_, err := store.queries.DuplicateDocument(ctx, dbgen.DuplicateDocumentParams{NewID: databaseUUID(newID), SourceID: databaseUUID(sourceID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, mapDocumentPersistenceError("duplicate document", err)
	}
	return store.Get(ctx, newID)
}

func (store *PostgresStore) Delete(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrConflict
	}
	_, err := store.queries.DeleteDocument(ctx, dbgen.DeleteDocumentParams{ID: databaseUUID(id), Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.classifyMissingWrite(ctx, id)
	}
	if err != nil {
		return mapDocumentPersistenceError("delete document", err)
	}
	return nil
}

func (store *PostgresStore) AssignCurrentUse(ctx context.Context, id Identifier, holderProfileID profile.Identifier) (CurrentUse, error) {
	if holderProfileID == (profile.Identifier{}) {
		return CurrentUse{}, ErrReferenceNotFound
	}
	value, err := store.queries.AssignDocumentCurrentUse(ctx, dbgen.AssignDocumentCurrentUseParams{
		DocumentID: databaseUUID(id), HolderProfileID: profileUUID(holderProfileID),
	})
	if err != nil {
		return CurrentUse{}, mapDocumentPersistenceError("assign document current use", err)
	}
	return currentUseFromDatabase(value.HolderProfileID, value.AssignedAt)
}

func (store *PostgresStore) ReturnCurrentUse(ctx context.Context, id Identifier) error {
	_, err := store.queries.ReturnDocumentCurrentUse(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCurrentUseNotFound
	}
	if err != nil {
		return fmt.Errorf("return document current use: %w", err)
	}
	return nil
}

func (store *PostgresStore) GetCurrentUse(ctx context.Context, id Identifier) (*CurrentUse, error) {
	value, err := store.queries.GetDocumentCurrentUse(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get document current use: %w", err)
	}
	currentUse, err := currentUseFromDatabase(value.HolderProfileID, value.AssignedAt)
	if err != nil {
		return nil, err
	}
	return &currentUse, nil
}

func (store *PostgresStore) classifyMissingWrite(ctx context.Context, id Identifier) error {
	_, err := store.queries.GetDocumentByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify document write: %w", err)
	}
	return ErrConflict
}

func (store *PostgresStore) classifyMissingTypeWrite(ctx context.Context, id Identifier) error {
	_, err := store.queries.GetDocumentTypeByID(ctx, databaseUUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrTypeNotFound
	}
	if err != nil {
		return fmt.Errorf("classify document type write: %w", err)
	}
	return ErrTypeConflict
}

func createDatabaseParams(id Identifier, values Values) dbgen.CreateDocumentParams {
	return dbgen.CreateDocumentParams{
		ID: databaseUUID(id), OwnerProfileID: profileUUID(values.OwnerProfileID),
		IdentifierValue: values.Identifier, DocumentDate: optionalDate(values.DocumentDate),
		Notes: optionalString(values.Notes), RecordState: string(values.RecordState),
		DocumentTypeID: databaseUUID(values.TypeID),
	}
}

func typeFromDatabase(value dbgen.DocumentType) (TypeDefinition, error) {
	id, err := identifierFromDatabase(value.ID, "document type")
	if err != nil {
		return TypeDefinition{}, err
	}
	createdAt, err := timeFromDatabase(value.CreatedAt, "document type created_at")
	if err != nil {
		return TypeDefinition{}, err
	}
	updatedAt, err := timeFromDatabase(value.UpdatedAt, "document type updated_at")
	if err != nil {
		return TypeDefinition{}, err
	}
	normalized, err := NormalizeType(TypeValues{
		TechnicalKey: value.TechnicalKey, Label: value.Label, Active: value.Active,
		UniquenessPolicy: UniquenessPolicy(value.UniquenessPolicy), ValidationRegex: stringValue(value.ValidationRegex),
		DateRequired: value.DateRequired,
	})
	if err != nil {
		return TypeDefinition{}, fmt.Errorf("map document type from database: %w", err)
	}
	return TypeDefinition{ID: id, Values: normalized, Version: value.Version, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func documentFromDatabase(value dbgen.Document, definition TypeDefinition) (Document, error) {
	id, err := identifierFromDatabase(value.ID, "document")
	if err != nil {
		return Document{}, err
	}
	ownerID, err := profileIdentifierFromDatabase(value.OwnerProfileID, "document owner")
	if err != nil {
		return Document{}, err
	}
	createdAt, err := timeFromDatabase(value.CreatedAt, "document created_at")
	if err != nil {
		return Document{}, err
	}
	updatedAt, err := timeFromDatabase(value.UpdatedAt, "document updated_at")
	if err != nil {
		return Document{}, err
	}
	normalized, err := Normalize(Values{
		OwnerProfileID: ownerID, TypeID: definition.ID, Identifier: value.IdentifierValue,
		DocumentDate: dateValue(value.DocumentDate), Notes: stringValue(value.Notes), RecordState: RecordState(value.RecordState),
	}, definition)
	if err != nil {
		return Document{}, fmt.Errorf("map document from database: %w", err)
	}
	return Document{ID: id, Values: normalized, Type: definition, Status: StatusAvailable, Version: value.Version, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func documentFromGetRow(value dbgen.GetDocumentByIDRow) (Document, error) {
	return documentFromJoinedRow(value.ID, value.OwnerProfileID, value.DocumentTypeID, value.IdentifierValue,
		value.DocumentDate, value.Notes, value.RecordState, value.Version, value.CreatedAt, value.UpdatedAt,
		value.TypeTechnicalKey, value.TypeLabel, value.TypeActive, value.UniquenessPolicy, value.TypeValidationRegex,
		value.TypeDateRequired, value.TypeVersion, value.TypeCreatedAt, value.TypeUpdatedAt,
		value.CurrentHolderProfileID, value.CurrentAssignedAt)
}

func documentFromListRow(value dbgen.ListDocumentsRow) (Document, error) {
	return documentFromJoinedRow(value.ID, value.OwnerProfileID, value.DocumentTypeID, value.IdentifierValue,
		value.DocumentDate, value.Notes, value.RecordState, value.Version, value.CreatedAt, value.UpdatedAt,
		value.TypeTechnicalKey, value.TypeLabel, value.TypeActive, value.UniquenessPolicy, value.TypeValidationRegex,
		value.TypeDateRequired, value.TypeVersion, value.TypeCreatedAt, value.TypeUpdatedAt,
		value.CurrentHolderProfileID, value.CurrentAssignedAt)
}

func documentFromJoinedRow(idValue, ownerValue, typeValue pgtype.UUID, identifier string, documentDate pgtype.Date,
	notes *string, state string, version int64, createdValue, updatedValue pgtype.Timestamptz,
	typeTechnicalKey, typeLabel string, typeActive bool, uniquenessPolicy string, validationRegex *string, dateRequired bool,
	typeVersion int64, typeCreatedValue, typeUpdatedValue pgtype.Timestamptz,
	holderValue pgtype.UUID, assignedValue pgtype.Timestamptz,
) (Document, error) {
	definition, err := typeFromDatabase(dbgen.DocumentType{
		ID: typeValue, TechnicalKey: typeTechnicalKey, Label: typeLabel, Active: typeActive,
		UniquenessPolicy: uniquenessPolicy, ValidationRegex: validationRegex, DateRequired: dateRequired,
		Version: typeVersion, CreatedAt: typeCreatedValue, UpdatedAt: typeUpdatedValue,
	})
	if err != nil {
		return Document{}, err
	}
	document, err := documentFromDatabase(dbgen.Document{
		ID: idValue, OwnerProfileID: ownerValue, DocumentTypeID: typeValue, IdentifierValue: identifier,
		UniquenessPolicy: uniquenessPolicy, DocumentDate: documentDate, Notes: notes, RecordState: state,
		Version: version, CreatedAt: createdValue, UpdatedAt: updatedValue,
	}, definition)
	if err != nil {
		return Document{}, err
	}
	if holderValue.Valid {
		currentUse, currentErr := currentUseFromDatabase(holderValue, assignedValue)
		if currentErr != nil {
			return Document{}, currentErr
		}
		document.CurrentUse = &currentUse
		document.Status = StatusInUse
	}
	return document, nil
}

func currentUseFromDatabase(holder pgtype.UUID, assignedAt pgtype.Timestamptz) (CurrentUse, error) {
	holderID, err := profileIdentifierFromDatabase(holder, "document current holder")
	if err != nil {
		return CurrentUse{}, err
	}
	assigned, err := timeFromDatabase(assignedAt, "document current use assigned_at")
	if err != nil {
		return CurrentUse{}, err
	}
	return CurrentUse{HolderProfileID: holderID, AssignedAt: assigned}, nil
}

func mapTypePersistenceError(operation string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return ErrTechnicalKeyConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func mapDocumentPersistenceError(operation string, err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "23505":
			return ErrUniquenessConflict
		case "23503":
			if strings.HasPrefix(operation, "delete") && strings.Contains(databaseError.ConstraintName, "current_uses") {
				return ErrCurrentUseExists
			}
			return ErrReferenceNotFound
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
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

func optionalDate(value string) pgtype.Date {
	if value == "" {
		return pgtype.Date{}
	}
	parsed, _ := time.Parse("2006-01-02", value)
	return pgtype.Date{Time: parsed, Valid: true}
}

func dateValue(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
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

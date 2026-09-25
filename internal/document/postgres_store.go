package document

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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
	ErrCurrentUseUnsupported = errors.New("document medium does not support current use")
	ErrDuplicateNotSupported = errors.New("document exemplar already exists for this person and medium")
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

type documentQueries interface {
	CreateDocumentType(context.Context, dbgen.CreateDocumentTypeParams) (dbgen.DocumentType, error)
	GetDocumentTypeByID(context.Context, pgtype.UUID) (dbgen.DocumentType, error)
	CountDocumentTypes(context.Context, dbgen.CountDocumentTypesParams) (int64, error)
	ListDocumentTypes(context.Context, dbgen.ListDocumentTypesParams) ([]dbgen.DocumentType, error)
	CountDocumentsByType(context.Context) ([]dbgen.CountDocumentsByTypeRow, error)
	DocumentTypeHasDocuments(context.Context, pgtype.UUID) (bool, error)
	UpdateDocumentType(context.Context, dbgen.UpdateDocumentTypeParams) (dbgen.DocumentType, error)
	DeleteDocumentType(context.Context, dbgen.DeleteDocumentTypeParams) (pgtype.UUID, error)
	GetDocumentPresence(context.Context, dbgen.GetDocumentPresenceParams) (dbgen.DocumentPresence, error)
	UpsertDocumentPresence(context.Context, dbgen.UpsertDocumentPresenceParams) (dbgen.DocumentPresence, error)
	CountExemplarsByPresence(context.Context, pgtype.UUID) (int64, error)
	CreateDocument(context.Context, dbgen.CreateDocumentParams) (dbgen.Document, error)
	GetDocumentByID(context.Context, pgtype.UUID) (dbgen.GetDocumentByIDRow, error)
	CountDocuments(context.Context, dbgen.CountDocumentsParams) (int64, error)
	ListDocuments(context.Context, dbgen.ListDocumentsParams) ([]dbgen.ListDocumentsRow, error)
	UpdateDocument(context.Context, dbgen.UpdateDocumentParams) (dbgen.Document, error)
	DuplicateDocument(context.Context, dbgen.DuplicateDocumentParams) (dbgen.Document, error)
	DeleteDocument(context.Context, dbgen.DeleteDocumentParams) (pgtype.UUID, error)
	DeleteDocumentCurrentUseForDelete(context.Context, pgtype.UUID) error
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
	counts, err := store.queries.CountDocumentsByType(ctx)
	if err != nil {
		return nil, fmt.Errorf("count documents by type: %w", err)
	}
	countByType := make(map[Identifier]int64, len(counts))
	for _, row := range counts {
		id, idErr := identifierFromDatabase(row.DocumentTypeID, "document type")
		if idErr != nil {
			return nil, idErr
		}
		countByType[id] = row.DocumentCount
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
	presence, err := store.upsertPresence(ctx, normalized, true)
	if err != nil {
		return Document{}, err
	}
	_, err = store.queries.CreateDocument(ctx, createDatabaseParams(id, presence.ID, normalized))
	if err != nil {
		return Document{}, mapDocumentPersistenceError("create document", err)
	}
	return store.Get(ctx, id)
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
		IdentifierFilter: normalize.SearchText(filters.Identifier), MediumFilter: string(filters.Medium),
		StatusFilter: string(filters.Status), HolderProfileIDFilter: optionalProfileUUID(filters.HolderProfileID),
		RestrictIds: filters.RestrictIDs, IDFilter: documentUUIDList(filters.IDFilter),
	})
	if err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	return count, nil
}

func (store *PostgresStore) List(ctx context.Context, options ListOptions) ([]Document, error) {
	values, err := store.queries.ListDocuments(ctx, dbgen.ListDocumentsParams{
		OwnerProfileIDFilter: optionalProfileUUID(options.Filters.OwnerProfileID), DocumentTypeIDFilter: optionalDatabaseUUID(options.Filters.TypeID),
		IdentifierFilter: normalize.SearchText(options.Filters.Identifier), MediumFilter: string(options.Filters.Medium),
		StatusFilter: string(options.Filters.Status), HolderProfileIDFilter: optionalProfileUUID(options.Filters.HolderProfileID),
		RestrictIds: options.Filters.RestrictIDs, IDFilter: documentUUIDList(options.Filters.IDFilter),
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
	presence, err := store.upsertPresence(ctx, normalized, false)
	if err != nil {
		return Document{}, err
	}
	params := createDatabaseParams(id, presence.ID, normalized)
	_, err = store.queries.UpdateDocument(ctx, dbgen.UpdateDocumentParams{
		PresenceID: params.PresenceID, IdleCustody: params.IdleCustody, DocumentDate: params.DocumentDate,
		ValidUntil: params.ValidUntil, Notes: params.Notes, Medium: params.Medium, ID: params.ID, Version: version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, store.classifyMissingWrite(ctx, id)
	}
	if err != nil {
		return Document{}, mapDocumentPersistenceError("update document", err)
	}
	return store.Get(ctx, id)
}

func (store *PostgresStore) Duplicate(ctx context.Context, newID, sourceID Identifier) (Document, error) {
	return Document{}, ErrDuplicateNotSupported
}

func (store *PostgresStore) Delete(ctx context.Context, id Identifier, version int64) error {
	if version <= 0 {
		return ErrConflict
	}
	if err := store.queries.DeleteDocumentCurrentUseForDelete(ctx, databaseUUID(id)); err != nil {
		return fmt.Errorf("clear document current use before delete: %w", err)
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
	if errors.Is(err, pgx.ErrNoRows) {
		return CurrentUse{}, ErrCurrentUseUnsupported
	}
	if err != nil {
		return CurrentUse{}, mapDocumentPersistenceError("assign document current use", err)
	}
	return currentUseFromDatabase(value.HolderProfileID, nil, value.AssignedAt)
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
	currentUse, err := currentUseFromDatabase(value.HolderProfileID, nil, value.AssignedAt)
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

func (store *PostgresStore) upsertPresence(ctx context.Context, values Values, keepExistingNumber bool) (dbgen.DocumentPresence, error) {
	existing, err := store.queries.GetDocumentPresence(ctx, dbgen.GetDocumentPresenceParams{
		ProfileID: profileUUID(values.OwnerProfileID), DocumentTypeID: databaseUUID(values.TypeID),
	})
	var existingState *PresenceState
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return dbgen.DocumentPresence{}, fmt.Errorf("get document presence: %w", err)
	}
	if err == nil {
		existingState = &PresenceState{Claim: Claim(existing.Claim), Identifier: stringValue(existing.IdentifierValue)}
	}
	claim, identifier := PresenceWrite(values.Identifier, existingState, keepExistingNumber)
	presenceID, err := NewIdentifier()
	if err != nil {
		return dbgen.DocumentPresence{}, fmt.Errorf("generate document presence identifier: %w", err)
	}
	if existingState != nil {
		presenceID = Identifier(existing.ID.Bytes)
	}
	presence, err := store.queries.UpsertDocumentPresence(ctx, dbgen.UpsertDocumentPresenceParams{
		ID: databaseUUID(presenceID), ProfileID: profileUUID(values.OwnerProfileID),
		Claim: string(claim), IdentifierValue: optionalString(identifier), DocumentTypeID: databaseUUID(values.TypeID),
	})
	if err != nil {
		return dbgen.DocumentPresence{}, mapDocumentPersistenceError("upsert document presence", err)
	}
	return presence, nil
}

func (store *PostgresStore) UpsertPresence(ctx context.Context, owner profile.Identifier, typeID Identifier, claim Claim, identifier string) (Presence, error) {
	definition, err := store.GetType(ctx, typeID)
	if err != nil {
		return Presence{}, err
	}
	if !definition.Values.Active {
		return Presence{}, ErrTypeInactive
	}
	existing, err := store.queries.GetDocumentPresence(ctx, dbgen.GetDocumentPresenceParams{
		ProfileID: profileUUID(owner), DocumentTypeID: databaseUUID(typeID),
	})
	hasExemplar := false
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Presence{}, fmt.Errorf("get document presence: %w", err)
	}
	if err == nil {
		count, countErr := store.queries.CountExemplarsByPresence(ctx, existing.ID)
		if countErr != nil {
			return Presence{}, fmt.Errorf("count document exemplars: %w", countErr)
		}
		hasExemplar = count > 0
	}
	if claim == ClaimInformedNumber {
		classified := ClassifyIdentifier(identifier)
		if classified.Action == IdentifierDelete && strings.TrimSpace(identifier) != "" {
			return Presence{}, &ValidationError{Fields: []FieldError{{Field: "identifier_value", Code: "refused"}}}
		}
		identifier = classified.Number
	}
	written, identifier, err := PresenceClaimWrite(claim, identifier, hasExemplar)
	if err != nil {
		return Presence{}, err
	}
	if written == ClaimInformedNumber && definition.Values.ValidationRegex != "" {
		if matched, matchErr := regexp.MatchString(definition.Values.ValidationRegex, identifier); matchErr != nil || !matched {
			return Presence{}, &ValidationError{Fields: []FieldError{{Field: "identifier_value", Code: "invalid_format"}}}
		}
	}
	presenceID, err := NewIdentifier()
	if err != nil {
		return Presence{}, fmt.Errorf("generate document presence identifier: %w", err)
	}
	if existing.ID.Valid {
		presenceID = Identifier(existing.ID.Bytes)
	}
	presence, err := store.queries.UpsertDocumentPresence(ctx, dbgen.UpsertDocumentPresenceParams{
		ID: databaseUUID(presenceID), ProfileID: profileUUID(owner),
		Claim: string(written), IdentifierValue: optionalString(identifier), DocumentTypeID: databaseUUID(typeID),
	})
	if err != nil {
		return Presence{}, mapDocumentPersistenceError("upsert document presence", err)
	}
	return presenceFromDatabase(presence)
}

func presenceFromDatabase(value dbgen.DocumentPresence) (Presence, error) {
	id, err := identifierFromDatabase(value.ID, "document presence")
	if err != nil {
		return Presence{}, err
	}
	profileID, err := profileIdentifierFromDatabase(value.ProfileID, "document presence profile")
	if err != nil {
		return Presence{}, err
	}
	typeID, err := identifierFromDatabase(value.DocumentTypeID, "document presence type")
	if err != nil {
		return Presence{}, err
	}
	return Presence{
		ID: id, ProfileID: profileID, TypeID: typeID, Claim: Claim(value.Claim),
		Identifier: stringValue(value.IdentifierValue), Version: value.Version,
	}, nil
}

func createDatabaseParams(id Identifier, presenceID pgtype.UUID, values Values) dbgen.CreateDocumentParams {
	return dbgen.CreateDocumentParams{
		ID: databaseUUID(id), PresenceID: presenceID, Medium: string(values.Medium),
		IdleCustody: optionalString(string(values.IdleCustody)), DocumentDate: optionalDate(values.DocumentDate),
		ValidUntil: optionalDate(values.ValidUntil), Notes: optionalString(values.Notes),
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

func documentFromGetRow(value dbgen.GetDocumentByIDRow) (Document, error) {
	return documentFromJoinedRow(value.ID, value.PresenceID, value.OwnerProfileID, value.OwnerFullName, value.DocumentTypeID, value.IdentifierValue,
		value.DocumentDate, value.ValidUntil, value.Notes, value.Medium, value.IdleCustody, value.Version, value.CreatedAt, value.UpdatedAt,
		value.TypeTechnicalKey, value.TypeLabel, value.TypeActive, value.UniquenessPolicy, value.TypeValidationRegex,
		value.TypeDateRequired, value.TypeVersion, value.TypeCreatedAt, value.TypeUpdatedAt,
		value.CurrentHolderProfileID, value.CurrentHolderFullName, value.CurrentAssignedAt)
}

func documentFromListRow(value dbgen.ListDocumentsRow) (Document, error) {
	return documentFromJoinedRow(value.ID, value.PresenceID, value.OwnerProfileID, value.OwnerFullName, value.DocumentTypeID, value.IdentifierValue,
		value.DocumentDate, value.ValidUntil, value.Notes, value.Medium, value.IdleCustody, value.Version, value.CreatedAt, value.UpdatedAt,
		value.TypeTechnicalKey, value.TypeLabel, value.TypeActive, value.UniquenessPolicy, value.TypeValidationRegex,
		value.TypeDateRequired, value.TypeVersion, value.TypeCreatedAt, value.TypeUpdatedAt,
		value.CurrentHolderProfileID, value.CurrentHolderFullName, value.CurrentAssignedAt)
}

func documentFromJoinedRow(idValue, presenceValue, ownerValue pgtype.UUID, ownerFullName string, typeValue pgtype.UUID, identifier *string, documentDate, validUntil pgtype.Date,
	notes *string, medium string, idleCustody *string, version int64, createdValue, updatedValue pgtype.Timestamptz,
	typeTechnicalKey, typeLabel string, typeActive bool, uniquenessPolicy string, validationRegex *string, dateRequired bool,
	typeVersion int64, typeCreatedValue, typeUpdatedValue pgtype.Timestamptz,
	holderValue pgtype.UUID, holderFullName *string, assignedValue pgtype.Timestamptz,
) (Document, error) {
	definition, err := typeFromDatabase(dbgen.DocumentType{
		ID: typeValue, TechnicalKey: typeTechnicalKey, Label: typeLabel, Active: typeActive,
		UniquenessPolicy: uniquenessPolicy, ValidationRegex: validationRegex, DateRequired: dateRequired,
		Version: typeVersion, CreatedAt: typeCreatedValue, UpdatedAt: typeUpdatedValue,
	})
	if err != nil {
		return Document{}, err
	}
	id, err := identifierFromDatabase(idValue, "document")
	if err != nil {
		return Document{}, err
	}
	ownerID, err := profileIdentifierFromDatabase(ownerValue, "document owner")
	if err != nil {
		return Document{}, err
	}
	createdAt, err := timeFromDatabase(createdValue, "document created_at")
	if err != nil {
		return Document{}, err
	}
	updatedAt, err := timeFromDatabase(updatedValue, "document updated_at")
	if err != nil {
		return Document{}, err
	}
	_ = presenceValue
	normalized, err := NormalizeStored(Values{
		OwnerProfileID: ownerID, TypeID: definition.ID, Identifier: stringValue(identifier),
		DocumentDate: dateValue(documentDate), ValidUntil: dateValue(validUntil), Notes: stringValue(notes),
		Medium: Medium(medium), IdleCustody: IdleCustody(stringValue(idleCustody)),
	}, definition)
	if err != nil {
		return Document{}, fmt.Errorf("map document from database: %w", err)
	}
	inUse := holderValue.Valid
	document := Document{
		ID: id, Values: normalized, OwnerFullName: ownerFullName, Type: definition,
		Status:  OperationalStatus(normalized.Medium, inUse, normalized.IdleCustody),
		Version: version, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	if inUse {
		currentUse, currentErr := currentUseFromDatabase(holderValue, holderFullName, assignedValue)
		if currentErr != nil {
			return Document{}, currentErr
		}
		document.CurrentUse = &currentUse
	}
	return document, nil
}

func currentUseFromDatabase(holder pgtype.UUID, holderFullName *string, assignedAt pgtype.Timestamptz) (CurrentUse, error) {
	holderID, err := profileIdentifierFromDatabase(holder, "document current holder")
	if err != nil {
		return CurrentUse{}, err
	}
	assigned, err := timeFromDatabase(assignedAt, "document current use assigned_at")
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
		case "23514":
			return ErrCurrentUseUnsupported
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func databaseUUID(identifier Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: identifier, Valid: true}
}

func documentUUIDList(ids []Identifier) []pgtype.UUID {
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

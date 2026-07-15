package document

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeDocumentQueries struct {
	typeValue         dbgen.DocumentType
	documentValue     dbgen.Document
	currentUseValue   dbgen.DocumentCurrentUse
	createdTypeParams dbgen.CreateDocumentTypeParams
	createdParams     dbgen.CreateDocumentParams
	updatedTypeParams dbgen.UpdateDocumentTypeParams
	assignedParams    dbgen.AssignDocumentCurrentUseParams
	hasDocuments      bool
	updateTypeErr     error
	getTypeErr        error
	getDocumentErr    error
	assignErr         error
}

func (q *fakeDocumentQueries) CreateDocumentType(_ context.Context, p dbgen.CreateDocumentTypeParams) (dbgen.DocumentType, error) {
	q.createdTypeParams = p
	return q.typeValue, nil
}
func (q *fakeDocumentQueries) GetDocumentTypeByID(context.Context, pgtype.UUID) (dbgen.DocumentType, error) {
	return q.typeValue, q.getTypeErr
}
func (q *fakeDocumentQueries) CountDocumentTypes(context.Context, dbgen.CountDocumentTypesParams) (int64, error) {
	return 0, nil
}
func (q *fakeDocumentQueries) ListDocumentTypes(context.Context, dbgen.ListDocumentTypesParams) ([]dbgen.DocumentType, error) {
	return nil, nil
}
func (q *fakeDocumentQueries) DocumentTypeHasDocuments(context.Context, pgtype.UUID) (bool, error) {
	return q.hasDocuments, nil
}
func (q *fakeDocumentQueries) UpdateDocumentType(_ context.Context, p dbgen.UpdateDocumentTypeParams) (dbgen.DocumentType, error) {
	q.updatedTypeParams = p
	return q.typeValue, q.updateTypeErr
}
func (q *fakeDocumentQueries) DeleteDocumentType(context.Context, dbgen.DeleteDocumentTypeParams) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}
func (q *fakeDocumentQueries) CreateDocument(_ context.Context, p dbgen.CreateDocumentParams) (dbgen.Document, error) {
	q.createdParams = p
	return q.documentValue, nil
}
func (q *fakeDocumentQueries) GetDocumentByID(context.Context, pgtype.UUID) (dbgen.GetDocumentByIDRow, error) {
	return dbgen.GetDocumentByIDRow{}, q.getDocumentErr
}
func (q *fakeDocumentQueries) CountDocuments(context.Context, dbgen.CountDocumentsParams) (int64, error) {
	return 0, nil
}
func (q *fakeDocumentQueries) ListDocuments(context.Context, dbgen.ListDocumentsParams) ([]dbgen.ListDocumentsRow, error) {
	return nil, nil
}
func (q *fakeDocumentQueries) UpdateDocument(context.Context, dbgen.UpdateDocumentParams) (dbgen.Document, error) {
	return q.documentValue, nil
}
func (q *fakeDocumentQueries) DuplicateDocument(context.Context, dbgen.DuplicateDocumentParams) (dbgen.Document, error) {
	return q.documentValue, nil
}
func (q *fakeDocumentQueries) DeleteDocument(context.Context, dbgen.DeleteDocumentParams) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}
func (q *fakeDocumentQueries) AssignDocumentCurrentUse(_ context.Context, p dbgen.AssignDocumentCurrentUseParams) (dbgen.DocumentCurrentUse, error) {
	q.assignedParams = p
	return q.currentUseValue, q.assignErr
}
func (q *fakeDocumentQueries) ReturnDocumentCurrentUse(context.Context, pgtype.UUID) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}
func (q *fakeDocumentQueries) GetDocumentCurrentUse(context.Context, pgtype.UUID) (dbgen.DocumentCurrentUse, error) {
	return q.currentUseValue, nil
}

func TestPostgresStoreCreatesNormalizedDocumentType(t *testing.T) {
	id, _ := NewIdentifier()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	queries := &fakeDocumentQueries{typeValue: databaseDocumentType(id, now)}
	store := &PostgresStore{queries: queries}

	value, err := store.CreateType(context.Background(), id, TypeValues{
		TechnicalKey: "  RG_GERAL ", Label: "  Registro   Geral ", Active: true,
		UniquenessPolicy: UniquenessPerProfile, ValidationRegex: `^[A-Z0-9]+$`, DateRequired: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if queries.createdTypeParams.TechnicalKey != "rg_geral" || queries.createdTypeParams.Label != "Registro Geral" || value.ID != id {
		t.Fatalf("params = %#v, value = %#v", queries.createdTypeParams, value)
	}
}

func TestPostgresStoreProtectsTypeRulesAfterUse(t *testing.T) {
	id, _ := NewIdentifier()
	now := time.Now().UTC()
	queries := &fakeDocumentQueries{typeValue: databaseDocumentType(id, now), hasDocuments: true}
	store := &PostgresStore{queries: queries}
	changed := TypeValues{TechnicalKey: "rg_geral", Label: "Registro Geral", Active: true, UniquenessPolicy: UniquenessGlobalByType, ValidationRegex: `^[A-Z0-9]+$`, DateRequired: true}
	if _, err := store.UpdateType(context.Background(), id, 1, changed); !errors.Is(err, ErrTypeInUse) {
		t.Fatalf("update type error = %v", err)
	}
	changed = queriesTypeValues(queries.typeValue)
	changed.TechnicalKey = "other_key"
	if _, err := store.UpdateType(context.Background(), id, 1, changed); !errors.Is(err, ErrTechnicalKeyImmutable) {
		t.Fatalf("technical key error = %v", err)
	}
}

func TestPostgresStoreCreatesDocumentPreservingIdentifier(t *testing.T) {
	typeID, _ := NewIdentifier()
	documentID, _ := NewIdentifier()
	ownerID, _ := profile.NewIdentifier()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	queries := &fakeDocumentQueries{
		typeValue: databaseDocumentType(typeID, now),
		documentValue: dbgen.Document{
			ID: databaseUUID(documentID), OwnerProfileID: profileUUID(ownerID), DocumentTypeID: databaseUUID(typeID),
			IdentifierValue: "00AB-009", UniquenessPolicy: string(UniquenessPerProfile),
			DocumentDate: pgtype.Date{Time: now, Valid: true}, RecordState: string(RecordCurrent), Version: 1,
			CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		},
	}
	store := &PostgresStore{queries: queries}
	value, err := store.Create(context.Background(), documentID, Values{OwnerProfileID: ownerID, TypeID: typeID, Identifier: "  00AB-009  ", DocumentDate: "2026-07-15"})
	if err != nil {
		t.Fatal(err)
	}
	if queries.createdParams.IdentifierValue != "00AB-009" || value.Values.Identifier != "00AB-009" || value.Status != StatusAvailable {
		t.Fatalf("params = %#v, value = %#v", queries.createdParams, value)
	}
}

func TestPostgresStoreClassifiesOptimisticTypeWrites(t *testing.T) {
	id, _ := NewIdentifier()
	queries := &fakeDocumentQueries{typeValue: databaseDocumentType(id, time.Now()), updateTypeErr: pgx.ErrNoRows}
	store := &PostgresStore{queries: queries}
	if _, err := store.UpdateType(context.Background(), id, 1, queriesTypeValues(queries.typeValue)); !errors.Is(err, ErrTypeConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	queries.getTypeErr = pgx.ErrNoRows
	if _, err := store.UpdateType(context.Background(), id, 1, queriesTypeValues(queries.typeValue)); !errors.Is(err, ErrTypeNotFound) {
		t.Fatalf("not found error = %v", err)
	}
}

func TestPostgresStoreAssignsCurrentUse(t *testing.T) {
	documentID, _ := NewIdentifier()
	holderID, _ := profile.NewIdentifier()
	now := time.Now().UTC()
	queries := &fakeDocumentQueries{currentUseValue: dbgen.DocumentCurrentUse{DocumentID: databaseUUID(documentID), HolderProfileID: profileUUID(holderID), AssignedAt: pgtype.Timestamptz{Time: now, Valid: true}}}
	store := &PostgresStore{queries: queries}
	value, err := store.AssignCurrentUse(context.Background(), documentID, holderID)
	if err != nil {
		t.Fatal(err)
	}
	if queries.assignedParams.DocumentID.Bytes != documentID || value.HolderProfileID != holderID || !value.AssignedAt.Equal(now) {
		t.Fatalf("params = %#v, value = %#v", queries.assignedParams, value)
	}
}

func databaseDocumentType(id Identifier, now time.Time) dbgen.DocumentType {
	regex := `^[A-Z0-9]+$`
	return dbgen.DocumentType{ID: databaseUUID(id), TechnicalKey: "rg_geral", Label: "Registro Geral", Active: true,
		UniquenessPolicy: string(UniquenessPerProfile), ValidationRegex: &regex, DateRequired: true, Version: 1,
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true}}
}

func queriesTypeValues(value dbgen.DocumentType) TypeValues {
	return TypeValues{TechnicalKey: value.TechnicalKey, Label: value.Label, Active: value.Active,
		UniquenessPolicy: UniquenessPolicy(value.UniquenessPolicy), ValidationRegex: stringValue(value.ValidationRegex), DateRequired: value.DateRequired}
}

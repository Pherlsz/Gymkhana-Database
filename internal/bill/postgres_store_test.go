package bill

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

type fakeBillQueries struct {
	typeValue         dbgen.BillType
	billValue         dbgen.Bill
	getBillValue      dbgen.GetBillByIDRow
	currentUseValue   dbgen.BillCurrentUse
	createdTypeParams dbgen.CreateBillTypeParams
	createdParams     dbgen.CreateBillParams
	assignedParams    dbgen.AssignBillCurrentUseParams
	hasBills          bool
	updateTypeErr     error
	getTypeErr        error
	getBillErr        error
	assignErr         error
}

func (q *fakeBillQueries) CreateBillType(_ context.Context, p dbgen.CreateBillTypeParams) (dbgen.BillType, error) {
	q.createdTypeParams = p
	return q.typeValue, nil
}
func (q *fakeBillQueries) GetBillTypeByID(context.Context, pgtype.UUID) (dbgen.BillType, error) {
	return q.typeValue, q.getTypeErr
}
func (q *fakeBillQueries) CountBillTypes(context.Context, dbgen.CountBillTypesParams) (int64, error) {
	return 0, nil
}
func (q *fakeBillQueries) CountBillsByType(context.Context) ([]dbgen.CountBillsByTypeRow, error) {
	return nil, nil
}
func (q *fakeBillQueries) ListBillTypes(context.Context, dbgen.ListBillTypesParams) ([]dbgen.BillType, error) {
	return nil, nil
}
func (q *fakeBillQueries) BillTypeHasBills(context.Context, pgtype.UUID) (bool, error) {
	return q.hasBills, nil
}
func (q *fakeBillQueries) UpdateBillType(context.Context, dbgen.UpdateBillTypeParams) (dbgen.BillType, error) {
	return q.typeValue, q.updateTypeErr
}
func (q *fakeBillQueries) DeleteBillType(context.Context, dbgen.DeleteBillTypeParams) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}
func (q *fakeBillQueries) CreateBill(_ context.Context, p dbgen.CreateBillParams) (dbgen.Bill, error) {
	q.createdParams = p
	return q.billValue, nil
}
func (q *fakeBillQueries) GetBillByID(context.Context, pgtype.UUID) (dbgen.GetBillByIDRow, error) {
	return q.getBillValue, q.getBillErr
}
func (q *fakeBillQueries) CountBills(context.Context, dbgen.CountBillsParams) (int64, error) {
	return 0, nil
}
func (q *fakeBillQueries) ListBills(context.Context, dbgen.ListBillsParams) ([]dbgen.ListBillsRow, error) {
	return nil, nil
}
func (q *fakeBillQueries) UpdateBill(context.Context, dbgen.UpdateBillParams) (dbgen.Bill, error) {
	return q.billValue, nil
}
func (q *fakeBillQueries) DuplicateBill(context.Context, dbgen.DuplicateBillParams) (dbgen.Bill, error) {
	return q.billValue, nil
}
func (q *fakeBillQueries) DeleteBill(context.Context, dbgen.DeleteBillParams) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}
func (q *fakeBillQueries) AssignBillCurrentUse(_ context.Context, p dbgen.AssignBillCurrentUseParams) (dbgen.BillCurrentUse, error) {
	q.assignedParams = p
	return q.currentUseValue, q.assignErr
}
func (q *fakeBillQueries) ReturnBillCurrentUse(context.Context, pgtype.UUID) (pgtype.UUID, error) {
	return pgtype.UUID{}, nil
}
func (q *fakeBillQueries) GetBillCurrentUse(context.Context, pgtype.UUID) (dbgen.BillCurrentUse, error) {
	return q.currentUseValue, nil
}

func TestPostgresStoreCreatesNormalizedBillType(t *testing.T) {
	id, _ := NewIdentifier()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	queries := &fakeBillQueries{typeValue: databaseBillType(id, now)}
	store := &PostgresStore{queries: queries}
	value, err := store.CreateType(context.Background(), id, TypeValues{TechnicalKey: "  ENERGY_BILL ", Label: "  Conta   de Energia ", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if queries.createdTypeParams.TechnicalKey != "energy_bill" || queries.createdTypeParams.Label != "Conta de Energia" || value.ID != id {
		t.Fatalf("params = %#v, value = %#v", queries.createdTypeParams, value)
	}
}

func TestPostgresStoreProtectsTechnicalKeyAfterUse(t *testing.T) {
	id, _ := NewIdentifier()
	now := time.Now().UTC()
	queries := &fakeBillQueries{typeValue: databaseBillType(id, now), hasBills: true}
	store := &PostgresStore{queries: queries}
	changed := billTypeValues(queries.typeValue)
	changed.TechnicalKey = "other_key"
	if _, err := store.UpdateType(context.Background(), id, 1, changed); !errors.Is(err, ErrTechnicalKeyImmutable) {
		t.Fatalf("technical key error = %v", err)
	}
}

func TestPostgresStoreCreatesBillWithDecimalAndPrintedData(t *testing.T) {
	typeID, _ := NewIdentifier()
	billID, _ := NewIdentifier()
	ownerID, _ := profile.NewIdentifier()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	amount, err := optionalNumeric("123.40")
	if err != nil {
		t.Fatal(err)
	}
	holder := "Ana da Silva"
	address := "Rua Original, 001"
	reference := "000A-99"
	competence := "2026-07"
	currency := "BRL"
	queries := &fakeBillQueries{
		typeValue: databaseBillType(typeID, now),
		billValue: dbgen.Bill{ID: databaseUUID(billID), OwnerProfileID: profileUUID(ownerID), BillTypeID: databaseUUID(typeID),
			PrintedHolderName: &holder, PrintedAddress: &address, ReferenceValue: &reference, Competence: &competence,
			Amount: amount, Currency: &currency, Medium: string(MediumPhysical), IdleCustody: optionalString(string(IdleCustodyOrganization)), Version: 1,
			CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true}},
		getBillValue: dbgen.GetBillByIDRow{ID: databaseUUID(billID), OwnerProfileID: profileUUID(ownerID), BillTypeID: databaseUUID(typeID),
			PrintedHolderName: &holder, PrintedAddress: &address, ReferenceValue: &reference, Competence: &competence,
			Amount: amount, Currency: &currency, Medium: string(MediumPhysical), IdleCustody: optionalString(string(IdleCustodyOrganization)), Version: 1,
			CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
			OwnerFullName: "Ana da Silva", TypeTechnicalKey: "energy_bill", TypeLabel: "Conta de Energia", TypeActive: true,
			TypeVersion: 1, TypeCreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, TypeUpdatedAt: pgtype.Timestamptz{Time: now, Valid: true}},
	}
	store := &PostgresStore{queries: queries}
	value, err := store.Create(context.Background(), billID, Values{OwnerProfileID: ownerID, TypeID: typeID,
		PrintedHolderName: "  Ana   da Silva ", PrintedAddress: " Rua   Original, 001 ", Reference: "  000A-99  ",
		Competence: "2026-07", Amount: "000123.4", Currency: "brl", Medium: MediumPhysical})
	if err != nil {
		t.Fatal(err)
	}
	if value.Values.Amount != "123.40" || value.Values.Reference != "000A-99" || value.Values.Currency != "BRL" || value.OwnerFullName != "Ana da Silva" {
		t.Fatalf("value = %#v", value)
	}
	storedAmount, err := numericValue(queries.createdParams.Amount)
	if err != nil || storedAmount != "123.40" || stringValue(queries.createdParams.PrintedHolderName) != "Ana da Silva" {
		t.Fatalf("params = %#v, amount = %q, error = %v", queries.createdParams, storedAmount, err)
	}
}

func TestPostgresStoreClassifiesOptimisticTypeWrites(t *testing.T) {
	id, _ := NewIdentifier()
	queries := &fakeBillQueries{typeValue: databaseBillType(id, time.Now()), updateTypeErr: pgx.ErrNoRows}
	store := &PostgresStore{queries: queries}
	if _, err := store.UpdateType(context.Background(), id, 1, billTypeValues(queries.typeValue)); !errors.Is(err, ErrTypeConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	queries.getTypeErr = pgx.ErrNoRows
	if _, err := store.UpdateType(context.Background(), id, 1, billTypeValues(queries.typeValue)); !errors.Is(err, ErrTypeNotFound) {
		t.Fatalf("not found error = %v", err)
	}
}

func TestPostgresStoreAssignsOnlyPhysicalCurrentUse(t *testing.T) {
	typeID, _ := NewIdentifier()
	billID, _ := NewIdentifier()
	ownerID, _ := profile.NewIdentifier()
	holderID, _ := profile.NewIdentifier()
	now := time.Now().UTC()
	queries := &fakeBillQueries{
		typeValue:       databaseBillType(typeID, now),
		getBillValue:    databaseJoinedBill(billID, ownerID, typeID, now, MediumPhysical),
		currentUseValue: dbgen.BillCurrentUse{BillID: databaseUUID(billID), HolderProfileID: profileUUID(holderID), AssignedAt: pgtype.Timestamptz{Time: now, Valid: true}},
	}
	store := &PostgresStore{queries: queries}
	value, err := store.AssignCurrentUse(context.Background(), billID, holderID)
	if err != nil {
		t.Fatal(err)
	}
	if queries.assignedParams.BillID.Bytes != billID || value.HolderProfileID != holderID {
		t.Fatalf("params = %#v, value = %#v", queries.assignedParams, value)
	}
	queries.getBillValue.Medium = string(MediumDigital)
	queries.getBillValue.IdleCustody = nil
	if _, err := store.AssignCurrentUse(context.Background(), billID, holderID); !errors.Is(err, ErrCurrentUseUnsupported) {
		t.Fatalf("unsupported error = %v", err)
	}
}

func databaseBillType(id Identifier, now time.Time) dbgen.BillType {
	return dbgen.BillType{ID: databaseUUID(id), TechnicalKey: "energy_bill", Label: "Conta de Energia", Active: true,
		Version: 1, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true}}
}

func billTypeValues(value dbgen.BillType) TypeValues {
	return TypeValues{TechnicalKey: value.TechnicalKey, Label: value.Label, Active: value.Active}
}

func databaseJoinedBill(id Identifier, owner profile.Identifier, typeID Identifier, now time.Time, medium Medium) dbgen.GetBillByIDRow {
	return dbgen.GetBillByIDRow{ID: databaseUUID(id), OwnerProfileID: profileUUID(owner), BillTypeID: databaseUUID(typeID),
		Medium: string(medium), IdleCustody: optionalIdleCustody(medium), Version: 1, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		TypeTechnicalKey: "energy_bill", TypeLabel: "Conta de Energia", TypeActive: true,
		TypeVersion: 1, TypeCreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, TypeUpdatedAt: pgtype.Timestamptz{Time: now, Valid: true}}
}

func optionalIdleCustody(medium Medium) *string {
	if medium != MediumPhysical {
		return nil
	}
	value := string(IdleCustodyOrganization)
	return &value
}

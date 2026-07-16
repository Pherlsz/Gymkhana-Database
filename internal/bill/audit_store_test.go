package bill

import (
	"context"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type fakeBillAuditQueries struct {
	fakeBillQueries
	auditParams dbgen.RecordBillAuditEventParams
	auditErr    error
}

func (queries *fakeBillAuditQueries) RecordBillAuditEvent(_ context.Context, params dbgen.RecordBillAuditEventParams) error {
	queries.auditParams = params
	return queries.auditErr
}

func TestPostgresStoreRecordsBillAuditMetadata(t *testing.T) {
	actorID, _ := auth.NewIdentifier()
	billID, _ := NewIdentifier()
	sourceID, _ := NewIdentifier()
	typeID, _ := NewIdentifier()
	holderID, _ := profile.NewIdentifier()
	eventID, _ := NewIdentifier()
	queries := &fakeBillAuditQueries{}
	store := &PostgresStore{queries: queries}

	err := store.RecordAuditEvent(context.Background(), AuditEvent{
		ID:              eventID,
		ActorUserID:     actorID,
		BillID:          &billID,
		SourceBillID:    &sourceID,
		BillTypeID:      &typeID,
		HolderProfileID: &holderID,
		EventType:       AuditEventUseAssigned,
		Outcome:         auth.AuditOutcomeSuccess,
		RequestID:       "request-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	params := queries.auditParams
	if params.ID.Bytes != eventID || params.ActorUserID.Bytes != actorID || !params.BillID.Valid || params.BillID.Bytes != billID {
		t.Fatalf("identity params = %#v", params)
	}
	if !params.SourceBillID.Valid || params.SourceBillID.Bytes != sourceID || !params.BillTypeID.Valid || params.BillTypeID.Bytes != typeID {
		t.Fatalf("bill metadata params = %#v", params)
	}
	if !params.HolderProfileID.Valid || params.HolderProfileID.Bytes != holderID || params.EventType != string(AuditEventUseAssigned) || params.Outcome != string(auth.AuditOutcomeSuccess) || params.RequestID != "request-id" {
		t.Fatalf("audit params = %#v", params)
	}
}

package document

import (
	"context"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres/dbgen"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type fakeDocumentAuditQueries struct {
	fakeDocumentQueries
	auditParams dbgen.RecordDocumentAuditEventParams
	auditErr    error
}

func (queries *fakeDocumentAuditQueries) RecordDocumentAuditEvent(_ context.Context, params dbgen.RecordDocumentAuditEventParams) error {
	queries.auditParams = params
	return queries.auditErr
}

func TestPostgresStoreRecordsDocumentAuditMetadata(t *testing.T) {
	actorID, _ := auth.NewIdentifier()
	documentID, _ := NewIdentifier()
	sourceID, _ := NewIdentifier()
	typeID, _ := NewIdentifier()
	holderID, _ := profile.NewIdentifier()
	eventID, _ := NewIdentifier()
	queries := &fakeDocumentAuditQueries{}
	store := &PostgresStore{queries: queries}

	err := store.RecordAuditEvent(context.Background(), AuditEvent{
		ID:               eventID,
		ActorUserID:      actorID,
		DocumentID:       &documentID,
		SourceDocumentID: &sourceID,
		DocumentTypeID:   &typeID,
		HolderProfileID:  &holderID,
		EventType:        AuditEventUseAssigned,
		Outcome:          auth.AuditOutcomeSuccess,
		RequestID:        "request-id",
	})
	if err != nil {
		t.Fatal(err)
	}
	params := queries.auditParams
	if params.ID.Bytes != eventID || params.ActorUserID.Bytes != actorID || !params.DocumentID.Valid || params.DocumentID.Bytes != documentID {
		t.Fatalf("identity params = %#v", params)
	}
	if !params.SourceDocumentID.Valid || params.SourceDocumentID.Bytes != sourceID || !params.DocumentTypeID.Valid || params.DocumentTypeID.Bytes != typeID {
		t.Fatalf("document metadata params = %#v", params)
	}
	if !params.HolderProfileID.Valid || params.HolderProfileID.Bytes != holderID || params.EventType != string(AuditEventUseAssigned) || params.Outcome != string(auth.AuditOutcomeSuccess) || params.RequestID != "request-id" {
		t.Fatalf("audit params = %#v", params)
	}
}

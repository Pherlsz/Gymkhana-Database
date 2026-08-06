//go:build integration

package ocr

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresOCRIdempotencyConcurrencyReviewApplyRecoveryAndAttachmentLifecycle(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for OCR PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	actorID, _ := auth.NewIdentifier()
	profileID, _ := attachment.NewIdentifier()
	typeID, _ := attachment.NewIdentifier()
	documentID, _ := attachment.NewIdentifier()
	attachmentID, _ := attachment.NewIdentifier()
	key := "ocr_" + strings.ReplaceAll(typeID.String(), "-", "")[:20]
	githubID := now.UnixNano()
	if githubID < 1 {
		githubID = -githubID + 1
	}
	if _, err := pool.Exec(ctx, `INSERT INTO app_users
(id,github_user_id,github_login,display_name,role,active)
VALUES($1,$2,$3,'OCR integration actor','EXTERNAL',true)`, authDatabaseUUID(actorID), githubID, key); err != nil {
		t.Fatalf("insert actor: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profiles(id,full_name) VALUES($1,'OCR Integration Profile')`, profileID.String()); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO document_types
(id,technical_key,label,active,uniqueness_policy,date_required)
VALUES($1,$2,'OCR integration type',true,'NONE',false)`, typeID.String(), key); err != nil {
		t.Fatalf("insert document type: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO documents
(id,owner_profile_id,document_type_id,identifier_value,uniqueness_policy)
VALUES($1,$2,$3,$4,'NONE')`, documentID.String(), profileID.String(), typeID.String(), key); err != nil {
		t.Fatalf("insert document: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO attachments
(id,owner_kind,document_id,original_filename,declared_mime,detected_mime,byte_size,sha256,object_key,lifecycle_state,version,created_at,updated_at)
VALUES($1,'DOCUMENT',$2,'private.pdf','application/pdf','application/pdf',100,$3,$4,'ACTIVE',1,$5,$5)`,
		attachmentID.String(), documentID.String(), bytes.Repeat([]byte{1}, 32), "integration/ocr/"+attachmentID.String(), now); err != nil {
		t.Fatalf("insert attachment: %v", err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM attachments WHERE id=$1`, attachmentID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM documents WHERE id=$1`, documentID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM document_types WHERE id=$1`, typeID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM profiles WHERE id=$1`, profileID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id=$1`, authDatabaseUUID(actorID))
	}()

	store := NewPostgresStore(pool)
	leftID, _ := NewIdentifier()
	rightID, _ := NewIdentifier()
	base := CreateJobInput{
		OwnerUserID: actorID, AttachmentID: attachmentID, IdempotencyKey: "ocr-integration-job-0001",
		RequestFingerprint: [32]byte{3}, SourceSHA256: [32]byte{1}, CatalogFingerprint: [32]byte{2},
		SourceMIME: "application/pdf", SourceBytes: 100, Now: now,
	}
	type createResult struct {
		job     Job
		created bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan createResult, 2)
	for _, id := range []Identifier{leftID, rightID} {
		go func(id Identifier) {
			<-start
			input := base
			input.ID = id
			job, created, createErr := store.CreateJob(ctx, input, now.Truncate(time.Hour), 10, 1000)
			results <- createResult{job: job, created: created, err: createErr}
		}(id)
	}
	close(start)
	left, right := <-results, <-results
	if left.err != nil || right.err != nil || left.job.ID != right.job.ID || left.created == right.created {
		t.Fatalf("concurrent create = %#v / %#v", left, right)
	}
	job := left.job
	if !left.created {
		job = right.job
	}
	replayedInput := base
	replayedInput.ID, _ = NewIdentifier()
	replayed, created, err := store.CreateJob(ctx, replayedInput, now.Truncate(time.Hour), 10, 1000)
	if err != nil || created || replayed.ID != job.ID {
		t.Fatalf("CreateJob(replay) = %#v, created=%t, error=%v", replayed, created, err)
	}

	job, err = store.AttachRiverJob(ctx, job.ID, actorID, 101, now)
	if err != nil || job.RiverJobID != 101 {
		t.Fatalf("AttachRiverJob() = %#v, error=%v", job, err)
	}
	job, claimed, err := store.ClaimJob(ctx, job.ID, now.Add(time.Second))
	if err != nil || !claimed || job.State != JobRunning || job.AttemptCount != 1 {
		t.Fatalf("ClaimJob() = %#v, claimed=%t, error=%v", job, claimed, err)
	}
	if _, err := store.RecordSourceValidated(ctx, job.ID, 2, 0, now.Add(2*time.Second)); err != nil {
		t.Fatalf("RecordSourceValidated() error = %v", err)
	}
	job, exceeded, err := store.AddProviderUsage(ctx, job.ID, actorID, 600, 500, now.Add(3*time.Second))
	if err != nil || !exceeded || job.ProviderUsage != 600 {
		t.Fatalf("AddProviderUsage() = %#v, exceeded=%t, error=%v", job, exceeded, err)
	}

	suggestionID, _ := NewIdentifier()
	field := FieldSchema{
		Key: "document.identifier", Label: "Identificador", Kind: ValueText, Required: true,
		Target: TargetReference{Kind: TargetDocument, ID: Identifier(documentID)}, TargetVersion: 1,
	}
	job, err = store.CompleteJob(ctx, CompleteJobInput{
		JobID: job.ID, PageCount: 2, Suggestions: []CompleteSuggestionInput{{
			ID: suggestionID, Ordinal: 1, Field: field, ProposedValue: "OCR-123", Evidence: Evidence{Page: 1, Excerpt: "OCR-123"},
		}}, Now: now.Add(4 * time.Second),
	})
	if err != nil || job.State != JobCompleted || job.SuggestionCount != 1 {
		t.Fatalf("CompleteJob() = %#v, error=%v", job, err)
	}
	events, err := store.ListEvents(ctx, job.ID, actorID, 0, MaximumEventPage)
	if err != nil || !events.Terminal || len(events.Events) == 0 {
		t.Fatalf("ListEvents(completed) = %#v, error=%v", events, err)
	}
	afterTerminal, err := store.ListEvents(ctx, job.ID, actorID, events.LastSequence, MaximumEventPage)
	if err != nil || !afterTerminal.Terminal || len(afterTerminal.Events) != 0 {
		t.Fatalf("ListEvents(after terminal) = %#v, error=%v", afterTerminal, err)
	}

	firstValue, secondValue := "OCR-123 conferido", "OCR-123 alternativo"
	reviewStart := make(chan struct{})
	reviews := make(chan struct {
		value Suggestion
		err   error
	}, 2)
	for _, value := range []*string{&firstValue, &secondValue} {
		go func(value *string) {
			<-reviewStart
			updated, reviewErr := store.ReviewSuggestion(ctx, ReviewSuggestionInput{
				SuggestionID: suggestionID, OwnerUserID: actorID, Action: ReviewAccept, Value: value,
				TargetVersion: 1, Version: 1, Now: now.Add(5 * time.Second),
			})
			reviews <- struct {
				value Suggestion
				err   error
			}{value: updated, err: reviewErr}
		}(value)
	}
	close(reviewStart)
	firstReview, secondReview := <-reviews, <-reviews
	var accepted Suggestion
	conflicts := 0
	for _, result := range []struct {
		value Suggestion
		err   error
	}{firstReview, secondReview} {
		if result.err == nil {
			accepted = result.value
		} else if errors.Is(result.err, ErrConflict) {
			conflicts++
		} else {
			t.Fatalf("concurrent ReviewSuggestion() error = %v", result.err)
		}
	}
	if accepted.ReviewState != ReviewAccepted || conflicts != 1 {
		t.Fatalf("concurrent review accepted=%#v conflicts=%d", accepted, conflicts)
	}

	receiptID, _ := NewIdentifier()
	receipt, created, err := store.CreateApplyReceipt(ctx, CreateApplyReceiptInput{
		ID: receiptID, JobID: job.ID, OwnerUserID: actorID, IdempotencyKey: "ocr-integration-apply-0001",
		RequestFingerprint: [32]byte{5}, Now: now.Add(6 * time.Second),
	})
	if err != nil || !created || receipt.State != ApplyRunning {
		t.Fatalf("CreateApplyReceipt() = %#v, created=%t, error=%v", receipt, created, err)
	}
	targetVersion := int64(2)
	if _, err := store.SaveApplyResults(ctx, receipt.ID, []ApplyResultInput{{
		SuggestionID: accepted.ID, Outcome: ApplyApplied, TargetVersion: &targetVersion,
	}}, now.Add(7*time.Second)); err != nil {
		t.Fatalf("SaveApplyResults() error = %v", err)
	}
	receipt, err = store.CompleteApplyReceipt(ctx, receipt.ID, actorID, now.Add(8*time.Second))
	if err != nil || receipt.State != ApplyCompleted || len(receipt.Results) != 1 || receipt.Results[0].Outcome != ApplyApplied {
		t.Fatalf("CompleteApplyReceipt() = %#v, error=%v", receipt, err)
	}

	auditID, _ := NewIdentifier()
	if err := store.SaveAudit(ctx, AuditEvent{
		ID: auditID, ActorUserID: &actorID, SuggestionID: &suggestionID, FieldKey: field.Key,
		ReviewAction: ReviewAccept, EventType: AuditSuggestionReviewed, Outcome: auth.AuditOutcomeSuccess, CreatedAt: now.Add(9 * time.Second),
	}); err != nil {
		t.Fatalf("SaveAudit() error = %v", err)
	}

	staleID, _ := NewIdentifier()
	staleInput := base
	staleInput.ID = staleID
	staleInput.IdempotencyKey = "ocr-integration-job-stale"
	staleInput.RequestFingerprint = [32]byte{7}
	staleInput.Now = now.Add(-10 * time.Minute)
	staleJob, _, err := store.CreateJob(ctx, staleInput, now.Truncate(time.Hour), 10, 1000)
	if err != nil {
		t.Fatalf("CreateJob(stale) error = %v", err)
	}
	if _, err := store.AttachRiverJob(ctx, staleJob.ID, actorID, 202, staleInput.Now); err != nil {
		t.Fatalf("AttachRiverJob(stale) error = %v", err)
	}
	if _, claimed, err := store.ClaimJob(ctx, staleJob.ID, staleInput.Now); err != nil || !claimed {
		t.Fatalf("ClaimJob(stale) claimed=%t, error=%v", claimed, err)
	}
	if affected, err := store.RecoverStaleJobs(ctx, now.Add(-5*time.Minute), now, 10); err != nil || affected != 1 {
		t.Fatalf("RecoverStaleJobs() = %d, error=%v", affected, err)
	}
	recovered, err := store.GetJobForWorker(ctx, staleJob.ID)
	if err != nil || recovered.State != JobFailed || recovered.ErrorCode != "worker_interrupted" {
		t.Fatalf("recovered job = %#v, error=%v", recovered, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE attachments SET lifecycle_state='TRASHED',deleted_at=$2,purge_after=$3,version=version+1,updated_at=$2 WHERE id=$1`,
		attachmentID.String(), now.Add(10*time.Second), now.Add(7*24*time.Hour)); err != nil {
		t.Fatalf("trash attachment: %v", err)
	}
	page, err := store.ListJobs(ctx, actorID, 100, 0)
	if err != nil || page.Total != 0 || len(page.Jobs) != 0 {
		t.Fatalf("ListJobs(trashed) = %#v, error=%v", page, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM attachments WHERE id=$1`, attachmentID.String()); err != nil {
		t.Fatalf("purge attachment: %v", err)
	}
	if _, err := store.GetJobForWorker(ctx, job.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetJobForWorker(purged) error = %v, want ErrNotFound", err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ocr_audit_events WHERE id=$1`, databaseUUID(auditID)).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("durable audit count = %d, error=%v", auditCount, err)
	}
}

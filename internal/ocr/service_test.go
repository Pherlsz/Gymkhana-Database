package ocr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func TestServiceRequiresReviewAndSeparateIdempotentApplication(t *testing.T) {
	fixture := newOCRServiceFixture(t, NewDeterministicFakeExtractor())
	job, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-job-lifecycle-0001", nil, "start")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	replayed, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-job-lifecycle-0001", nil, "replay")
	if err != nil || replayed.ID != job.ID || fixture.jobs.enqueued != 1 {
		t.Fatalf("StartJob(replay) = %#v, error=%v, enqueued=%d", replayed, err, fixture.jobs.enqueued)
	}
	if err := fixture.service.RunJob(context.Background(), job.ID); err != nil {
		t.Fatalf("RunJob() error = %v", err)
	}
	completed := fixture.store.job
	if completed.State != JobCompleted || completed.SuggestionCount != 1 || completed.ProviderUsage != 1 {
		t.Fatalf("completed job = %#v", completed)
	}
	if fixture.targets.applyCalls != 0 {
		t.Fatalf("extraction applied target %d time(s), want zero", fixture.targets.applyCalls)
	}
	if fixture.source.openCalls != 1 || fixture.source.getCalls < 2 {
		t.Fatalf("source authorization calls: open=%d get=%d, want open=1 and immediate reauthorization", fixture.source.openCalls, fixture.source.getCalls)
	}

	suggestion := fixture.store.onlySuggestion(t)
	reviewedValue := "Valor conferido"
	reviewed, err := fixture.service.ReviewSuggestion(context.Background(), fixture.actor, suggestion.ID, ReviewInput{
		Action: ReviewAccept, Value: &reviewedValue, Version: suggestion.Version,
	}, "review")
	if err != nil {
		t.Fatalf("ReviewSuggestion() error = %v", err)
	}
	if reviewed.ReviewState != ReviewAccepted || reviewed.ReviewedValue == nil || *reviewed.ReviewedValue != reviewedValue {
		t.Fatalf("reviewed suggestion = %#v", reviewed)
	}
	if fixture.targets.applyCalls != 0 {
		t.Fatal("accepting a suggestion must not mutate the target")
	}

	receipt, err := fixture.service.Apply(context.Background(), fixture.actor, job.ID, []ApplySelection{{
		SuggestionID: reviewed.ID, Version: reviewed.Version,
	}}, "ocr-apply-lifecycle-0001", "apply")
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if receipt.State != ApplyCompleted || len(receipt.Results) != 1 || receipt.Results[0].Outcome != ApplyApplied || fixture.targets.applyCalls != 1 {
		t.Fatalf("Apply() receipt=%#v applyCalls=%d", receipt, fixture.targets.applyCalls)
	}
	if fixture.targets.currentValue != reviewedValue {
		t.Fatalf("target value = %q, want %q", fixture.targets.currentValue, reviewedValue)
	}

	replayedReceipt, err := fixture.service.Apply(context.Background(), fixture.actor, job.ID, []ApplySelection{{
		SuggestionID: reviewed.ID, Version: reviewed.Version,
	}}, "ocr-apply-lifecycle-0001", "apply-replay")
	if err != nil || replayedReceipt.ID != receipt.ID || fixture.targets.applyCalls != 1 {
		t.Fatalf("Apply(replay) receipt=%#v error=%v applyCalls=%d", replayedReceipt, err, fixture.targets.applyCalls)
	}
	for _, event := range fixture.store.audits {
		if event.EventType == AuditSuggestionReviewed && (event.FieldKey != fixture.targets.field.Key || event.ReviewAction != ReviewAccept) {
			t.Fatalf("review audit = %#v", event)
		}
	}
}

func TestServiceRetriesOnlyPreProviderUnavailableWorkAndBoundsAttempts(t *testing.T) {
	fixture := newOCRServiceFixture(t, NewDeterministicFakeExtractor())
	fixture.source.failGet = map[int]error{
		2: attachment.ErrStorageUnavailable,
		3: attachment.ErrStorageUnavailable,
		4: attachment.ErrStorageUnavailable,
	}
	job, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-job-retry-0001", nil, "start")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	for attempt := 1; attempt <= MaximumAttempts; attempt++ {
		err := fixture.service.RunJob(context.Background(), job.ID)
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("RunJob(attempt %d) error = %v, want ErrUnavailable", attempt, err)
		}
		if fixture.store.job.AttemptCount != attempt {
			t.Fatalf("attempt count = %d, want %d", fixture.store.job.AttemptCount, attempt)
		}
		if attempt < MaximumAttempts && fixture.store.job.State != JobRunning {
			t.Fatalf("state after retryable attempt %d = %s, want RUNNING", attempt, fixture.store.job.State)
		}
	}
	if fixture.store.job.State != JobFailed || fixture.store.job.ErrorCode != "unavailable" {
		t.Fatalf("bounded retry terminal job = %#v", fixture.store.job)
	}
	if len(fixture.extractor.Requests()) != 0 {
		t.Fatal("provider was called during pre-provider authorization failures")
	}
}

func TestServiceDoesNotAutomaticallyRepeatAmbiguousProviderFailure(t *testing.T) {
	extractor := NewFakeExtractor(FakeExtractionStep{Err: errors.New("provider transport failed")})
	fixture := newOCRServiceFixture(t, extractor)
	job, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-job-provider-0001", nil, "start")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	if err := fixture.service.RunJob(context.Background(), job.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("RunJob() error = %v, want ErrUnavailable", err)
	}
	if fixture.store.job.State != JobFailed || len(extractor.Requests()) != 1 {
		t.Fatalf("provider failure job=%#v requests=%d", fixture.store.job, len(extractor.Requests()))
	}
	if err := fixture.service.RunJob(context.Background(), job.ID); err != nil {
		t.Fatalf("RunJob(terminal replay) error = %v", err)
	}
	if len(extractor.Requests()) != 1 {
		t.Fatalf("provider requests = %d, want one", len(extractor.Requests()))
	}
}

func TestServiceAccountsProviderUsageBeforeRejectingMalformedOutput(t *testing.T) {
	extractor := NewFakeExtractor(FakeExtractionStep{Response: ExtractionResponse{
		Suggestions: []ProviderSuggestion{{FieldKey: "unknown.field", Value: "unsafe", Evidence: Evidence{Page: 1}}},
		Usage:       17,
	}})
	fixture := newOCRServiceFixture(t, extractor)
	job, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-job-malformed-usage-0001", nil, "start")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	if err := fixture.service.RunJob(context.Background(), job.ID); !errors.Is(err, ErrMalformedProvider) {
		t.Fatalf("RunJob() error = %v, want ErrMalformedProvider", err)
	}
	if fixture.store.job.ProviderUsage != 17 || fixture.store.job.State != JobFailed {
		t.Fatalf("malformed provider job = %#v", fixture.store.job)
	}
}

func TestServiceAccountsProviderUsageBeforeHonoringConcurrentCancellation(t *testing.T) {
	fixture := newOCRServiceFixture(t, NewDeterministicFakeExtractor())
	fixture.service.extractor = extractorFunc(func(_ context.Context, _ ExtractionRequest) (ExtractionResponse, error) {
		now := fixture.service.now().UTC()
		fixture.store.job.CancelRequestedAt = &now
		return ExtractionResponse{Usage: 23}, nil
	})
	job, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-job-cancel-usage-0001", nil, "start")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	if err := fixture.service.RunJob(context.Background(), job.ID); !errors.Is(err, ErrCancelled) {
		t.Fatalf("RunJob() error = %v, want ErrCancelled", err)
	}
	if fixture.store.job.ProviderUsage != 23 || fixture.store.job.State != JobCancelled {
		t.Fatalf("cancelled provider job = %#v", fixture.store.job)
	}
}

func TestServiceRunsBoundedStaleJobRecoveryAndAuditsTheResult(t *testing.T) {
	fixture := newOCRServiceFixture(t, NewDeterministicFakeExtractor())
	fixture.store.recovered = 3
	affected, err := fixture.service.RecoverStaleJobs(context.Background())
	if err != nil || affected != 3 {
		t.Fatalf("RecoverStaleJobs() = %d, error=%v", affected, err)
	}
	if len(fixture.store.audits) != 1 || fixture.store.audits[0].EventType != AuditJobRecovered ||
		fixture.store.audits[0].AffectedCount == nil || *fixture.store.audits[0].AffectedCount != 3 {
		t.Fatalf("recovery audits = %#v", fixture.store.audits)
	}
}

type extractorFunc func(context.Context, ExtractionRequest) (ExtractionResponse, error)

func (extractor extractorFunc) Extract(ctx context.Context, request ExtractionRequest) (ExtractionResponse, error) {
	return extractor(ctx, request)
}

type ocrServiceFixture struct {
	service   *Service
	store     *memoryOCRStore
	jobs      *memoryOCRJobs
	source    *memoryOCRSource
	targets   *memoryOCRTargets
	extractor *FakeExtractor
	actor     auth.Session
}

func newOCRServiceFixture(t *testing.T, extractor *FakeExtractor) *ocrServiceFixture {
	t.Helper()
	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	user := auth.User{ID: auth.Identifier{1}, Email: "member", Role: auth.RoleExternal, Active: true}
	payload := ocrTestPNG(t, 2, 2)
	source := attachment.Attachment{
		ID: attachment.Identifier{2}, Owner: attachment.OwnerReference{Kind: attachment.OwnerBill, ID: attachment.Identifier{3}},
		OriginalFileName: "conta.png", DeclaredMIME: "image/png", DetectedMIME: "image/png", ByteSize: int64(len(payload)),
		SHA256: sha256Bytes(payload), ObjectKey: "attachments/private/conta.png", LifecycleState: attachment.LifecycleActive,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	field := FieldSchema{
		Key: "bill.printed_holder_name", Label: "Titular impresso", Kind: ValueText,
		Target: TargetReference{Kind: TargetBill, ID: Identifier{3}}, TargetVersion: 1,
	}
	store := &memoryOCRStore{user: user, suggestions: make(map[Identifier]Suggestion)}
	jobs := &memoryOCRJobs{}
	sources := &memoryOCRSource{value: source, bytes: payload}
	targets := &memoryOCRTargets{field: field, fingerprint: [32]byte{9}, currentValue: "Valor atual", currentVersion: 1}
	service, err := NewService(store, jobs, sources, targets, extractor, ServiceOptions{
		Now: func() time.Time { return now }, Timeout: 30 * time.Second, MaximumRate: 10,
		MaximumProviderUsage: 1000, MaximumSourceBytes: MaximumSourceBytes, RecoveryBatch: 10,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return &ocrServiceFixture{
		service: service, store: store, jobs: jobs, source: sources, targets: targets, extractor: extractor,
		actor: auth.Session{User: user},
	}
}

func sha256Bytes(value []byte) [32]byte {
	return sha256.Sum256(value)
}

type memoryOCRJobs struct {
	enqueued  int
	cancelled int64
}

func (jobs *memoryOCRJobs) EnqueueExtraction(_ context.Context, _ Identifier) (int64, error) {
	jobs.enqueued++
	return int64(jobs.enqueued), nil
}

func (jobs *memoryOCRJobs) Cancel(_ context.Context, id int64) error {
	jobs.cancelled = id
	return nil
}

type memoryOCRSource struct {
	value     attachment.Attachment
	bytes     []byte
	getCalls  int
	openCalls int
	failGet   map[int]error
}

func (source *memoryOCRSource) GetForProcessing(_ context.Context, _ auth.Session, id attachment.Identifier) (attachment.Attachment, error) {
	source.getCalls++
	if err := source.failGet[source.getCalls]; err != nil {
		return attachment.Attachment{}, err
	}
	if id != source.value.ID {
		return attachment.Attachment{}, attachment.ErrAttachmentNotFound
	}
	return source.value, nil
}

func (source *memoryOCRSource) OpenForProcessing(_ context.Context, _ auth.Session, id attachment.Identifier) (attachment.Attachment, io.ReadCloser, error) {
	source.openCalls++
	if id != source.value.ID {
		return attachment.Attachment{}, nil, attachment.ErrAttachmentNotFound
	}
	return source.value, io.NopCloser(bytes.NewReader(source.bytes)), nil
}

type memoryOCRTargets struct {
	field          FieldSchema
	fingerprint    [32]byte
	currentValue   string
	currentVersion int64
	applyCalls     int
}

func (targets *memoryOCRTargets) Catalog(_ context.Context, _ auth.Session, _ attachment.OwnerReference) (Catalog, error) {
	field := targets.field
	field.TargetVersion = targets.currentVersion
	return Catalog{Fields: []FieldSchema{field}, Fingerprint: targets.fingerprint}, nil
}

func (targets *memoryOCRTargets) CurrentField(_ context.Context, _ auth.Session, field FieldSchema) (CurrentField, error) {
	if field.Key != targets.field.Key || field.Target != targets.field.Target {
		return CurrentField{}, ErrStaleTarget
	}
	return CurrentField{Value: targets.currentValue, Version: targets.currentVersion}, nil
}

func (targets *memoryOCRTargets) ApplyTarget(_ context.Context, _ auth.Session, target TargetReference, expectedVersion int64, changes []ApprovedChange, _ string) (int64, error) {
	if target != targets.field.Target || expectedVersion != targets.currentVersion || len(changes) != 1 {
		return 0, ErrStaleTarget
	}
	targets.applyCalls++
	targets.currentValue = changes[0].Value
	targets.currentVersion++
	return targets.currentVersion, nil
}

type memoryOCRStore struct {
	user        auth.User
	job         Job
	suggestions map[Identifier]Suggestion
	receipt     ApplyReceipt
	audits      []AuditEvent
	recovered   int
	recoveryErr error
}

func (store *memoryOCRStore) CurrentUser(_ context.Context, id auth.Identifier) (auth.User, error) {
	if id != store.user.ID {
		return auth.User{}, ErrForbidden
	}
	return store.user, nil
}

func (store *memoryOCRStore) CreateJob(_ context.Context, input CreateJobInput, _ time.Time, _ int, _ int64) (Job, bool, error) {
	if !store.job.ID.IsZero() {
		if store.job.OwnerUserID != input.OwnerUserID || store.job.IdempotencyKey != input.IdempotencyKey {
			return Job{}, false, ErrConflict
		}
		if store.job.RequestFingerprint != input.RequestFingerprint {
			return Job{}, false, ErrConflict
		}
		return store.job, false, nil
	}
	store.job = Job{
		ID: input.ID, OwnerUserID: input.OwnerUserID, AttachmentID: input.AttachmentID,
		RetryOfJobID: input.RetryOfJobID, IdempotencyKey: input.IdempotencyKey,
		RequestFingerprint: input.RequestFingerprint, SourceSHA256: input.SourceSHA256,
		CatalogFingerprint: input.CatalogFingerprint, SchemaVersion: SchemaVersion,
		SourceMIME: input.SourceMIME, SourceBytes: input.SourceBytes, State: JobQueued,
		Version: 1, CreatedAt: input.Now, UpdatedAt: input.Now,
	}
	return store.job, true, nil
}

func (store *memoryOCRStore) AttachRiverJob(_ context.Context, id Identifier, owner auth.Identifier, riverJobID int64, now time.Time) (Job, error) {
	if store.job.ID != id || store.job.OwnerUserID != owner || store.job.RiverJobID != 0 || store.job.State != JobQueued {
		return Job{}, ErrConflict
	}
	store.job.RiverJobID = riverJobID
	store.job.Version++
	store.job.UpdatedAt = now
	return store.job, nil
}

func (store *memoryOCRStore) ListJobs(_ context.Context, owner auth.Identifier, limit, offset int) (JobPage, error) {
	page := JobPage{Limit: limit, Offset: offset}
	if store.job.OwnerUserID == owner && offset == 0 {
		page.Jobs = []Job{store.job}
		page.Total = 1
	}
	return page, nil
}

func (store *memoryOCRStore) GetJob(_ context.Context, id Identifier, owner auth.Identifier) (Job, error) {
	if store.job.ID != id || store.job.OwnerUserID != owner {
		return Job{}, ErrNotFound
	}
	return store.job, nil
}

func (store *memoryOCRStore) GetJobForWorker(_ context.Context, id Identifier) (Job, error) {
	if store.job.ID != id {
		return Job{}, ErrNotFound
	}
	return store.job, nil
}

func (store *memoryOCRStore) ClaimJob(_ context.Context, id Identifier, now time.Time) (Job, bool, error) {
	if store.job.ID != id {
		return Job{}, false, ErrNotFound
	}
	if store.job.State.Terminal() || store.job.CancelRequestedAt != nil || store.job.AttemptCount >= MaximumAttempts {
		return store.job, false, nil
	}
	store.job.State = JobRunning
	store.job.AttemptCount++
	if store.job.StartedAt == nil {
		started := now
		store.job.StartedAt = &started
	}
	store.job.Version++
	store.job.UpdatedAt = now
	return store.job, true, nil
}

func (store *memoryOCRStore) RecordSourceValidated(_ context.Context, id Identifier, pages int, pixels int64, now time.Time) (Job, error) {
	if store.job.ID != id || store.job.State != JobRunning {
		return Job{}, ErrInvalidState
	}
	store.job.PageCount = pages
	store.job.PixelCount = pixels
	store.job.Version++
	store.job.UpdatedAt = now
	return store.job, nil
}

func (store *memoryOCRStore) AddProviderUsage(_ context.Context, id Identifier, owner auth.Identifier, usage, maximum int64, now time.Time) (Job, bool, error) {
	if store.job.ID != id || store.job.OwnerUserID != owner || store.job.State != JobRunning {
		return Job{}, false, ErrInvalidState
	}
	store.job.ProviderUsage += usage
	store.job.Version++
	store.job.UpdatedAt = now
	return store.job, store.job.ProviderUsage > maximum, nil
}

func (store *memoryOCRStore) CompleteJob(_ context.Context, input CompleteJobInput) (Job, error) {
	if store.job.ID != input.JobID || store.job.State != JobRunning {
		return Job{}, ErrInvalidState
	}
	for _, candidate := range input.Suggestions {
		store.suggestions[candidate.ID] = Suggestion{
			ID: candidate.ID, JobID: input.JobID, Ordinal: candidate.Ordinal,
			Target: candidate.Field.Target, TargetVersion: candidate.Field.TargetVersion,
			FieldKey: candidate.Field.Key, FieldLabel: candidate.Field.Label, Kind: candidate.Field.Kind,
			ProposedValue: candidate.ProposedValue, Evidence: candidate.Evidence, ReviewState: ReviewPending,
			Version: 1, CreatedAt: input.Now, UpdatedAt: input.Now,
		}
	}
	completed := input.Now
	store.job.State = JobCompleted
	store.job.PageCount = input.PageCount
	store.job.PixelCount = input.PixelCount
	store.job.SuggestionCount = len(input.Suggestions)
	store.job.CompletedAt = &completed
	store.job.Version++
	store.job.UpdatedAt = input.Now
	return store.job, nil
}

func (store *memoryOCRStore) FailJob(_ context.Context, id Identifier, errorCode string, state JobState, now time.Time) (Job, error) {
	if store.job.ID != id {
		return Job{}, ErrNotFound
	}
	if store.job.State.Terminal() {
		return store.job, nil
	}
	completed := now
	store.job.State = state
	store.job.ErrorCode = errorCode
	store.job.CompletedAt = &completed
	store.job.Version++
	store.job.UpdatedAt = now
	return store.job, nil
}

func (store *memoryOCRStore) RequestCancellation(_ context.Context, id Identifier, owner auth.Identifier, now time.Time) (Job, error) {
	if store.job.ID != id || store.job.OwnerUserID != owner {
		return Job{}, ErrNotFound
	}
	cancelled := now
	store.job.CancelRequestedAt = &cancelled
	store.job.State = JobCancelled
	store.job.ErrorCode = "cancelled"
	store.job.CompletedAt = &cancelled
	store.job.Version++
	store.job.UpdatedAt = now
	return store.job, nil
}

func (store *memoryOCRStore) ListEvents(_ context.Context, id Identifier, owner auth.Identifier, after int64, _ int) (EventPage, error) {
	if store.job.ID != id || store.job.OwnerUserID != owner {
		return EventPage{}, ErrNotFound
	}
	return EventPage{LastSequence: after, Terminal: store.job.State.Terminal()}, nil
}

func (store *memoryOCRStore) ListSuggestions(_ context.Context, jobID Identifier, owner auth.Identifier, limit, offset int) ([]Suggestion, int, error) {
	if store.job.ID != jobID || store.job.OwnerUserID != owner {
		return nil, 0, ErrNotFound
	}
	values := make([]Suggestion, 0, len(store.suggestions))
	for _, value := range store.suggestions {
		values = append(values, value)
	}
	if offset >= len(values) {
		return nil, len(values), nil
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return values[offset:end], len(values), nil
}

func (store *memoryOCRStore) GetSuggestion(_ context.Context, id Identifier, owner auth.Identifier) (Suggestion, Job, error) {
	value, exists := store.suggestions[id]
	if !exists || store.job.OwnerUserID != owner {
		return Suggestion{}, Job{}, ErrNotFound
	}
	return value, store.job, nil
}

func (store *memoryOCRStore) ReviewSuggestion(_ context.Context, input ReviewSuggestionInput) (Suggestion, error) {
	value, exists := store.suggestions[input.SuggestionID]
	if !exists || store.job.OwnerUserID != input.OwnerUserID {
		return Suggestion{}, ErrNotFound
	}
	if value.Version != input.Version {
		return Suggestion{}, ErrConflict
	}
	value.TargetVersion = input.TargetVersion
	value.ReviewedByUserID = &input.OwnerUserID
	reviewedAt := input.Now
	value.ReviewedAt = &reviewedAt
	value.ReviewedValue = input.Value
	value.ReviewState = ReviewAccepted
	if input.Action == ReviewReject {
		value.ReviewState = ReviewRejected
		value.ReviewedValue = nil
	}
	value.Version++
	value.UpdatedAt = input.Now
	store.suggestions[value.ID] = value
	return value, nil
}

func (store *memoryOCRStore) GetSuggestionsForApply(_ context.Context, jobID Identifier, owner auth.Identifier, selections []ApplySelection) ([]Suggestion, error) {
	if store.job.ID != jobID || store.job.OwnerUserID != owner {
		return nil, ErrNotFound
	}
	values := make([]Suggestion, 0, len(selections))
	for _, selection := range selections {
		value, exists := store.suggestions[selection.SuggestionID]
		if !exists {
			return nil, ErrNotFound
		}
		if value.Version != selection.Version {
			return nil, ErrConflict
		}
		if value.ReviewState != ReviewAccepted || value.ReviewedValue == nil {
			return nil, ErrInvalidState
		}
		values = append(values, value)
	}
	return values, nil
}

func (store *memoryOCRStore) CreateApplyReceipt(_ context.Context, input CreateApplyReceiptInput) (ApplyReceipt, bool, error) {
	if !store.receipt.ID.IsZero() {
		if store.receipt.OwnerUserID != input.OwnerUserID || store.receipt.IdempotencyKey != input.IdempotencyKey ||
			store.receipt.JobID != input.JobID || store.receipt.RequestFingerprint != input.RequestFingerprint {
			return ApplyReceipt{}, false, ErrConflict
		}
		return store.receipt, false, nil
	}
	store.receipt = ApplyReceipt{
		ID: input.ID, JobID: input.JobID, OwnerUserID: input.OwnerUserID,
		IdempotencyKey: input.IdempotencyKey, RequestFingerprint: input.RequestFingerprint,
		State: ApplyRunning, CreatedAt: input.Now, UpdatedAt: input.Now,
	}
	return store.receipt, true, nil
}

func (store *memoryOCRStore) ResumeApplyReceipt(_ context.Context, id Identifier, owner auth.Identifier, _, now time.Time) (ApplyReceipt, bool, error) {
	if store.receipt.ID != id || store.receipt.OwnerUserID != owner {
		return ApplyReceipt{}, false, ErrNotFound
	}
	if store.receipt.State == ApplyCompleted {
		return store.receipt, false, nil
	}
	store.receipt.UpdatedAt = now
	return store.receipt, true, nil
}

func (store *memoryOCRStore) SaveApplyResults(_ context.Context, receiptID Identifier, inputs []ApplyResultInput, now time.Time) ([]ApplyResult, error) {
	if store.receipt.ID != receiptID || store.receipt.State != ApplyRunning {
		return nil, ErrInvalidState
	}
	results := make([]ApplyResult, 0, len(inputs))
	for _, input := range inputs {
		result := ApplyResult{
			ReceiptID: receiptID, SuggestionID: input.SuggestionID, Outcome: input.Outcome,
			TargetVersion: input.TargetVersion, ErrorCode: input.ErrorCode, CreatedAt: now,
		}
		store.receipt.Results = append(store.receipt.Results, result)
		value := store.suggestions[input.SuggestionID]
		if input.Outcome == ApplyApplied {
			value.ReviewState = ReviewApplied
			appliedAt := now
			value.AppliedAt = &appliedAt
		} else if input.Outcome == ApplyStale {
			value.ReviewState = ReviewStale
		}
		value.Version++
		value.UpdatedAt = now
		store.suggestions[value.ID] = value
		results = append(results, result)
	}
	store.receipt.UpdatedAt = now
	return results, nil
}

func (store *memoryOCRStore) CompleteApplyReceipt(_ context.Context, id Identifier, owner auth.Identifier, now time.Time) (ApplyReceipt, error) {
	if store.receipt.ID != id || store.receipt.OwnerUserID != owner {
		return ApplyReceipt{}, ErrNotFound
	}
	completed := now
	store.receipt.State = ApplyCompleted
	store.receipt.CompletedAt = &completed
	store.receipt.UpdatedAt = now
	return store.receipt, nil
}

func (store *memoryOCRStore) RecoverStaleJobs(_ context.Context, _, _ time.Time, _ int) (int, error) {
	return store.recovered, store.recoveryErr
}

func (store *memoryOCRStore) SaveAudit(_ context.Context, event AuditEvent) error {
	store.audits = append(store.audits, event)
	return nil
}

func (store *memoryOCRStore) onlySuggestion(t *testing.T) Suggestion {
	t.Helper()
	if len(store.suggestions) != 1 {
		t.Fatalf("suggestion count = %d, want one", len(store.suggestions))
	}
	for _, value := range store.suggestions {
		return value
	}
	return Suggestion{}
}

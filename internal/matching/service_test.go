package matching

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
)

func TestServiceAnalysisLifecycleAndPermissions(t *testing.T) {
	now := time.Date(2026, 7, 17, 18, 0, 0, 0, time.UTC)
	store := newFakeStore(now)
	jobs := &fakeJobs{jobID: 42}
	service, err := NewService(store, jobs, ServiceOptions{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	member := matchingActor(auth.RoleMember)
	if catalog, err := service.Catalog(member); err != nil || len(catalog) != len(evidenceCatalog) {
		t.Fatalf("Catalog() = %d definitions, error=%v", len(catalog), err)
	}
	analysis, err := service.StartAnalysis(context.Background(), member, "analysis-key-0001", "request-start")
	if err != nil || analysis.RiverJobID != 42 || jobs.enqueued != analysis.ID {
		t.Fatalf("StartAnalysis() = %#v, enqueued=%s, error=%v", analysis, jobs.enqueued, err)
	}
	if _, err := service.StartAnalysis(context.Background(), matchingActor(auth.Role("UNKNOWN")), "analysis-key-0002", "denied"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("StartAnalysis(unknown role) error = %v", err)
	}
	store.analysis.State = AnalysisRunning
	if err := service.RunAnalysis(context.Background(), analysis.ID); err != nil || store.generated != analysis.ID {
		t.Fatalf("RunAnalysis() generated=%s, error=%v", store.generated, err)
	}
	store.analysis.State = AnalysisQueued
	cancelled, err := service.CancelAnalysis(context.Background(), member, analysis.ID, "request-cancel")
	if err != nil || cancelled.State != AnalysisCancelled || jobs.cancelled != 42 {
		t.Fatalf("CancelAnalysis() = %#v, cancelled job=%d, error=%v", cancelled, jobs.cancelled, err)
	}
	if len(store.audits) < 3 {
		t.Fatalf("matching audits = %#v", store.audits)
	}
}

func TestServiceReplaysUnattachedAnalysisThroughUniqueQueue(t *testing.T) {
	now := time.Date(2026, 7, 17, 18, 30, 0, 0, time.UTC)
	actor := matchingActor(auth.RoleMember)
	analysisID := newTestIdentifier()
	store := newFakeStore(now)
	store.created = false
	store.analysis = Analysis{
		ID: analysisID, ActorUserID: actor.User.ID, IdempotencyKey: "analysis-recovery-0001",
		State: AnalysisQueued, ExpiresAt: now.Add(24 * time.Hour), Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	jobs := &fakeJobs{jobID: 73}
	service, err := NewService(store, jobs, ServiceOptions{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	recovered, err := service.StartAnalysis(context.Background(), actor, "analysis-recovery-0001", "recovery-request")
	if err != nil || jobs.enqueued != analysisID || recovered.RiverJobID != 73 {
		t.Fatalf("recovered analysis = %#v, enqueued=%s, error=%v", recovered, jobs.enqueued, err)
	}
}

func TestServiceMergeRequiresAdministrativeExplicitPreview(t *testing.T) {
	now := time.Date(2026, 7, 17, 19, 0, 0, 0, time.UTC)
	store := newFakeStore(now)
	service, _ := NewService(store, &fakeJobs{jobID: 1}, ServiceOptions{Now: func() time.Time { return now }})
	caseID, survivorID, sourceID := newTestIdentifier(), newTestIdentifier(), newTestIdentifier()
	input := MergePreviewInput{CaseID: caseID, SurvivorID: survivorID, SourceID: sourceID, SurvivorVersion: 2, SourceVersion: 3,
		Choices: []FieldChoice{{FieldKey: "full_name", Source: FieldFromSurvivor}}}
	store.preview = MergePreview{CaseID: caseID, Survivor: ProfileSnapshot{ID: survivorID, FullName: "Ana", Version: 2},
		Source: ProfileSnapshot{ID: sourceID, FullName: "Anna", Version: 3}, Confirmation: "MESCLAR Ana"}
	store.preview.PreviewFingerprint = sha256.Sum256([]byte("preview"))
	if _, err := service.PreviewMerge(context.Background(), matchingActor(auth.RoleMember), input, "member-preview"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("PreviewMerge(member) error = %v", err)
	}
	admin := matchingActor(auth.RoleAdmin)
	preview, err := service.PreviewMerge(context.Background(), admin, input, "admin-preview")
	if err != nil || preview.Confirmation != "MESCLAR Ana" {
		t.Fatalf("PreviewMerge(admin) = %#v, error=%v", preview, err)
	}
	merge := MergeInput{MergePreviewInput: input, PreviewFingerprint: preview.PreviewFingerprint,
		IdempotencyKey: "merge-key-0001", Confirmation: preview.Confirmation}
	store.result = MergeResult{CaseID: caseID, SurvivorProfileID: survivorID, SourceProfileID: sourceID, SurvivorVersion: 3, MergedAt: now}
	if _, err := service.Merge(context.Background(), matchingActor(auth.RoleMember), merge, "member-merge"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Merge(member) error = %v", err)
	}
	if result, err := service.Merge(context.Background(), admin, merge, "merge-request"); err != nil || result.SurvivorProfileID != survivorID {
		t.Fatalf("Merge(admin) = %#v, error=%v", result, err)
	}
	if result, err := service.Merge(context.Background(), matchingActor(auth.RoleSuperAdmin), merge, "superadmin-merge"); err != nil || result.SurvivorProfileID != survivorID {
		t.Fatalf("Merge(superadmin) = %#v, error=%v", result, err)
	}
	merge.Confirmation = "MESCLAR"
	if _, err := service.Merge(context.Background(), admin, merge, "bad-confirmation"); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("Merge(invalid confirmation) error = %v", err)
	}
}

type fakeStore struct {
	now       time.Time
	analysis  Analysis
	created   bool
	generated Identifier
	preview   MergePreview
	result    MergeResult
	audits    []AuditEvent
}

func newFakeStore(now time.Time) *fakeStore { return &fakeStore{now: now, created: true} }

func (store *fakeStore) CreateAnalysis(_ context.Context, input CreateAnalysisInput, _ time.Time, _ int) (Analysis, bool, error) {
	if store.analysis.ID.IsZero() {
		store.analysis = Analysis{ID: input.ID, ActorUserID: input.ActorUserID, IdempotencyKey: input.IdempotencyKey,
			State: AnalysisQueued, ExpiresAt: input.ExpiresAt, Version: 1, CreatedAt: store.now, UpdatedAt: store.now}
	}
	return store.analysis, store.created, nil
}

func (store *fakeStore) CreateAnalysisWithJob(ctx context.Context, input CreateAnalysisInput, window time.Time, maximumRate int, jobs AnalysisJobInserter) (Analysis, bool, error) {
	analysis, created, err := store.CreateAnalysis(ctx, input, window, maximumRate)
	if err != nil || !created {
		return analysis, created, err
	}
	jobID, err := jobs.EnqueueAnalysisTx(ctx, nil, analysis.ID)
	if err != nil {
		return Analysis{}, false, err
	}
	analysis.RiverJobID = jobID
	store.analysis = analysis
	return analysis, true, nil
}

func (store *fakeStore) AttachAnalysisJob(_ context.Context, _ Identifier, _ auth.Identifier, jobID int64) (Analysis, error) {
	store.analysis.RiverJobID = jobID
	return store.analysis, nil
}

func (store *fakeStore) GetAnalysis(_ context.Context, _ Identifier, _ auth.Identifier) (Analysis, error) {
	return store.analysis, nil
}

func (store *fakeStore) RequestAnalysisCancellation(_ context.Context, _ Identifier, _ auth.Identifier, now time.Time) (Analysis, error) {
	store.analysis.State, store.analysis.CompletedAt = AnalysisCancelled, &now
	return store.analysis, nil
}

func (store *fakeStore) ClaimAnalysis(_ context.Context, _ Identifier, _ time.Time) (Analysis, bool, error) {
	return store.analysis, true, nil
}

func (store *fakeStore) GenerateCandidates(_ context.Context, id Identifier, _ int, _ time.Duration, _ time.Time) (AnalysisStats, error) {
	store.generated = id
	return AnalysisStats{ProfilesScanned: 3, CandidateCount: 1, RefreshedCount: 1}, nil
}

func (store *fakeStore) FailAnalysis(context.Context, Identifier, string, AnalysisState, time.Time) error {
	return nil
}

func (store *fakeStore) ListCases(_ context.Context, options CaseListOptions) (CasePage, error) {
	return CasePage{Limit: options.Limit, Offset: options.Offset}, nil
}

func (store *fakeStore) GetCase(context.Context, Identifier) (Case, error) { return Case{}, nil }

func (store *fakeStore) DismissCase(context.Context, Identifier, auth.Identifier, int64, string, time.Time) (Case, error) {
	return Case{}, nil
}

func (store *fakeStore) PreviewMerge(context.Context, MergePreviewInput, time.Time) (MergePreview, error) {
	return store.preview, nil
}

func (store *fakeStore) Merge(context.Context, auth.Identifier, MergeInput, string, time.Time) (MergeResult, bool, error) {
	return store.result, true, nil
}

func (store *fakeStore) CleanupAnalyses(context.Context, time.Time, int) (int, error) { return 0, nil }

func (store *fakeStore) RecordAudit(_ context.Context, event AuditEvent) error {
	store.audits = append(store.audits, event)
	return nil
}

type fakeJobs struct {
	jobID     int64
	enqueued  Identifier
	cancelled int64
}

func (jobs *fakeJobs) EnqueueAnalysis(_ context.Context, id Identifier) (int64, error) {
	jobs.enqueued = id
	return jobs.jobID, nil
}

func (jobs *fakeJobs) EnqueueAnalysisTx(_ context.Context, _ pgx.Tx, id Identifier) (int64, error) {
	jobs.enqueued = id
	return jobs.jobID, nil
}

func (jobs *fakeJobs) Cancel(_ context.Context, id int64) error {
	jobs.cancelled = id
	return nil
}

func matchingActor(role auth.Role) auth.Session {
	return auth.Session{User: auth.User{ID: auth.Identifier(newTestIdentifier()), Login: "matching-test", Role: role, Active: true}}
}

func newTestIdentifier() Identifier {
	value, err := NewIdentifier()
	if err != nil {
		panic(err)
	}
	return value
}

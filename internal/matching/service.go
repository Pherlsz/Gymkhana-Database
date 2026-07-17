package matching

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	Now             func() time.Time
	AnalysisTimeout time.Duration
	Retention       time.Duration
	MaximumRate     int
	MaximumResults  int
	CleanupBatch    int
	OnAuditFailure  AuditFailureHandler
}

type Service struct {
	store          Store
	jobs           Jobs
	now            func() time.Time
	timeout        time.Duration
	retention      time.Duration
	maximumRate    int
	maximumResults int
	cleanupBatch   int
	onAuditFailure AuditFailureHandler
}

func NewService(store Store, jobs Jobs, options ServiceOptions) (*Service, error) {
	if store == nil || jobs == nil {
		return nil, ErrInvalidInput
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.AnalysisTimeout == 0 {
		options.AnalysisTimeout = AnalysisTimeout
	}
	if options.Retention == 0 {
		options.Retention = AnalysisRetention
	}
	if options.MaximumRate == 0 {
		options.MaximumRate = MaximumAnalysisRate
	}
	if options.MaximumResults == 0 {
		options.MaximumResults = MaximumCandidates
	}
	if options.CleanupBatch == 0 {
		options.CleanupBatch = CleanupBatchSize
	}
	if options.AnalysisTimeout < time.Second || options.AnalysisTimeout > time.Minute ||
		options.Retention < 24*time.Hour || options.Retention > 365*24*time.Hour ||
		options.MaximumRate < 1 || options.MaximumRate > 100 ||
		options.MaximumResults < 1 || options.MaximumResults > MaximumCandidates ||
		options.CleanupBatch < 1 || options.CleanupBatch > 1_000 {
		return nil, ErrInvalidInput
	}
	return &Service{
		store: store, jobs: jobs, now: options.Now, timeout: options.AnalysisTimeout,
		retention: options.Retention, maximumRate: options.MaximumRate,
		maximumResults: options.MaximumResults, cleanupBatch: options.CleanupBatch,
		onAuditFailure: options.OnAuditFailure,
	}, nil
}

func (service *Service) Catalog(actor auth.Session) ([]EvidenceDefinition, error) {
	if !canReview(actor) {
		return nil, ErrForbidden
	}
	return EvidenceCatalog(), nil
}

func (service *Service) StartAnalysis(ctx context.Context, actor auth.Session, idempotencyKey, requestID string) (Analysis, error) {
	if !canReview(actor) {
		service.audit(ctx, &actor.User.ID, nil, nil, AuditAnalysisCreated, auth.AuditOutcomeDenied, nil, nil, "forbidden", requestID)
		return Analysis{}, ErrForbidden
	}
	if !validIdempotencyKey(idempotencyKey) {
		return Analysis{}, ErrInvalidInput
	}
	id, err := NewIdentifier()
	if err != nil {
		return Analysis{}, fmt.Errorf("generate matching analysis identifier: %w", err)
	}
	now := service.now().UTC()
	analysis, created, err := service.store.CreateAnalysis(ctx, CreateAnalysisInput{
		ID: id, ActorUserID: actor.User.ID, IdempotencyKey: idempotencyKey, ExpiresAt: now.Add(service.retention),
	}, now.Truncate(AnalysisWindow), service.maximumRate)
	if err != nil {
		outcome := auth.AuditOutcomeFailure
		if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrConflict) {
			outcome = auth.AuditOutcomeDenied
		}
		service.audit(ctx, &actor.User.ID, nil, nil, AuditAnalysisCreated, outcome, nil, nil, publicErrorCode(err), requestID)
		return Analysis{}, err
	}
	if !created {
		return analysis, nil
	}
	jobID, err := service.jobs.EnqueueAnalysis(ctx, analysis.ID)
	if err != nil {
		_ = service.store.FailAnalysis(ctx, analysis.ID, "queue_unavailable", AnalysisFailed, now)
		service.audit(ctx, &actor.User.ID, &analysis.ID, nil, AuditAnalysisFailed, auth.AuditOutcomeFailure, nil, nil, "queue_unavailable", requestID)
		return Analysis{}, fmt.Errorf("enqueue matching analysis: %w", err)
	}
	analysis, err = service.store.AttachAnalysisJob(ctx, analysis.ID, actor.User.ID, jobID)
	if err != nil {
		_ = service.jobs.Cancel(ctx, jobID)
		_ = service.store.FailAnalysis(ctx, analysis.ID, "queue_conflict", AnalysisFailed, now)
		return Analysis{}, err
	}
	service.audit(ctx, &actor.User.ID, &analysis.ID, nil, AuditAnalysisCreated, auth.AuditOutcomeSuccess, nil, nil, "", requestID)
	return analysis, nil
}

func (service *Service) Analysis(ctx context.Context, actor auth.Session, id Identifier) (Analysis, error) {
	if !canReview(actor) || id.IsZero() {
		return Analysis{}, ErrForbidden
	}
	return service.store.GetAnalysis(ctx, id, actor.User.ID)
}

func (service *Service) CancelAnalysis(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Analysis, error) {
	if !canReview(actor) || id.IsZero() {
		service.audit(ctx, &actor.User.ID, &id, nil, AuditAnalysisCancelled, auth.AuditOutcomeDenied, nil, nil, "forbidden", requestID)
		return Analysis{}, ErrForbidden
	}
	now := service.now().UTC()
	analysis, err := service.store.RequestAnalysisCancellation(ctx, id, actor.User.ID, now)
	if err != nil {
		service.audit(ctx, &actor.User.ID, &id, nil, AuditAnalysisCancelled, auditOutcome(err), nil, nil, publicErrorCode(err), requestID)
		return Analysis{}, err
	}
	if analysis.RiverJobID > 0 {
		if err := service.jobs.Cancel(ctx, analysis.RiverJobID); err != nil {
			return Analysis{}, err
		}
	}
	service.audit(ctx, &actor.User.ID, &id, nil, AuditAnalysisCancelled, auth.AuditOutcomeSuccess, nil, nil, "", requestID)
	return analysis, nil
}

func (service *Service) RunAnalysis(ctx context.Context, id Identifier) error {
	if id.IsZero() {
		return ErrInvalidInput
	}
	now := service.now().UTC()
	analysis, claimed, err := service.store.ClaimAnalysis(ctx, id, now)
	if err != nil || !claimed {
		return err
	}
	service.audit(ctx, &analysis.ActorUserID, &analysis.ID, nil, AuditAnalysisStarted, auth.AuditOutcomeSuccess, nil, nil, "", "worker")
	stats, err := service.store.GenerateCandidates(ctx, id, service.maximumResults, service.timeout, now)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		state := AnalysisFailed
		event := AuditAnalysisFailed
		if errors.Is(err, ErrCancelled) {
			state, event = AnalysisCancelled, AuditAnalysisCancelled
		}
		code := publicErrorCode(err)
		if failErr := service.store.FailAnalysis(ctx, id, code, state, service.now().UTC()); failErr != nil {
			return errors.Join(err, failErr)
		}
		service.audit(ctx, &analysis.ActorUserID, &analysis.ID, nil, event, auth.AuditOutcomeFailure, nil, nil, code, "worker")
		return nil
	}
	affected := stats.CandidateCount
	service.audit(ctx, &analysis.ActorUserID, &analysis.ID, nil, AuditAnalysisCompleted, auth.AuditOutcomeSuccess, nil, &affected, "", "worker")
	return nil
}

func (service *Service) ListCases(ctx context.Context, actor auth.Session, options CaseListOptions) (CasePage, error) {
	if !canReview(actor) {
		return CasePage{}, ErrForbidden
	}
	normalized, err := normalizeCaseListOptions(options)
	if err != nil {
		return CasePage{}, err
	}
	return service.store.ListCases(ctx, normalized)
}

func (service *Service) Case(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Case, error) {
	if !canReview(actor) || id.IsZero() {
		return Case{}, ErrForbidden
	}
	value, err := service.store.GetCase(ctx, id)
	if err == nil {
		service.audit(ctx, &actor.User.ID, nil, &id, AuditCaseReviewed, auth.AuditOutcomeSuccess, &value.ScoreBand, nil, "", requestID)
	}
	return value, err
}

func (service *Service) DismissCase(ctx context.Context, actor auth.Session, id Identifier, version int64, requestID string) (Case, error) {
	if !canReview(actor) || id.IsZero() {
		service.audit(ctx, &actor.User.ID, nil, &id, AuditCaseDismissed, auth.AuditOutcomeDenied, nil, nil, "forbidden", requestID)
		return Case{}, ErrForbidden
	}
	if version <= 0 {
		return Case{}, ErrInvalidInput
	}
	value, err := service.store.DismissCase(ctx, id, actor.User.ID, version, requestIDValue(requestID), service.now().UTC())
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, &id, AuditCaseDismissed, auditOutcome(err), nil, nil, publicErrorCode(err), requestID)
		return Case{}, err
	}
	service.audit(ctx, &actor.User.ID, nil, &id, AuditCaseDismissed, auth.AuditOutcomeSuccess, &value.ScoreBand, nil, "", requestID)
	return value, nil
}

func (service *Service) PreviewMerge(ctx context.Context, actor auth.Session, input MergePreviewInput, requestID string) (MergePreview, error) {
	if !canMerge(actor) {
		service.audit(ctx, &actor.User.ID, nil, &input.CaseID, AuditMergeDenied, auth.AuditOutcomeDenied, nil, nil, "forbidden", requestID)
		return MergePreview{}, ErrForbidden
	}
	if err := validatePreviewInput(input); err != nil {
		return MergePreview{}, err
	}
	preview, err := service.store.PreviewMerge(ctx, input, service.now().UTC())
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, &input.CaseID, AuditMergeDenied, auditOutcome(err), nil, nil, publicErrorCode(err), requestID)
		return MergePreview{}, err
	}
	service.audit(ctx, &actor.User.ID, nil, &input.CaseID, AuditMergePreviewed, auth.AuditOutcomeSuccess, nil, nil, "", requestID)
	return preview, nil
}

func (service *Service) Merge(ctx context.Context, actor auth.Session, input MergeInput, requestID string) (MergeResult, error) {
	if !canMerge(actor) {
		service.audit(ctx, &actor.User.ID, nil, &input.CaseID, AuditMergeDenied, auth.AuditOutcomeDenied, nil, nil, "forbidden", requestID)
		return MergeResult{}, ErrForbidden
	}
	if !strings.HasPrefix(input.Confirmation, MergeConfirmation+" ") || input.PreviewFingerprint == ([sha256.Size]byte{}) ||
		!validIdempotencyKey(input.IdempotencyKey) {
		service.audit(ctx, &actor.User.ID, nil, &input.CaseID, AuditMergeDenied, auth.AuditOutcomeDenied, nil, nil, "invalid_confirmation", requestID)
		return MergeResult{}, ErrInvalidConfirmation
	}
	if err := validatePreviewInput(input.MergePreviewInput); err != nil {
		return MergeResult{}, err
	}
	result, _, err := service.store.Merge(ctx, actor.User.ID, input, requestIDValue(requestID), service.now().UTC())
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, &input.CaseID, AuditMergeDenied, auditOutcome(err), nil, nil, publicErrorCode(err), requestID)
		return MergeResult{}, err
	}
	affected := 1
	service.audit(ctx, &actor.User.ID, nil, &input.CaseID, AuditMergeCompleted, auth.AuditOutcomeSuccess, nil, &affected, "", requestID)
	return result, nil
}

func (service *Service) Cleanup(ctx context.Context) (int, error) {
	return service.store.CleanupAnalyses(ctx, service.now().UTC(), service.cleanupBatch)
}

func canReview(actor auth.Session) bool {
	return actor.User.Active && actor.User.Role.CanReviewProfileMatches()
}

func canMerge(actor auth.Session) bool {
	return actor.User.Active && actor.User.Role.CanMergeProfiles()
}

func normalizeCaseListOptions(options CaseListOptions) (CaseListOptions, error) {
	if options.Limit == 0 {
		options.Limit = MaximumCasePageSize
	}
	if options.Limit < 1 || options.Limit > MaximumCasePageSize || options.Offset < 0 || options.Offset > 10_000 {
		return CaseListOptions{}, ErrInvalidInput
	}
	if len(options.States) == 0 {
		options.States = []CaseState{CasePending}
	}
	seenStates := make(map[CaseState]struct{}, len(options.States))
	for _, state := range options.States {
		if !state.Valid() {
			return CaseListOptions{}, ErrInvalidInput
		}
		seenStates[state] = struct{}{}
	}
	if len(seenStates) != len(options.States) {
		return CaseListOptions{}, ErrInvalidInput
	}
	seenBands := make(map[ScoreBand]struct{}, len(options.Bands))
	for _, band := range options.Bands {
		if !band.Valid() {
			return CaseListOptions{}, ErrInvalidInput
		}
		seenBands[band] = struct{}{}
	}
	if len(seenBands) != len(options.Bands) {
		return CaseListOptions{}, ErrInvalidInput
	}
	return options, nil
}

func validatePreviewInput(input MergePreviewInput) error {
	if input.CaseID.IsZero() || input.SurvivorID.IsZero() || input.SourceID.IsZero() ||
		input.SurvivorID == input.SourceID || input.SurvivorVersion <= 0 || input.SourceVersion <= 0 ||
		len(input.Choices) > MaximumCanonicalFields+256 {
		return ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(input.Choices))
	for _, choice := range input.Choices {
		if !validLogicalFieldKey(choice.FieldKey) || !choice.Source.Valid() {
			return ErrInvalidInput
		}
		if _, exists := seen[choice.FieldKey]; exists {
			return ErrInvalidInput
		}
		seen[choice.FieldKey] = struct{}{}
	}
	return nil
}

func validLogicalFieldKey(value string) bool {
	if len(value) < 2 || len(value) > 80 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value {
		if unicode.IsLower(character) || unicode.IsDigit(character) || character == '_' || character == '.' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validIdempotencyKey(value string) bool {
	if len(value) < 8 || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func requestIDValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if len(value) > 128 {
		return value[:128]
	}
	return value
}

func auditOutcome(err error) auth.AuditOutcome {
	if errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) ||
		errors.Is(err, ErrInvalidState) || errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrStalePreview) ||
		errors.Is(err, ErrDependencyConflict) || errors.Is(err, ErrRateLimited) {
		return auth.AuditOutcomeDenied
	}
	return auth.AuditOutcomeFailure
}

func publicErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrCancelled):
		return "cancelled"
	case errors.Is(err, ErrConflict):
		return "conflict"
	case errors.Is(err, ErrDependencyConflict):
		return "dependency_conflict"
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	case errors.Is(err, ErrInvalidConfirmation):
		return "invalid_confirmation"
	case errors.Is(err, ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, ErrInvalidState):
		return "invalid_state"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrStalePreview):
		return "stale_preview"
	case errors.Is(err, ErrTimeout):
		return "timeout"
	default:
		return "internal_error"
	}
}

func (service *Service) audit(
	ctx context.Context,
	actorID *auth.Identifier,
	analysisID *Identifier,
	caseID *Identifier,
	eventType AuditEventType,
	outcome auth.AuditOutcome,
	band *ScoreBand,
	affected *int,
	errorCode string,
	requestID string,
) {
	eventID, err := NewIdentifier()
	if err != nil {
		return
	}
	event := AuditEvent{
		ID: eventID, ActorUserID: actorID, AnalysisID: analysisID, CaseID: caseID,
		EventType: eventType, Outcome: outcome, ScoreBand: band, AffectedCount: affected,
		ErrorCode: errorCode, RequestID: requestIDValue(requestID), CreatedAt: service.now().UTC(),
	}
	if err := service.store.RecordAudit(ctx, event); err != nil && service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

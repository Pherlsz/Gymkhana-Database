package taskengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

var taskIdempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	Now            func() time.Time
	Timeout        time.Duration
	Retention      time.Duration
	MaximumRate    int
	RecoveryBatch  int
	SolveLimits    SolveLimits
	OnAuditFailure AuditFailureHandler
}

type Service struct {
	store          Store
	jobs           Jobs
	query          QueryGateway
	interpreter    Interpreter
	now            func() time.Time
	timeout        time.Duration
	retention      time.Duration
	maximumRate    int
	recoveryBatch  int
	solveLimits    SolveLimits
	onAuditFailure AuditFailureHandler
}

func NewService(store Store, jobs Jobs, query QueryGateway, interpreter Interpreter, options ServiceOptions) (*Service, error) {
	if store == nil || jobs == nil || query == nil {
		return nil, ErrInvalidSetup
	}
	if interpreter == nil {
		interpreter = DisabledInterpreter{}
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Timeout == 0 {
		options.Timeout = DefaultTaskTimeout
	}
	if options.Retention == 0 {
		options.Retention = DefaultRetention
	}
	if options.MaximumRate == 0 {
		options.MaximumRate = DefaultMaximumRate
	}
	if options.RecoveryBatch == 0 {
		options.RecoveryBatch = DefaultRecoveryBatch
	}
	if options.SolveLimits.MaximumBranches == 0 {
		options.SolveLimits.MaximumBranches = 100_000
	}
	if options.SolveLimits.MaximumCandidatesPerRole == 0 {
		options.SolveLimits.MaximumCandidatesPerRole = MaximumCandidateRows
	}
	if options.SolveLimits.MaximumCompositions == 0 {
		options.SolveLimits.MaximumCompositions = MaximumSolutions
	}
	if options.SolveLimits.MaximumEvidence == 0 {
		options.SolveLimits.MaximumEvidence = MaximumEvidencePerResult
	}
	if options.SolveLimits.MaximumBytes == 0 {
		options.SolveLimits.MaximumBytes = 5 << 20
	}
	if options.Timeout < time.Second || options.Timeout > 10*time.Minute ||
		options.Retention < time.Hour || options.Retention > 7*24*time.Hour ||
		options.MaximumRate < 1 || options.MaximumRate > 1000 ||
		options.RecoveryBatch < 1 || options.RecoveryBatch > 1000 {
		return nil, ErrInvalidSetup
	}
	return &Service{
		store: store, jobs: jobs, query: query, interpreter: interpreter, now: options.Now,
		timeout: options.Timeout, retention: options.Retention, maximumRate: options.MaximumRate,
		recoveryBatch: options.RecoveryBatch, solveLimits: options.SolveLimits,
		onAuditFailure: options.OnAuditFailure,
	}, nil
}

func DefaultCapability() Capability {
	return Capability{
		Enabled: false, SemanticInterpretation: false, DirectTypedSpecifications: true,
		MaximumTaskTextRunes: MaximumTaskTextRunes, MaximumRequirements: MaximumRequirements,
		MaximumRoles: MaximumRoles, MaximumConstraints: MaximumConstraints,
		MaximumCandidatesPerRole: MaximumCandidateRows, MaximumBranches: 100_000,
		MaximumSolutions: MaximumSolutions, MaximumDuration: DefaultTaskTimeout,
		MaximumRequests: DefaultMaximumRate, Retention: DefaultRetention,
	}
}

func (service *Service) Capability() Capability {
	value := DefaultCapability()
	value.Enabled = service != nil
	_, disabled := service.interpreter.(DisabledInterpreter)
	value.SemanticInterpretation = !disabled
	value.MaximumDuration = service.timeout
	value.MaximumRequests = service.maximumRate
	value.Retention = service.retention
	value.MaximumCandidatesPerRole = service.solveLimits.MaximumCandidatesPerRole
	value.MaximumBranches = service.solveLimits.MaximumBranches
	value.MaximumSolutions = service.solveLimits.MaximumCompositions
	return value
}

func (service *Service) Interpret(ctx context.Context, actor auth.Session, taskText, requestID string) (Proposal, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return Proposal{}, err
	}
	catalog, err := service.query.CatalogV2(ctx, auth.Session{User: user}, requestID)
	if err != nil {
		return Proposal{}, normalizeQueryError(err)
	}
	proposal, err := service.interpreter.Interpret(ctx, InterpreterInput{TaskText: taskText, Catalog: catalog})
	if err != nil {
		return Proposal{}, err
	}
	proposal.RequiresHumanReview = true
	proposal.Spec.State = SpecProposed
	return proposal, nil
}

func (service *Service) CreateDraft(ctx context.Context, actor auth.Session, spec TaskSpec, requestID string) (Draft, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return Draft{}, err
	}
	catalog, err := service.query.CatalogV2(ctx, auth.Session{User: user}, requestID)
	if err != nil {
		return Draft{}, normalizeQueryError(err)
	}
	requireReviewed := spec.State == SpecReviewed
	normalized, fingerprintText, err := NormalizeAndValidate(spec, catalog, requireReviewed)
	if err != nil {
		return Draft{}, err
	}
	fingerprint, err := decodeFingerprint(fingerprintText)
	if err != nil {
		return Draft{}, err
	}
	id, err := queryengine.NewIdentifier()
	if err != nil {
		return Draft{}, fmt.Errorf("generate task draft identifier: %w", err)
	}
	now := service.now().UTC()
	state := DraftProposed
	if normalized.State == SpecReviewed {
		state = DraftReviewed
	}
	draft, err := service.store.CreateDraft(ctx, CreateDraftInput{
		ID: id, OwnerUserID: user.ID, CatalogVersion: catalog.Version, State: state,
		Spec: normalized, SpecFingerprint: fingerprint, ExpiresAt: now.Add(service.retention), Now: now,
	})
	if err != nil {
		return Draft{}, err
	}
	service.audit(ctx, &user.ID, &draft.ID, nil, AuditDraftCreated, auth.AuditOutcomeSuccess, nil, nil, requestID)
	return draft, nil
}

func (service *Service) ReviewDraft(ctx context.Context, actor auth.Session, id Identifier, spec TaskSpec, version int64, requestID string) (Draft, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() || version < 1 {
		if err == nil {
			err = ErrInvalidInput
		}
		return Draft{}, err
	}
	current, err := service.store.GetDraft(ctx, id, user.ID)
	if err != nil {
		return Draft{}, err
	}
	if !current.ExpiresAt.After(service.now().UTC()) {
		return Draft{}, ErrExpired
	}
	catalog, err := service.query.CatalogV2(ctx, auth.Session{User: user}, requestID)
	if err != nil {
		return Draft{}, normalizeQueryError(err)
	}
	spec.State = SpecReviewed
	normalized, fingerprintText, err := NormalizeAndValidate(spec, catalog, true)
	if err != nil {
		return Draft{}, err
	}
	fingerprint, err := decodeFingerprint(fingerprintText)
	if err != nil {
		return Draft{}, err
	}
	updated, err := service.store.UpdateDraft(ctx, UpdateDraftInput{
		ID: id, OwnerUserID: user.ID, CatalogVersion: catalog.Version, Spec: normalized,
		SpecFingerprint: fingerprint, Version: version, Now: service.now().UTC(),
	})
	if err != nil {
		return Draft{}, err
	}
	service.audit(ctx, &user.ID, &id, nil, AuditDraftReviewed, auth.AuditOutcomeSuccess, nil, nil, requestID)
	return updated, nil
}

func (service *Service) Draft(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Draft, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() {
		return Draft{}, ErrNotFound
	}
	value, err := service.store.GetDraft(ctx, id, user.ID)
	if err != nil {
		return Draft{}, err
	}
	if !value.ExpiresAt.After(service.now().UTC()) {
		return Draft{}, ErrExpired
	}
	return value, nil
}

func (service *Service) StartJob(ctx context.Context, actor auth.Session, draftID Identifier, idempotencyKey string, retryOf *Identifier, requestID string) (Job, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return Job{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if draftID.IsZero() || !validTaskIdempotency(idempotencyKey) || retryOf != nil && retryOf.IsZero() {
		return Job{}, ErrInvalidInput
	}
	draft, err := service.store.GetDraft(ctx, draftID, user.ID)
	if err != nil {
		return Job{}, err
	}
	if draft.State != DraftReviewed || !draft.ExpiresAt.After(service.now().UTC()) {
		return Job{}, ErrConflict
	}
	if retryOf != nil {
		previous, previousErr := service.store.GetJob(ctx, *retryOf, user.ID)
		if previousErr != nil || !previous.State.Terminal() || previous.DraftID != draftID {
			return Job{}, ErrInvalidInput
		}
	}
	fingerprint := taskJobFingerprint(user.ID, draft, retryOf)
	id, err := queryengine.NewIdentifier()
	if err != nil {
		return Job{}, fmt.Errorf("generate task job identifier: %w", err)
	}
	now := service.now().UTC()
	job, created, err := service.store.CreateJob(ctx, CreateJobInput{
		ID: id, OwnerUserID: user.ID, DraftID: draftID, RetryOfJobID: retryOf,
		IdempotencyKey: idempotencyKey, RequestFingerprint: fingerprint,
		CatalogVersion: draft.CatalogVersion, ExpiresAt: now.Add(service.retention), Now: now,
	}, now.Truncate(time.Hour), service.maximumRate)
	if err != nil {
		return Job{}, err
	}
	if !created {
		if job.RiverJobID == 0 && !job.State.Terminal() {
			return service.enqueue(ctx, user, job, requestID)
		}
		return job, nil
	}
	service.audit(ctx, &user.ID, &draftID, &job.ID, AuditJobCreated, auth.AuditOutcomeSuccess, nil, nil, requestID)
	return service.enqueue(ctx, user, job, requestID)
}

func (service *Service) enqueue(ctx context.Context, user auth.User, job Job, requestID string) (Job, error) {
	riverID, err := service.jobs.EnqueueSolve(ctx, job.ID)
	if err != nil {
		_, _ = service.store.FailJob(ctx, job.ID, "queue_unavailable", JobFailed, service.now().UTC())
		return Job{}, ErrUnavailable
	}
	updated, err := service.store.AttachRiverJob(ctx, job.ID, user.ID, riverID, service.now().UTC())
	if err != nil {
		_ = service.jobs.Cancel(ctx, riverID)
		return Job{}, err
	}
	return updated, nil
}

func (service *Service) Jobs(ctx context.Context, actor auth.Session, limit, offset int, requestID string) (JobPage, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return JobPage{}, err
	}
	if limit == 0 {
		limit = MaximumJobPage
	}
	if limit < 1 || limit > MaximumJobPage || offset < 0 || offset > 10_000 {
		return JobPage{}, ErrInvalidInput
	}
	page, err := service.store.ListJobs(ctx, user.ID, limit, offset)
	if err == nil {
		count := len(page.Jobs)
		service.audit(ctx, &user.ID, nil, nil, AuditJobRead, auth.AuditOutcomeSuccess, &count, nil, requestID)
	}
	return page, err
}

func (service *Service) Job(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Job, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() {
		return Job{}, ErrNotFound
	}
	job, err := service.store.GetJob(ctx, id, user.ID)
	if err != nil {
		return Job{}, err
	}
	if !job.ExpiresAt.After(service.now().UTC()) {
		return Job{}, ErrExpired
	}
	service.audit(ctx, &user.ID, nil, &job.ID, AuditJobRead, auth.AuditOutcomeSuccess, nil, nil, requestID)
	return job, nil
}

func (service *Service) CancelJob(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Job, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() {
		return Job{}, ErrNotFound
	}
	job, err := service.store.RequestCancellation(ctx, id, user.ID, service.now().UTC())
	if err != nil {
		return Job{}, err
	}
	if job.RiverJobID > 0 {
		if err := service.jobs.Cancel(ctx, job.RiverJobID); err != nil {
			return Job{}, err
		}
	}
	service.audit(ctx, &user.ID, nil, &id, AuditJobCancelled, auth.AuditOutcomeSuccess, nil, nil, requestID)
	return job, nil
}

func (service *Service) Events(ctx context.Context, actor auth.Session, id Identifier, after int64, limit int) (EventPage, error) {
	job, err := service.Job(ctx, actor, id, "events")
	if err != nil {
		return EventPage{}, err
	}
	if after < 0 {
		return EventPage{}, ErrInvalidInput
	}
	if limit == 0 {
		limit = MaximumEventPage
	}
	if limit < 1 || limit > MaximumEventPage {
		return EventPage{}, ErrInvalidInput
	}
	return service.store.ListEvents(ctx, job.ID, job.OwnerUserID, after, limit)
}

func (service *Service) Results(ctx context.Context, actor auth.Session, id Identifier, limit, offset int, requestID string) (ResultPage, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return ResultPage{}, err
	}
	if limit == 0 {
		limit = MaximumResultPage
	}
	if limit < 1 || limit > MaximumResultPage || offset < 0 || offset > 10_000 {
		return ResultPage{}, ErrInvalidInput
	}
	job, err := service.store.GetJob(ctx, id, user.ID)
	if err != nil {
		return ResultPage{}, err
	}
	if !job.ExpiresAt.After(service.now().UTC()) {
		return ResultPage{}, ErrExpired
	}
	if !job.State.Terminal() || job.State == JobFailed || job.State == JobCancelled {
		return ResultPage{}, ErrConflict
	}
	page, err := service.store.ListResults(ctx, id, user.ID, limit, offset)
	if err != nil {
		return ResultPage{}, err
	}
	if !safeResultPage(page) {
		return ResultPage{}, ErrUnsafeResult
	}
	count := len(page.Compositions)
	service.audit(ctx, &user.ID, nil, &id, AuditResultRead, auth.AuditOutcomeSuccess, &count, nil, requestID)
	return page, nil
}

func (service *Service) RunJob(ctx context.Context, id Identifier) error {
	if service == nil || id.IsZero() {
		return ErrInvalidInput
	}
	now := service.now().UTC()
	job, claimed, err := service.store.ClaimJob(ctx, id, now)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	user, err := service.store.CurrentUser(ctx, job.OwnerUserID)
	if err != nil || !user.Active || !user.Role.Valid() || !user.Role.CanSearch() {
		return service.failWorkerJob(ctx, job, "permission_revoked", ErrForbidden)
	}
	draft, err := service.store.GetDraftForWorker(ctx, job.DraftID)
	if err != nil || draft.OwnerUserID != job.OwnerUserID || draft.State != DraftReviewed || !draft.ExpiresAt.After(now) {
		return service.failWorkerJob(ctx, job, "draft_unavailable", ErrConflict)
	}
	actor := auth.Session{User: user}
	catalog, err := service.query.CatalogV2(ctx, actor, "task-worker")
	if err != nil {
		return service.failWorkerJob(ctx, job, "catalog_unavailable", normalizeQueryError(err))
	}
	if catalog.Version != draft.CatalogVersion || job.CatalogVersion != draft.CatalogVersion {
		return service.failWorkerJob(ctx, job, "stale_catalog", queryengine.ErrStaleCatalog)
	}
	normalized, fingerprintText, err := NormalizeAndValidate(draft.Spec, catalog, true)
	if err != nil {
		return service.failWorkerJob(ctx, job, "invalid_spec", err)
	}
	fingerprint, err := decodeFingerprint(fingerprintText)
	if err != nil || fingerprint != draft.SpecFingerprint {
		return service.failWorkerJob(ctx, job, "stale_spec", ErrConflict)
	}
	if _, err := service.store.RecordProgress(ctx, job.ID, EventCatalogValidated, 0, len(normalized.Roles), 0, service.now().UTC()); err != nil {
		return err
	}
	plans, err := BuildCandidatePlans(normalized, catalog)
	if err != nil {
		return service.failWorkerJob(ctx, job, "candidate_plan_invalid", err)
	}
	if _, err := service.store.RecordProgress(ctx, job.ID, EventCandidatesStarted, 0, len(plans), 0, service.now().UTC()); err != nil {
		return err
	}
	sets := make([]CandidateSet, 0, len(plans))
	totalCandidates := 0
	for index, plan := range plans {
		if err := ctx.Err(); err != nil {
			return service.cancelWorkerJob(ctx, job, err)
		}
		current, loadErr := service.store.GetJobForWorker(ctx, job.ID)
		if loadErr != nil {
			return loadErr
		}
		if current.CancelRequestedAt != nil || current.State == JobCancelled {
			return service.cancelWorkerJob(ctx, job, ErrCancelled)
		}
		result, runErr := service.query.RunV2(ctx, actor, plan.Plan, "task-worker")
		if runErr != nil {
			return service.failWorkerJob(ctx, job, "candidate_query_failed", normalizeQueryError(runErr))
		}
		candidates, candidateErr := candidatesFromResult(plan, result)
		if candidateErr != nil {
			return service.failWorkerJob(ctx, job, "unsafe_candidate_result", candidateErr)
		}
		totalCandidates += len(candidates)
		sets = append(sets, CandidateSet{Role: plan.Role, Candidates: candidates})
		if _, err := service.store.RecordProgress(ctx, job.ID, EventCandidatesReady, index+1, len(plans), totalCandidates, service.now().UTC()); err != nil {
			return err
		}
	}
	if _, err := service.store.RecordProgress(ctx, job.ID, EventSolverStarted, len(plans), len(plans), totalCandidates, service.now().UTC()); err != nil {
		return err
	}
	solveContext, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()
	result, err := Solve(solveContext, normalized, sets, service.solveLimits)
	if err != nil {
		return service.failWorkerJob(ctx, job, "solver_failed", err)
	}
	if solveContext.Err() != nil && result.State != SolveCancelled {
		result.State, result.LimitCode = SolveIncomplete, "duration_limit"
	}
	state, code := taskTerminalState(result)
	completed, err := service.store.CompleteJob(context.WithoutCancel(ctx), CompleteJobInput{JobID: job.ID, State: state, Result: result, ErrorCode: code, Now: service.now().UTC()})
	if err != nil {
		return err
	}
	count := completed.CompositionCount
	outcome := auth.AuditOutcomeSuccess
	eventType := AuditJobCompleted
	if state == JobFailed || state == JobCancelled {
		outcome, eventType = auth.AuditOutcomeFailure, AuditJobFailed
	}
	service.audit(context.WithoutCancel(ctx), &job.OwnerUserID, &job.DraftID, &job.ID, eventType, outcome, &count, errorForCode(code), "task-worker")
	return nil
}

func (service *Service) Recover(ctx context.Context) (int, error) {
	now := service.now().UTC()
	count, err := service.store.RecoverStaleJobs(ctx, now.Add(-service.timeout-time.Minute), now, service.recoveryBatch)
	if err == nil && count > 0 {
		service.audit(ctx, nil, nil, nil, AuditJobRecovered, auth.AuditOutcomeSuccess, &count, nil, "task-recovery")
	}
	return count, err
}

func (service *Service) Cleanup(ctx context.Context) (int, error) {
	return service.store.DeleteExpired(ctx, service.now().UTC(), service.recoveryBatch)
}

func (service *Service) failWorkerJob(ctx context.Context, job Job, code string, cause error) error {
	_, failErr := service.store.FailJob(context.WithoutCancel(ctx), job.ID, code, JobFailed, service.now().UTC())
	service.audit(context.WithoutCancel(ctx), &job.OwnerUserID, &job.DraftID, &job.ID, AuditJobFailed, auth.AuditOutcomeFailure, nil, cause, "task-worker")
	if failErr != nil {
		return errors.Join(cause, failErr)
	}
	return cause
}

func (service *Service) cancelWorkerJob(ctx context.Context, job Job, cause error) error {
	_, failErr := service.store.FailJob(context.WithoutCancel(ctx), job.ID, "cancelled", JobCancelled, service.now().UTC())
	if failErr != nil && !errors.Is(failErr, ErrConflict) {
		return errors.Join(cause, failErr)
	}
	return cause
}

func (service *Service) authorize(ctx context.Context, actor auth.Session) (auth.User, error) {
	if actor.User.ID == (auth.Identifier{}) {
		return auth.User{}, ErrForbidden
	}
	user, err := service.store.CurrentUser(ctx, actor.User.ID)
	if err != nil {
		return auth.User{}, err
	}
	if user.ID != actor.User.ID || !user.Active || !user.Role.Valid() || !user.Role.CanSearch() {
		return auth.User{}, ErrForbidden
	}
	return user, nil
}

func (service *Service) audit(ctx context.Context, actor *auth.Identifier, draftID, jobID *Identifier, eventType AuditEventType, outcome auth.AuditOutcome, affected *int, cause error, requestID string) {
	id, err := queryengine.NewIdentifier()
	if err != nil {
		return
	}
	event := AuditEvent{ID: id, ActorUserID: actor, DraftID: draftID, JobID: jobID, EventType: eventType, Outcome: outcome, AffectedCount: affected, ErrorCode: stableTaskErrorCode(cause), RequestID: truncateTask(requestID, 128), CreatedAt: service.now().UTC()}
	if err := service.store.SaveAudit(context.WithoutCancel(ctx), event); err != nil && service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

func candidatesFromResult(plan CandidatePlan, result queryengine.AdvancedResult) ([]Candidate, error) {
	columns := make(map[int]string, len(result.Columns))
	for _, column := range result.Columns {
		columns[column.Position] = column.FieldKey
	}
	values := make([]Candidate, 0, len(result.Rows))
	for _, row := range result.Rows {
		candidate := Candidate{Entity: row.EntityKind, ID: row.EntityID, Label: row.EntityLabel, Values: make(map[string]string), Evidence: make([]Evidence, 0, len(plan.RequirementKeys))}
		for _, cell := range row.Cells {
			field, ok := columns[cell.ColumnPosition]
			if !ok || cell.IsNull {
				continue
			}
			text, ok := taskCellText(cell)
			if !ok {
				return nil, ErrUnsafeResult
			}
			candidate.Values[field] = text
		}
		for _, requirement := range plan.RequirementKeys {
			candidate.Evidence = append(candidate.Evidence, Evidence{Requirement: requirement, Role: plan.Role, SourceEntity: row.EntityKind, SourceID: row.EntityID, Satisfied: true, Code: "requirement_satisfied"})
		}
		values = append(values, candidate)
	}
	return values, nil
}

func taskCellText(cell queryengine.ResultCell) (string, bool) {
	switch {
	case cell.TextValue != nil:
		return *cell.TextValue, true
	case cell.IntegerValue != nil:
		return fmt.Sprintf("%d", *cell.IntegerValue), true
	case cell.DecimalValue != nil:
		return *cell.DecimalValue, true
	case cell.BooleanValue != nil:
		if *cell.BooleanValue {
			return "true", true
		}
		return "false", true
	case cell.CivilDateValue != nil:
		return *cell.CivilDateValue, true
	case cell.TimestampValue != nil:
		return cell.TimestampValue.UTC().Format(time.RFC3339Nano), true
	default:
		return "", false
	}
}

func taskTerminalState(result SolveResult) (JobState, string) {
	switch result.State {
	case SolveComplete, SolveNoSolution:
		return JobCompleted, ""
	case SolveIncomplete:
		return JobIncomplete, truncateTask(result.LimitCode, 80)
	case SolveCancelled:
		return JobCancelled, "cancelled"
	default:
		return JobFailed, "solver_failed"
	}
}

func safeResultPage(page ResultPage) bool {
	if page.Total < 0 || page.Total > MaximumSolutions || page.Limit < 1 || page.Limit > MaximumResultPage || page.Offset < 0 || len(page.Compositions) > page.Limit {
		return false
	}
	for index, composition := range page.Compositions {
		if composition.Position != page.Offset+index || len(composition.Evidence) > MaximumEvidencePerResult {
			return false
		}
		for _, evidence := range composition.Evidence {
			if evidence.SourceEntity == "" || evidence.SourceID == "" || evidence.Role == "" {
				return false
			}
		}
	}
	return true
}

func validTaskIdempotency(value string) bool {
	return len(value) >= MinimumIdempotency && len(value) <= MaximumIdempotency && taskIdempotencyPattern.MatchString(value)
}

func taskJobFingerprint(owner auth.Identifier, draft Draft, retryOf *Identifier) [32]byte {
	payload := struct {
		Owner string `json:"owner"`
		Draft string `json:"draft"`
		Spec  string `json:"spec"`
		Retry string `json:"retry,omitempty"`
	}{Owner: fmt.Sprintf("%x", owner), Draft: draft.ID.String(), Spec: hex.EncodeToString(draft.SpecFingerprint[:])}
	if retryOf != nil {
		payload.Retry = retryOf.String()
	}
	encoded, _ := json.Marshal(payload)
	return sha256.Sum256(encoded)
}

func decodeFingerprint(value string) ([32]byte, error) {
	var result [32]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != len(result) {
		return result, ErrInvalidSpec
	}
	copy(result[:], decoded)
	return result, nil
}

func normalizeQueryError(err error) error {
	switch {
	case errors.Is(err, queryengine.ErrForbidden):
		return ErrForbidden
	case errors.Is(err, queryengine.ErrStaleCatalog):
		return ErrStaleCatalog
	case errors.Is(err, queryengine.ErrTimeout):
		return ErrTimeout
	case errors.Is(err, queryengine.ErrCancelled):
		return ErrCancelled
	case errors.Is(err, queryengine.ErrRateLimited):
		return ErrRateLimited
	default:
		return err
	}
}

func stableTaskErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	case errors.Is(err, ErrStaleCatalog), errors.Is(err, queryengine.ErrStaleCatalog):
		return "stale_catalog"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrTimeout):
		return "timeout"
	case errors.Is(err, ErrCancelled), errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, ErrInvalidSpec):
		return "invalid_spec"
	default:
		return "failure"
	}
}

func errorForCode(value string) error {
	if value == "" {
		return nil
	}
	return errors.New(value)
}

func truncateTask(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maximum {
		return value
	}
	return value[:maximum]
}

func sortCandidates(values []Candidate) {
	sort.Slice(values, func(left, right int) bool {
		if values[left].Entity != values[right].Entity {
			return values[left].Entity < values[right].Entity
		}
		return values[left].ID < values[right].ID
	})
}

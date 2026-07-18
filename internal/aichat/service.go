package aichat

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	defaultRunRateLimit = 30
	defaultUsageLimit   = 200_000
	defaultCleanupBatch = 100
	defaultRunTimeout   = 45 * time.Second
	staleRunGrace       = 5 * time.Second
)

var idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	Now            func() time.Time
	Retention      time.Duration
	RunTimeout     time.Duration
	RateLimit      int
	UsageLimit     int64
	CleanupBatch   int
	OnAuditFailure AuditFailureHandler
}

type Service struct {
	store          Store
	now            func() time.Time
	retention      time.Duration
	runTimeout     time.Duration
	rateLimit      int
	usageLimit     int64
	cleanupBatch   int
	onAuditFailure AuditFailureHandler
}

func NewService(store Store, options ServiceOptions) (*Service, error) {
	if store == nil || options.Retention < time.Hour || options.Retention > 365*24*time.Hour {
		return nil, ErrInvalidSetup
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.RunTimeout == 0 {
		options.RunTimeout = defaultRunTimeout
	}
	if options.RateLimit == 0 {
		options.RateLimit = defaultRunRateLimit
	}
	if options.UsageLimit == 0 {
		options.UsageLimit = defaultUsageLimit
	}
	if options.CleanupBatch == 0 {
		options.CleanupBatch = defaultCleanupBatch
	}
	if options.RunTimeout < time.Second || options.RunTimeout > 5*time.Minute ||
		options.RateLimit < 1 || options.RateLimit > 10_000 ||
		options.UsageLimit < 1 || options.UsageLimit > 100_000_000 ||
		options.CleanupBatch < 1 || options.CleanupBatch > 1000 {
		return nil, ErrInvalidSetup
	}
	return &Service{
		store: store, now: options.Now, retention: options.Retention, runTimeout: options.RunTimeout,
		rateLimit: options.RateLimit, usageLimit: options.UsageLimit, cleanupBatch: options.CleanupBatch, onAuditFailure: options.OnAuditFailure,
	}, nil
}

func (service *Service) Capability() Capability {
	capability := DefaultCapability()
	capability.Enabled = true
	capability.MaximumUsage = service.usageLimit
	capability.MaximumDuration = service.runTimeout
	return capability
}

func (service *Service) CreateThread(ctx context.Context, actor auth.Session, title, requestID string) (Thread, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, nil, nil, AuditThreadCreated, auditOutcome(err), nil, nil, err, requestID)
		return Thread{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = DefaultThreadTitle
	}
	if !validText(title, MaximumThreadTitleRunes, false) {
		service.audit(ctx, &user.ID, nil, nil, nil, AuditThreadCreated, auth.AuditOutcomeDenied, nil, nil, ErrInvalidInput, requestID)
		return Thread{}, ErrInvalidInput
	}
	id, err := NewIdentifier()
	if err != nil {
		return Thread{}, fmt.Errorf("generate AI Chat thread identifier: %w", err)
	}
	now := service.now().UTC()
	thread, err := service.store.CreateThread(ctx, CreateThreadInput{
		ID: id, OwnerUserID: user.ID, Title: title, RetentionExpiresAt: now.Add(service.retention), Now: now,
	})
	service.audit(ctx, &user.ID, &id, nil, nil, AuditThreadCreated, auditOutcome(err), nil, nil, err, requestID)
	return thread, err
}

func (service *Service) Threads(ctx context.Context, actor auth.Session, limit, offset int, requestID string) (ThreadPage, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, nil, nil, AuditThreadRead, auditOutcome(err), nil, nil, err, requestID)
		return ThreadPage{}, err
	}
	if limit == 0 {
		limit = MaximumThreadsPage
	}
	if limit < 1 || limit > MaximumThreadsPage || offset < 0 || offset > 10_000 {
		return ThreadPage{}, ErrInvalidInput
	}
	page, err := service.store.ListThreads(ctx, user.ID, service.now().UTC(), limit, offset)
	affected := len(page.Threads)
	service.audit(ctx, &user.ID, nil, nil, nil, AuditThreadRead, auditOutcome(err), nil, &affected, err, requestID)
	return page, err
}

func (service *Service) Thread(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Thread, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() {
		if err == nil {
			err = ErrNotFound
		}
		service.audit(ctx, auditActor(actor), &id, nil, nil, AuditThreadRead, auditOutcome(err), nil, nil, err, requestID)
		return Thread{}, err
	}
	thread, err := service.store.GetThread(ctx, id, user.ID)
	if err == nil && !thread.RetentionExpiresAt.After(service.now().UTC()) {
		err = ErrStaleContext
	}
	service.audit(ctx, &user.ID, &id, nil, nil, AuditThreadRead, auditOutcome(err), nil, nil, err, requestID)
	return thread, err
}

func (service *Service) RenameThread(ctx context.Context, actor auth.Session, id Identifier, title string, version int64, requestID string) (Thread, error) {
	user, err := service.authorize(ctx, actor)
	title = strings.TrimSpace(title)
	if err != nil || id.IsZero() || version < 1 || !validText(title, MaximumThreadTitleRunes, false) {
		if err == nil {
			err = ErrInvalidInput
		}
		service.audit(ctx, auditActor(actor), &id, nil, nil, AuditThreadRenamed, auditOutcome(err), nil, nil, err, requestID)
		return Thread{}, err
	}
	current, err := service.store.GetThread(ctx, id, user.ID)
	if err != nil || !current.RetentionExpiresAt.After(service.now().UTC()) {
		if err == nil {
			err = ErrStaleContext
		}
		service.audit(ctx, &user.ID, &id, nil, nil, AuditThreadRenamed, auditOutcome(err), nil, nil, err, requestID)
		return Thread{}, err
	}
	thread, err := service.store.RenameThread(ctx, id, user.ID, title, version, service.now().UTC())
	service.audit(ctx, &user.ID, &id, nil, nil, AuditThreadRenamed, auditOutcome(err), nil, nil, err, requestID)
	return thread, err
}

func (service *Service) DeleteThread(ctx context.Context, actor auth.Session, id Identifier, requestID string) error {
	user, err := service.authorize(ctx, actor)
	if err == nil && id.IsZero() {
		err = ErrNotFound
	}
	if err == nil {
		err = service.store.DeleteThread(ctx, id, user.ID)
	}
	service.audit(ctx, auditActorOrUser(actor, user), &id, nil, nil, AuditThreadDeleted, auditOutcome(err), nil, nil, err, requestID)
	return err
}

func (service *Service) Messages(ctx context.Context, actor auth.Session, threadID Identifier, limit, offset int, requestID string) (MessagePage, error) {
	user, err := service.authorize(ctx, actor)
	var auditedThread *Identifier
	if !threadID.IsZero() {
		auditedThread = &threadID
	}
	if err != nil || threadID.IsZero() {
		if err == nil {
			err = ErrNotFound
		}
		service.audit(ctx, auditActor(actor), auditedThread, nil, nil, AuditMessagesRead, auditOutcome(err), nil, nil, err, requestID)
		return MessagePage{}, err
	}
	if limit == 0 {
		limit = MaximumMessagesPage
	}
	if limit < 1 || limit > MaximumMessagesPage || offset < 0 || offset > 100_000 {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditMessagesRead, auth.AuditOutcomeDenied, nil, nil, ErrInvalidInput, requestID)
		return MessagePage{}, ErrInvalidInput
	}
	thread, err := service.store.GetThread(ctx, threadID, user.ID)
	if err != nil {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditMessagesRead, auditOutcome(err), nil, nil, err, requestID)
		return MessagePage{}, err
	}
	if !thread.RetentionExpiresAt.After(service.now().UTC()) {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditMessagesRead, auth.AuditOutcomeDenied, nil, nil, ErrStaleContext, requestID)
		return MessagePage{}, ErrStaleContext
	}
	page, err := service.store.ListMessages(ctx, threadID, user.ID, limit, offset)
	affected := len(page.Messages)
	service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditMessagesRead, auditOutcome(err), nil, &affected, err, requestID)
	return page, err
}

func (service *Service) StartTurn(ctx context.Context, actor auth.Session, threadID Identifier, content, idempotencyKey string, retryOf *Identifier, requestID string) (RunCreation, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), &threadID, nil, nil, AuditRunCreated, auditOutcome(err), nil, nil, err, requestID)
		return RunCreation{}, err
	}
	content = strings.TrimSpace(content)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	var auditedThread *Identifier
	if !threadID.IsZero() {
		auditedThread = &threadID
	}
	if threadID.IsZero() || !validText(content, MaximumMessageRunes, true) || !validIdempotencyKey(idempotencyKey) || (retryOf != nil && retryOf.IsZero()) {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditRunCreated, auth.AuditOutcomeDenied, nil, nil, ErrInvalidInput, requestID)
		return RunCreation{}, ErrInvalidInput
	}
	thread, err := service.store.GetThread(ctx, threadID, user.ID)
	if err != nil {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditRunCreated, auditOutcome(err), nil, nil, err, requestID)
		return RunCreation{}, err
	}
	if !thread.RetentionExpiresAt.After(service.now().UTC()) {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditRunCreated, auth.AuditOutcomeDenied, nil, nil, ErrStaleContext, requestID)
		return RunCreation{}, ErrStaleContext
	}
	fingerprint, err := turnFingerprint(threadID, content, thread.ActiveResultReferenceID, retryOf)
	if err != nil {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditRunCreated, auth.AuditOutcomeFailure, nil, nil, err, requestID)
		return RunCreation{}, fmt.Errorf("fingerprint AI Chat turn: %w", err)
	}
	runID, err := NewIdentifier()
	if err != nil {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditRunCreated, auth.AuditOutcomeFailure, nil, nil, err, requestID)
		return RunCreation{}, fmt.Errorf("generate AI Chat run identifier: %w", err)
	}
	messageID, err := NewIdentifier()
	if err != nil {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditRunCreated, auth.AuditOutcomeFailure, nil, nil, err, requestID)
		return RunCreation{}, fmt.Errorf("generate AI Chat message identifier: %w", err)
	}
	now := service.now().UTC()
	created, err := service.store.CreateRun(ctx, CreateRunInput{
		ID: runID, MessageID: messageID, ThreadID: threadID, OwnerUserID: user.ID, RetryOfRunID: retryOf,
		ActiveResultReferenceID: thread.ActiveResultReferenceID, IdempotencyKey: idempotencyKey,
		RequestFingerprint: fingerprint, Content: content, Now: now,
	}, now.Truncate(time.Hour), service.rateLimit, service.usageLimit)
	var auditedRun *Identifier
	if !created.Run.ID.IsZero() {
		auditedRun = &created.Run.ID
		auditedThread = nil
	}
	service.audit(ctx, &user.ID, auditedThread, auditedRun, nil, AuditRunCreated, auditOutcome(err), nil, nil, err, requestID)
	return created, err
}

func (service *Service) Run(ctx context.Context, actor auth.Session, id Identifier) (Run, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() {
		if err == nil {
			err = ErrNotFound
		}
		return Run{}, err
	}
	return service.store.GetRun(ctx, id, user.ID)
}

func (service *Service) CancelRun(ctx context.Context, actor auth.Session, id Identifier, requestID string) (Run, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || id.IsZero() {
		if err == nil {
			err = ErrNotFound
		}
		service.audit(ctx, auditActor(actor), nil, &id, nil, AuditRunCancelled, auditOutcome(err), nil, nil, err, requestID)
		return Run{}, err
	}
	run, err := service.store.RequestCancellation(ctx, id, user.ID, service.now().UTC())
	service.audit(ctx, &user.ID, nil, &id, nil, AuditRunCancelled, auditOutcome(err), nil, nil, err, requestID)
	return run, err
}

func (service *Service) Events(ctx context.Context, actor auth.Session, runID Identifier, afterSequence int64, limit int) (EventPage, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil || runID.IsZero() {
		if err == nil {
			err = ErrNotFound
		}
		return EventPage{}, err
	}
	if limit == 0 {
		limit = MaximumEventsPage
	}
	if afterSequence < 0 || limit < 1 || limit > MaximumEventsPage {
		return EventPage{}, ErrInvalidInput
	}
	return service.store.ListRunEvents(ctx, runID, user.ID, afterSequence, limit)
}

func (service *Service) ResultReference(ctx context.Context, actor auth.Session, id Identifier, requestID string) (ResultReference, error) {
	user, err := service.authorize(ctx, actor)
	var auditedReference *Identifier
	if !id.IsZero() {
		auditedReference = &id
	}
	if err != nil || id.IsZero() {
		if err == nil {
			err = ErrNotFound
		}
		service.audit(ctx, auditActor(actor), nil, nil, auditedReference, AuditResultRead, auditOutcome(err), nil, nil, err, requestID)
		return ResultReference{}, err
	}
	value, err := service.store.GetResultReference(ctx, id, user.ID)
	if err == nil && !value.ExpiresAt.After(service.now().UTC()) {
		err = ErrStaleContext
	}
	service.audit(ctx, &user.ID, nil, nil, &id, AuditResultRead, auditOutcome(err), nil, nil, err, requestID)
	return value, err
}

func (service *Service) SetActiveResult(ctx context.Context, actor auth.Session, threadID Identifier, referenceID *Identifier, requestID string) (Thread, error) {
	user, err := service.authorize(ctx, actor)
	var auditedThread *Identifier
	if !threadID.IsZero() {
		auditedThread = &threadID
	}
	if err != nil || threadID.IsZero() || (referenceID != nil && referenceID.IsZero()) {
		if err == nil {
			err = ErrInvalidInput
		}
		service.audit(ctx, auditActor(actor), auditedThread, nil, nil, AuditContextChanged, auditOutcome(err), nil, nil, err, requestID)
		return Thread{}, err
	}
	thread, err := service.store.GetThread(ctx, threadID, user.ID)
	if err != nil {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditContextChanged, auditOutcome(err), nil, nil, err, requestID)
		return Thread{}, err
	}
	if !thread.RetentionExpiresAt.After(service.now().UTC()) {
		service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditContextChanged, auth.AuditOutcomeDenied, nil, nil, ErrStaleContext, requestID)
		return Thread{}, ErrStaleContext
	}
	updated, err := service.store.SetActiveResultReference(ctx, threadID, user.ID, referenceID, service.now().UTC())
	service.audit(ctx, &user.ID, auditedThread, nil, nil, AuditContextChanged, auditOutcome(err), nil, nil, err, requestID)
	return updated, err
}

func (service *Service) CleanupExpired(ctx context.Context) (int, error) {
	now := service.now().UTC()
	recovered, recoveryErr := service.store.FailStaleRuns(ctx, now.Add(-service.runTimeout-staleRunGrace), now, service.cleanupBatch)
	service.audit(ctx, nil, nil, nil, nil, AuditRunRecovered, auditOutcome(recoveryErr), nil, &recovered, recoveryErr, "run-recovery")
	if recoveryErr != nil {
		return 0, recoveryErr
	}
	affected, err := service.store.CleanupExpired(ctx, now, service.cleanupBatch)
	service.audit(ctx, nil, nil, nil, nil, AuditRetentionClean, auditOutcome(err), nil, &affected, err, "retention-cleanup")
	return affected, err
}

func (service *Service) authorize(ctx context.Context, actor auth.Session) (auth.User, error) {
	if actor.User.ID == (auth.Identifier{}) || !actor.User.Active || !actor.User.Role.CanSearch() {
		return auth.User{}, ErrForbidden
	}
	current, err := service.store.CurrentUser(ctx, actor.User.ID)
	if err != nil {
		return auth.User{}, err
	}
	if !current.Active || !current.Role.CanSearch() {
		return auth.User{}, ErrForbidden
	}
	return current, nil
}

func validIdempotencyKey(value string) bool {
	return len(value) >= MinimumIdempotencySize && len(value) <= MaximumIdempotencySize && idempotencyPattern.MatchString(value)
}

func validText(value string, maximum int, allowNewlines bool) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) && (!allowNewlines || (character != '\n' && character != '\r' && character != '\t')) {
			return false
		}
	}
	return true
}

func turnFingerprint(threadID Identifier, content string, active, retry *Identifier) ([sha256.Size]byte, error) {
	type payload struct {
		ThreadID string `json:"thread_id"`
		Content  string `json:"content"`
		Active   string `json:"active_result_reference_id,omitempty"`
		Retry    string `json:"retry_of_run_id,omitempty"`
	}
	value := payload{ThreadID: threadID.String(), Content: content}
	if active != nil {
		value.Active = active.String()
	}
	if retry != nil {
		value.Retry = retry.String()
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

func (service *Service) audit(ctx context.Context, actor *auth.Identifier, thread, run, reference *Identifier, eventType AuditEventType, outcome auth.AuditOutcome, tool *ToolKind, affected *int, sourceErr error, requestID string) {
	id, err := NewIdentifier()
	if err != nil {
		if service.onAuditFailure != nil {
			service.onAuditFailure(ctx, AuditEvent{EventType: eventType, Outcome: outcome, RequestID: safeRequestID(requestID)}, err)
		}
		return
	}
	event := AuditEvent{ID: id, ActorUserID: actor, ThreadID: thread, RunID: run, ResultReferenceID: reference,
		EventType: eventType, Outcome: outcome, ToolKind: tool, AffectedCount: affected, ErrorCode: publicErrorCode(sourceErr),
		RequestID: safeRequestID(requestID), CreatedAt: service.now().UTC()}
	if err := service.store.SaveAudit(ctx, event); err != nil && service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

func auditOutcome(err error) auth.AuditOutcome {
	switch {
	case err == nil:
		return auth.AuditOutcomeSuccess
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrNotFound), errors.Is(err, ErrConflict),
		errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidState), errors.Is(err, ErrRateLimited),
		errors.Is(err, ErrQuotaExceeded), errors.Is(err, ErrStaleContext):
		return auth.AuditOutcomeDenied
	default:
		return auth.AuditOutcomeFailure
	}
}

func publicErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrConflict):
		return "conflict"
	case errors.Is(err, ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, ErrInvalidState):
		return "invalid_state"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrQuotaExceeded):
		return "quota_exceeded"
	case errors.Is(err, ErrUnavailable):
		return "unavailable"
	case errors.Is(err, ErrTimeout):
		return "timeout"
	case errors.Is(err, ErrCancelled):
		return "cancelled"
	case errors.Is(err, ErrMalformedProvider):
		return "malformed_provider"
	case errors.Is(err, ErrStaleContext):
		return "stale_context"
	case errors.Is(err, ErrToolFailed):
		return "tool_failed"
	case errors.Is(err, ErrUnsafeResult):
		return "unsafe_result"
	default:
		return "internal_error"
	}
}

func errorFromPublicCode(code string) error {
	switch code {
	case "forbidden":
		return ErrForbidden
	case "not_found":
		return ErrNotFound
	case "conflict":
		return ErrConflict
	case "invalid_input":
		return ErrInvalidInput
	case "invalid_state":
		return ErrInvalidState
	case "rate_limited":
		return ErrRateLimited
	case "quota_exceeded":
		return ErrQuotaExceeded
	case "unavailable":
		return ErrUnavailable
	case "timeout":
		return ErrTimeout
	case "cancelled":
		return ErrCancelled
	case "malformed_provider":
		return ErrMalformedProvider
	case "stale_context":
		return ErrStaleContext
	case "tool_failed":
		return ErrToolFailed
	case "unsafe_result":
		return ErrUnsafeResult
	default:
		return errors.New("AI Chat internal failure")
	}
}

func auditActor(actor auth.Session) *auth.Identifier {
	if actor.User.ID == (auth.Identifier{}) {
		return nil
	}
	value := actor.User.ID
	return &value
}

func auditActorOrUser(actor auth.Session, user auth.User) *auth.Identifier {
	if user.ID != (auth.Identifier{}) {
		value := user.ID
		return &value
	}
	return auditActor(actor)
}

func safeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		return value[:128]
	}
	return value
}

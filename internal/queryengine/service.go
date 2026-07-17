package queryengine

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	defaultExecutionTimeout = 3 * time.Second
	defaultResultRetention  = time.Hour
	defaultExecutionRate    = 30
	defaultCleanupBatch     = 100
	maximumResultTextRunes  = 5000
)

var idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	Now             func() time.Time
	Timeout         time.Duration
	ResultRetention time.Duration
	RateLimit       int
	MaximumCost     int
	CleanupBatch    int
	OnAuditFailure  AuditFailureHandler
}

type Service struct {
	store          Store
	now            func() time.Time
	timeout        time.Duration
	retention      time.Duration
	rateLimit      int
	maximumCost    int
	cleanupBatch   int
	onAuditFailure AuditFailureHandler
}

func NewService(store Store, options ServiceOptions) (*Service, error) {
	if store == nil {
		return nil, ErrInvalidSetup
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Timeout == 0 {
		options.Timeout = defaultExecutionTimeout
	}
	if options.ResultRetention == 0 {
		options.ResultRetention = defaultResultRetention
	}
	if options.RateLimit == 0 {
		options.RateLimit = defaultExecutionRate
	}
	if options.MaximumCost == 0 {
		options.MaximumCost = defaultMaximumCost
	}
	if options.CleanupBatch == 0 {
		options.CleanupBatch = defaultCleanupBatch
	}
	if options.Timeout < 100*time.Millisecond || options.Timeout > 10*time.Second ||
		options.ResultRetention < 5*time.Minute || options.ResultRetention > 24*time.Hour ||
		options.RateLimit < 1 || options.RateLimit > 10_000 ||
		options.MaximumCost < 1 || options.CleanupBatch < 1 || options.CleanupBatch > 1000 {
		return nil, ErrInvalidSetup
	}
	return &Service{
		store: store, now: options.Now, timeout: options.Timeout, retention: options.ResultRetention,
		rateLimit: options.RateLimit, maximumCost: options.MaximumCost, cleanupBatch: options.CleanupBatch,
		onAuditFailure: options.OnAuditFailure,
	}, nil
}

func (service *Service) Catalog(ctx context.Context, actor auth.Session, requestID string) (Catalog, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, AuditCatalogRead, auditOutcome(err), nil, requestID)
		return Catalog{}, err
	}
	resolved, err := loadCatalog(ctx, service.store, user.Role)
	if err != nil {
		service.audit(ctx, &user.ID, nil, AuditCatalogRead, auditOutcome(err), nil, requestID)
		return Catalog{}, err
	}
	service.audit(ctx, &user.ID, nil, AuditCatalogRead, auth.AuditOutcomeSuccess, nil, requestID)
	return resolved.Public, nil
}

func (service *Service) Validate(ctx context.Context, actor auth.Session, plan QueryPlan, requestID string) (PlanEstimate, error) {
	user, catalog, err := service.authorizedCatalog(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, AuditPlanValidated, auditOutcome(err), nil, requestID)
		return PlanEstimate{}, err
	}
	compiled, _, err := compilePlan(plan, catalog, service.maximumCost)
	if err != nil {
		service.audit(ctx, &user.ID, nil, AuditPlanValidated, auditOutcome(err), nil, requestID)
		return PlanEstimate{}, err
	}
	service.audit(ctx, &user.ID, nil, AuditPlanValidated, auth.AuditOutcomeSuccess, nil, requestID)
	return PlanEstimate{Valid: true, Fingerprint: fingerprintString(compiled.Fingerprint), Cost: compiled.Cost, Columns: compiled.Columns}, nil
}

func (service *Service) Execute(ctx context.Context, actor auth.Session, plan QueryPlan, idempotencyKey, requestID string) (Execution, error) {
	user, catalog, err := service.authorizedCatalog(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, AuditExecutionStarted, auditOutcome(err), nil, requestID)
		return Execution{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) < MinimumIdempotencySize || len(idempotencyKey) > MaximumIdempotencySize || !idempotencyPattern.MatchString(idempotencyKey) {
		service.audit(ctx, &user.ID, nil, AuditExecutionStarted, auth.AuditOutcomeDenied, nil, requestID)
		return Execution{}, ErrInvalidPlan
	}
	compiled, _, err := compilePlan(plan, catalog, service.maximumCost)
	if err != nil {
		service.audit(ctx, &user.ID, nil, AuditExecutionStarted, auditOutcome(err), nil, requestID)
		return Execution{}, err
	}
	id, err := NewIdentifier()
	if err != nil {
		service.audit(ctx, &user.ID, nil, AuditExecutionStarted, auth.AuditOutcomeFailure, nil, requestID)
		return Execution{}, fmt.Errorf("generate query execution identifier: %w", err)
	}
	now := service.now().UTC()
	input := ExecutionInput{
		ID: id, OwnerUserID: user.ID, IdempotencyKey: idempotencyKey,
		PlanFingerprint: compiled.Fingerprint, CatalogVersion: compiled.CatalogVersion,
		RootEntity: compiled.RootEntity, MaximumRows: compiled.MaximumRows,
		StartedAt: now, ExpiresAt: now.Add(service.retention),
	}
	execution, created, err := service.store.CreateExecution(ctx, input, now.Truncate(time.Minute), service.rateLimit)
	if err != nil {
		service.audit(ctx, &user.ID, nil, AuditExecutionStarted, auditOutcome(err), nil, requestID)
		return Execution{}, err
	}
	if !created {
		return execution, nil
	}
	service.audit(ctx, &user.ID, &execution.ID, AuditExecutionStarted, auth.AuditOutcomeSuccess, nil, requestID)

	queryContext, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()
	rawRows, err := service.store.ExecuteReadOnly(queryContext, compiled, service.timeout)
	if err != nil {
		return Execution{}, service.failExecution(ctx, execution, user.ID, queryContext, err, requestID)
	}
	rows, err := materializeRows(rawRows, compiled)
	if err != nil {
		return Execution{}, service.failExecution(ctx, execution, user.ID, queryContext, err, requestID)
	}
	completed, err := service.store.CompleteExecution(ctx, execution.ID, compiled.Columns, rows, service.now().UTC())
	if err != nil {
		return Execution{}, service.failExecution(ctx, execution, user.ID, queryContext, err, requestID)
	}
	affected := len(rows)
	service.audit(ctx, &user.ID, &completed.ID, AuditExecutionComplete, auth.AuditOutcomeSuccess, &affected, requestID)
	return completed, nil
}

func (service *Service) Result(ctx context.Context, actor auth.Session, executionID Identifier, limit, offset int, requestID string) (ResultPage, error) {
	user, catalog, err := service.authorizedCatalog(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), &executionID, AuditResultRead, auditOutcome(err), nil, requestID)
		return ResultPage{}, err
	}
	if executionID.IsZero() {
		service.audit(ctx, &user.ID, nil, AuditResultRead, auth.AuditOutcomeDenied, nil, requestID)
		return ResultPage{}, ErrNotFound
	}
	if limit == 0 {
		limit = MaximumPageSize
	}
	if limit < 1 || limit > MaximumPageSize || offset < 0 || offset > MaximumRows {
		service.audit(ctx, &user.ID, &executionID, AuditResultRead, auth.AuditOutcomeDenied, nil, requestID)
		return ResultPage{}, ErrInvalidPlan
	}
	execution, err := service.store.GetExecution(ctx, executionID, user.ID)
	if err != nil {
		service.audit(ctx, &user.ID, &executionID, AuditResultRead, auditOutcome(err), nil, requestID)
		return ResultPage{}, err
	}
	now := service.now().UTC()
	if !execution.ExpiresAt.After(now) {
		service.audit(ctx, &user.ID, &executionID, AuditResultExpired, auth.AuditOutcomeDenied, nil, requestID)
		return ResultPage{}, ErrExpired
	}
	if execution.State != ExecutionCompleted {
		return ResultPage{}, ErrConflict
	}
	if _, permitted := catalog.Entities[execution.RootEntity]; !permitted {
		service.audit(ctx, &user.ID, &executionID, AuditResultRead, auth.AuditOutcomeDenied, nil, requestID)
		return ResultPage{}, ErrForbidden
	}
	page, err := service.store.GetResultPage(ctx, executionID, user.ID, limit, offset)
	if err != nil {
		service.audit(ctx, &user.ID, &executionID, AuditResultRead, auditOutcome(err), nil, requestID)
		return ResultPage{}, err
	}
	if !authorizedResultPage(page, execution, catalog) {
		service.audit(ctx, &user.ID, &executionID, AuditResultRead, auth.AuditOutcomeDenied, nil, requestID)
		return ResultPage{}, ErrUnsafeResult
	}
	affected := len(page.Rows)
	service.audit(ctx, &user.ID, &executionID, AuditResultRead, auth.AuditOutcomeSuccess, &affected, requestID)
	return page, nil
}

func (service *Service) CleanupExpired(ctx context.Context) (int, error) {
	return service.store.DeleteExpired(ctx, service.now().UTC(), service.cleanupBatch)
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

func (service *Service) authorizedCatalog(ctx context.Context, actor auth.Session) (auth.User, resolvedCatalog, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return auth.User{}, resolvedCatalog{}, err
	}
	catalog, err := loadCatalog(ctx, service.store, user.Role)
	if err != nil {
		return auth.User{}, resolvedCatalog{}, err
	}
	return user, catalog, nil
}

func (service *Service) failExecution(ctx context.Context, execution Execution, actorID auth.Identifier, queryContext context.Context, cause error, requestID string) error {
	state := ExecutionFailed
	code := "execution_failed"
	publicError := cause
	switch {
	case errors.Is(cause, ErrTimeout), errors.Is(cause, context.DeadlineExceeded), errors.Is(queryContext.Err(), context.DeadlineExceeded):
		code, publicError = "timeout", ErrTimeout
	case errors.Is(cause, ErrCancelled), errors.Is(cause, context.Canceled), errors.Is(queryContext.Err(), context.Canceled):
		state, code, publicError = ExecutionCancelled, "cancelled", ErrCancelled
	case errors.Is(cause, ErrUnsafeResult):
		code, publicError = "unsafe_result", ErrUnsafeResult
	}
	failErr := service.store.FailExecution(context.WithoutCancel(ctx), execution.ID, code, state, service.now().UTC())
	service.audit(context.WithoutCancel(ctx), &actorID, &execution.ID, AuditExecutionFailed, auth.AuditOutcomeFailure, nil, requestID)
	if failErr != nil {
		return errors.Join(publicError, failErr)
	}
	return publicError
}

func materializeRows(rawRows []RawResultRow, plan CompiledPlan) ([]ResultRow, error) {
	if len(rawRows) > plan.MaximumRows || len(rawRows) > MaximumRows || len(plan.Columns) > MaximumProjections {
		return nil, ErrUnsafeResult
	}
	rows := make([]ResultRow, 0, len(rawRows))
	seen := make(map[string]struct{}, len(rawRows))
	for position, raw := range rawRows {
		if raw.EntityKind != plan.EntityKind || !safeEntityValue(raw.EntityID, 200) || !safeEntityValue(raw.EntityLabel, 300) || raw.UpdatedAt.IsZero() || len(raw.Values) != len(plan.Columns) {
			return nil, ErrUnsafeResult
		}
		key := raw.EntityKind + "\x00" + raw.EntityID
		if _, duplicate := seen[key]; duplicate {
			return nil, ErrUnsafeResult
		}
		seen[key] = struct{}{}
		row := ResultRow{Position: position, EntityKind: raw.EntityKind, EntityID: raw.EntityID, EntityLabel: raw.EntityLabel, UpdatedAt: raw.UpdatedAt.UTC(), Cells: make([]ResultCell, 0, len(raw.Values))}
		for index, value := range raw.Values {
			cell, err := materializeCell(index, plan.Columns[index].Kind, value)
			if err != nil {
				return nil, err
			}
			row.Cells = append(row.Cells, cell)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func materializeCell(position int, kind ValueKind, value *string) (ResultCell, error) {
	cell := ResultCell{ColumnPosition: position, Kind: kind, IsNull: value == nil}
	if value == nil {
		return cell, nil
	}
	if !utf8.ValidString(*value) || utf8.RuneCountInString(*value) > maximumResultTextRunes || strings.ContainsRune(*value, '\x00') {
		return ResultCell{}, ErrUnsafeResult
	}
	switch kind {
	case ValueInteger:
		parsed, err := strconv.ParseInt(*value, 10, 64)
		if err != nil {
			return ResultCell{}, ErrUnsafeResult
		}
		cell.IntegerValue = &parsed
	case ValueDecimal:
		if !decimalPattern.MatchString(*value) {
			return ResultCell{}, ErrUnsafeResult
		}
		copyValue := *value
		cell.DecimalValue = &copyValue
	case ValueBoolean:
		parsed, err := strconv.ParseBool(*value)
		if err != nil {
			return ResultCell{}, ErrUnsafeResult
		}
		cell.BooleanValue = &parsed
	case ValueCivilDate:
		if _, err := time.Parse("2006-01-02", *value); err != nil {
			return ResultCell{}, ErrUnsafeResult
		}
		copyValue := *value
		cell.CivilDateValue = &copyValue
	case ValueTimestamp:
		parsed, err := parseTimestamp(*value)
		if err != nil {
			return ResultCell{}, ErrUnsafeResult
		}
		parsed = parsed.UTC()
		cell.TimestampValue = &parsed
	case ValueCivilMonth:
		if !civilMonthPattern.MatchString(*value) {
			return ResultCell{}, ErrUnsafeResult
		}
		copyValue := *value
		cell.TextValue = &copyValue
	case ValueText, ValueLongText, ValueIdentifier, ValueEnum:
		copyValue := *value
		cell.TextValue = &copyValue
	default:
		return ResultCell{}, ErrUnsafeResult
	}
	return cell, nil
}

func parseTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999-07"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, ErrUnsafeResult
}

func safeEntityValue(value string, maximum int) bool {
	return value == strings.TrimSpace(value) && value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum && !strings.ContainsRune(value, '\x00')
}

func authorizedResultPage(page ResultPage, expected Execution, catalog resolvedCatalog) bool {
	if page.Execution.ID != expected.ID || page.Execution.OwnerUserID != expected.OwnerUserID || page.Execution.State != ExecutionCompleted ||
		page.Total < 0 || page.Total > expected.MaximumRows || page.Limit < 1 || page.Limit > MaximumPageSize || page.Offset < 0 || len(page.Columns) != expected.ColumnCount {
		return false
	}
	root, ok := catalog.Entities[expected.RootEntity]
	if !ok {
		return false
	}
	for position, column := range page.Columns {
		definition, ok := catalog.Fields[column.FieldKey]
		if !ok || definition.Public.Entity != expected.RootEntity || definition.Public.Kind != column.Kind || !definition.Public.Projectable || column.Position != position || column.Label == "" {
			return false
		}
	}
	for _, row := range page.Rows {
		if row.EntityKind != root.Public.Kind || !safeEntityValue(row.EntityID, 200) || !safeEntityValue(row.EntityLabel, 300) || row.UpdatedAt.IsZero() || row.Position < page.Offset || row.Position >= page.Offset+page.Limit || len(row.Cells) != len(page.Columns) {
			return false
		}
		for position, cell := range row.Cells {
			if cell.ColumnPosition != position || cell.Kind != page.Columns[position].Kind || !validMaterializedCell(cell) {
				return false
			}
		}
	}
	return true
}

func validMaterializedCell(cell ResultCell) bool {
	nonNull := 0
	for _, present := range []bool{cell.TextValue != nil, cell.IntegerValue != nil, cell.DecimalValue != nil, cell.BooleanValue != nil, cell.CivilDateValue != nil, cell.TimestampValue != nil} {
		if present {
			nonNull++
		}
	}
	if cell.IsNull {
		return nonNull == 0
	}
	if nonNull != 1 {
		return false
	}
	switch cell.Kind {
	case ValueInteger:
		return cell.IntegerValue != nil
	case ValueDecimal:
		return cell.DecimalValue != nil && decimalPattern.MatchString(*cell.DecimalValue)
	case ValueBoolean:
		return cell.BooleanValue != nil
	case ValueCivilDate:
		_, err := time.Parse("2006-01-02", valueOrEmpty(cell.CivilDateValue))
		return err == nil
	case ValueTimestamp:
		return cell.TimestampValue != nil && !cell.TimestampValue.IsZero()
	case ValueCivilMonth:
		return cell.TextValue != nil && civilMonthPattern.MatchString(*cell.TextValue)
	case ValueText, ValueLongText, ValueIdentifier, ValueEnum:
		return cell.TextValue != nil && utf8.ValidString(*cell.TextValue) && utf8.RuneCountInString(*cell.TextValue) <= maximumResultTextRunes && !strings.ContainsRune(*cell.TextValue, '\x00')
	default:
		return false
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (service *Service) audit(ctx context.Context, actorID *auth.Identifier, executionID *Identifier, eventType AuditEventType, outcome auth.AuditOutcome, affected *int, requestID string) {
	id, err := NewIdentifier()
	if err != nil {
		service.reportAuditFailure(ctx, AuditEvent{EventType: eventType}, err)
		return
	}
	event := AuditEvent{ID: id, ActorUserID: actorID, ExecutionID: executionID, EventType: eventType, Outcome: string(outcome),
		AffectedCount: affected, RequestID: truncateRunes(requestID, 128), CreatedAt: service.now().UTC()}
	if err := service.store.SaveAudit(ctx, event); err != nil {
		service.reportAuditFailure(ctx, event, err)
	}
}

func (service *Service) reportAuditFailure(ctx context.Context, event AuditEvent, err error) {
	if service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

func auditActor(actor auth.Session) *auth.Identifier {
	if actor.User.ID == (auth.Identifier{}) {
		return nil
	}
	id := actor.User.ID
	return &id
}

func auditOutcome(err error) auth.AuditOutcome {
	switch {
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrInvalidPlan), errors.Is(err, ErrStaleCatalog),
		errors.Is(err, ErrCostLimit), errors.Is(err, ErrRateLimited), errors.Is(err, ErrConflict), errors.Is(err, ErrExpired):
		return auth.AuditOutcomeDenied
	default:
		return auth.AuditOutcomeFailure
	}
}

package queryengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

// AdvancedResult is the bounded in-memory result used by the task worker. It
// deliberately exposes logical columns and materialized rows, never SQL.
type AdvancedResult struct {
	CatalogVersion string         `json:"catalog_version"`
	RootEntity     string         `json:"root_entity"`
	Columns        []ResultColumn `json:"columns"`
	Rows           []ResultRow    `json:"rows"`
	Cost           int            `json:"cost"`
}

func (service *Service) CatalogV2(ctx context.Context, actor auth.Session, requestID string) (Catalog, error) {
	user, catalog, err := service.authorizedAdvancedCatalog(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, AuditCatalogRead, auditOutcome(err), nil, requestID)
		return Catalog{}, err
	}
	service.audit(ctx, &user.ID, nil, AuditCatalogRead, auth.AuditOutcomeSuccess, nil, requestID)
	return catalog.Public, nil
}

func (service *Service) ValidateV2(ctx context.Context, actor auth.Session, plan QueryPlan, requestID string) (PlanEstimate, error) {
	user, catalog, err := service.authorizedAdvancedCatalog(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, AuditPlanValidated, auditOutcome(err), nil, requestID)
		return PlanEstimate{}, err
	}
	compiled, _, err := compileAdvancedForStore(plan, catalog, service.maximumCost)
	if err != nil {
		service.audit(ctx, &user.ID, nil, AuditPlanValidated, auditOutcome(err), nil, requestID)
		return PlanEstimate{}, err
	}
	service.audit(ctx, &user.ID, nil, AuditPlanValidated, auth.AuditOutcomeSuccess, nil, requestID)
	return PlanEstimate{Valid: true, Fingerprint: fingerprintString(compiled.Fingerprint), Cost: compiled.Cost, Columns: compiled.Columns}, nil
}

func (service *Service) ExecuteV2(ctx context.Context, actor auth.Session, plan QueryPlan, idempotencyKey, requestID string) (Execution, error) {
	user, catalog, err := service.authorizedAdvancedCatalog(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), nil, AuditExecutionStarted, auditOutcome(err), nil, requestID)
		return Execution{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) < MinimumIdempotencySize || len(idempotencyKey) > MaximumIdempotencySize || !idempotencyPattern.MatchString(idempotencyKey) {
		service.audit(ctx, &user.ID, nil, AuditExecutionStarted, auth.AuditOutcomeDenied, nil, requestID)
		return Execution{}, ErrInvalidPlan
	}
	compiled, _, err := compileAdvancedForStore(plan, catalog, service.maximumCost)
	if err != nil {
		service.audit(ctx, &user.ID, nil, AuditExecutionStarted, auditOutcome(err), nil, requestID)
		return Execution{}, err
	}
	id, err := NewIdentifier()
	if err != nil {
		return Execution{}, fmt.Errorf("generate advanced query execution identifier: %w", err)
	}
	now := service.now().UTC()
	execution, created, err := service.store.CreateExecution(ctx, ExecutionInput{
		ID: id, OwnerUserID: user.ID, IdempotencyKey: idempotencyKey,
		PlanFingerprint: compiled.Fingerprint, CatalogVersion: compiled.CatalogVersion,
		RootEntity: compiled.RootEntity, MaximumRows: compiled.MaximumRows,
		StartedAt: now, ExpiresAt: now.Add(service.retention),
	}, now.Truncate(time.Minute), service.rateLimit)
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

func (service *Service) ResultV2(ctx context.Context, actor auth.Session, executionID Identifier, limit, offset int, requestID string) (ResultPage, error) {
	user, catalog, err := service.authorizedAdvancedCatalog(ctx, actor)
	if err != nil {
		service.audit(ctx, auditActor(actor), &executionID, AuditResultRead, auditOutcome(err), nil, requestID)
		return ResultPage{}, err
	}
	if executionID.IsZero() {
		return ResultPage{}, ErrNotFound
	}
	if limit == 0 {
		limit = MaximumPageSize
	}
	if limit < 1 || limit > MaximumPageSize || offset < 0 || offset > MaximumRows {
		return ResultPage{}, ErrInvalidPlan
	}
	execution, err := service.store.GetExecution(ctx, executionID, user.ID)
	if err != nil {
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
	page, err := service.store.GetResultPage(ctx, executionID, user.ID, limit, offset)
	if err != nil {
		return ResultPage{}, err
	}
	if !authorizedAdvancedResultPage(page, execution, catalog) {
		service.audit(ctx, &user.ID, &executionID, AuditResultRead, auth.AuditOutcomeDenied, nil, requestID)
		return ResultPage{}, ErrForbidden
	}
	affected := len(page.Rows)
	service.audit(ctx, &user.ID, &executionID, AuditResultRead, auth.AuditOutcomeSuccess, &affected, requestID)
	return page, nil
}

// RunV2 executes an already reviewed logical plan for an internal read-only
// workflow. It reauthorizes the actor and catalog on every call and does not
// persist a second copy of candidate rows.
func (service *Service) RunV2(ctx context.Context, actor auth.Session, plan QueryPlan, requestID string) (AdvancedResult, error) {
	_, catalog, err := service.authorizedAdvancedCatalog(ctx, actor)
	if err != nil {
		return AdvancedResult{}, err
	}
	compiled, _, err := compileAdvancedForStore(plan, catalog, service.maximumCost)
	if err != nil {
		return AdvancedResult{}, err
	}
	queryContext, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()
	rawRows, err := service.store.ExecuteReadOnly(queryContext, compiled, service.timeout)
	if err != nil {
		if errors.Is(queryContext.Err(), context.DeadlineExceeded) {
			return AdvancedResult{}, ErrTimeout
		}
		if errors.Is(queryContext.Err(), context.Canceled) {
			return AdvancedResult{}, ErrCancelled
		}
		return AdvancedResult{}, err
	}
	rows, err := materializeRows(rawRows, compiled)
	if err != nil {
		return AdvancedResult{}, err
	}
	return AdvancedResult{CatalogVersion: compiled.CatalogVersion, RootEntity: compiled.RootEntity, Columns: compiled.Columns, Rows: rows, Cost: compiled.Cost}, nil
}

func (service *Service) authorizedAdvancedCatalog(ctx context.Context, actor auth.Session) (auth.User, resolvedCatalog, error) {
	user, err := service.authorize(ctx, actor)
	if err != nil {
		return auth.User{}, resolvedCatalog{}, err
	}
	catalog, err := loadCatalog(ctx, service.store, user.Role)
	if err != nil {
		return auth.User{}, resolvedCatalog{}, err
	}
	catalog.Public.Advanced = pointerAdvancedCatalog(advancedCatalogFor(catalog))
	catalog.Public.Version = ""
	encoded, err := json.Marshal(catalog.Public)
	if err != nil {
		return auth.User{}, resolvedCatalog{}, fmt.Errorf("fingerprint advanced query catalog: %w", err)
	}
	digest := sha256.Sum256(encoded)
	catalog.Public.Version = hex.EncodeToString(digest[:])
	return user, catalog, nil
}

func pointerAdvancedCatalog(value AdvancedCatalog) *AdvancedCatalog { return &value }

func compileAdvancedForStore(plan QueryPlan, catalog resolvedCatalog, maximumCost int) (CompiledPlan, QueryPlan, error) {
	advanced, normalized, err := compileAdvancedPlan(plan, catalog, maximumCost)
	if err != nil {
		return CompiledPlan{}, QueryPlan{}, err
	}
	rootEntity := advancedRootEntity(normalized)
	if rootEntity == "" {
		return CompiledPlan{}, QueryPlan{}, ErrInvalidPlan
	}
	columns := sanitizeAdvancedColumns(advanced.Columns, rootEntity)
	query := advanced.SQL
	if strings.HasPrefix(strings.TrimSpace(query), "WITH ") {
		query = "SELECT * FROM (" + query + ") AS advanced_result"
	}
	return CompiledPlan{
		SQL: query, Arguments: advanced.Arguments, Columns: columns, Fingerprint: advanced.Fingerprint,
		CatalogVersion: advanced.CatalogVersion, RootEntity: rootEntity, EntityKind: advanced.EntityKind,
		MaximumRows: advanced.MaximumRows, Cost: advanced.Cost,
	}, normalized, nil
}

func advancedRootEntity(plan QueryPlan) string {
	if plan.Combination != nil {
		return "combination"
	}
	if plan.Set != nil {
		return setRootEntity(*plan.Set)
	}
	return plan.RootEntity
}

func setRootEntity(value SetExpression) string {
	if value.Plan != nil {
		return advancedRootEntity(*value.Plan)
	}
	if len(value.Inputs) == 0 {
		return ""
	}
	return setRootEntity(value.Inputs[0])
}

func sanitizeAdvancedColumns(columns []ResultColumn, rootEntity string) []ResultColumn {
	result := make([]ResultColumn, len(columns))
	copy(result, columns)
	for index := range result {
		column := &result[index]
		if column.AggregateKey != "" {
			column.FieldKey = "aggregate." + column.AggregateKey
			continue
		}
		if rootEntity == "combination" && len(column.Lineage) > 0 {
			role := strings.TrimSuffix(strings.TrimSuffix(column.FieldKey, ".entity_id"), ".entity_label")
			suffix := "entity_label"
			if strings.HasSuffix(column.FieldKey, ".entity_id") {
				suffix = "entity_id"
			}
			column.FieldKey = "combination." + role + "." + column.Lineage[0].Entity + "." + suffix
		}
	}
	return result
}

func authorizedAdvancedResultPage(page ResultPage, expected Execution, catalog resolvedCatalog) bool {
	if page.Execution.ID != expected.ID || page.Execution.OwnerUserID != expected.OwnerUserID || page.Execution.State != ExecutionCompleted ||
		page.Total < 0 || page.Total > expected.MaximumRows || page.Limit < 1 || page.Limit > MaximumPageSize || page.Offset < 0 || len(page.Columns) != expected.ColumnCount {
		return false
	}
	if expected.RootEntity != "combination" {
		if _, ok := catalog.Entities[expected.RootEntity]; !ok {
			return false
		}
	}
	for position, column := range page.Columns {
		if column.Position != position || column.Label == "" || !column.Kind.Valid() {
			return false
		}
		if definition, ok := catalog.Fields[column.FieldKey]; ok {
			if !definition.Public.Projectable || definition.Public.Kind != column.Kind {
				return false
			}
			continue
		}
		if strings.HasPrefix(column.FieldKey, "aggregate.") {
			if expected.RootEntity == "combination" || len(strings.TrimPrefix(column.FieldKey, "aggregate.")) == 0 {
				return false
			}
			continue
		}
		if strings.HasPrefix(column.FieldKey, "combination.") {
			parts := strings.Split(column.FieldKey, ".")
			if len(parts) < 5 {
				return false
			}
			entity := strings.Join(parts[2:len(parts)-1], ".")
			if _, ok := catalog.Entities[entity]; !ok {
				return false
			}
			continue
		}
		return false
	}
	for _, row := range page.Rows {
		if !safeEntityValue(row.EntityID, 200) || !safeEntityValue(row.EntityLabel, 300) || row.UpdatedAt.IsZero() ||
			row.Position < page.Offset || row.Position >= page.Offset+page.Limit || len(row.Cells) != len(page.Columns) {
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

package queryengine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func TestServiceExecutesMaterializesAndAuditsTypedResult(t *testing.T) {
	store := newFakeQueryStore()
	now := time.Date(2026, time.July, 17, 15, 0, 0, 0, time.UTC)
	store.rawRows = []RawResultRow{{
		EntityKind: "profile", EntityID: "11111111-1111-4111-8111-111111111111", EntityLabel: "Ana Silva", UpdatedAt: now,
		Values: []*string{stringPointer("Ana Silva"), stringPointer("2026-07-17 12:00:00+00")},
	}}
	service, err := NewService(store, ServiceOptions{Now: func() time.Time { return now }, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	catalog, err := service.Catalog(context.Background(), queryActor(store), "catalog-request")
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	plan := QueryPlan{Version: PlanVersionV1, CatalogVersion: catalog.Version, RootEntity: "profiles",
		Projections: []string{"profile.full_name", "profile.updated_at"}, MaximumRows: 10}
	execution, err := service.Execute(context.Background(), queryActor(store), plan, "query-success-01", "execute-request")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if execution.State != ExecutionCompleted || execution.RowCount != 1 || execution.ColumnCount != 2 || store.executeCalls != 1 || store.completeCalls != 1 {
		t.Fatalf("completed execution/store calls = %#v / %d / %d", execution, store.executeCalls, store.completeCalls)
	}
	if store.lastTimeout != 2*time.Second || len(store.lastRows) != 1 || len(store.lastRows[0].Cells) != 2 ||
		store.lastRows[0].Cells[0].TextValue == nil || store.lastRows[0].Cells[1].TimestampValue == nil {
		t.Fatalf("materialized result = %#v", store.lastRows)
	}
	if !hasAudit(store.audits, AuditExecutionStarted, string(auth.AuditOutcomeSuccess)) ||
		!hasAudit(store.audits, AuditExecutionComplete, string(auth.AuditOutcomeSuccess)) {
		t.Fatalf("execution audits = %#v", store.audits)
	}
}

func TestServiceHonorsIdempotentReplayWithoutReexecution(t *testing.T) {
	store := newFakeQueryStore()
	service, _ := NewService(store, ServiceOptions{})
	catalog, _ := service.Catalog(context.Background(), queryActor(store), "catalog")
	store.created = false
	store.existing = Execution{ID: Identifier{7}, OwnerUserID: store.user.ID, State: ExecutionCompleted,
		CatalogVersion: catalog.Version, RootEntity: "profiles", MaximumRows: 10, RowCount: 1, ColumnCount: 1}
	plan := QueryPlan{Version: PlanVersionV1, CatalogVersion: catalog.Version, RootEntity: "profiles", Projections: []string{"profile.full_name"}, MaximumRows: 10}
	result, err := service.Execute(context.Background(), queryActor(store), plan, "replay-key-01", "replay")
	if err != nil || result.ID != store.existing.ID || store.executeCalls != 0 {
		t.Fatalf("idempotent Execute() = %#v, calls=%d, error=%v", result, store.executeCalls, err)
	}
}

func TestServiceRevalidatesAuthorizationAndFailsUnsafeOrTimedOutExecution(t *testing.T) {
	store := newFakeQueryStore()
	service, _ := NewService(store, ServiceOptions{})
	actor := queryActor(store)
	store.user.Active = false
	if _, err := service.Catalog(context.Background(), actor, "denied"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Catalog(inactive current user) error = %v", err)
	}

	store = newFakeQueryStore()
	service, _ = NewService(store, ServiceOptions{})
	catalog, _ := service.Catalog(context.Background(), queryActor(store), "catalog")
	plan := QueryPlan{Version: PlanVersionV1, CatalogVersion: catalog.Version, RootEntity: "profiles", Projections: []string{"profile.full_name"}, MaximumRows: 10}
	store.executeErr = ErrTimeout
	if _, err := service.Execute(context.Background(), queryActor(store), plan, "timeout-key-01", "timeout"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("Execute(timeout) error = %v", err)
	}
	if store.failedState != ExecutionFailed || store.failedCode != "timeout" {
		t.Fatalf("timeout failure = %q / %q", store.failedState, store.failedCode)
	}

	store = newFakeQueryStore()
	service, _ = NewService(store, ServiceOptions{})
	catalog, _ = service.Catalog(context.Background(), queryActor(store), "catalog")
	plan.CatalogVersion = catalog.Version
	store.rawRows = []RawResultRow{{EntityKind: "bill", EntityID: "id", EntityLabel: "label", UpdatedAt: time.Now(), Values: []*string{stringPointer("Ana")}}}
	if _, err := service.Execute(context.Background(), queryActor(store), plan, "unsafe-key-01", "unsafe"); !errors.Is(err, ErrUnsafeResult) {
		t.Fatalf("Execute(unsafe) error = %v", err)
	}
	if store.failedCode != "unsafe_result" {
		t.Fatalf("unsafe failure code = %q", store.failedCode)
	}
}

func TestServiceResultIsOwnerScopedReauthorizedAndTyped(t *testing.T) {
	store := newFakeQueryStore()
	now := time.Date(2026, time.July, 17, 15, 0, 0, 0, time.UTC)
	service, _ := NewService(store, ServiceOptions{Now: func() time.Time { return now }})
	executionID := Identifier{4}
	completed := now.Add(-time.Minute)
	store.page = ResultPage{
		Execution: Execution{ID: executionID, OwnerUserID: store.user.ID, State: ExecutionCompleted, CatalogVersion: "old-catalog",
			RootEntity: "profiles", MaximumRows: 10, RowCount: 1, ColumnCount: 1, CompletedAt: &completed, ExpiresAt: now.Add(time.Hour)},
		Columns: []ResultColumn{{Position: 0, FieldKey: "profile.full_name", Label: "Nome completo", Kind: ValueText}},
		Rows: []ResultRow{{Position: 0, EntityKind: "profile", EntityID: "11111111-1111-4111-8111-111111111111", EntityLabel: "Ana Silva", UpdatedAt: now,
			Cells: []ResultCell{{ColumnPosition: 0, Kind: ValueText, TextValue: stringPointer("Ana Silva")}}}},
		Total: 1, Limit: MaximumPageSize,
	}
	page, err := service.Result(context.Background(), queryActor(store), executionID, 0, 0, "result")
	if err != nil || page.Total != 1 || len(page.Rows) != 1 {
		t.Fatalf("Result() = %#v, error=%v", page, err)
	}
	store.page.Execution.State = ExecutionRunning
	store.audits = nil
	if _, err := service.Result(context.Background(), queryActor(store), executionID, 0, 0, "running-result"); !errors.Is(err, ErrConflict) ||
		!hasAudit(store.audits, AuditResultRead, string(auth.AuditOutcomeDenied)) {
		t.Fatalf("Result(running) error/audits = %v / %#v", err, store.audits)
	}
	store.page.Execution.State = ExecutionCompleted
	store.page.Columns[0].FieldKey = "profile.secret"
	if _, err := service.Result(context.Background(), queryActor(store), executionID, 0, 0, "unsafe-result"); !errors.Is(err, ErrUnsafeResult) {
		t.Fatalf("Result(removed field) error = %v", err)
	}
	store.user.Active = false
	if _, err := service.Result(context.Background(), queryActor(store), executionID, 0, 0, "revoked-result"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Result(revoked user) error = %v", err)
	}
}

func TestMaterializeCellCoversAllValueKinds(t *testing.T) {
	values := map[ValueKind]string{
		ValueText: "texto", ValueLongText: "linha 1\nlinha 2", ValueIdentifier: "00123", ValueInteger: "42",
		ValueDecimal: "12.3400", ValueBoolean: "true", ValueCivilDate: "2026-07-17", ValueCivilMonth: "2026-07",
		ValueTimestamp: "2026-07-17T15:00:00Z", ValueEnum: "active",
	}
	for kind, value := range values {
		cell, err := materializeCell(0, kind, &value)
		if err != nil || !validMaterializedCell(cell) {
			t.Fatalf("materializeCell(%s) = %#v, error=%v", kind, cell, err)
		}
	}
	cell, err := materializeCell(0, ValueText, nil)
	if err != nil || !cell.IsNull || !validMaterializedCell(cell) {
		t.Fatalf("materializeCell(null) = %#v, error=%v", cell, err)
	}
	invalid := "not-a-date"
	if _, err := materializeCell(0, ValueCivilDate, &invalid); !errors.Is(err, ErrUnsafeResult) {
		t.Fatalf("materializeCell(invalid date) error = %v", err)
	}
}

func hasAudit(events []AuditEvent, eventType AuditEventType, outcome string) bool {
	for _, event := range events {
		if event.EventType == eventType && event.Outcome == outcome {
			return true
		}
	}
	return false
}

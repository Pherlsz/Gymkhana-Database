package queryengine

import (
	"context"
	"sync"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type fakeQueryStore struct {
	mu sync.Mutex

	user        auth.User
	definitions CatalogDefinitions
	currentErr  error
	catalogErr  error
	createErr   error
	executeErr  error
	completeErr error
	failErr     error
	getErr      error
	pageErr     error
	cleanupErr  error
	auditErr    error

	created           bool
	existing          Execution
	rawRows           []RawResultRow
	matchCount        int64
	scanRows          [][]string
	page              ResultPage
	executeCalls      int
	completeCalls     int
	lastTimeout       time.Duration
	lastCompiled      CompiledPlan
	lastColumns       []ResultColumn
	lastRows          []ResultRow
	failedID          Identifier
	failedCode        string
	failedState       ExecutionState
	cleanupCount      int
	audits            []AuditEvent
	createExecution   func(context.Context, ExecutionInput, time.Time, int) (Execution, bool, error)
	executeReadOnly   func(context.Context, CompiledPlan, time.Duration) ([]RawResultRow, error)
	completeExecution func(context.Context, Identifier, []ResultColumn, []ResultRow, time.Time) (Execution, error)
}

func newFakeQueryStore() *fakeQueryStore {
	userID := auth.Identifier{1}
	return &fakeQueryStore{
		user:    auth.User{ID: userID, Email: "member@test.com", DisplayName: "Member", Role: auth.RoleExternal, Active: true},
		created: true,
	}
}

func (store *fakeQueryStore) CurrentUser(context.Context, auth.Identifier) (auth.User, error) {
	return store.user, store.currentErr
}

func (store *fakeQueryStore) CatalogDefinitions(context.Context) (CatalogDefinitions, error) {
	return store.definitions, store.catalogErr
}

func (store *fakeQueryStore) CreateExecution(ctx context.Context, input ExecutionInput, window time.Time, rate int) (Execution, bool, error) {
	if store.createExecution != nil {
		return store.createExecution(ctx, input, window, rate)
	}
	if store.createErr != nil {
		return Execution{}, false, store.createErr
	}
	if !store.created {
		return store.existing, false, nil
	}
	return Execution{
		ID: input.ID, OwnerUserID: input.OwnerUserID, State: ExecutionRunning,
		IdempotencyKey: input.IdempotencyKey, PlanFingerprint: input.PlanFingerprint,
		CatalogVersion: input.CatalogVersion, RootEntity: input.RootEntity, MaximumRows: input.MaximumRows,
		StartedAt: input.StartedAt, ExpiresAt: input.ExpiresAt, Version: 1,
		CreatedAt: input.StartedAt, UpdatedAt: input.StartedAt,
	}, true, nil
}

func (store *fakeQueryStore) CountReadOnly(context.Context, string, []any, time.Duration) (int64, error) {
	return store.matchCount, store.executeErr
}

func (store *fakeQueryStore) ScanTexts(context.Context, string, []any, int, int, time.Duration) ([][]string, error) {
	return store.scanRows, store.executeErr
}

func (store *fakeQueryStore) ExecuteReadOnly(ctx context.Context, plan CompiledPlan, timeout time.Duration) ([]RawResultRow, error) {
	store.executeCalls++
	store.lastTimeout = timeout
	store.lastCompiled = plan
	if store.executeReadOnly != nil {
		return store.executeReadOnly(ctx, plan, timeout)
	}
	return store.rawRows, store.executeErr
}

func (store *fakeQueryStore) CompleteExecution(ctx context.Context, id Identifier, columns []ResultColumn, rows []ResultRow, completedAt time.Time) (Execution, error) {
	store.completeCalls++
	store.lastColumns = append([]ResultColumn(nil), columns...)
	store.lastRows = append([]ResultRow(nil), rows...)
	if store.completeExecution != nil {
		return store.completeExecution(ctx, id, columns, rows, completedAt)
	}
	if store.completeErr != nil {
		return Execution{}, store.completeErr
	}
	return Execution{ID: id, OwnerUserID: store.user.ID, State: ExecutionCompleted, CatalogVersion: store.lastCompiled.CatalogVersion,
		RootEntity: store.lastCompiled.RootEntity, MaximumRows: store.lastCompiled.MaximumRows, RowCount: len(rows), ColumnCount: len(columns),
		StartedAt: completedAt.Add(-time.Second), CompletedAt: &completedAt, ExpiresAt: completedAt.Add(time.Hour), Version: 2,
		CreatedAt: completedAt.Add(-time.Second), UpdatedAt: completedAt}, nil
}

func (store *fakeQueryStore) FailExecution(_ context.Context, id Identifier, code string, state ExecutionState, _ time.Time) error {
	store.failedID = id
	store.failedCode = code
	store.failedState = state
	return store.failErr
}

func (store *fakeQueryStore) GetExecution(context.Context, Identifier, auth.Identifier) (Execution, error) {
	if store.getErr != nil {
		return Execution{}, store.getErr
	}
	return store.page.Execution, nil
}

func (store *fakeQueryStore) GetResultPage(context.Context, Identifier, auth.Identifier, int, int) (ResultPage, error) {
	return store.page, store.pageErr
}

func (store *fakeQueryStore) DeleteExpired(context.Context, time.Time, int) (int, error) {
	return store.cleanupCount, store.cleanupErr
}

func (store *fakeQueryStore) SaveAudit(_ context.Context, event AuditEvent) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.audits = append(store.audits, event)
	return store.auditErr
}

func queryActor(store *fakeQueryStore) auth.Session {
	return auth.Session{ID: auth.Identifier{9}, User: store.user}
}

func stringPointer(value string) *string { return &value }

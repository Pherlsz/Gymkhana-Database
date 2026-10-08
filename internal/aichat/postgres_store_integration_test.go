//go:build integration

package aichat

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)


func mustAuthorizeIntegration(t *testing.T, ctx context.Context, service *Service, actor auth.Session) auth.User {
	t.Helper()
	user, err := service.authorize(ctx, actor)
	if err != nil {
		t.Fatalf("authorize() error = %v", err)
	}
	return user
}

func TestPostgresAIChatOwnershipIdempotencyConcurrencyCancellationReferencesAndRetention(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for AI Chat PostgreSQL integration tests")
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
	otherID, _ := auth.NewIdentifier()
	key := "chat_" + strings.ReplaceAll(actorID.String(), "-", "")[:16]
	insertChatActor(t, ctx, pool, actorID, key, key)
	insertChatActor(t, ctx, pool, otherID, key+"_other", key+"_other")
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM ai_chat_audit_events WHERE actor_user_id IN ($1,$2) OR (event_type IN ('RETENTION_CLEANUP','RUN_RECOVERED') AND created_at >= $3)`, actorID.String(), otherID.String(), now)
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id IN ($1,$2)`, actorID.String(), otherID.String())
	}()

	store := NewPostgresStore(pool)
	current := now
	service, err := NewService(store, ServiceOptions{Now: func() time.Time { return current }, Retention: time.Hour, RateLimit: 100, UsageLimit: 1000})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	actor := auth.Session{User: auth.User{ID: actorID, Email: key + "@example.test", Role: auth.RoleExternal, Active: true}}
	other := auth.Session{User: auth.User{ID: otherID, Email: key + "_other@example.test", Role: auth.RoleExternal, Active: true}}
	thread, err := service.CreateThread(ctx, actor, "Integração privada", "integration-create")
	if err != nil {
		t.Fatalf("CreateThread() error = %v", err)
	}
	if _, err := service.Thread(ctx, other, thread.ID, "integration-cross-thread"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other-owner Thread() error = %v", err)
	}

	first, err := service.StartTurn(ctx, actor, thread.ID, "Primeira pergunta", "integration-turn-0001", nil, "integration-turn")
	if err != nil || !first.Created {
		t.Fatalf("StartTurn() = %#v, error=%v", first, err)
	}
	replay, err := service.StartTurn(ctx, actor, thread.ID, "Primeira pergunta", "integration-turn-0001", nil, "integration-replay")
	if err != nil || replay.Created || replay.Run.ID != first.Run.ID {
		t.Fatalf("StartTurn(replay) = %#v, error=%v", replay, err)
	}
	if _, err := service.StartTurn(ctx, actor, thread.ID, "Fingerprint diferente", "integration-turn-0001", nil, "integration-conflict"); !errors.Is(err, ErrConflict) {
		t.Fatalf("StartTurn(fingerprint conflict) error = %v", err)
	}
	if _, err := service.StartTurn(ctx, actor, thread.ID, "Outro run ativo", "integration-turn-0002", nil, "integration-active"); !errors.Is(err, ErrConflict) {
		t.Fatalf("StartTurn(active exclusion) error = %v", err)
	}
	if _, err := service.CancelRun(ctx, actor, first.Run.ID, "integration-cancel"); err != nil {
		t.Fatalf("CancelRun(first) error = %v", err)
	}
	retry, err := service.StartTurn(ctx, actor, thread.ID, "Repetir", "integration-turn-0003", &first.Run.ID, "integration-retry")
	if err != nil || retry.Run.RetryOfRunID == nil || *retry.Run.RetryOfRunID != first.Run.ID {
		t.Fatalf("StartTurn(retry) = %#v, error=%v", retry, err)
	}
	if _, err := service.CancelRun(ctx, actor, retry.Run.ID, "integration-cancel-retry"); err != nil {
		t.Fatalf("CancelRun(retry) error = %v", err)
	}

	type creationResult struct {
		value RunCreation
		err   error
	}
	start := make(chan struct{})
	results := make(chan creationResult, 2)
	for range 2 {
		go func() {
			<-start
			value, createErr := service.StartTurn(ctx, actor, thread.ID, "Turn concorrente", "integration-concurrent-key", nil, "integration-concurrent")
			results <- creationResult{value: value, err: createErr}
		}()
	}
	close(start)
	left, right := <-results, <-results
	if left.err != nil || right.err != nil || left.value.Run.ID != right.value.Run.ID || left.value.Created == right.value.Created {
		t.Fatalf("concurrent idempotency = %#v / %#v", left, right)
	}
	concurrentRun := left.value.Run
	if _, err := service.CancelRun(ctx, actor, concurrentRun.ID, "integration-cancel-concurrent"); err != nil {
		t.Fatalf("CancelRun(concurrent) error = %v", err)
	}

	race, err := service.StartTurn(ctx, actor, thread.ID, "Corrida terminal", "integration-race-turn", nil, "integration-race")
	if err != nil {
		t.Fatalf("StartTurn(race) error = %v", err)
	}
	if _, err := service.startRun(ctx, mustAuthorizeIntegration(t, ctx, service, actor), race.Run.ID, "integration-race-start"); err != nil {
		t.Fatalf("startRun(race) error = %v", err)
	}
	assistantID, _ := NewIdentifier()
	raceStart := make(chan struct{})
	cancelResult := make(chan error, 1)
	completeResult := make(chan error, 1)
	go func() {
		<-raceStart
		_, cancelErr := store.RequestCancellation(ctx, race.Run.ID, actorID, current.Add(time.Second))
		cancelResult <- cancelErr
	}()
	go func() {
		<-raceStart
		_, _, completeErr := store.CompleteRun(ctx, CompleteRunInput{MessageID: assistantID, RunID: race.Run.ID, OwnerUserID: actorID,
			Content: "Uma única resposta", InputUsage: 1, OutputUsage: 1, Now: current.Add(time.Second)})
		completeResult <- completeErr
	}()
	close(raceStart)
	if cancelErr := <-cancelResult; cancelErr != nil {
		t.Fatalf("RequestCancellation(race) error = %v", cancelErr)
	}
	if completeErr := <-completeResult; completeErr != nil && !errors.Is(completeErr, ErrCancelled) {
		t.Fatalf("CompleteRun(race) error = %v", completeErr)
	}
	terminal, err := service.Run(ctx, actor, race.Run.ID)
	if err != nil || !terminal.State.Terminal() || (terminal.State != RunCompleted && terminal.State != RunCancelled) {
		t.Fatalf("race terminal run = %#v, error=%v", terminal, err)
	}
	events, err := service.Events(ctx, actor, terminal.ID, 0, 100)
	if err != nil || countTerminalEvents(events.Events) != 1 {
		t.Fatalf("race terminal events = %#v, error=%v", events.Events, err)
	}

	toolRun, err := service.StartTurn(ctx, actor, thread.ID, "Produza referência", "integration-tool-turn", nil, "integration-tool")
	if err != nil {
		t.Fatalf("StartTurn(tool) error = %v", err)
	}
	if _, err := service.startRun(ctx, mustAuthorizeIntegration(t, ctx, service, actor), toolRun.Run.ID, "integration-tool-start"); err != nil {
		t.Fatalf("startRun(tool) error = %v", err)
	}
	stepID, referenceID := mustChatIdentifier(t), mustChatIdentifier(t)
	step, err := store.BeginTool(ctx, BeginToolInput{ID: stepID, RunID: toolRun.Run.ID, OwnerUserID: actorID, Kind: ToolSearch,
		ArgumentsFingerprint: [32]byte{1}, Now: current.Add(2 * time.Second)})
	if err != nil {
		t.Fatalf("BeginTool() error = %v", err)
	}
	referenceInput := &CreateResultReferenceInput{ID: referenceID, ThreadID: thread.ID, RunID: toolRun.Run.ID, OwnerUserID: actorID,
		Kind: ResultReferenceSearch, LogicalRequest: []byte(`{"terms":["Ana"]}`), ContextFingerprint: [32]byte{2}, Label: "Busca: Ana",
		RowCount: 1, ColumnCount: 1, ExpiresAt: current.Add(30 * time.Minute), Now: current.Add(2 * time.Second)}
	if _, reference, err := store.CompleteTool(ctx, CompleteToolInput{StepID: step.ID, RunID: toolRun.Run.ID, OwnerUserID: actorID,
		ResultReference: referenceInput, RowCount: 1, ResultBytes: 32, Now: current.Add(3 * time.Second)}); err != nil || reference == nil {
		t.Fatalf("CompleteTool() reference=%#v, error=%v", reference, err)
	}
	if _, err := store.GetResultReference(ctx, referenceID, otherID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other-owner GetResultReference() error = %v", err)
	}
	if _, _, err := store.CompleteRun(ctx, CompleteRunInput{MessageID: mustChatIdentifier(t), RunID: toolRun.Run.ID, OwnerUserID: actorID,
		Content: "Referência pronta", Now: current.Add(4 * time.Second)}); err != nil {
		t.Fatalf("CompleteRun(tool) error = %v", err)
	}

	orphan, err := service.StartTurn(ctx, actor, thread.ID, "Run órfão", "integration-orphan-turn", nil, "integration-orphan")
	if err != nil {
		t.Fatalf("StartTurn(orphan) error = %v", err)
	}
	if _, err := service.startRun(ctx, mustAuthorizeIntegration(t, ctx, service, actor), orphan.Run.ID, "integration-orphan-start"); err != nil {
		t.Fatalf("startRun(orphan) error = %v", err)
	}
	orphanStep, err := store.BeginTool(ctx, BeginToolInput{ID: mustChatIdentifier(t), RunID: orphan.Run.ID, OwnerUserID: actorID,
		Kind: ToolCatalog, ArgumentsFingerprint: [32]byte{3}, Now: current})
	if err != nil {
		t.Fatalf("BeginTool(orphan) error = %v", err)
	}
	current = current.Add(defaultRunTimeout + staleRunGrace + time.Second)
	if deleted, err := service.CleanupExpired(ctx); err != nil || deleted != 0 {
		t.Fatalf("CleanupExpired(orphan) = %d, error=%v", deleted, err)
	}
	recovered, err := service.Run(ctx, actor, orphan.Run.ID)
	if err != nil || recovered.State != RunFailed || recovered.ErrorCode != "timeout" {
		t.Fatalf("recovered orphan = %#v, error=%v", recovered, err)
	}
	var orphanStepState ToolStepState
	if err := pool.QueryRow(ctx, `SELECT state FROM ai_chat_tool_steps WHERE id=$1`, orphanStep.ID.String()).Scan(&orphanStepState); err != nil || orphanStepState != ToolStepFailed {
		t.Fatalf("recovered tool state = %s, error=%v", orphanStepState, err)
	}
	recoveryEvents, err := service.Events(ctx, actor, orphan.Run.ID, 0, MaximumEventsPage)
	if err != nil || countTerminalEvents(recoveryEvents.Events) != 1 || recoveryEvents.Events[len(recoveryEvents.Events)-1].Kind != EventRunFailed {
		t.Fatalf("recovery events = %#v, error=%v", recoveryEvents.Events, err)
	}
	recoveredRetry, err := service.StartTurn(ctx, actor, thread.ID, "Run órfão", "integration-orphan-retry", &orphan.Run.ID, "integration-orphan-retry")
	if err != nil {
		t.Fatalf("StartTurn(orphan retry) error = %v", err)
	}
	if _, err := service.CancelRun(ctx, actor, recoveredRetry.Run.ID, "integration-orphan-retry-cancel"); err != nil {
		t.Fatalf("CancelRun(orphan retry) error = %v", err)
	}

	current = current.Add(2 * time.Hour)
	deleted, err := service.CleanupExpired(ctx)
	if err != nil || deleted != 1 {
		t.Fatalf("CleanupExpired() = %d, error=%v", deleted, err)
	}
	if _, err := service.Thread(ctx, actor, thread.ID, "integration-cleaned"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Thread(cleaned) error = %v", err)
	}

	limitedService, err := NewService(store, ServiceOptions{Now: func() time.Time { return current }, Retention: time.Hour, RateLimit: 1})
	if err != nil {
		t.Fatalf("NewService(rate limited) error = %v", err)
	}
	limitedThread, err := limitedService.CreateThread(ctx, actor, "Limite persistente", "integration-rate-thread")
	if err != nil {
		t.Fatalf("CreateThread(rate limited) error = %v", err)
	}
	limitedRun, err := limitedService.StartTurn(ctx, actor, limitedThread.ID, "Primeira na janela", "integration-rate-0001", nil, "integration-rate-first")
	if err != nil {
		t.Fatalf("StartTurn(rate first) error = %v", err)
	}
	if _, err := limitedService.CancelRun(ctx, actor, limitedRun.Run.ID, "integration-rate-cancel"); err != nil {
		t.Fatalf("CancelRun(rate first) error = %v", err)
	}
	if _, err := limitedService.StartTurn(ctx, actor, limitedThread.ID, "Segunda na janela", "integration-rate-0002", nil, "integration-rate-second"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("StartTurn(persistent rate) error = %v", err)
	}
	usageService, err := NewService(store, ServiceOptions{Now: func() time.Time { return current }, Retention: time.Hour, RateLimit: 100, UsageLimit: 1})
	if err != nil {
		t.Fatalf("NewService(usage limited) error = %v", err)
	}
	usageThread, err := usageService.CreateThread(ctx, actor, "Quota persistente", "integration-usage-thread")
	if err != nil {
		t.Fatalf("CreateThread(usage limited) error = %v", err)
	}
	usageRun, err := usageService.StartTurn(ctx, actor, usageThread.ID, "Consuma quota", "integration-usage-0001", nil, "integration-usage-first")
	if err != nil {
		t.Fatalf("StartTurn(usage first) error = %v", err)
	}
	if _, err := usageService.startRun(ctx, mustAuthorizeIntegration(t, ctx, usageService, actor), usageRun.Run.ID, "integration-usage-start"); err != nil {
		t.Fatalf("startRun(usage) error = %v", err)
	}
	if _, exceeded, err := usageService.addUsage(ctx, mustAuthorizeIntegration(t, ctx, usageService, actor), usageRun.Run.ID, ModelUsage{InputUnits: 2}); err != nil || !exceeded {
		t.Fatalf("addUsage() exceeded=%t, error=%v", exceeded, err)
	}
	if _, err := usageService.failRun(ctx, mustAuthorizeIntegration(t, ctx, usageService, actor), usageRun.Run.ID, "quota_exceeded", "integration-usage-fail"); err != nil {
		t.Fatalf("failRun(usage) error = %v", err)
	}
	if _, err := usageService.StartTurn(ctx, actor, usageThread.ID, "Nova tentativa", "integration-usage-0002", nil, "integration-usage-second"); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("StartTurn(persistent usage quota) error = %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE app_users SET active=false WHERE id=$1`, actorID.String()); err != nil {
		t.Fatalf("revoke AI Chat actor: %v", err)
	}
	if _, err := limitedService.Thread(ctx, actor, limitedThread.ID, "integration-revoked"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Thread(revoked current user) error = %v", err)
	}
}

func insertChatActor(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id auth.Identifier, subject, login string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO app_users(id,subject,email,display_name,role,active)
VALUES($1,$2,lower($3) || '@example.test','AI Chat integration','EXTERNAL',true)`, id.String(), subject, login); err != nil {
		t.Fatalf("insert AI Chat actor: %v", err)
	}
}

func mustChatIdentifier(t *testing.T) Identifier {
	t.Helper()
	id, err := NewIdentifier()
	if err != nil {
		t.Fatalf("NewIdentifier() error = %v", err)
	}
	return id
}

func countTerminalEvents(events []RunEvent) int {
	count := 0
	for _, event := range events {
		if event.Kind == EventRunCompleted || event.Kind == EventRunFailed || event.Kind == EventRunCancelled {
			count++
		}
	}
	return count
}

package aichat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func TestServicePrivateLifecycleIdempotencyRetryAndRetention(t *testing.T) {
	actor, user := chatTestActor(t, "member")
	otherActor, otherUser := chatTestActor(t, "other")
	current := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	store := newMemoryStore(user, otherUser)
	service, err := NewService(store, ServiceOptions{Now: func() time.Time { return current }, Retention: 2 * time.Hour, RateLimit: 10})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	thread, err := service.CreateThread(context.Background(), actor, "  ", "create-thread")
	if err != nil || thread.Title != DefaultThreadTitle || !thread.RetentionExpiresAt.Equal(current.Add(2*time.Hour)) {
		t.Fatalf("CreateThread() = %#v, error=%v", thread, err)
	}
	if _, err := service.Thread(context.Background(), otherActor, thread.ID, "other-read"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other-owner Thread() error = %v", err)
	}

	created, err := service.StartTurn(context.Background(), actor, thread.ID, "Quem está em Recife?", "turn-private-0001", nil, "start-turn")
	if err != nil || !created.Created || created.Run.State != RunQueued || created.UserMessage.Content != "Quem está em Recife?" {
		t.Fatalf("StartTurn() = %#v, error=%v", created, err)
	}
	replayed, err := service.StartTurn(context.Background(), actor, thread.ID, "Quem está em Recife?", "turn-private-0001", nil, "replay-turn")
	if err != nil || replayed.Created || replayed.Run.ID != created.Run.ID || replayed.UserMessage.ID != created.UserMessage.ID {
		t.Fatalf("StartTurn(replay) = %#v, error=%v", replayed, err)
	}
	if _, err := service.StartTurn(context.Background(), actor, thread.ID, "Conte outra coisa", "turn-private-0001", nil, "changed-replay"); !errors.Is(err, ErrConflict) {
		t.Fatalf("StartTurn(changed replay) error = %v", err)
	}
	if _, err := service.StartTurn(context.Background(), actor, thread.ID, "Outra pergunta", "turn-private-0002", nil, "active-conflict"); !errors.Is(err, ErrConflict) {
		t.Fatalf("StartTurn(active conflict) error = %v", err)
	}
	cancelled, err := service.CancelRun(context.Background(), actor, created.Run.ID, "cancel-queued")
	if err != nil || cancelled.State != RunCancelled || cancelled.CompletedAt == nil {
		t.Fatalf("CancelRun(queued) = %#v, error=%v", cancelled, err)
	}
	retry, err := service.StartTurn(context.Background(), actor, thread.ID, "Tente novamente", "turn-private-0003", &created.Run.ID, "retry")
	if err != nil || retry.Run.RetryOfRunID == nil || *retry.Run.RetryOfRunID != created.Run.ID {
		t.Fatalf("StartTurn(retry) = %#v, error=%v", retry, err)
	}
	if _, err := service.CancelRun(context.Background(), actor, retry.Run.ID, "cancel-retry"); err != nil {
		t.Fatalf("CancelRun(retry) error = %v", err)
	}

	renamed, err := service.RenameThread(context.Background(), actor, thread.ID, "  Pessoas de Recife  ", thread.Version, "rename")
	if err != nil || renamed.Title != "Pessoas de Recife" || renamed.Version != thread.Version+1 {
		t.Fatalf("RenameThread() = %#v, error=%v", renamed, err)
	}
	if _, err := service.RenameThread(context.Background(), actor, thread.ID, "Conflito", thread.Version, "stale-rename"); !errors.Is(err, ErrConflict) {
		t.Fatalf("RenameThread(stale) error = %v", err)
	}

	current = current.Add(3 * time.Hour)
	deleted, err := service.CleanupExpired(context.Background())
	if err != nil || deleted != 1 {
		t.Fatalf("CleanupExpired() = %d, error=%v", deleted, err)
	}
	if _, err := service.Thread(context.Background(), actor, thread.ID, "expired"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Thread(expired) error = %v", err)
	}
	if len(store.audits) == 0 || store.audits[len(store.audits)-1].EventType != AuditThreadRead {
		t.Fatalf("audits = %#v", store.audits)
	}
	foundCleanup := false
	for _, event := range store.audits {
		if event.EventType == AuditRetentionClean && event.AffectedCount != nil && *event.AffectedCount == 1 {
			foundCleanup = true
		}
	}
	if !foundCleanup {
		t.Fatalf("retention cleanup audit missing: %#v", store.audits)
	}
}

func TestServiceReauthorizesAndCancellationWinsCompletion(t *testing.T) {
	actor, user := chatTestActor(t, "member")
	now := time.Date(2026, time.July, 18, 15, 0, 0, 0, time.UTC)
	store := newMemoryStore(user)
	service, _ := NewService(store, ServiceOptions{Now: func() time.Time { return now }, Retention: time.Hour})
	thread, _ := service.CreateThread(context.Background(), actor, "Teste", "create")
	creation, _ := service.StartTurn(context.Background(), actor, thread.ID, "Mensagem", "running-turn-01", nil, "turn")
	if _, err := service.startRun(context.Background(), actor, creation.Run.ID, "start"); err != nil {
		t.Fatalf("startRun() error = %v", err)
	}
	now = now.Add(time.Second)
	run, err := service.CancelRun(context.Background(), actor, creation.Run.ID, "cancel")
	if err != nil || run.CancelRequestedAt == nil || run.State != RunRunning {
		t.Fatalf("CancelRun(running) = %#v, error=%v", run, err)
	}
	if _, _, err := service.completeRun(context.Background(), actor, run.ID, "Resposta tardia", 1, 1, "late-complete"); !errors.Is(err, ErrCancelled) {
		t.Fatalf("completeRun(after cancel) error = %v", err)
	}
	terminal, _ := service.Run(context.Background(), actor, run.ID)
	if terminal.State != RunCancelled || terminal.ErrorCode != "cancelled" {
		t.Fatalf("terminal run = %#v", terminal)
	}

	current := store.users[user.ID]
	current.Active = false
	store.users[user.ID] = current
	if _, err := service.Thread(context.Background(), actor, thread.ID, "revoked"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Thread(revoked actor) error = %v", err)
	}
}

func TestToolCancellationWinsFailureAndIsAudited(t *testing.T) {
	actor, user := chatTestActor(t, "member")
	now := time.Date(2026, time.July, 18, 15, 30, 0, 0, time.UTC)
	store := newMemoryStore(user)
	service, _ := NewService(store, ServiceOptions{Now: func() time.Time { return now }, Retention: time.Hour})
	thread, _ := service.CreateThread(context.Background(), actor, "Cancelamento", "create")
	creation, _ := service.StartTurn(context.Background(), actor, thread.ID, "Mensagem", "cancel-tool-turn", nil, "turn")
	if _, err := service.startRun(context.Background(), actor, creation.Run.ID, "start"); err != nil {
		t.Fatalf("startRun() error = %v", err)
	}
	step, err := service.beginTool(context.Background(), actor, creation.Run.ID, "search", [32]byte{1})
	if err != nil {
		t.Fatalf("beginTool() error = %v", err)
	}
	if _, err := service.CancelRun(context.Background(), actor, creation.Run.ID, "cancel"); err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}
	failed, err := service.failTool(context.Background(), actor, step.ID, creation.Run.ID, "tool_failed", "fail-tool")
	if err != nil || failed.State != ToolStepCancelled || failed.ErrorCode != "cancelled" {
		t.Fatalf("failTool() = %#v, error=%v", failed, err)
	}
	terminal, err := service.failRun(context.Background(), actor, creation.Run.ID, "tool_failed", "terminal")
	if err != nil || terminal.State != RunCancelled || terminal.ErrorCode != "cancelled" {
		t.Fatalf("failRun() = %#v, error=%v", terminal, err)
	}
	foundAudit := false
	for _, event := range store.audits {
		if event.EventType == AuditToolExecuted && event.ToolKind != nil && *event.ToolKind == ToolSearch && event.ErrorCode == "cancelled" {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("cancelled tool audit missing: %#v", store.audits)
	}
}

func TestServiceRequiresExplicitSafeConfigurationAndBounds(t *testing.T) {
	actor, user := chatTestActor(t, "member")
	store := newMemoryStore(user)
	for name, options := range map[string]ServiceOptions{
		"missing retention": {},
		"short retention":   {Retention: time.Minute},
		"long retention":    {Retention: 366 * 24 * time.Hour},
		"short timeout":     {Retention: time.Hour, RunTimeout: time.Millisecond},
		"zero rate":         {Retention: time.Hour, RateLimit: -1},
		"large cleanup":     {Retention: time.Hour, CleanupBatch: 1001},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewService(store, options); !errors.Is(err, ErrInvalidSetup) {
				t.Fatalf("NewService() error = %v", err)
			}
		})
	}
	service, err := NewService(store, ServiceOptions{Retention: time.Hour})
	if err != nil {
		t.Fatalf("NewService(default limits) error = %v", err)
	}
	capability := service.Capability()
	if !capability.Enabled || capability.MaximumToolCalls != MaximumToolCalls || capability.MaximumRows != MaximumToolRows || capability.MaximumUsage == 0 {
		t.Fatalf("Capability() = %#v", capability)
	}
	thread, _ := service.CreateThread(context.Background(), actor, "Limites", "create")
	if _, err := service.StartTurn(context.Background(), actor, thread.ID, "texto\x00oculto", "valid-key-0001", nil, "control"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("StartTurn(control character) error = %v", err)
	}
	if _, err := service.StartTurn(context.Background(), actor, thread.ID, "texto", "short", nil, "short-key"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("StartTurn(short key) error = %v", err)
	}
}

func TestTurnFingerprintIsStableAndContextSensitive(t *testing.T) {
	thread := Identifier{1}
	reference := Identifier{2}
	retry := Identifier{3}
	first, err := turnFingerprint(thread, "conteúdo", &reference, &retry)
	if err != nil {
		t.Fatalf("turnFingerprint() error = %v", err)
	}
	second, _ := turnFingerprint(thread, "conteúdo", &reference, &retry)
	changedContent, _ := turnFingerprint(thread, "outro", &reference, &retry)
	changedContext, _ := turnFingerprint(thread, "conteúdo", nil, &retry)
	if first != second || first == changedContent || first == changedContext || first == ([32]byte{}) {
		t.Fatalf("fingerprints = %x / %x / %x / %x", first, second, changedContent, changedContext)
	}
}

func TestCleanupRecoversOrphanedRunAndAllowsExplicitRetry(t *testing.T) {
	actor, user := chatTestActor(t, "member")
	current := time.Date(2026, time.July, 18, 15, 0, 0, 0, time.UTC)
	store := newMemoryStore(user)
	service, err := NewService(store, ServiceOptions{
		Now: func() time.Time { return current }, Retention: time.Hour, RunTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	thread, _ := service.CreateThread(context.Background(), actor, "Recuperação", "create")
	creation, _ := service.StartTurn(context.Background(), actor, thread.ID, "Pergunta", "orphaned-run-01", nil, "turn")
	current = current.Add(staleRunGrace + 3*time.Second)
	if deleted, err := service.CleanupExpired(context.Background()); err != nil || deleted != 0 {
		t.Fatalf("CleanupExpired() = %d, error=%v", deleted, err)
	}
	recovered, err := service.Run(context.Background(), actor, creation.Run.ID)
	if err != nil || recovered.State != RunFailed || recovered.ErrorCode != "timeout" || recovered.CompletedAt == nil {
		t.Fatalf("recovered run = %#v, error=%v", recovered, err)
	}
	events, _ := service.Events(context.Background(), actor, recovered.ID, 0, MaximumEventsPage)
	if len(events.Events) < 2 || events.Events[len(events.Events)-1].Kind != EventRunFailed {
		t.Fatalf("recovery events = %#v", events.Events)
	}
	retry, err := service.StartTurn(context.Background(), actor, thread.ID, "Pergunta", "orphaned-retry-01", &recovered.ID, "retry")
	if err != nil || retry.Run.RetryOfRunID == nil || *retry.Run.RetryOfRunID != recovered.ID {
		t.Fatalf("retry = %#v, error=%v", retry, err)
	}
	foundAudit := false
	for _, event := range store.audits {
		if event.EventType == AuditRunRecovered && event.AffectedCount != nil && *event.AffectedCount == 1 {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("run recovery audit missing: %#v", store.audits)
	}
}

func TestTerminalEventHistoryRemainsPagedUntilEveryEventIsReadable(t *testing.T) {
	actor, user := chatTestActor(t, "member")
	now := time.Date(2026, time.July, 18, 16, 0, 0, 0, time.UTC)
	store := newMemoryStore(user)
	service, err := NewService(store, ServiceOptions{Now: func() time.Time { return now }, Retention: time.Hour})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	thread, _ := service.CreateThread(context.Background(), actor, "Eventos", "create")
	creation, _ := service.StartTurn(context.Background(), actor, thread.ID, "Pergunta", "paged-events-01", nil, "turn")
	for index := 0; index < MaximumEventsPage; index++ {
		store.appendEvent(RunEvent{RunID: creation.Run.ID, Kind: EventTextDelta, TextDelta: "x", CreatedAt: now})
	}
	run := store.runs[creation.Run.ID]
	run.State, run.CompletedAt = RunCompleted, timePointer(now)
	store.runs[run.ID] = run
	store.appendEvent(RunEvent{RunID: run.ID, Kind: EventRunCompleted, CreatedAt: now})

	first, err := service.Events(context.Background(), actor, run.ID, 0, MaximumEventsPage)
	if err != nil || len(first.Events) != MaximumEventsPage || first.Terminal || first.LastSequence != MaximumEventsPage {
		t.Fatalf("first Events() = %#v, error=%v", first, err)
	}
	second, err := service.Events(context.Background(), actor, run.ID, first.LastSequence, MaximumEventsPage)
	if err != nil || len(second.Events) != 2 || !second.Terminal || second.Events[len(second.Events)-1].Kind != EventRunCompleted {
		t.Fatalf("second Events() = %#v, error=%v", second, err)
	}
}

func chatTestActor(t *testing.T, login string) (auth.Session, auth.User) {
	t.Helper()
	id, err := auth.NewIdentifier()
	if err != nil {
		t.Fatalf("auth.NewIdentifier() error = %v", err)
	}
	user := auth.User{ID: id, Login: login, DisplayName: login, Role: auth.RoleExternal, Active: true}
	return auth.Session{User: user}, user
}

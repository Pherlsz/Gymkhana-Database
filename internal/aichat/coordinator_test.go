package aichat

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type blockingTurnRunner struct {
	started chan Identifier
	mutex   sync.Mutex
	errors  []error
}

func (runner *blockingTurnRunner) RunTurn(ctx context.Context, _ auth.Session, id Identifier, _ string) error {
	runner.started <- id
	<-ctx.Done()
	runner.mutex.Lock()
	runner.errors = append(runner.errors, ctx.Err())
	runner.mutex.Unlock()
	return ctx.Err()
}

func TestCoordinatorRunsOnceCancelsAndWaits(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	defer stop()
	runner := &blockingTurnRunner{started: make(chan Identifier, 1)}
	coordinator, err := NewCoordinator(parent, runner)
	if err != nil {
		t.Fatalf("NewCoordinator() error = %v", err)
	}
	runID := Identifier{1}
	if !coordinator.Start(auth.Session{}, runID, "request") || coordinator.Start(auth.Session{}, runID, "duplicate") {
		t.Fatal("Coordinator did not enforce one local runner per run")
	}
	select {
	case started := <-runner.started:
		if started != runID {
			t.Fatalf("started run = %s", started.String())
		}
	case <-time.After(time.Second):
		t.Fatal("coordinator did not start runner")
	}
	if !coordinator.Cancel(runID) {
		t.Fatal("Coordinator.Cancel() did not find active run")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := coordinator.Wait(waitCtx); err != nil {
		t.Fatalf("Coordinator.Wait() error = %v", err)
	}
	runner.mutex.Lock()
	defer runner.mutex.Unlock()
	if len(runner.errors) != 1 || !errors.Is(runner.errors[0], context.Canceled) {
		t.Fatalf("runner errors = %#v", runner.errors)
	}
}

func TestCoordinatorRequiresParentAndRunner(t *testing.T) {
	//lint:ignore SA1012 This deliberately verifies the constructor's nil-context guard.
	if _, err := NewCoordinator(nil, &blockingTurnRunner{}); !errors.Is(err, ErrInvalidSetup) {
		t.Fatalf("nil parent error = %v", err)
	}
	if _, err := NewCoordinator(context.Background(), nil); !errors.Is(err, ErrInvalidSetup) {
		t.Fatalf("nil runner error = %v", err)
	}
}

type staticTurnRunner struct {
	err error
}

func (runner staticTurnRunner) RunTurn(context.Context, auth.Session, Identifier, string) error {
	return runner.err
}

func TestCoordinatorLogsRunTurnError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	runner := staticTurnRunner{err: errors.New("provider unavailable for test")}
	coordinator, err := NewCoordinator(context.Background(), runner)
	if err != nil {
		t.Fatalf("NewCoordinator() error = %v", err)
	}
	coordinator.logger = logger
	runID := Identifier{2}
	if !coordinator.Start(auth.Session{}, runID, "log-error") {
		t.Fatal("Coordinator.Start() = false")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := coordinator.Wait(waitCtx); err != nil {
		t.Fatalf("Coordinator.Wait() error = %v", err)
	}
	if !strings.Contains(buf.String(), "provider unavailable for test") {
		t.Fatalf("RunTurn error was dropped, log = %q", buf.String())
	}
}

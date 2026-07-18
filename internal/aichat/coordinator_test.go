package aichat

import (
	"context"
	"errors"
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
	if _, err := NewCoordinator(nil, &blockingTurnRunner{}); !errors.Is(err, ErrInvalidSetup) {
		t.Fatalf("nil parent error = %v", err)
	}
	if _, err := NewCoordinator(context.Background(), nil); !errors.Is(err, ErrInvalidSetup) {
		t.Fatalf("nil runner error = %v", err)
	}
}

package aichat

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type Coordinator struct {
	parent context.Context
	runner TurnRunner
	logger *slog.Logger

	mutex   sync.Mutex
	active  map[Identifier]context.CancelFunc
	waiting sync.WaitGroup
}

func NewCoordinator(parent context.Context, runner TurnRunner) (*Coordinator, error) {
	if parent == nil || runner == nil {
		return nil, ErrInvalidSetup
	}
	return &Coordinator{parent: parent, runner: runner, active: make(map[Identifier]context.CancelFunc)}, nil
}

func (coordinator *Coordinator) log() *slog.Logger {
	if coordinator != nil && coordinator.logger != nil {
		return coordinator.logger
	}
	return slog.Default()
}

func (coordinator *Coordinator) Start(actor auth.Session, runID Identifier, requestID string) bool {
	if coordinator == nil || runID.IsZero() {
		return false
	}
	coordinator.mutex.Lock()
	if _, exists := coordinator.active[runID]; exists {
		coordinator.mutex.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(coordinator.parent)
	coordinator.active[runID] = cancel
	coordinator.waiting.Add(1)
	coordinator.mutex.Unlock()
	go func() {
		defer coordinator.waiting.Done()
		defer func() {
			coordinator.mutex.Lock()
			delete(coordinator.active, runID)
			coordinator.mutex.Unlock()
			cancel()
		}()
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			coordinator.log().Error("AI Chat turn panicked", "run_id", runID.String(), "request_id", requestID, "panic", fmt.Sprint(recovered))
			if failer, ok := coordinator.runner.(panickedTurnFailer); ok {
				failer.failPanickedRun(ctx, actor, runID, requestID)
			}
		}()
		if err := coordinator.runner.RunTurn(ctx, actor, runID, requestID); err != nil {
			coordinator.log().Error("AI Chat turn failed", "run_id", runID.String(), "request_id", requestID, "error", err)
		}
	}()
	return true
}

func (coordinator *Coordinator) Cancel(runID Identifier) bool {
	if coordinator == nil {
		return false
	}
	coordinator.mutex.Lock()
	cancel, exists := coordinator.active[runID]
	coordinator.mutex.Unlock()
	if exists {
		cancel()
	}
	return exists
}

func (coordinator *Coordinator) Wait(ctx context.Context) error {
	if coordinator == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		coordinator.waiting.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

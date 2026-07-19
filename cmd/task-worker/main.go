package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
	"github.com/Pherlsz/Gymkhana-Database/internal/taskengine"
)

func main() {
	if err := run(); err != nil {
		slog.Error("task worker stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("task worker requires DATABASE_URL")
	}
	logger := logging.New(string(cfg.LogLevel))
	slog.SetDefault(logger)
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if pool == nil {
		return errors.New("task worker requires a database connection")
	}
	defer pool.Close()
	query, err := queryengine.NewService(queryengine.NewPostgresStore(pool), queryengine.ServiceOptions{})
	if err != nil {
		return fmt.Errorf("configure task Query gateway: %w", err)
	}
	service, client, err := taskengine.NewRuntime(pool, query, taskengine.DisabledInterpreter{}, taskengine.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event taskengine.AuditEvent, auditErr error) {
			logger.Error("task audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error_type", fmt.Sprintf("%T", auditErr))
		},
	}, true)
	if err != nil {
		return fmt.Errorf("configure task worker: %w", err)
	}
	if err := client.Start(rootCtx); err != nil {
		return fmt.Errorf("start task worker: %w", err)
	}
	maintenanceDone := make(chan struct{})
	go runMaintenance(rootCtx, logger, service, maintenanceDone)
	logger.Info("task worker started", "queue", taskengine.Queue)
	select {
	case <-rootCtx.Done():
	case <-client.Stopped():
		if rootCtx.Err() == nil {
			return errors.New("task worker stopped unexpectedly")
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := client.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stop task worker: %w", err)
	}
	select {
	case <-maintenanceDone:
	case <-shutdownCtx.Done():
		return shutdownCtx.Err()
	}
	return nil
}

func runMaintenance(ctx context.Context, logger *slog.Logger, service *taskengine.Service, done chan<- struct{}) {
	defer close(done)
	run := func() {
		maintenanceCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if recovered, err := service.Recover(maintenanceCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("task recovery failed", "error_type", fmt.Sprintf("%T", err))
		} else if recovered > 0 {
			logger.Info("task recovery completed", "recovered_jobs", recovered)
		}
		if deleted, err := service.Cleanup(maintenanceCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("task cleanup failed", "error_type", fmt.Sprintf("%T", err))
		} else if deleted > 0 {
			logger.Info("task cleanup completed", "expired_drafts", deleted)
		}
	}
	run()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

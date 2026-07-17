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

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("operations worker stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	storageCfg, err := config.LoadStorage()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("operations worker requires DATABASE_URL")
	}
	if !storageCfg.Enabled {
		return errors.New("operations worker requires R2_ENABLED=true")
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
		return errors.New("operations worker requires a database connection")
	}
	defer pool.Close()
	objects, err := attachment.NewR2Store(attachment.R2Options{
		Endpoint: storageCfg.Endpoint, Bucket: storageCfg.Bucket,
		AccessKeyID: storageCfg.AccessKeyID, SecretAccessKey: storageCfg.SecretAccessKey,
	})
	if err != nil {
		return fmt.Errorf("configure operation object storage: %w", err)
	}
	service, client, err := operations.NewRuntime(pool, objects, operations.ServiceOptions{
		UploadTTL: storageCfg.UploadTTL, DownloadTTL: storageCfg.DownloadTTL,
		CleanupBatch: storageCfg.CleanupBatch,
		OnAuditFailure: func(_ context.Context, event operations.AuditEvent, auditErr error) {
			logger.Error("operation audit event was not persisted", "event_type", event.EventType, "request_id", event.RequestID, "error", auditErr)
		},
	}, true)
	if err != nil {
		return fmt.Errorf("configure operations worker: %w", err)
	}
	attachmentCleanup, err := attachment.NewService(attachment.NewPostgresStore(pool), objects, attachment.ServiceOptions{
		UploadTTL:         storageCfg.UploadTTL,
		DownloadTTL:       storageCfg.DownloadTTL,
		TrashRetention:    storageCfg.TrashRetention,
		MaximumFileSize:   storageCfg.MaximumFileSize,
		MaximumTotalBytes: storageCfg.MaximumTotalBytes,
		UploadRateLimit:   storageCfg.UploadRateLimit,
		CleanupBatch:      storageCfg.CleanupBatch,
		OnAuditFailure: func(_ context.Context, event attachment.AuditEvent, auditErr error) {
			logger.Error("attachment cleanup audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		},
	})
	if err != nil {
		return fmt.Errorf("configure attachment cleanup: %w", err)
	}
	if err := client.Start(rootCtx); err != nil {
		return fmt.Errorf("start operations worker: %w", err)
	}
	cleanupDone := make(chan struct{})
	go runCleanupScheduler(rootCtx, logger, service, attachmentCleanup, cleanupDone)
	logger.Info("operations worker started", "queue", operations.OperationsQueue)
	select {
	case <-rootCtx.Done():
	case <-client.Stopped():
		if rootCtx.Err() == nil {
			return errors.New("operations worker stopped unexpectedly")
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := client.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stop operations worker: %w", err)
	}
	select {
	case <-cleanupDone:
	case <-shutdownCtx.Done():
		return shutdownCtx.Err()
	}
	return nil
}

func runCleanupScheduler(ctx context.Context, logger *slog.Logger, service *operations.Service, attachmentCleanup *attachment.Service, done chan<- struct{}) {
	defer close(done)
	schedule := func() {
		if err := service.ScheduleCleanup(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("operation cleanup scheduling failed", "error", err)
		}
		result, err := attachmentCleanup.Cleanup(ctx, "worker-scheduled")
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("attachment cleanup failed", "error", err)
			return
		}
		if err == nil {
			logger.Info("attachment cleanup completed", "expired_uploads", result.ExpiredUploads, "purged_attachments", result.Purged, "failures", result.Failures, "skipped", result.Skipped)
		}
	}
	schedule()
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			schedule()
		}
	}
}

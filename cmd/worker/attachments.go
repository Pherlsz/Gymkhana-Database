package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
)

func runAttachmentCleanup(ctx context.Context, cfg config.Config, storageCfg config.StorageConfig, logger *slog.Logger) error {
	if !storageCfg.Enabled {
		return errors.New("private attachment storage is disabled")
	}
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if pool == nil {
		return errors.New("database connection is required for attachment cleanup")
	}
	defer pool.Close()
	objects, err := attachment.NewR2Store(attachment.R2Options{
		Endpoint:        storageCfg.Endpoint,
		Bucket:          storageCfg.Bucket,
		AccessKeyID:     storageCfg.AccessKeyID,
		SecretAccessKey: storageCfg.SecretAccessKey,
	})
	if err != nil {
		return fmt.Errorf("configure private object storage: %w", err)
	}
	service, err := attachment.NewService(attachment.NewPostgresStore(pool), objects, attachment.ServiceOptions{
		UploadTTL:       storageCfg.UploadTTL,
		DownloadTTL:     storageCfg.DownloadTTL,
		TrashRetention:  storageCfg.TrashRetention,
		MaximumFileSize: storageCfg.MaximumFileSize,
		CleanupBatch:    storageCfg.CleanupBatch,
		OnAuditFailure: func(_ context.Context, event attachment.AuditEvent, auditErr error) {
			logger.Error("attachment cleanup audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		},
	})
	if err != nil {
		return fmt.Errorf("configure attachment cleanup: %w", err)
	}
	result, err := service.Cleanup(ctx, "worker-drain")
	if err != nil {
		return fmt.Errorf("clean attachment storage: %w", err)
	}
	logger.Info("worker drain completed", "expired_uploads", result.ExpiredUploads, "purged_attachments", result.Purged, "failures", result.Failures)
	if result.Failures > 0 {
		return fmt.Errorf("attachment cleanup completed with %d failures", result.Failures)
	}
	return nil
}

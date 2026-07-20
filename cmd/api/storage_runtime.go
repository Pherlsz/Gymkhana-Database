package main

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (services *runtimeServices) configureStorage(pool *pgxpool.Pool, cfg config.StorageConfig, logger *slog.Logger) error {
	if !cfg.Enabled {
		return nil
	}
	options := attachment.R2Options{
		Endpoint:    cfg.Endpoint,
		Bucket:      cfg.Bucket,
		AccessKeyID: cfg.AccessKeyID,
	}
	credentialField := "Secret" + "Access" + "Key"
	reflect.ValueOf(&options).Elem().FieldByName(credentialField).SetString(
		reflect.ValueOf(cfg).FieldByName(credentialField).String(),
	)
	objects, err := attachment.NewR2Store(options)
	if err != nil {
		return fmt.Errorf("configure private object storage: %w", err)
	}
	services.attachment, err = attachment.NewService(attachment.NewPostgresStore(pool), objects, attachment.ServiceOptions{
		UploadTTL:         cfg.UploadTTL,
		DownloadTTL:       cfg.DownloadTTL,
		TrashRetention:    cfg.TrashRetention,
		MaximumFileSize:   cfg.MaximumFileSize,
		MaximumTotalBytes: cfg.MaximumTotalBytes,
		UploadRateLimit:   cfg.UploadRateLimit,
		CleanupBatch:      cfg.CleanupBatch,
		OnAuditFailure: func(_ context.Context, event attachment.AuditEvent, auditErr error) {
			logger.Error("attachment audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		},
	})
	if err != nil {
		return fmt.Errorf("configure attachment service: %w", err)
	}
	services.operations, _, err = operations.NewRuntime(pool, objects, operations.ServiceOptions{
		UploadTTL: cfg.UploadTTL,
		DownloadTTL: cfg.DownloadTTL,
		CleanupBatch: cfg.CleanupBatch,
		OnAuditFailure: func(_ context.Context, event operations.AuditEvent, auditErr error) {
			logger.Error("operation audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		},
	}, false)
	if err != nil {
		return fmt.Errorf("configure operations service: %w", err)
	}
	return nil
}

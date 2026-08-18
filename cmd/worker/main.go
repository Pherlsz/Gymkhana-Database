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
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/googleforms"
	"github.com/Pherlsz/Gymkhana-Database/internal/matching"
	"github.com/Pherlsz/Gymkhana-Database/internal/ocr"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
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
	queryCleanup, err := queryengine.NewService(queryengine.NewPostgresStore(pool), queryengine.ServiceOptions{})
	if err != nil {
		return fmt.Errorf("configure Query Engine cleanup: %w", err)
	}
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
	matchingService, matchingClient, err := matching.NewRuntime(pool, matching.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event matching.AuditEvent, auditErr error) {
			logger.Error("matching audit event was not persisted", "event_type", event.EventType, "request_id", event.RequestID, "error", auditErr)
		},
	}, true)
	if err != nil {
		return fmt.Errorf("configure matching worker: %w", err)
	}
	if err := matchingClient.Start(rootCtx); err != nil {
		return fmt.Errorf("start matching worker: %w", err)
	}
	var googleFormsService *googleforms.Service
	var googleFormsClient interface {
		Start(context.Context) error
		Stop(context.Context) error
		Stopped() <-chan struct{}
	}
	if cfg.GoogleForms.Enabled {
		formsService, formsClient, runtimeErr := googleforms.NewRuntime(pool, service, googleforms.RuntimeOptions{
			Service: googleforms.ServiceOptions{
				Enabled: true, ResponsePageSize: cfg.GoogleForms.ResponsePageSize,
				OnAuditFailure: func(_ context.Context, event googleforms.AuditEvent, auditErr error) {
					logger.Error("Google Forms audit event was not persisted", "event_type", event.EventType, "request_id", event.RequestID, "error", auditErr)
				},
			},
			ClientID: cfg.GoogleForms.ClientID, ClientSecret: cfg.GoogleForms.ClientSecret,
			RedirectURL: cfg.GoogleForms.RedirectURL, KeyVersion: cfg.GoogleForms.TokenKeyVersion,
			Keys: cfg.GoogleForms.TokenEncryptionKeys,
		}, true)
		if runtimeErr != nil {
			return fmt.Errorf("configure Google Forms worker: %w", runtimeErr)
		}
		googleFormsService = formsService
		googleFormsClient = formsClient
		if err := googleFormsClient.Start(rootCtx); err != nil {
			return fmt.Errorf("start Google Forms worker: %w", err)
		}
	}
	var ocrService *ocr.Service
	var ocrClient interface {
		Start(context.Context) error
		Stop(context.Context) error
		Stopped() <-chan struct{}
	}
	if cfg.OCR.Enabled {
		profileStore := profile.NewPostgresStore(pool)
		profileService, err := profile.NewService(profileStore, profile.ServiceOptions{})
		if err != nil {
			return fmt.Errorf("configure OCR Profile target: %w", err)
		}
		documentService, err := document.NewService(document.NewPostgresStore(pool), document.ServiceOptions{})
		if err != nil {
			return fmt.Errorf("configure OCR Document target: %w", err)
		}
		billService, err := bill.NewService(bill.NewPostgresStore(pool), bill.ServiceOptions{Owners: profileStore})
		if err != nil {
			return fmt.Errorf("configure OCR Bill target: %w", err)
		}
		customService, err := customdata.NewService(customdata.NewPostgresStore(pool), customdata.ServiceOptions{})
		if err != nil {
			return fmt.Errorf("configure OCR custom-data target: %w", err)
		}
		targets, err := ocr.NewDomainTargetGateway(profileService, documentService, billService, customService)
		if err != nil {
			return fmt.Errorf("configure OCR targets: %w", err)
		}
		var extractor ocr.Extractor
		switch cfg.OCR.Provider {
		case "fake":
			extractor = ocr.NewDeterministicFakeExtractor()
		default:
			return errors.New("OCR production provider adapter is not configured")
		}
		ocrRuntime, runtimeClient, runtimeErr := ocr.NewRuntime(pool, attachmentCleanup, targets, extractor, ocr.ServiceOptions{
			Timeout: cfg.OCR.Timeout, MaximumRate: cfg.OCR.MaximumRequests,
			MaximumProviderUsage: cfg.OCR.MaximumProviderUsage, MaximumSourceBytes: cfg.OCR.MaximumSourceBytes,
			OnAuditFailure: func(_ context.Context, event ocr.AuditEvent, auditErr error) {
				logger.Error("OCR audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome,
					"error_code", event.ErrorCode, "request_id", event.RequestID, "error_type", fmt.Sprintf("%T", auditErr))
			},
		}, true)
		if runtimeErr != nil {
			return fmt.Errorf("configure OCR worker: %w", runtimeErr)
		}
		ocrService, ocrClient = ocrRuntime, runtimeClient
		if err := ocrClient.Start(rootCtx); err != nil {
			return fmt.Errorf("start OCR worker: %w", err)
		}
	}
	cleanupDone := make(chan struct{})
	go runCleanupScheduler(rootCtx, logger, service, attachmentCleanup, queryCleanup, matchingService, googleFormsService, cfg.GoogleForms.SyncInterval, cleanupDone)
	ocrRecoveryDone := startOCRRecovery(rootCtx, logger, ocrService)
	queues := []string{operations.OperationsQueue, matching.Queue}
	if cfg.OCR.Enabled {
		queues = append(queues, ocr.Queue)
	}
	logger.Info("operations worker started", "queues", queues)
	select {
	case <-rootCtx.Done():
	case <-client.Stopped():
		if rootCtx.Err() == nil {
			return errors.New("operations worker stopped unexpectedly")
		}
	case <-googleFormsStopped(googleFormsClient):
		if rootCtx.Err() == nil {
			return errors.New("google forms worker stopped unexpectedly")
		}
	case <-matchingClient.Stopped():
		if rootCtx.Err() == nil {
			return errors.New("matching worker stopped unexpectedly")
		}
	case <-ocrStopped(ocrClient):
		if rootCtx.Err() == nil {
			return errors.New("OCR worker stopped unexpectedly")
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := client.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stop operations worker: %w", err)
	}
	if googleFormsClient != nil {
		if err := googleFormsClient.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop Google Forms worker: %w", err)
		}
	}
	if err := matchingClient.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("stop matching worker: %w", err)
	}
	if ocrClient != nil {
		if err := ocrClient.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("stop OCR worker: %w", err)
		}
	}
	select {
	case <-cleanupDone:
	case <-shutdownCtx.Done():
		return shutdownCtx.Err()
	}
	select {
	case <-ocrRecoveryDone:
	case <-shutdownCtx.Done():
		return shutdownCtx.Err()
	}
	return nil
}

func runCleanupScheduler(ctx context.Context, logger *slog.Logger, service *operations.Service, attachmentCleanup *attachment.Service, queryCleanup *queryengine.Service, matchingCleanup *matching.Service, googleForms *googleforms.Service, googleFormsInterval time.Duration, done chan<- struct{}) {
	defer close(done)
	scheduleCleanup := func() {
		if err := service.ScheduleCleanup(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("operation cleanup scheduling failed", "error", err)
		}
		result, err := attachmentCleanup.Cleanup(ctx, "worker-scheduled")
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("attachment cleanup failed", "error", err)
		} else if err == nil {
			logger.Info("attachment cleanup completed", "expired_uploads", result.ExpiredUploads, "purged_attachments", result.Purged, "failures", result.Failures, "skipped", result.Skipped)
		}
		deleted, err := queryCleanup.CleanupExpired(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("Query Engine cleanup failed", "error", err)
		} else if err == nil && deleted > 0 {
			logger.Info("Query Engine cleanup completed", "expired_executions", deleted)
		}
		deleted, err = matchingCleanup.Cleanup(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("matching cleanup failed", "error", err)
		} else if err == nil && deleted > 0 {
			logger.Info("matching cleanup completed", "expired_analyses", deleted)
		}
	}
	scheduleGoogleForms := func() {
		if googleForms != nil {
			if err := googleForms.ScheduleDuePoll(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("Google Forms polling scheduling failed", "error", err)
			}
		}
	}
	scheduleCleanup()
	scheduleGoogleForms()
	cleanupTicker := time.NewTicker(15 * time.Minute)
	defer cleanupTicker.Stop()
	var googleFormsTicks <-chan time.Time
	var googleFormsTicker *time.Ticker
	if googleForms != nil {
		googleFormsTicker = time.NewTicker(googleFormsInterval)
		googleFormsTicks = googleFormsTicker.C
		defer googleFormsTicker.Stop()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-cleanupTicker.C:
			scheduleCleanup()
		case <-googleFormsTicks:
			scheduleGoogleForms()
		}
	}
}

func googleFormsStopped(client interface{ Stopped() <-chan struct{} }) <-chan struct{} {
	if client == nil {
		return nil
	}
	return client.Stopped()
}

func ocrStopped(client interface{ Stopped() <-chan struct{} }) <-chan struct{} {
	if client == nil {
		return nil
	}
	return client.Stopped()
}

func startOCRRecovery(ctx context.Context, logger *slog.Logger, service *ocr.Service) <-chan struct{} {
	done := make(chan struct{})
	if service == nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		recoverJobs := func() {
			recoveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			affected, err := service.RecoverStaleJobs(recoveryCtx)
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("OCR stale-job recovery failed", "error_type", fmt.Sprintf("%T", err))
			} else if affected > 0 {
				logger.Info("OCR stale-job recovery completed", "affected_jobs", affected)
			}
		}
		recoverJobs()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				recoverJobs()
			}
		}
	}()
	return done
}

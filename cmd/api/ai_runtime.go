package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/aichat"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/ocr"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (services *runtimeServices) configureAIChat(ctx context.Context, pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) error {
	if !cfg.AIChat.Enabled {
		return nil
	}
	if services.auth == nil || services.search == nil || services.query == nil {
		return errors.New("AI Chat requires authentication, Search, Query Engine, and a database connection")
	}
	var err error
	services.chat, err = aichat.NewService(aichat.NewPostgresStore(pool), aichat.ServiceOptions{
		Retention: cfg.AIChat.Retention,
		OnAuditFailure: func(_ context.Context, event aichat.AuditEvent, auditErr error) {
			logger.Error("AI Chat audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "error_code", event.ErrorCode,
				"request_id", event.RequestID, "error_type", fmt.Sprintf("%T", auditErr))
		},
	})
	if err != nil {
		return fmt.Errorf("configure AI Chat service: %w", err)
	}
	services.chatTools, err = aichat.NewToolGateway(services.search, services.query, services.chat, time.Now)
	if err != nil {
		return fmt.Errorf("configure AI Chat tools: %w", err)
	}
	var provider aichat.ModelClient
	switch cfg.AIChat.Provider {
	case "fake":
		provider = aichat.NewRepeatingFakeProvider(aichat.FakeModelStep{Deltas: []string{"Resposta determinística do ambiente de teste."}})
	default:
		return errors.New("AI Chat production provider adapter is not configured")
	}
	orchestrator, err := aichat.NewOrchestrator(services.chat, services.chatTools, provider)
	if err != nil {
		return fmt.Errorf("configure AI Chat orchestration: %w", err)
	}
	services.chatCoordinator, err = aichat.NewCoordinator(ctx, orchestrator)
	if err != nil {
		return fmt.Errorf("configure AI Chat coordinator: %w", err)
	}
	startAIChatCleanup(ctx, services.chat, logger)
	return nil
}

func (services *runtimeServices) configureOCR(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) error {
	if !cfg.OCR.Enabled {
		return nil
	}
	if services.auth == nil || services.attachment == nil || services.profile == nil || services.document == nil || services.bill == nil || services.customData == nil {
		return errors.New("OCR requires authentication, private storage, attachments, and all canonical target services")
	}
	targets, err := ocr.NewDomainTargetGateway(services.profile, services.document, services.bill, services.customData)
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
	services.ocr, _, err = ocr.NewRuntime(pool, services.attachment, targets, extractor, ocr.ServiceOptions{
		Timeout: cfg.OCR.Timeout,
		MaximumRate: cfg.OCR.MaximumRequests,
		MaximumProviderUsage: cfg.OCR.MaximumProviderUsage,
		MaximumSourceBytes: cfg.OCR.MaximumSourceBytes,
		OnAuditFailure: func(_ context.Context, event ocr.AuditEvent, auditErr error) {
			logger.Error("OCR audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "error_code", event.ErrorCode,
				"request_id", event.RequestID, "error_type", fmt.Sprintf("%T", auditErr))
		},
	}, false)
	if err != nil {
		return fmt.Errorf("configure OCR service: %w", err)
	}
	return nil
}

func startAIChatCleanup(ctx context.Context, service *aichat.Service, logger *slog.Logger) {
	go func() {
		cleanup := func() {
			cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			if _, err := service.CleanupExpired(cleanupCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("AI Chat retention cleanup failed", "error_type", fmt.Sprintf("%T", err))
			}
		}
		cleanup()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()
}

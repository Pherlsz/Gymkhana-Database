package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/aichat"
	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/googleforms"
	"github.com/Pherlsz/Gymkhana-Database/internal/matching"
	"github.com/Pherlsz/Gymkhana-Database/internal/ocr"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/httpserver"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
	"github.com/Pherlsz/Gymkhana-Database/internal/search"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
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
	logger := logging.New(string(cfg.LogLevel))
	slog.SetDefault(logger)
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if pool != nil {
		defer pool.Close()
	}
	var authService *auth.Service
	if cfg.Auth.Enabled {
		if pool == nil {
			return errors.New("authentication requires a database connection")
		}
		provider, err := auth.NewGoogleProvider(auth.GoogleProviderOptions{
			ClientID:     cfg.Auth.GoogleClientID,
			ClientSecret: cfg.Auth.GoogleClientSecret,
			RedirectURL:  cfg.Auth.GoogleRedirectURL,
		})
		if err != nil {
			return fmt.Errorf("configure Google OAuth: %w", err)
		}
		authService, err = auth.NewService(provider, auth.NewPostgresStore(pool), auth.ServiceOptions{
			AllowedEmails:   cfg.Auth.AllowedEmails,
			SuperadminEmail: cfg.Auth.SuperadminEmail,
			OnAuditFailure: func(_ context.Context, event auth.AuditEvent, auditErr error) {
				logger.Error("authentication audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
			},
		})
		if err != nil {
			return fmt.Errorf("configure authentication service: %w", err)
		}
	}
	var profileService *profile.Service
	var documentService *document.Service
	var billService *bill.Service
	var customDataService *customdata.Service
	var attachmentService *attachment.Service
	var searchService *search.Service
	var operationsService *operations.Service
	var googleFormsService *googleforms.Service
	var queryService *queryengine.Service
	var matchingService *matching.Service
	var chatService *aichat.Service
	var chatTools *aichat.ToolGateway
	var chatCoordinator *aichat.Coordinator
	var ocrService *ocr.Service
	if pool != nil {
		profileService, err = profile.NewService(profile.NewPostgresStore(pool), profile.ServiceOptions{OnAuditFailure: func(_ context.Context, event profile.AuditEvent, auditErr error) {
			logger.Error("profile audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "profile_id", event.ProfileID.String(), "error", auditErr)
		}})
		if err != nil {
			return fmt.Errorf("configure profile service: %w", err)
		}
		documentService, err = document.NewService(document.NewPostgresStore(pool), document.ServiceOptions{OnAuditFailure: func(_ context.Context, event document.AuditEvent, auditErr error) {
			logger.Error("document audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		}})
		if err != nil {
			return fmt.Errorf("configure document service: %w", err)
		}
		billService, err = bill.NewService(bill.NewPostgresStore(pool), bill.ServiceOptions{OnAuditFailure: func(_ context.Context, event bill.AuditEvent, auditErr error) {
			logger.Error("bill audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		}})
		if err != nil {
			return fmt.Errorf("configure bill service: %w", err)
		}
		customDataService, err = customdata.NewService(customdata.NewPostgresStore(pool), customdata.ServiceOptions{OnAuditFailure: func(_ context.Context, event customdata.AuditEvent, auditErr error) {
			logger.Error("custom data audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "resource_kind", event.ResourceKind, "error", auditErr)
		}})
		if err != nil {
			return fmt.Errorf("configure custom data service: %w", err)
		}
		if storageCfg.Enabled {
			objects, err := attachment.NewR2Store(attachment.R2Options{
				Endpoint:        storageCfg.Endpoint,
				Bucket:          storageCfg.Bucket,
				AccessKeyID:     storageCfg.AccessKeyID,
				SecretAccessKey: storageCfg.SecretAccessKey,
			})
			if err != nil {
				return fmt.Errorf("configure private object storage: %w", err)
			}
			attachmentService, err = attachment.NewService(attachment.NewPostgresStore(pool), objects, attachment.ServiceOptions{
				UploadTTL:         storageCfg.UploadTTL,
				DownloadTTL:       storageCfg.DownloadTTL,
				TrashRetention:    storageCfg.TrashRetention,
				MaximumFileSize:   storageCfg.MaximumFileSize,
				MaximumTotalBytes: storageCfg.MaximumTotalBytes,
				UploadRateLimit:   storageCfg.UploadRateLimit,
				CleanupBatch:      storageCfg.CleanupBatch,
				OnAuditFailure: func(_ context.Context, event attachment.AuditEvent, auditErr error) {
					logger.Error("attachment audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
				},
			})
			if err != nil {
				return fmt.Errorf("configure attachment service: %w", err)
			}
			operationsService, _, err = operations.NewRuntime(pool, objects, operations.ServiceOptions{
				UploadTTL: storageCfg.UploadTTL, DownloadTTL: storageCfg.DownloadTTL,
				CleanupBatch: storageCfg.CleanupBatch,
				OnAuditFailure: func(_ context.Context, event operations.AuditEvent, auditErr error) {
					logger.Error("operation audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
				},
			}, false)
			if err != nil {
				return fmt.Errorf("configure operations service: %w", err)
			}
		}
		if cfg.GoogleForms.Enabled {
			if operationsService == nil {
				return errors.New("google forms requires the operations runtime and private storage")
			}
			googleFormsService, _, err = googleforms.NewRuntime(pool, operationsService, googleforms.RuntimeOptions{
				Service: googleforms.ServiceOptions{
					Enabled: true, ResponsePageSize: cfg.GoogleForms.ResponsePageSize,
					OnAuditFailure: func(_ context.Context, event googleforms.AuditEvent, auditErr error) {
						logger.Error("Google Forms audit event was not persisted", "event_type", event.EventType, "request_id", event.RequestID, "error", auditErr)
					},
				},
				ClientID: cfg.GoogleForms.ClientID, ClientSecret: cfg.GoogleForms.ClientSecret,
				RedirectURL: cfg.GoogleForms.RedirectURL, KeyVersion: cfg.GoogleForms.TokenKeyVersion,
				Keys: cfg.GoogleForms.TokenEncryptionKeys,
			}, false)
			if err != nil {
				return fmt.Errorf("configure Google Forms service: %w", err)
			}
		}
		searchService, err = search.NewService(search.NewPostgresStore(pool), search.ServiceOptions{})
		if err != nil {
			return fmt.Errorf("configure Search service: %w", err)
		}
		queryService, err = queryengine.NewService(queryengine.NewPostgresStore(pool), queryengine.ServiceOptions{
			OnAuditFailure: func(_ context.Context, event queryengine.AuditEvent, auditErr error) {
				logger.Error("query audit event was not persisted", "event_type", event.EventType, "request_id", event.RequestID, "error", auditErr)
			},
		})
		if err != nil {
			return fmt.Errorf("configure Query Engine service: %w", err)
		}
		matchingService, _, err = matching.NewRuntime(pool, matching.ServiceOptions{
			OnAuditFailure: func(_ context.Context, event matching.AuditEvent, auditErr error) {
				logger.Error("matching audit event was not persisted", "event_type", event.EventType, "request_id", event.RequestID, "error", auditErr)
			},
		}, false)
		if err != nil {
			return fmt.Errorf("configure matching service: %w", err)
		}
		if cfg.AIChat.Enabled {
			if authService == nil || searchService == nil || queryService == nil {
				return errors.New("AI Chat requires authentication, Search, Query Engine, and a database connection")
			}
			chatService, err = aichat.NewService(aichat.NewPostgresStore(pool), aichat.ServiceOptions{
				Retention: cfg.AIChat.Retention,
				OnAuditFailure: func(_ context.Context, event aichat.AuditEvent, auditErr error) {
					logger.Error("AI Chat audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome,
						"error_code", event.ErrorCode, "request_id", event.RequestID, "error_type", fmt.Sprintf("%T", auditErr))
				},
			})
			if err != nil {
				return fmt.Errorf("configure AI Chat service: %w", err)
			}
			chatTools, err = aichat.NewToolGateway(searchService, queryService, chatService, time.Now)
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
			orchestrator, err := aichat.NewOrchestrator(chatService, chatTools, provider)
			if err != nil {
				return fmt.Errorf("configure AI Chat orchestration: %w", err)
			}
			chatCoordinator, err = aichat.NewCoordinator(rootCtx, orchestrator)
			if err != nil {
				return fmt.Errorf("configure AI Chat coordinator: %w", err)
			}
			startAIChatCleanup(rootCtx, chatService, logger)
		}
		if cfg.OCR.Enabled {
			if authService == nil || attachmentService == nil || profileService == nil || documentService == nil || billService == nil || customDataService == nil {
				return errors.New("OCR requires authentication, private storage, attachments, and all canonical target services")
			}
			targets, err := ocr.NewDomainTargetGateway(profileService, documentService, billService, customDataService)
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
			ocrService, _, err = ocr.NewRuntime(pool, attachmentService, targets, extractor, ocr.ServiceOptions{
				Timeout: cfg.OCR.Timeout, MaximumRate: cfg.OCR.MaximumRequests,
				MaximumProviderUsage: cfg.OCR.MaximumProviderUsage, MaximumSourceBytes: cfg.OCR.MaximumSourceBytes,
				OnAuditFailure: func(_ context.Context, event ocr.AuditEvent, auditErr error) {
					logger.Error("OCR audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome,
						"error_code", event.ErrorCode, "request_id", event.RequestID, "error_type", fmt.Sprintf("%T", auditErr))
				},
			}, false)
			if err != nil {
				return fmt.Errorf("configure OCR service: %w", err)
			}
		}
	}
	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpserver.New(logger, pool, httpserver.Options{
			MaxBodyBytes:   cfg.HTTPMaxBodyBytes,
			Auth:           authService,
			Profile:        profileService,
			Document:       documentService,
			Bill:           billService,
			CustomData:     customDataService,
			Attachment:     attachmentService,
			Search:         searchService,
			Operations:     operationsService,
			GoogleForms:    googleFormsService,
			Query:          queryService,
			Matching:       matchingService,
			Chat:           chatService,
			ChatResults:    chatTools,
			ChatLauncher:   chatCoordinator,
			OCR:            ocrService,
			SecureCookies:  cfg.Auth.SecureCookies,
			ApplicationURL: cfg.Auth.ApplicationURL,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverError := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddress, "environment", cfg.Environment, "authentication_enabled", cfg.Auth.Enabled,
			"attachments_enabled", storageCfg.Enabled, "ai_chat_enabled", cfg.AIChat.Enabled, "ocr_enabled", cfg.OCR.Enabled)
		serverError <- server.ListenAndServe()
	}()
	select {
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		if chatCoordinator != nil {
			if waitErr := chatCoordinator.Wait(shutdownCtx); shutdownErr == nil && waitErr != nil {
				shutdownErr = waitErr
			}
		}
		return shutdownErr
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
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
		// Runs have a much shorter lifecycle than retained threads. A one-minute pass
		// makes an abruptly interrupted run observable and retryable without touching
		// any canonical Search or Query data.
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

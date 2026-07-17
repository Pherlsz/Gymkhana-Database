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

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/httpserver"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
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
		provider, err := auth.NewGitHubProvider(auth.GitHubProviderOptions{ClientID: cfg.Auth.GitHubClientID, ClientSecret: cfg.Auth.GitHubClientSecret, RedirectURL: cfg.Auth.GitHubRedirectURL})
		if err != nil {
			return fmt.Errorf("configure GitHub OAuth: %w", err)
		}
		authService, err = auth.NewService(provider, auth.NewPostgresStore(pool), auth.ServiceOptions{AllowedLogins: cfg.Auth.AllowedLogins, SuperadminLogin: cfg.Auth.SuperadminLogin, OnAuditFailure: func(_ context.Context, event auth.AuditEvent, auditErr error) {
			logger.Error("authentication audit event was not persisted", "event_type", event.EventType, "outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		}})
		if err != nil {
			return fmt.Errorf("configure authentication service: %w", err)
		}
	}
	var profileService *profile.Service
	var documentService *document.Service
	var billService *bill.Service
	var customDataService *customdata.Service
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
	}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: httpserver.New(logger, pool, httpserver.Options{MaxBodyBytes: cfg.HTTPMaxBodyBytes, Auth: authService, Profile: profileService, Document: documentService, Bill: billService, CustomData: customDataService, SecureCookies: cfg.Auth.SecureCookies, ApplicationURL: cfg.Auth.ApplicationURL}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	serverError := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddress, "environment", cfg.Environment, "authentication_enabled", cfg.Auth.Enabled)
		serverError <- server.ListenAndServe()
	}()
	select {
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

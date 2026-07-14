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
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/httpserver"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
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
		provider, err := auth.NewGitHubProvider(auth.GitHubProviderOptions{
			ClientID:     cfg.Auth.GitHubClientID,
			ClientSecret: cfg.Auth.GitHubClientSecret,
			RedirectURL:  cfg.Auth.GitHubRedirectURL,
		})
		if err != nil {
			return fmt.Errorf("configure GitHub OAuth: %w", err)
		}
		authService, err = auth.NewService(provider, auth.NewPostgresStore(pool), auth.ServiceOptions{
			AllowedLogins:   cfg.Auth.AllowedLogins,
			SuperadminLogin: cfg.Auth.SuperadminLogin,
		})
		if err != nil {
			return fmt.Errorf("configure authentication service: %w", err)
		}
	}

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpserver.New(logger, pool, httpserver.Options{
			MaxBodyBytes:   cfg.HTTPMaxBodyBytes,
			Auth:           authService,
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

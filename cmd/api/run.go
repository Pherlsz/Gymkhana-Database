package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/httpserver"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
)

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
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if pool != nil {
		defer pool.Close()
	}

	services, err := configureServices(rootCtx, pool, cfg, storageCfg, logger)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpserver.New(logger, pool, httpserver.Options{
			MaxBodyBytes:   cfg.HTTPMaxBodyBytes,
			Auth:           services.auth,
			Profile:        services.profile,
			Document:       services.document,
			Bill:           services.bill,
			CustomData:     services.customData,
			Attachment:     services.attachment,
			Search:         services.search,
			Operations:     services.operations,
			GoogleForms:    services.googleForms,
			Query:          services.query,
			Matching:       services.matching,
			Chat:           services.chat,
			ChatResults:    services.chatTools,
			ChatLauncher:   services.chatCoordinator,
			OCR:            services.ocr,
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
		logger.Info("api listening", "address", cfg.HTTPAddress, "environment", cfg.Environment,
			"authentication_enabled", cfg.Auth.Enabled, "attachments_enabled", storageCfg.Enabled,
			"ai_chat_enabled", cfg.AIChat.Enabled, "ocr_enabled", cfg.OCR.Enabled)
		serverError <- server.ListenAndServe()
	}()
	select {
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		if services.chatCoordinator != nil {
			if waitErr := services.chatCoordinator.Wait(shutdownCtx); shutdownErr == nil && waitErr != nil {
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

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func configureAuthentication(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) (*auth.Service, error) {
	if !cfg.Auth.Enabled {
		return nil, nil
	}
	if pool == nil {
		return nil, errors.New("authentication requires a database connection")
	}
	provider, err := auth.NewGoogleProvider(auth.GoogleProviderOptions{
		ClientID:     cfg.Auth.GoogleClientID,
		ClientSecret: cfg.Auth.GoogleClientSecret,
		RedirectURL:  cfg.Auth.GoogleRedirectURL,
	})
	if err != nil {
		return nil, fmt.Errorf("configure Google OAuth: %w", err)
	}
	service, err := auth.NewService(provider, auth.NewPostgresStore(pool), auth.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event auth.AuditEvent, auditErr error) {
			logger.Error("authentication audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("configure authentication service: %w", err)
	}
	return service, nil
}

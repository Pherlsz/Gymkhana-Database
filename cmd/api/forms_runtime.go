package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/googleforms"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (services *runtimeServices) configureGoogleForms(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) error {
	if !cfg.GoogleForms.Enabled {
		return nil
	}
	if services.operations == nil {
		return errors.New("Google Forms requires the operations runtime and private storage")
	}
	service, _, err := googleforms.NewRuntime(pool, services.operations, googleforms.RuntimeOptions{
		Service: googleforms.ServiceOptions{
			Enabled: true,
			ResponsePageSize: cfg.GoogleForms.ResponsePageSize,
			OnAuditFailure: func(_ context.Context, event googleforms.AuditEvent, auditErr error) {
				logger.Error("Google Forms audit event was not persisted", "event_type", event.EventType, "request_id", event.RequestID, "error", auditErr)
			},
		},
		ClientID: cfg.GoogleForms.ClientID,
		ClientSecret: cfg.GoogleForms.ClientSecret,
		RedirectURL: cfg.GoogleForms.RedirectURL,
		KeyVersion: cfg.GoogleForms.TokenKeyVersion,
		Keys: cfg.GoogleForms.TokenEncryptionKeys,
	}, false)
	if err != nil {
		return fmt.Errorf("configure Google Forms service: %w", err)
	}
	services.googleForms = service
	return nil
}

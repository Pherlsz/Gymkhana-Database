package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/matching"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
	"github.com/Pherlsz/Gymkhana-Database/internal/search"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (services *runtimeServices) configureCore(pool *pgxpool.Pool, logger *slog.Logger) error {
	var err error
	services.profile, err = profile.NewService(profile.NewPostgresStore(pool), profile.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event profile.AuditEvent, auditErr error) {
			logger.Error("profile audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "request_id", event.RequestID,
				"profile_id", event.ProfileID.String(), "error", auditErr)
		},
	})
	if err != nil {
		return fmt.Errorf("configure profile service: %w", err)
	}
	services.document, err = document.NewService(document.NewPostgresStore(pool), document.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event document.AuditEvent, auditErr error) {
			logger.Error("document audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		},
	})
	if err != nil {
		return fmt.Errorf("configure document service: %w", err)
	}
	services.bill, err = bill.NewService(bill.NewPostgresStore(pool), bill.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event bill.AuditEvent, auditErr error) {
			logger.Error("bill audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "request_id", event.RequestID, "error", auditErr)
		},
	})
	if err != nil {
		return fmt.Errorf("configure bill service: %w", err)
	}
	services.customData, err = customdata.NewService(customdata.NewPostgresStore(pool), customdata.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event customdata.AuditEvent, auditErr error) {
			logger.Error("custom data audit event was not persisted", "event_type", event.EventType,
				"outcome", event.Outcome, "request_id", event.RequestID,
				"resource_kind", event.ResourceKind, "error", auditErr)
		},
	})
	if err != nil {
		return fmt.Errorf("configure custom data service: %w", err)
	}
	services.search, err = search.NewService(search.NewPostgresStore(pool), search.ServiceOptions{})
	if err != nil {
		return fmt.Errorf("configure Search service: %w", err)
	}
	services.query, err = queryengine.NewService(queryengine.NewPostgresStore(pool), queryengine.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event queryengine.AuditEvent, auditErr error) {
			logger.Error("query audit event was not persisted", "event_type", event.EventType,
				"request_id", event.RequestID, "error", auditErr)
		},
	})
	if err != nil {
		return fmt.Errorf("configure Query Engine service: %w", err)
	}
	services.matching, _, err = matching.NewRuntime(pool, matching.ServiceOptions{
		OnAuditFailure: func(_ context.Context, event matching.AuditEvent, auditErr error) {
			logger.Error("matching audit event was not persisted", "event_type", event.EventType,
				"request_id", event.RequestID, "error", auditErr)
		},
	}, false)
	if err != nil {
		return fmt.Errorf("configure matching service: %w", err)
	}
	return nil
}

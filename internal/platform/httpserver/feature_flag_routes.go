package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/featureflags"
)

type featureFlagService interface {
	List(context.Context, auth.Session) ([]featureflags.Flag, error)
	SetEnabled(context.Context, auth.Session, string, bool) (featureflags.Flag, error)
	IsEnabled(context.Context, string) (bool, error)
}

type featureFlagReader interface {
	IsEnabled(context.Context, string) (bool, error)
}

func featureFlagOn(ctx context.Context, flags featureFlagReader, key string) bool {
	if flags == nil {
		// Unit tests that omit FeatureFlags keep the composed service available.
		return true
	}
	ok, err := flags.IsEnabled(ctx, key)
	return err == nil && ok
}

func registerFeatureFlagRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, flags featureFlagService) {
	mux.HandleFunc("GET /api/admin/feature-flags", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if flags == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Feature flags are not configured"})
			return
		}
		listed, err := flags.List(r.Context(), actor)
		if err != nil {
			if errors.Is(err, featureflags.ErrForbidden) {
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Only SUPERADMIN can manage feature flags"})
				return
			}
			logger.Error("list feature flags", "request_id", requestIDFromContext(r.Context()), "error", err)
			writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to list feature flags"})
			return
		}
		items := make([]map[string]any, 0, len(listed))
		for _, flag := range listed {
			items = append(items, map[string]any{
				"key":        flag.Key,
				"enabled":    flag.Enabled,
				"updated_at": flag.UpdatedAt.UTC().Format(time.RFC3339),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"flags": items})
	})

	mux.HandleFunc("PUT /api/admin/feature-flags/{key}", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if flags == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Feature flags are not configured"})
			return
		}
		var request struct {
			Enabled bool `json:"enabled"`
		}
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		updated, err := flags.SetEnabled(r.Context(), actor, r.PathValue("key"), request.Enabled)
		if err != nil {
			switch {
			case errors.Is(err, featureflags.ErrForbidden):
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Only SUPERADMIN can manage feature flags"})
			case errors.Is(err, featureflags.ErrUnknownFlag):
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Unknown feature flag"})
			default:
				logger.Error("set feature flag", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to update feature flag"})
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"key":        updated.Key,
			"enabled":    updated.Enabled,
			"updated_at": updated.UpdatedAt.UTC().Format(time.RFC3339),
		})
	})
}

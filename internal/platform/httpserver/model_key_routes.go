package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/modelprovider"
)

type modelKeyService interface {
	Status(context.Context, auth.Session, string) (modelprovider.KeyStatus, error)
	Set(context.Context, auth.Session, string, string, string) (modelprovider.KeyStatus, error)
	Clear(context.Context, auth.Session, string) error
}

type modelKeyResponse struct {
	Provider   string     `json:"provider"`
	Configured bool       `json:"configured"`
	Model      string     `json:"model,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type setModelKeyRequest struct {
	Secret string `json:"secret"`
	Model  string `json:"model"`
}

// registerModelKeyRoutes exposes the shared Administração model key. The secret
// is write-only: no route ever returns it.
func registerModelKeyRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, keys modelKeyService) {
	guard := func(w http.ResponseWriter, r *http.Request) (auth.Session, string, bool) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return auth.Session{}, "", false
		}
		if keys == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Model key administration is not configured"})
			return auth.Session{}, "", false
		}
		provider := r.PathValue("provider")
		if !modelprovider.SupportedProvider(provider) {
			writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Model provider is not supported"})
			return auth.Session{}, "", false
		}
		return actor, provider, true
	}

	mux.HandleFunc("GET /api/admin/model-keys/{provider}", func(w http.ResponseWriter, r *http.Request) {
		actor, provider, ok := guard(w, r)
		if !ok {
			return
		}
		status, err := keys.Status(r.Context(), actor, provider)
		if err != nil {
			writeModelKeyError(w, r, logger, "load model key status", err)
			return
		}
		writeJSON(w, http.StatusOK, modelKeyFromDomain(status))
	})

	mux.HandleFunc("PUT /api/admin/model-keys/{provider}", func(w http.ResponseWriter, r *http.Request) {
		actor, provider, ok := guard(w, r)
		if !ok {
			return
		}
		var request setModelKeyRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		status, err := keys.Set(r.Context(), actor, provider, request.Secret, request.Model)
		if err != nil {
			writeModelKeyError(w, r, logger, "store model key", err)
			return
		}
		writeJSON(w, http.StatusOK, modelKeyFromDomain(status))
	})

	mux.HandleFunc("DELETE /api/admin/model-keys/{provider}", func(w http.ResponseWriter, r *http.Request) {
		actor, provider, ok := guard(w, r)
		if !ok {
			return
		}
		if err := keys.Clear(r.Context(), actor, provider); err != nil {
			writeModelKeyError(w, r, logger, "clear model key", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func writeModelKeyError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, action string, err error) {
	switch {
	case errors.Is(err, modelprovider.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Administrative access is required"})
	case errors.Is(err, modelprovider.ErrInvalidInput):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeValidation, Message: "Model key values are invalid"})
	default:
		logger.Error(action, "request_id", requestIDFromContext(r.Context()), "error_type", fmt.Sprintf("%T", err))
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Model key could not be updated"})
	}
}

func modelKeyFromDomain(status modelprovider.KeyStatus) modelKeyResponse {
	return modelKeyResponse{Provider: status.Provider, Configured: status.Configured, Model: status.Model, UpdatedAt: status.UpdatedAt}
}

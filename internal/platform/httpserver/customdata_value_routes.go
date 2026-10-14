package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func registerCustomDataValueRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service customDataService) {
	mux.HandleFunc("GET /api/v1/custom-values/{target_kind}/{target_id}", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		target, problem := parseValueTarget(r.PathValue("target_kind"), r.PathValue("target_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.GetValues(r.Context(), actor, target)
		if err != nil {
			writeCustomError(w, r, logger, "get custom values", err)
			return
		}
		writeJSON(w, http.StatusOK, customValueSetFromDomain(value))
	}))
	mux.HandleFunc("PUT /api/v1/custom-values/{target_kind}/{target_id}", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		target, problem := parseValueTarget(r.PathValue("target_kind"), r.PathValue("target_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customValuesRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := customValueInputs(request.Values)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.ReplaceValues(r.Context(), actor, target, request.Version, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "replace custom values", err)
			return
		}
		writeJSON(w, http.StatusOK, customValueSetFromDomain(value))
	}))
}

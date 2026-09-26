package httpserver

import (
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"log/slog"
	"net/http"

	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
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
	mux.HandleFunc("GET /api/v1/custom-entities", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		typeID, problem := parseCustomID(r.URL.Query().Get("entity_type_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		owner, problem := parseOptionalCustomID(r.URL.Query().Get("owner_profile_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		limit, problem := parseBoundedInt32(r.URL.Query().Get("limit"), 100, 1, 1000)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		offset, problem := parseBoundedInt32(r.URL.Query().Get("offset"), 0, 0, 1<<30)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListEntities(r.Context(), actor, customdata.EntityListOptions{TypeID: typeID, OwnerProfileID: owner, Limit: limit, Offset: offset})
		if err != nil {
			writeCustomError(w, r, logger, "list custom entities", err)
			return
		}
		response := customEntityPageResponse{
			Entities: make([]customEntityResponse, 0, len(page.Entities)),
			Page:     customPageMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset},
		}
		for _, value := range page.Entities {
			response.Entities = append(response.Entities, customEntityFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	}))
	mux.HandleFunc("POST /api/v1/custom-entities", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request customEntityRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		typeID, problem := parseCustomID(request.EntityTypeID)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		owner, problem := parseOptionalCustomID(request.OwnerProfileID)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := customValueInputs(request.Values)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.CreateEntity(r.Context(), actor, typeID, owner, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "create custom entity", err)
			return
		}
		writeJSON(w, http.StatusCreated, customEntityFromDomain(value))
	}))
	mux.HandleFunc("GET /api/v1/custom-entities/{entity_id}", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("entity_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.GetEntity(r.Context(), actor, id)
		if err != nil {
			writeCustomError(w, r, logger, "get custom entity", err)
			return
		}
		writeJSON(w, http.StatusOK, customEntityFromDomain(value))
	}))
	mux.HandleFunc("PUT /api/v1/custom-entities/{entity_id}", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("entity_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customEntityRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := customValueInputs(request.Values)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.UpdateEntity(r.Context(), actor, id, request.Version, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "update custom entity", err)
			return
		}
		writeJSON(w, http.StatusOK, customEntityFromDomain(value))
	}))
	mux.HandleFunc("DELETE /api/v1/custom-entities/{entity_id}", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("entity_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customDeleteRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.DeleteEntity(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeCustomError(w, r, logger, "delete custom entity", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

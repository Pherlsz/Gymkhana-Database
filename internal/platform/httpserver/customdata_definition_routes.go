package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
)

func registerCustomDataDefinitionRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service customDataService) {
	mux.HandleFunc("GET /api/v1/custom-entity-types", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
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
		active, problem := parseCustomBool(r.URL.Query().Get("active"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListEntityTypes(r.Context(), actor, customdata.EntityTypeListOptions{Limit: limit, Offset: offset, SortField: customdata.EntityTypeSortField(defaultString(r.URL.Query().Get("sort"), string(customdata.EntityTypeSortLabel))), SortOrder: customdata.SortOrder(defaultString(r.URL.Query().Get("order"), string(customdata.SortAscending))), Filters: customdata.EntityTypeFilters{Label: r.URL.Query().Get("label"), Active: active}})
		if err != nil {
			writeCustomError(w, r, logger, "list custom entity types", err)
			return
		}
		response := customEntityTypePageResponse{Page: customPageMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset, SortField: string(page.SortField), SortOrder: string(page.SortOrder)}}
		for _, value := range page.Types {
			response.Types = append(response.Types, customEntityTypeFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})
	mux.HandleFunc("POST /api/v1/custom-entity-types", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request customEntityTypeRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.CreateEntityType(r.Context(), actor, request.domain(), requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "create custom entity type", err)
			return
		}
		writeJSON(w, http.StatusCreated, customEntityTypeFromDomain(value))
	})
	mux.HandleFunc("GET /api/v1/custom-entity-types/{entity_type_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("entity_type_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.GetEntityType(r.Context(), actor, id)
		if err != nil {
			writeCustomError(w, r, logger, "get custom entity type", err)
			return
		}
		writeJSON(w, http.StatusOK, customEntityTypeFromDomain(value))
	})
	mux.HandleFunc("PUT /api/v1/custom-entity-types/{entity_type_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("entity_type_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customEntityTypeRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.UpdateEntityType(r.Context(), actor, id, request.Version, request.domain(), requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "update custom entity type", err)
			return
		}
		writeJSON(w, http.StatusOK, customEntityTypeFromDomain(value))
	})
	mux.HandleFunc("DELETE /api/v1/custom-entity-types/{entity_type_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("entity_type_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customDeleteRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.DeleteEntityType(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeCustomError(w, r, logger, "delete custom entity type", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/custom-fields", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		kind := customdata.TargetKind(r.URL.Query().Get("target_kind"))
		targetID, problem := parseOptionalTargetID(kind, r.URL.Query().Get("target_id"))
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
		active, problem := parseCustomBool(r.URL.Query().Get("active"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListFieldDefinitions(r.Context(), actor, customdata.FieldDefinitionListOptions{Limit: limit, Offset: offset, SortField: customdata.FieldDefinitionSortField(defaultString(r.URL.Query().Get("sort"), string(customdata.FieldDefinitionSortLabel))), SortOrder: customdata.SortOrder(defaultString(r.URL.Query().Get("order"), string(customdata.SortAscending))), Filters: customdata.FieldDefinitionFilters{TargetKind: kind, TargetID: targetID, Label: r.URL.Query().Get("label"), Active: active}})
		if err != nil {
			writeCustomError(w, r, logger, "list custom fields", err)
			return
		}
		response := customFieldPageResponse{Page: customPageMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset, SortField: string(page.SortField), SortOrder: string(page.SortOrder)}}
		for _, value := range page.Definitions {
			response.Fields = append(response.Fields, customFieldFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})
	mux.HandleFunc("POST /api/v1/custom-fields", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request customFieldRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := request.domain()
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.CreateFieldDefinition(r.Context(), actor, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "create custom field", err)
			return
		}
		writeJSON(w, http.StatusCreated, customFieldFromDomain(value))
	})
	mux.HandleFunc("GET /api/v1/custom-fields/{field_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("field_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.GetFieldDefinition(r.Context(), actor, id)
		if err != nil {
			writeCustomError(w, r, logger, "get custom field", err)
			return
		}
		writeJSON(w, http.StatusOK, customFieldFromDomain(value))
	})
	mux.HandleFunc("PUT /api/v1/custom-fields/{field_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("field_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customFieldRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := request.domain()
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.UpdateFieldDefinition(r.Context(), actor, id, request.Version, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "update custom field", err)
			return
		}
		writeJSON(w, http.StatusOK, customFieldFromDomain(value))
	})
	mux.HandleFunc("DELETE /api/v1/custom-fields/{field_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseCustomID(r.PathValue("field_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customDeleteRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.DeleteFieldDefinition(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeCustomError(w, r, logger, "delete custom field", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/custom-fields/{field_id}/options", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		fieldID, problem := parseCustomID(r.PathValue("field_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		items, err := service.ListOptions(r.Context(), actor, fieldID)
		if err != nil {
			writeCustomError(w, r, logger, "list custom options", err)
			return
		}
		response := customOptionPageResponse{}
		for _, value := range items {
			response.Options = append(response.Options, customOptionFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})
	mux.HandleFunc("POST /api/v1/custom-fields/{field_id}/options", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		fieldID, problem := parseCustomID(r.PathValue("field_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customOptionRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.CreateOption(r.Context(), actor, fieldID, request.domain(), requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "create custom option", err)
			return
		}
		writeJSON(w, http.StatusCreated, customOptionFromDomain(value))
	})
	mux.HandleFunc("PUT /api/v1/custom-fields/{field_id}/options/{option_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		fieldID, problem := parseCustomID(r.PathValue("field_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		optionID, problem := parseCustomID(r.PathValue("option_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customOptionRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.UpdateOption(r.Context(), actor, fieldID, optionID, request.Version, request.domain(), requestIDFromContext(r.Context()))
		if err != nil {
			writeCustomError(w, r, logger, "update custom option", err)
			return
		}
		writeJSON(w, http.StatusOK, customOptionFromDomain(value))
	})
	mux.HandleFunc("DELETE /api/v1/custom-fields/{field_id}/options/{option_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := customActor(w, r, authentication, service)
		if !ok {
			return
		}
		fieldID, problem := parseCustomID(r.PathValue("field_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		optionID, problem := parseCustomID(r.PathValue("option_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request customDeleteRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.DeleteOption(r.Context(), actor, fieldID, optionID, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeCustomError(w, r, logger, "delete custom option", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

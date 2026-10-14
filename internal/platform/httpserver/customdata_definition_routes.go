package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
)

func registerCustomDataDefinitionRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service customDataService) {
	mux.HandleFunc("GET /api/v1/custom-fields", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
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
	}))
	mux.HandleFunc("GET /api/v1/custom-fields/{field_id}/options", requireCapability(auth.CapCustomData, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
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
	}))
}

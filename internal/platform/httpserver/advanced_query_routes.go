package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

type advancedQueryService interface {
	CatalogV2(context.Context, auth.Session, string) (querydomain.Catalog, error)
	ValidateV2(context.Context, auth.Session, querydomain.QueryPlan, string) (querydomain.PlanEstimate, error)
	ExecuteV2(context.Context, auth.Session, querydomain.QueryPlan, string, string) (querydomain.Execution, error)
	ResultV2(context.Context, auth.Session, querydomain.Identifier, int, int, string) (querydomain.ResultPage, error)
}

func registerAdvancedQueryRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service queryService) {
	advanced, ok := service.(advancedQueryService)
	if !ok || advanced == nil {
		return
	}
	mux.HandleFunc("GET /api/v1/query/v2/catalog", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := queryActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, err := advanced.CatalogV2(r.Context(), actor, requestIDFromContext(r.Context()))
		if err != nil {
			writeQueryError(w, r, logger, "read Query v2 catalog", err)
			return
		}
		writeJSON(w, http.StatusOK, value)
	})
	mux.HandleFunc("POST /api/v1/query/v2/validate", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := queryActor(w, r, authentication, service)
		if !ok {
			return
		}
		var plan querydomain.QueryPlan
		if problem := DecodeJSON(w, r, &plan); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := advanced.ValidateV2(r.Context(), actor, plan, requestIDFromContext(r.Context()))
		if err != nil {
			writeQueryError(w, r, logger, "validate Query v2 plan", err)
			return
		}
		writeJSON(w, http.StatusOK, value)
	})
	mux.HandleFunc("POST /api/v1/query/v2/executions", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := queryActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request queryExecuteRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := advanced.ExecuteV2(r.Context(), actor, request.Plan, request.IdempotencyKey, requestIDFromContext(r.Context()))
		if err != nil {
			writeQueryError(w, r, logger, "execute Query v2 plan", err)
			return
		}
		writeJSON(w, http.StatusOK, queryExecutionFromDomain(value))
	})
	mux.HandleFunc("GET /api/v1/query/v2/executions/{execution_id}/result", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := queryActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, err := querydomain.ParseIdentifier(r.PathValue("execution_id"))
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador de consulta inválido"})
			return
		}
		limit, offset, problem := queryPagination(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := advanced.ResultV2(r.Context(), actor, id, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			writeQueryError(w, r, logger, "read Query v2 result", err)
			return
		}
		writeJSON(w, http.StatusOK, queryResultPageFromDomain(value))
	})
}

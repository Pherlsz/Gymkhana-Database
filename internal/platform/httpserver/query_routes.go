package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

type queryService interface {
	Catalog(context.Context, auth.Session, string) (querydomain.Catalog, error)
	Validate(context.Context, auth.Session, querydomain.QueryPlan, string) (querydomain.PlanEstimate, error)
	Execute(context.Context, auth.Session, querydomain.QueryPlan, string, string) (querydomain.Execution, error)
	Result(context.Context, auth.Session, querydomain.Identifier, int, int, string) (querydomain.ResultPage, error)
}

type queryExecuteRequest struct {
	IdempotencyKey string                `json:"idempotency_key"`
	Plan           querydomain.QueryPlan `json:"plan"`
}

type queryExecutionResponse struct {
	ID             string                     `json:"id"`
	State          querydomain.ExecutionState `json:"state"`
	CatalogVersion string                     `json:"catalog_version"`
	RootEntity     string                     `json:"root_entity"`
	MaximumRows    int                        `json:"maximum_rows"`
	RowCount       int                        `json:"row_count"`
	ColumnCount    int                        `json:"column_count"`
	ErrorCode      string                     `json:"error_code,omitempty"`
	StartedAt      time.Time                  `json:"started_at"`
	CompletedAt    *time.Time                 `json:"completed_at,omitempty"`
	ExpiresAt      time.Time                  `json:"expires_at"`
	Version        int64                      `json:"version"`
}

type queryResultCellResponse struct {
	ColumnPosition int                   `json:"column_position"`
	Kind           querydomain.ValueKind `json:"kind"`
	IsNull         bool                  `json:"is_null"`
	TextValue      *string               `json:"text_value,omitempty"`
	IntegerValue   *int64                `json:"integer_value,omitempty"`
	DecimalValue   *string               `json:"decimal_value,omitempty"`
	BooleanValue   *bool                 `json:"boolean_value,omitempty"`
	CivilDateValue *string               `json:"civil_date_value,omitempty"`
	TimestampValue *time.Time            `json:"timestamp_value,omitempty"`
}

type queryResultRowResponse struct {
	Position    int                       `json:"position"`
	EntityKind  string                    `json:"entity_kind"`
	EntityID    string                    `json:"entity_id"`
	EntityLabel string                    `json:"entity_label"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	Cells       []queryResultCellResponse `json:"cells"`
}

type queryResultPageResponse struct {
	Execution queryExecutionResponse     `json:"execution"`
	Columns   []querydomain.ResultColumn `json:"columns"`
	Rows      []queryResultRowResponse   `json:"rows"`
	Total     int                        `json:"total"`
	Limit     int                        `json:"limit"`
	Offset    int                        `json:"offset"`
}

func registerQueryRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service queryService) {
	mux.HandleFunc("GET /api/v1/query/catalog", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := queryActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, err := service.Catalog(r.Context(), actor, requestIDFromContext(r.Context()))
		if err != nil {
			writeQueryError(w, r, logger, "read Query catalog", err)
			return
		}
		writeJSON(w, http.StatusOK, value)
	})

	mux.HandleFunc("POST /api/v1/query/validate", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := queryActor(w, r, authentication, service)
		if !ok {
			return
		}
		var plan querydomain.QueryPlan
		if problem := DecodeJSON(w, r, &plan); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Validate(r.Context(), actor, plan, requestIDFromContext(r.Context()))
		if err != nil {
			writeQueryError(w, r, logger, "validate Query plan", err)
			return
		}
		writeJSON(w, http.StatusOK, value)
	})

	mux.HandleFunc("POST /api/v1/query/executions", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := queryActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request queryExecuteRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Execute(r.Context(), actor, request.Plan, request.IdempotencyKey, requestIDFromContext(r.Context()))
		if err != nil {
			writeQueryError(w, r, logger, "execute Query plan", err)
			return
		}
		writeJSON(w, http.StatusOK, queryExecutionFromDomain(value))
	})

	mux.HandleFunc("GET /api/v1/query/executions/{execution_id}/result", func(w http.ResponseWriter, r *http.Request) {
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
		value, err := service.Result(r.Context(), actor, id, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			writeQueryError(w, r, logger, "read Query result", err)
			return
		}
		writeJSON(w, http.StatusOK, queryResultPageFromDomain(value))
	})
}

func queryActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service queryService) (auth.Session, bool) {
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	if service == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O construtor de consultas não está configurado"})
		return auth.Session{}, false
	}
	return actor, true
}

func queryPagination(r *http.Request) (int, int, *Problem) {
	limit, offset := querydomain.MaximumPageSize, 0
	for key, destination := range map[string]*int{"limit": &limit, "offset": &offset} {
		raw := strings.TrimSpace(r.URL.Query().Get(key))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação inválida"}
		}
		*destination = value
	}
	if limit < 1 || limit > querydomain.MaximumPageSize || offset < 0 || offset > querydomain.MaximumRows {
		return 0, 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação fora dos limites"}
	}
	return limit, offset, nil
}

func queryExecutionFromDomain(value querydomain.Execution) queryExecutionResponse {
	return queryExecutionResponse{
		ID: value.ID.String(), State: value.State, CatalogVersion: value.CatalogVersion,
		RootEntity: value.RootEntity, MaximumRows: value.MaximumRows, RowCount: value.RowCount,
		ColumnCount: value.ColumnCount, ErrorCode: value.ErrorCode, StartedAt: value.StartedAt,
		CompletedAt: value.CompletedAt, ExpiresAt: value.ExpiresAt, Version: value.Version,
	}
}

func queryResultPageFromDomain(value querydomain.ResultPage) queryResultPageResponse {
	response := queryResultPageResponse{
		Execution: queryExecutionFromDomain(value.Execution), Columns: value.Columns,
		Rows: make([]queryResultRowResponse, 0, len(value.Rows)), Total: value.Total, Limit: value.Limit, Offset: value.Offset,
	}
	for _, row := range value.Rows {
		converted := queryResultRowResponse{Position: row.Position, EntityKind: row.EntityKind, EntityID: row.EntityID,
			EntityLabel: row.EntityLabel, UpdatedAt: row.UpdatedAt, Cells: make([]queryResultCellResponse, 0, len(row.Cells))}
		for _, cell := range row.Cells {
			converted.Cells = append(converted.Cells, queryResultCellResponse{
				ColumnPosition: cell.ColumnPosition, Kind: cell.Kind, IsNull: cell.IsNull,
				TextValue: cell.TextValue, IntegerValue: cell.IntegerValue, DecimalValue: cell.DecimalValue,
				BooleanValue: cell.BooleanValue, CivilDateValue: cell.CivilDateValue, TimestampValue: cell.TimestampValue,
			})
		}
		response.Rows = append(response.Rows, converted)
	}
	return response
}

func writeQueryError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	var validation *querydomain.ValidationError
	switch {
	case errors.As(err, &validation):
		fields := make([]FieldProblem, 0, len(validation.Fields))
		for _, field := range validation.Fields {
			fields = append(fields, FieldProblem{Field: field.Field, Code: field.Code, Message: queryFieldMessage(field.Code)})
		}
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "Revise o plano da consulta", FieldErrors: fields})
	case errors.Is(err, querydomain.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para consultar estes dados"})
	case errors.Is(err, querydomain.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Consulta não encontrada"})
	case errors.Is(err, querydomain.ErrExpired):
		writeProblem(w, r, Problem{Status: http.StatusGone, Code: ErrorCodeQueryExpired, Message: "O resultado expirou. Execute a consulta novamente"})
	case errors.Is(err, querydomain.ErrStaleCatalog):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeQueryCatalogStale, Message: "O catálogo mudou. Revise o plano antes de executar"})
	case errors.Is(err, querydomain.ErrCancelled):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeQueryCancelled, Message: "A consulta foi cancelada antes de produzir um resultado"})
	case errors.Is(err, querydomain.ErrConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A consulta está em execução ou conflita com outra solicitação"})
	case errors.Is(err, querydomain.ErrRateLimited):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeRateLimited, Message: "Limite de consultas atingido. Aguarde antes de tentar novamente"})
	case errors.Is(err, querydomain.ErrCostLimit):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeQueryTooCostly, Message: "O plano excede o limite seguro. Reduza filtros, relações, colunas ou linhas"})
	case errors.Is(err, querydomain.ErrTimeout):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeQueryTimeout, Message: "A consulta excedeu o tempo seguro de execução"})
	case errors.Is(err, querydomain.ErrInvalidPlan):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O plano ou a chave de idempotência é inválido"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a consulta"})
	}
}

func queryFieldMessage(code string) string {
	switch code {
	case "required":
		return "Informe ao menos uma coluna"
	case "too_many":
		return "Há itens demais neste campo"
	case "too_deep", "relation_too_deep":
		return "A combinação de filtros excede a profundidade permitida"
	case "wrong_arity":
		return "A quantidade de valores não corresponde ao operador"
	case "invalid_type":
		return "O valor não corresponde ao tipo do campo"
	case "duplicate":
		return "O item está repetido"
	case "entity_mismatch":
		return "O campo não pertence à entidade principal"
	case "out_of_range":
		return "O valor está fora do intervalo permitido"
	case "invalid_shape":
		return "A estrutura deste filtro é inválida"
	default:
		return "O identificador lógico ou valor não é suportado"
	}
}

package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	searchdomain "github.com/Pherlsz/Gymkhana-Database/internal/search"
)

type searchService interface {
	Catalog(context.Context, auth.Session) (searchdomain.Catalog, error)
	Search(context.Context, auth.Session, searchdomain.Query) (searchdomain.Page, error)
	MatchIDs(context.Context, auth.Session, searchdomain.Query, searchdomain.Module) ([]string, error)
	MatchProfileHits(context.Context, auth.Session, searchdomain.Query) ([]searchdomain.ProfileHit, error)
	Suggest(context.Context, auth.Session, searchdomain.SuggestQuery) ([]searchdomain.SuggestHit, error)
}

type searchRequest struct {
	Q       string                 `json:"q,omitempty"`
	Terms   []string               `json:"terms"`
	Modules []searchdomain.Module  `json:"modules,omitempty"`
	Fields  []string               `json:"fields,omitempty"`
	Limit   int32                  `json:"limit,omitempty"`
	Offset  int32                  `json:"offset,omitempty"`
	Sort    searchdomain.SortField `json:"sort,omitempty"`
	Order   searchdomain.SortOrder `json:"order,omitempty"`
}

type searchCatalogModuleResponse struct {
	Key   searchdomain.Module `json:"key"`
	Label string              `json:"label"`
}

type searchCatalogFieldResponse struct {
	Key    string              `json:"key"`
	Module searchdomain.Module `json:"module"`
	Group  searchdomain.Module `json:"group"`
	Label  string              `json:"label"`
	Kind   string              `json:"kind"`
}

type searchCatalogLimitsResponse struct {
	MaximumTerms             int   `json:"maximum_terms"`
	MaximumTermLength        int   `json:"maximum_term_length"`
	MaximumFields            int   `json:"maximum_fields"`
	MaximumPageSize          int32 `json:"maximum_page_size"`
	MaximumOffset            int32 `json:"maximum_offset"`
	MaximumResultCardinality int64 `json:"maximum_result_cardinality"`
}

type searchCatalogResponse struct {
	Modules   []searchCatalogModuleResponse `json:"modules"`
	Fields    []searchCatalogFieldResponse  `json:"fields"`
	Operators []searchOperatorResponse      `json:"operators"`
	Limits    searchCatalogLimitsResponse   `json:"limits"`
}

type searchOperatorResponse struct {
	Token       string `json:"token"`
	Insert      string `json:"insert"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

type searchSuggestResponse struct {
	Suggestions []searchSuggestHitResponse `json:"suggestions"`
}

type searchSuggestHitResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type searchResultResponse struct {
	Module      searchdomain.Module `json:"module"`
	EntityKind  string              `json:"entity_kind"`
	EntityID    string              `json:"entity_id"`
	ProfileID   string              `json:"profile_id,omitempty"`
	TargetKind  string              `json:"target_kind"`
	TargetID    string              `json:"target_id"`
	EntityLabel string              `json:"entity_label"`
	FieldKey    string              `json:"field_key"`
	FieldLabel  string              `json:"field_label"`
	Preview     string              `json:"preview"`
	Score       int32               `json:"score"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type searchPageMetaResponse struct {
	Total     int64                  `json:"total"`
	Limit     int32                  `json:"limit"`
	Offset    int32                  `json:"offset"`
	Sort      searchdomain.SortField `json:"sort"`
	SortOrder searchdomain.SortOrder `json:"sort_order"`
}

type searchPageResponse struct {
	Results []searchResultResponse `json:"results"`
	Page    searchPageMetaResponse `json:"page"`
}

func registerSearchRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service searchService) {
	mux.HandleFunc("GET /api/v1/search/catalog", requireCapability(auth.CapSearch, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de busca não está configurado"})
			return
		}
		catalog, err := service.Catalog(r.Context(), actor)
		if err != nil {
			writeSearchError(w, r, logger, "read Search catalog", err)
			return
		}
		writeJSON(w, http.StatusOK, searchCatalogFromDomain(catalog))
	}))

	mux.HandleFunc("POST /api/v1/search", requireCapability(auth.CapSearch, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de busca não está configurado"})
			return
		}
		var request searchRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.Search(r.Context(), actor, searchdomain.Query{
			Q: request.Q, Terms: request.Terms, Modules: request.Modules, Fields: request.Fields,
			Limit: request.Limit, Offset: request.Offset, Sort: request.Sort, Order: request.Order,
		})
		if err != nil {
			writeSearchError(w, r, logger, "execute Search", err)
			return
		}
		writeJSON(w, http.StatusOK, searchPageFromDomain(page))
	}))

	mux.HandleFunc("GET /api/v1/search/suggest", requireCapability(auth.CapSearch, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de busca não está configurado"})
			return
		}
		limit, parseProblem := parseBoundedInt32(r.URL.Query().Get("limit"), 50, 1, 50)
		if parseProblem != nil {
			writeProblem(w, r, *parseProblem)
			return
		}
		hits, err := service.Suggest(r.Context(), actor, searchdomain.SuggestQuery{
			Field: r.URL.Query().Get("field"),
			Q:     r.URL.Query().Get("q"),
			Limit: limit,
			Grain: searchdomain.Module(r.URL.Query().Get("grain")),
		})
		if err != nil {
			writeSearchError(w, r, logger, "suggest Search values", err)
			return
		}
		response := searchSuggestResponse{Suggestions: make([]searchSuggestHitResponse, 0, len(hits))}
		for _, hit := range hits {
			response.Suggestions = append(response.Suggestions, searchSuggestHitResponse{Value: hit.Value, Label: hit.Label})
		}
		writeJSON(w, http.StatusOK, response)
	}))
}

func searchCatalogFromDomain(catalog searchdomain.Catalog) searchCatalogResponse {
	response := searchCatalogResponse{
		Modules:   make([]searchCatalogModuleResponse, 0, len(catalog.Modules)),
		Fields:    make([]searchCatalogFieldResponse, 0, len(catalog.Fields)),
		Operators: make([]searchOperatorResponse, 0, len(catalog.Operators)),
		Limits: searchCatalogLimitsResponse{
			MaximumTerms: catalog.Limits.MaximumTerms, MaximumTermLength: catalog.Limits.MaximumTermLength,
			MaximumFields: catalog.Limits.MaximumFields, MaximumPageSize: catalog.Limits.MaximumPageSize,
			MaximumOffset: catalog.Limits.MaximumOffset, MaximumResultCardinality: catalog.Limits.MaximumResultCardinality,
		},
	}
	for _, module := range catalog.Modules {
		response.Modules = append(response.Modules, searchCatalogModuleResponse{Key: module.Key, Label: module.Label})
	}
	for _, field := range catalog.Fields {
		group := field.Group
		if group == "" {
			group = field.Module
		}
		response.Fields = append(response.Fields, searchCatalogFieldResponse{
			Key: field.Key, Module: field.Module, Group: group, Label: field.Label, Kind: field.Kind,
		})
	}
	for _, operator := range catalog.Operators {
		response.Operators = append(response.Operators, searchOperatorResponse{
			Token: operator.Token, Insert: operator.Insert, Kind: string(operator.Kind), Description: operator.Description,
		})
	}
	return response
}

func searchPageFromDomain(page searchdomain.Page) searchPageResponse {
	response := searchPageResponse{
		Results: make([]searchResultResponse, 0, len(page.Results)),
		Page:    searchPageMetaResponse{Total: page.Total, Limit: page.Limit, Offset: page.Offset, Sort: page.Sort, SortOrder: page.Order},
	}
	for _, result := range page.Results {
		response.Results = append(response.Results, searchResultResponse{
			Module: result.Module, EntityKind: result.EntityKind, EntityID: result.EntityID,
			ProfileID: result.ProfileID, TargetKind: result.TargetKind, TargetID: result.TargetID,
			EntityLabel: result.EntityLabel, FieldKey: result.FieldKey, FieldLabel: result.FieldLabel,
			Preview: result.Preview, Score: result.Score, UpdatedAt: result.UpdatedAt,
		})
	}
	return response
}

func writeSearchError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	var validation *searchdomain.ValidationError
	switch {
	case errors.As(err, &validation):
		fields := make([]FieldProblem, 0, len(validation.Fields))
		for _, field := range validation.Fields {
			message := searchFieldMessage(field.Code)
			if field.Detail != "" {
				message = message + ": " + field.Detail
			}
			fields = append(fields, FieldProblem{Field: field.Field, Code: field.Code, Message: message})
		}
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "Revise os parâmetros da busca", FieldErrors: fields})
	case errors.Is(err, searchdomain.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para buscar estes dados"})
	case errors.Is(err, searchdomain.ErrRateLimited):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeRateLimited, Message: "Limite de buscas atingido. Aguarde antes de tentar novamente"})
	case errors.Is(err, searchdomain.ErrCostLimit):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeQueryTooCostly, Message: "A busca excede o limite seguro. Reduza campos, termos ou página"})
	case errors.Is(err, searchdomain.ErrCardinalityLimit):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeResultSetTooLarge, Message: "A busca encontrou candidatos demais. Use filtros mais específicos"})
	case errors.Is(err, searchdomain.ErrQueryTimeout):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeSearchTimeout, Message: "A busca excedeu o tempo seguro de execução"})
	case errors.Is(err, searchdomain.ErrInvalidQuery):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "A busca é inválida"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível executar a busca"})
	}
}

func searchFieldMessage(code string) string {
	switch code {
	case "required":
		return "Informe ao menos um termo"
	case "unknown_field":
		return "Campo desconhecido. Use o menu / para ver os operadores em português"
	case "physical_name":
		return "Use o nome lógico do campo, não o nome da tabela"
	case "unknown_module":
		return "Módulo desconhecido. Use em:pessoas, em:documentos, em:contas ou em:anexos"
	case "unclosed_quote":
		return "Aspas sem fechamento"
	case "invalid_syntax":
		return "A consulta não pôde ser lida"
	case "empty":
		return "Termos vazios não são permitidos"
	case "too_many":
		return "Há itens demais neste campo"
	case "too_long":
		return "O termo excede o tamanho permitido"
	case "invalid_characters":
		return "O termo contém caracteres de controle"
	case "out_of_range":
		return "O valor está fora do intervalo permitido"
	case "module_mismatch":
		return "O campo não pertence aos módulos selecionados"
	case "selection_required":
		return "Selecione até o limite permitido de campos"
	default:
		return "O identificador lógico não é suportado"
	}
}

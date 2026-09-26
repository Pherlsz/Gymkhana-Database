package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	searchdomain "github.com/Pherlsz/Gymkhana-Database/internal/search"
)

func applySearchQ(r *http.Request, actor auth.Session, service searchService, grain searchdomain.Module) (bool, []string, *Problem) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		return false, nil, nil
	}
	if service == nil {
		return false, nil, &Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de busca não está configurado"}
	}
	matched, err := service.MatchIDs(r.Context(), actor, searchdomain.Query{Q: q}, grain)
	if err != nil {
		return false, nil, searchListProblem(err)
	}
	return true, matched, nil
}

func searchListProblem(err error) *Problem {
	var validation *searchdomain.ValidationError
	if errors.As(err, &validation) {
		fields := make([]FieldProblem, 0, len(validation.Fields))
		for _, field := range validation.Fields {
			message := searchFieldMessage(field.Code)
			if field.Detail != "" {
				message = message + ": " + field.Detail
			}
			fields = append(fields, FieldProblem{Field: field.Field, Code: field.Code, Message: message})
		}
		return &Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "Revise os parâmetros da busca", FieldErrors: fields}
	}
	switch {
	case errors.Is(err, searchdomain.ErrForbidden):
		return &Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para buscar estes dados"}
	case errors.Is(err, searchdomain.ErrCostLimit), errors.Is(err, searchdomain.ErrCardinalityLimit):
		return &Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeQueryTooCostly, Message: "A busca ficou ampla demais"}
	case errors.Is(err, searchdomain.ErrQueryTimeout):
		return &Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeSearchTimeout, Message: "A busca excedeu o tempo seguro de execução"}
	default:
		return &Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível executar a busca"}
	}
}

func profileIDsFromSearch(ids []string) []profile.Identifier {
	out := make([]profile.Identifier, 0, len(ids))
	for _, value := range ids {
		id, err := profile.ParseIdentifier(value)
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

func documentIDsFromSearch(ids []string) []document.Identifier {
	out := make([]document.Identifier, 0, len(ids))
	for _, value := range ids {
		id, err := document.ParseIdentifier(value)
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

func billIDsFromSearch(ids []string) []bill.Identifier {
	out := make([]bill.Identifier, 0, len(ids))
	for _, value := range ids {
		id, err := bill.ParseIdentifier(value)
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

package httpserver

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
)

func writeCustomError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	var validation *customdata.ValidationError
	switch {
	case errors.As(err, &validation):
		fields := make([]FieldProblem, 0, len(validation.Fields))
		for _, field := range validation.Fields {
			fields = append(fields, FieldProblem{Field: field.Field, Code: field.Code, Message: "Custom data field is invalid"})
		}
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "Custom data validation failed", FieldErrors: fields})
	case errors.Is(err, customdata.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "This operation is not allowed"})
	case errors.Is(err, customdata.ErrNotFound), errors.Is(err, customdata.ErrReferenceNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Custom data resource was not found"})
	case errors.Is(err, customdata.ErrConflict), errors.Is(err, customdata.ErrTechnicalKeyConflict), errors.Is(err, customdata.ErrTechnicalKeyImmutable), errors.Is(err, customdata.ErrDefinitionInUse), errors.Is(err, customdata.ErrDefinitionChangeUnsafe), errors.Is(err, customdata.ErrOptionInUse), errors.Is(err, customdata.ErrEntityTypeInUse), errors.Is(err, customdata.ErrEntityTypeInactive), errors.Is(err, customdata.ErrDefinitionInactive), errors.Is(err, customdata.ErrCardinalityConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Custom data changed or conflicts with existing data"})
	case errors.Is(err, customdata.ErrInvalidConfirmation), errors.Is(err, customdata.ErrInvalidTarget), errors.Is(err, customdata.ErrInvalidListOptions):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Custom data request is invalid"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Custom data operation failed"})
	}
}

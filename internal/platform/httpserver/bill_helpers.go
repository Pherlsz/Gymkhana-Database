package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

func billActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service billService) (auth.Session, bool) {
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	if service == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de contas/comprovantes não está configurado"})
		return auth.Session{}, false
	}
	return actor, true
}

func (request billTypeValuesRequest) domainValues() bill.TypeValues {
	return bill.TypeValues{TechnicalKey: request.TechnicalKey, Label: request.Label, Active: request.Active, SupportsCurrentUse: request.SupportsCurrentUse}
}
func (request updateBillTypeRequest) domainValues() bill.TypeValues {
	return billTypeValuesRequest{TechnicalKey: request.TechnicalKey, Label: request.Label, Active: request.Active, SupportsCurrentUse: request.SupportsCurrentUse}.domainValues()
}
func (request billValuesRequest) domainValues() (bill.Values, *Problem) {
	owner, err := profile.ParseIdentifier(request.OwnerProfileID)
	if err != nil {
		return bill.Values{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador da pessoa proprietária é inválido"}
	}
	typeID, err := bill.ParseIdentifier(request.BillTypeID)
	if err != nil {
		return bill.Values{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador do tipo de conta/comprovante é inválido"}
	}
	return bill.Values{OwnerProfileID: owner, TypeID: typeID, PrintedHolderName: request.PrintedHolderName, PrintedAddress: request.PrintedAddress, Reference: request.Reference, Competence: request.Competence, Amount: request.Amount, Currency: request.Currency, Notes: request.Notes, RecordState: request.RecordState}, nil
}
func (request updateBillRequest) domainValues() (bill.Values, *Problem) {
	return billValuesRequest{OwnerProfileID: request.OwnerProfileID, BillTypeID: request.BillTypeID, PrintedHolderName: request.PrintedHolderName, PrintedAddress: request.PrintedAddress, Reference: request.Reference, Competence: request.Competence, Amount: request.Amount, Currency: request.Currency, Notes: request.Notes, RecordState: request.RecordState}.domainValues()
}
func billTypeFromDomain(value bill.TypeDefinition) billTypeResponse {
	return billTypeResponse{ID: value.ID.String(), TechnicalKey: value.Values.TechnicalKey, Label: value.Values.Label, Active: value.Values.Active, SupportsCurrentUse: value.Values.SupportsCurrentUse, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func billFromDomain(value bill.Bill) billResponse {
	response := billResponse{ID: value.ID.String(), OwnerProfileID: value.Values.OwnerProfileID.String(), BillTypeID: value.Values.TypeID.String(), PrintedHolderName: value.Values.PrintedHolderName, PrintedAddress: value.Values.PrintedAddress, Reference: value.Values.Reference, Competence: value.Values.Competence, Amount: value.Values.Amount, Currency: value.Values.Currency, Notes: value.Values.Notes, RecordState: value.Values.RecordState, Status: value.Status, Type: billTypeFromDomain(value.Type), Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if value.CurrentUse != nil {
		response.CurrentUse = &billCurrentUseResponse{HolderProfileID: value.CurrentUse.HolderProfileID.String(), AssignedAt: value.CurrentUse.AssignedAt}
	}
	return response
}

func parseBillIdentifier(value, message string) (bill.Identifier, *Problem) {
	id, err := bill.ParseIdentifier(value)
	if err != nil {
		return bill.Identifier{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: message}
	}
	return id, nil
}

func parseOptionalBillBool(value string) (*bool, *Problem) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O filtro de estado ativo é inválido"}
	}
	return &parsed, nil
}

func billListOptionsFromRequest(r *http.Request) (bill.ListOptions, *Problem) {
	limit, problem := parseBoundedInt32(r.URL.Query().Get("limit"), 100, 1, 1000)
	if problem != nil {
		return bill.ListOptions{}, problem
	}
	offset, problem := parseBoundedInt32(r.URL.Query().Get("offset"), 0, 0, 1_000_000)
	if problem != nil {
		return bill.ListOptions{}, problem
	}
	owner, problem := optionalProfileIdentifier(r.URL.Query().Get("owner_profile_id"), "O filtro de pessoa proprietária é inválido")
	if problem != nil {
		return bill.ListOptions{}, problem
	}
	holder, problem := optionalProfileIdentifier(r.URL.Query().Get("holder_profile_id"), "O filtro de pessoa em uso é inválido")
	if problem != nil {
		return bill.ListOptions{}, problem
	}
	typeID, problem := optionalBillIdentifier(r.URL.Query().Get("bill_type_id"))
	if problem != nil {
		return bill.ListOptions{}, problem
	}
	return bill.ListOptions{Limit: limit, Offset: offset, SortField: bill.SortField(r.URL.Query().Get("sort")), SortOrder: bill.SortOrder(r.URL.Query().Get("order")), Filters: bill.Filters{OwnerProfileID: owner, TypeID: typeID, Reference: r.URL.Query().Get("reference"), Competence: r.URL.Query().Get("competence"), RecordState: bill.RecordState(r.URL.Query().Get("record_state")), Status: bill.Status(r.URL.Query().Get("status")), HolderProfileID: holder}}, nil
}

func optionalBillIdentifier(value string) (*bill.Identifier, *Problem) {
	if value == "" {
		return nil, nil
	}
	id, err := bill.ParseIdentifier(value)
	if err != nil {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O filtro de tipo de conta/comprovante é inválido"}
	}
	return &id, nil
}

func writeBillError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	var validation *bill.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "Revise os campos informados", FieldErrors: billFieldProblems(validation)})
	case errors.Is(err, bill.ErrInvalidListOptions), errors.Is(err, bill.ErrInvalidTypeListOptions):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Filtros, ordenação ou paginação são inválidos"})
	case errors.Is(err, bill.ErrInvalidConfirmation):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Digite Confirmar para excluir permanentemente"})
	case errors.Is(err, bill.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para esta operação"})
	case errors.Is(err, bill.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "O registro não foi encontrado"})
	case errors.Is(err, bill.ErrTypeNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "O tipo de conta/comprovante não foi encontrado"})
	case errors.Is(err, bill.ErrReferenceNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Uma pessoa referenciada não foi encontrada"})
	case errors.Is(err, bill.ErrCurrentUseNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "O registro não possui uso atual"})
	case errors.Is(err, bill.ErrConflict), errors.Is(err, bill.ErrTypeConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O registro foi alterado desde o último carregamento"})
	case errors.Is(err, bill.ErrTechnicalKeyConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A chave técnica já está em uso"})
	case errors.Is(err, bill.ErrTechnicalKeyImmutable):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A chave técnica não pode ser alterada"})
	case errors.Is(err, bill.ErrTypeInUse):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O tipo possui contas/comprovantes e suas regras não podem ser alteradas ou excluídas"})
	case errors.Is(err, bill.ErrTypeInactive):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O tipo de conta/comprovante está inativo"})
	case errors.Is(err, bill.ErrCurrentUseUnsupported):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Este tipo de conta/comprovante não permite uso atual"})
	case errors.Is(err, bill.ErrCurrentUseExists):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Devolva o registro antes de excluí-lo"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a operação"})
	}
}

func billFieldProblems(validation *bill.ValidationError) []FieldProblem {
	problems := make([]FieldProblem, 0, len(validation.Fields))
	for _, field := range validation.Fields {
		problems = append(problems, FieldProblem{Field: field.Field, Code: field.Code, Message: billFieldMessage(field.Code)})
	}
	return problems
}

func billFieldMessage(code string) string {
	switch code {
	case "required":
		return "Campo obrigatório"
	case "too_long":
		return "Valor maior que o permitido"
	case "invalid_value":
		return "Valor não permitido"
	default:
		return "Formato inválido"
	}
}

package httpserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

func documentActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service documentService) (auth.Session, bool) {
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	if service == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de documentos não está configurado"})
		return auth.Session{}, false
	}
	return actor, true
}

func (request documentTypeValuesRequest) domainValues() document.TypeValues {
	return document.TypeValues{TechnicalKey: request.TechnicalKey, Label: request.Label, Active: request.Active, UniquenessPolicy: request.UniquenessPolicy, ValidationRegex: request.ValidationRegex, DateRequired: request.DateRequired}
}

func (request updateDocumentTypeRequest) domainValues() document.TypeValues {
	return documentTypeValuesRequest{TechnicalKey: request.TechnicalKey, Label: request.Label, Active: request.Active, UniquenessPolicy: request.UniquenessPolicy, ValidationRegex: request.ValidationRegex, DateRequired: request.DateRequired}.domainValues()
}

func (request documentValuesRequest) domainValues() (document.Values, *Problem) {
	owner, err := profile.ParseIdentifier(request.OwnerProfileID)
	if err != nil {
		return document.Values{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador da pessoa proprietária é inválido"}
	}
	typeID, err := document.ParseIdentifier(request.DocumentTypeID)
	if err != nil {
		return document.Values{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador do tipo de documento é inválido"}
	}
	return document.Values{OwnerProfileID: owner, TypeID: typeID, Identifier: request.Identifier, DocumentDate: request.DocumentDate, ValidUntil: request.ValidUntil, Notes: request.Notes, Medium: request.Medium, IdleCustody: request.IdleCustody}, nil
}

func (request updateDocumentRequest) domainValues() (document.Values, *Problem) {
	return documentValuesRequest{OwnerProfileID: request.OwnerProfileID, DocumentTypeID: request.DocumentTypeID, Identifier: request.Identifier, DocumentDate: request.DocumentDate, ValidUntil: request.ValidUntil, Notes: request.Notes, Medium: request.Medium, IdleCustody: request.IdleCustody}.domainValues()
}

func documentTypeFromDomain(value document.TypeDefinition) documentTypeResponse {
	return documentTypeResponse{ID: value.ID.String(), TechnicalKey: value.Values.TechnicalKey, Label: value.Values.Label, Active: value.Values.Active, UniquenessPolicy: value.Values.UniquenessPolicy, ValidationRegex: value.Values.ValidationRegex, DateRequired: value.Values.DateRequired, Count: value.ExemplarCount, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func documentFromDomain(value document.Document) documentResponse {
	response := documentResponse{ID: value.ID.String(), OwnerProfileID: value.Values.OwnerProfileID.String(), OwnerFullName: value.OwnerFullName, DocumentTypeID: value.Values.TypeID.String(), Identifier: value.Values.Identifier, DocumentDate: value.Values.DocumentDate, ValidUntil: value.Values.ValidUntil, Notes: value.Values.Notes, Medium: value.Values.Medium, IdleCustody: value.Values.IdleCustody, Status: value.Status, Type: documentTypeFromDomain(value.Type), CustomValues: map[string]string{}, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if value.CurrentUse != nil {
		response.CurrentUse = &documentCurrentUseResponse{HolderProfileID: value.CurrentUse.HolderProfileID.String(), HolderFullName: value.CurrentUse.HolderFullName, AssignedAt: value.CurrentUse.AssignedAt}
	}
	return response
}

func parseDocumentIdentifier(value, message string) (document.Identifier, *Problem) {
	id, err := document.ParseIdentifier(value)
	if err != nil {
		return document.Identifier{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: message}
	}
	return id, nil
}

func parseOptionalDocumentBool(value string) (*bool, *Problem) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O filtro de estado ativo é inválido"}
	}
	return &parsed, nil
}

func documentListOptionsFromRequest(r *http.Request) (document.ListOptions, *Problem) {
	limit, problem := parseBoundedInt32(r.URL.Query().Get("limit"), 100, 1, 1000)
	if problem != nil {
		return document.ListOptions{}, problem
	}
	offset, problem := parseBoundedInt32(r.URL.Query().Get("offset"), 0, 0, 1_000_000)
	if problem != nil {
		return document.ListOptions{}, problem
	}
	owner, problem := optionalProfileIdentifier(r.URL.Query().Get("owner_profile_id"), "O filtro de pessoa proprietária é inválido")
	if problem != nil {
		return document.ListOptions{}, problem
	}
	holder, problem := optionalProfileIdentifier(r.URL.Query().Get("holder_profile_id"), "O filtro de pessoa em uso é inválido")
	if problem != nil {
		return document.ListOptions{}, problem
	}
	typeID, problem := optionalDocumentIdentifier(r.URL.Query().Get("document_type_id"))
	if problem != nil {
		return document.ListOptions{}, problem
	}
	return document.ListOptions{Limit: limit, Offset: offset, SortField: document.SortField(r.URL.Query().Get("sort")), SortOrder: document.SortOrder(r.URL.Query().Get("order")), Filters: document.Filters{OwnerProfileID: owner, TypeID: typeID, Identifier: r.URL.Query().Get("identifier"), Medium: document.Medium(r.URL.Query().Get("medium")), Status: document.Status(r.URL.Query().Get("status")), HolderProfileID: holder}}, nil
}

func optionalProfileIdentifier(value, message string) (*profile.Identifier, *Problem) {
	if value == "" {
		return nil, nil
	}
	id, err := profile.ParseIdentifier(value)
	if err != nil {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: message}
	}
	return &id, nil
}

func optionalDocumentIdentifier(value string) (*document.Identifier, *Problem) {
	if value == "" {
		return nil, nil
	}
	id, err := document.ParseIdentifier(value)
	if err != nil {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O filtro de tipo de documento é inválido"}
	}
	return &id, nil
}

func writeDocumentError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	var validation *document.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "Revise os campos informados", FieldErrors: documentFieldProblems(validation)})
	case errors.Is(err, document.ErrInvalidListOptions), errors.Is(err, document.ErrInvalidTypeListOptions):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Filtros, ordenação ou paginação são inválidos"})
	case errors.Is(err, document.ErrInvalidConfirmation):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Digite Confirmar para excluir permanentemente"})
	case errors.Is(err, document.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para esta operação"})
	case errors.Is(err, document.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "O documento não foi encontrado"})
	case errors.Is(err, document.ErrTypeNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "O tipo de documento não foi encontrado"})
	case errors.Is(err, document.ErrReferenceNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Uma pessoa referenciada não foi encontrada"})
	case errors.Is(err, document.ErrCurrentUseNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "O documento não possui uso atual"})
	case errors.Is(err, document.ErrConflict), errors.Is(err, document.ErrTypeConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O registro foi alterado desde o último carregamento"})
	case errors.Is(err, document.ErrTechnicalKeyConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A chave técnica já está em uso"})
	case errors.Is(err, document.ErrUniquenessConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Já existe um documento com esse identificador para a política configurada"})
	case errors.Is(err, document.ErrTechnicalKeyImmutable):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A chave técnica não pode ser alterada"})
	case errors.Is(err, document.ErrTypeInUse):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O tipo possui documentos e suas regras não podem ser alteradas ou excluídas"})
	case errors.Is(err, document.ErrTypeInactive):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O tipo de documento está inativo"})
	case errors.Is(err, document.ErrCurrentUseExists):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Devolva o documento antes de excluí-lo"})
	case errors.Is(err, document.ErrCurrentUseUnsupported):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Somente exemplar físico permite uso atual"})
	case errors.Is(err, document.ErrDuplicateNotSupported):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Já existe um exemplar deste meio para esta pessoa e tipo"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a operação"})
	}
}

func documentFieldProblems(validation *document.ValidationError) []FieldProblem {
	problems := make([]FieldProblem, 0, len(validation.Fields))
	for _, field := range validation.Fields {
		problems = append(problems, FieldProblem{Field: field.Field, Code: field.Code, Message: documentFieldMessage(field.Code)})
	}
	return problems
}

func documentFieldMessage(code string) string {
	switch code {
	case "required":
		return "Campo obrigatório"
	case "too_long":
		return "Valor maior que o permitido"
	case "invalid_value":
		return "Valor não permitido"
	case "refused":
		return "Este valor não é um número de documento"
	case "has_exemplar":
		return "Não é possível declarar ausência ou indicação enquanto houver exemplar"
	case "unexpected":
		return "Este campo não deve ser informado"
	default:
		return "Formato inválido"
	}
}

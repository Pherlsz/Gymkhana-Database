package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type documentService interface {
	ListTypes(context.Context, auth.Session, document.TypeListOptions) (document.TypePage, error)
	GetType(context.Context, auth.Session, document.Identifier) (document.TypeDefinition, error)
	CreateType(context.Context, auth.Session, document.TypeValues, string) (document.TypeDefinition, error)
	UpdateType(context.Context, auth.Session, document.Identifier, int64, document.TypeValues, string) (document.TypeDefinition, error)
	DeleteType(context.Context, auth.Session, document.Identifier, int64, string, string) error
	List(context.Context, auth.Session, document.ListOptions) (document.Page, error)
	Get(context.Context, auth.Session, document.Identifier) (document.Document, error)
	Create(context.Context, auth.Session, document.Values, string) (document.Document, error)
	Update(context.Context, auth.Session, document.Identifier, int64, document.Values, string) (document.Document, error)
	Duplicate(context.Context, auth.Session, document.Identifier, string) (document.Document, error)
	Delete(context.Context, auth.Session, document.Identifier, int64, string, string) error
	AssignCurrentUse(context.Context, auth.Session, document.Identifier, profile.Identifier, string) (document.CurrentUse, error)
	ReturnCurrentUse(context.Context, auth.Session, document.Identifier, string) error
}

type documentTypeValuesRequest struct {
	TechnicalKey     string                    `json:"technical_key"`
	Label            string                    `json:"label"`
	Active           bool                      `json:"active"`
	UniquenessPolicy document.UniquenessPolicy `json:"uniqueness_policy"`
	ValidationRegex  string                    `json:"validation_regex"`
	DateRequired     bool                      `json:"date_required"`
}

type updateDocumentTypeRequest struct {
	TechnicalKey     string                    `json:"technical_key"`
	Label            string                    `json:"label"`
	Active           bool                      `json:"active"`
	UniquenessPolicy document.UniquenessPolicy `json:"uniqueness_policy"`
	ValidationRegex  string                    `json:"validation_regex"`
	DateRequired     bool                      `json:"date_required"`
	Version          int64                     `json:"version"`
}

type documentValuesRequest struct {
	OwnerProfileID string               `json:"owner_profile_id"`
	DocumentTypeID string               `json:"document_type_id"`
	Identifier     string               `json:"identifier_value"`
	DocumentDate   string               `json:"document_date"`
	Notes          string               `json:"notes"`
	RecordState    document.RecordState `json:"record_state"`
}

type updateDocumentRequest struct {
	OwnerProfileID string               `json:"owner_profile_id"`
	DocumentTypeID string               `json:"document_type_id"`
	Identifier     string               `json:"identifier_value"`
	DocumentDate   string               `json:"document_date"`
	Notes          string               `json:"notes"`
	RecordState    document.RecordState `json:"record_state"`
	Version        int64                `json:"version"`
}

type deleteDocumentResourceRequest struct {
	Version      int64  `json:"version"`
	Confirmation string `json:"confirmation"`
}

type assignDocumentCurrentUseRequest struct {
	HolderProfileID string `json:"holder_profile_id"`
}

type documentTypeResponse struct {
	ID               string                    `json:"id"`
	TechnicalKey     string                    `json:"technical_key"`
	Label            string                    `json:"label"`
	Active           bool                      `json:"active"`
	UniquenessPolicy document.UniquenessPolicy `json:"uniqueness_policy"`
	ValidationRegex  string                    `json:"validation_regex"`
	DateRequired     bool                      `json:"date_required"`
	Version          int64                     `json:"version"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

type documentCurrentUseResponse struct {
	HolderProfileID string    `json:"holder_profile_id"`
	AssignedAt      time.Time `json:"assigned_at"`
}

type documentResponse struct {
	ID             string                      `json:"id"`
	OwnerProfileID string                      `json:"owner_profile_id"`
	DocumentTypeID string                      `json:"document_type_id"`
	Identifier     string                      `json:"identifier_value"`
	DocumentDate   string                      `json:"document_date"`
	Notes          string                      `json:"notes"`
	RecordState    document.RecordState        `json:"record_state"`
	Status         document.Status             `json:"status"`
	Type           documentTypeResponse        `json:"type"`
	CurrentUse     *documentCurrentUseResponse `json:"current_use"`
	Version        int64                       `json:"version"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

type documentPageMeta struct {
	Total     int64  `json:"total"`
	Limit     int32  `json:"limit"`
	Offset    int32  `json:"offset"`
	SortField string `json:"sort_field"`
	SortOrder string `json:"sort_order"`
}

type documentTypePageResponse struct {
	Types []documentTypeResponse `json:"types"`
	Page  documentPageMeta       `json:"page"`
}

type documentPageResponse struct {
	Documents []documentResponse `json:"documents"`
	Page      documentPageMeta   `json:"page"`
}

func registerDocumentRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service documentService) {
	mux.HandleFunc("GET /api/v1/document-types", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		limit, problem := parseBoundedInt32(r.URL.Query().Get("limit"), 100, 1, 1000)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		offset, problem := parseBoundedInt32(r.URL.Query().Get("offset"), 0, 0, 1_000_000)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		active, problem := parseOptionalDocumentBool(r.URL.Query().Get("active"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListTypes(r.Context(), actor, document.TypeListOptions{Limit: limit, Offset: offset, SortField: document.TypeSortField(r.URL.Query().Get("sort")), SortOrder: document.SortOrder(r.URL.Query().Get("order")), Filters: document.TypeFilters{Label: r.URL.Query().Get("label"), Active: active}})
		if err != nil {
			writeDocumentError(w, r, logger, "list document types", err)
			return
		}
		response := documentTypePageResponse{Types: make([]documentTypeResponse, 0, len(page.Types)), Page: documentPageMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset, SortField: string(page.SortField), SortOrder: string(page.SortOrder)}}
		for _, value := range page.Types {
			response.Types = append(response.Types, documentTypeFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("POST /api/v1/document-types", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request documentTypeValuesRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		created, err := service.CreateType(r.Context(), actor, request.domainValues(), requestIDFromContext(r.Context()))
		if err != nil {
			writeDocumentError(w, r, logger, "create document type", err)
			return
		}
		writeJSON(w, http.StatusCreated, documentTypeFromDomain(created))
	})

	mux.HandleFunc("GET /api/v1/document-types/{document_type_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_type_id"), "O identificador do tipo de documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.GetType(r.Context(), actor, id)
		if err != nil {
			writeDocumentError(w, r, logger, "get document type", err)
			return
		}
		writeJSON(w, http.StatusOK, documentTypeFromDomain(value))
	})

	mux.HandleFunc("PUT /api/v1/document-types/{document_type_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_type_id"), "O identificador do tipo de documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request updateDocumentTypeRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		updated, err := service.UpdateType(r.Context(), actor, id, request.Version, request.domainValues(), requestIDFromContext(r.Context()))
		if err != nil {
			writeDocumentError(w, r, logger, "update document type", err)
			return
		}
		writeJSON(w, http.StatusOK, documentTypeFromDomain(updated))
	})

	mux.HandleFunc("DELETE /api/v1/document-types/{document_type_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_type_id"), "O identificador do tipo de documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request deleteDocumentResourceRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.DeleteType(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeDocumentError(w, r, logger, "delete document type", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		options, problem := documentListOptionsFromRequest(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.List(r.Context(), actor, options)
		if err != nil {
			writeDocumentError(w, r, logger, "list documents", err)
			return
		}
		response := documentPageResponse{Documents: make([]documentResponse, 0, len(page.Documents)), Page: documentPageMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset, SortField: string(page.SortField), SortOrder: string(page.SortOrder)}}
		for _, value := range page.Documents {
			response.Documents = append(response.Documents, documentFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("POST /api/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request documentValuesRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := request.domainValues()
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		created, err := service.Create(r.Context(), actor, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeDocumentError(w, r, logger, "create document", err)
			return
		}
		writeJSON(w, http.StatusCreated, documentFromDomain(created))
	})

	mux.HandleFunc("GET /api/v1/documents/{document_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_id"), "O identificador do documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Get(r.Context(), actor, id)
		if err != nil {
			writeDocumentError(w, r, logger, "get document", err)
			return
		}
		writeJSON(w, http.StatusOK, documentFromDomain(value))
	})

	mux.HandleFunc("PUT /api/v1/documents/{document_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_id"), "O identificador do documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request updateDocumentRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		values, problem := request.domainValues()
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		updated, err := service.Update(r.Context(), actor, id, request.Version, values, requestIDFromContext(r.Context()))
		if err != nil {
			writeDocumentError(w, r, logger, "update document", err)
			return
		}
		writeJSON(w, http.StatusOK, documentFromDomain(updated))
	})

	mux.HandleFunc("POST /api/v1/documents/{document_id}/duplicate", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_id"), "O identificador do documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		duplicated, err := service.Duplicate(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeDocumentError(w, r, logger, "duplicate document", err)
			return
		}
		writeJSON(w, http.StatusCreated, documentFromDomain(duplicated))
	})

	mux.HandleFunc("DELETE /api/v1/documents/{document_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_id"), "O identificador do documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request deleteDocumentResourceRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.Delete(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeDocumentError(w, r, logger, "delete document", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("PUT /api/v1/documents/{document_id}/current-use", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_id"), "O identificador do documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request assignDocumentCurrentUseRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		holder, err := profile.ParseIdentifier(request.HolderProfileID)
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador da pessoa em uso é inválido"})
			return
		}
		currentUse, err := service.AssignCurrentUse(r.Context(), actor, id, holder, requestIDFromContext(r.Context()))
		if err != nil {
			writeDocumentError(w, r, logger, "assign document current use", err)
			return
		}
		writeJSON(w, http.StatusOK, documentCurrentUseResponse{HolderProfileID: currentUse.HolderProfileID.String(), AssignedAt: currentUse.AssignedAt})
	})

	mux.HandleFunc("DELETE /api/v1/documents/{document_id}/current-use", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := documentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseDocumentIdentifier(r.PathValue("document_id"), "O identificador do documento é inválido")
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.ReturnCurrentUse(r.Context(), actor, id, requestIDFromContext(r.Context())); err != nil {
			writeDocumentError(w, r, logger, "return document current use", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

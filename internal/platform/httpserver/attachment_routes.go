package httpserver

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type attachmentService interface {
	CreateUploadIntent(context.Context, auth.Session, attachment.CreateUploadIntentInput, string) (attachment.UploadGrant, error)
	ConfirmUpload(context.Context, auth.Session, attachment.Identifier, string) (attachment.Attachment, error)
	List(context.Context, auth.Session, attachment.OwnerReference, bool) ([]attachment.Attachment, error)
	Download(context.Context, auth.Session, attachment.Identifier, string) (attachment.SignedRequest, error)
	Trash(context.Context, auth.Session, attachment.Identifier, int64, string, string) (attachment.Attachment, error)
	Restore(context.Context, auth.Session, attachment.Identifier, int64, string) (attachment.Attachment, error)
}

type attachmentOwnerRequest struct {
	OwnerKind         attachment.OwnerKind        `json:"owner_kind"`
	OwnerID           string                      `json:"owner_id"`
	CustomTargetKind  attachment.CustomTargetKind `json:"custom_target_kind,omitempty"`
	FieldDefinitionID string                      `json:"field_definition_id,omitempty"`
}

type attachmentUploadIntentRequest struct {
	attachmentOwnerRequest
	OriginalFileName string `json:"original_filename"`
	DeclaredMIME     string `json:"declared_mime"`
	ExpectedSize     int64  `json:"expected_size"`
}

type attachmentMutationRequest struct {
	Version      int64  `json:"version"`
	Confirmation string `json:"confirmation,omitempty"`
}

type signedRequestResponse struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type attachmentUploadGrantResponse struct {
	IntentID  string                `json:"intent_id"`
	Upload    signedRequestResponse `json:"upload"`
	ExpiresAt time.Time             `json:"expires_at"`
}

type attachmentResponse struct {
	ID                string                      `json:"id"`
	OwnerKind         attachment.OwnerKind        `json:"owner_kind"`
	OwnerID           string                      `json:"owner_id"`
	CustomTargetKind  attachment.CustomTargetKind `json:"custom_target_kind,omitempty"`
	FieldDefinitionID string                      `json:"field_definition_id,omitempty"`
	OriginalFileName  string                      `json:"original_filename"`
	DeclaredMIME      string                      `json:"declared_mime"`
	DetectedMIME      string                      `json:"detected_mime"`
	ByteSize          int64                       `json:"byte_size"`
	SHA256            string                      `json:"sha256"`
	LifecycleState    attachment.LifecycleState   `json:"lifecycle_state"`
	DeletedAt         *time.Time                  `json:"deleted_at,omitempty"`
	PurgeAfter        *time.Time                  `json:"purge_after,omitempty"`
	Version           int64                       `json:"version"`
	CreatedAt         time.Time                   `json:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at"`
}

type attachmentListResponse struct {
	Attachments []attachmentResponse `json:"attachments"`
}

func registerAttachmentRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service attachmentService) {
	mux.HandleFunc("POST /api/v1/attachment-upload-intents", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := attachmentActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request attachmentUploadIntentRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		owner, problem := request.attachmentOwnerRequest.domain()
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		grant, err := service.CreateUploadIntent(r.Context(), actor, attachment.CreateUploadIntentInput{
			Owner: owner, OriginalFileName: request.OriginalFileName, DeclaredMIME: request.DeclaredMIME, ExpectedSize: request.ExpectedSize,
		}, requestIDFromContext(r.Context()))
		if err != nil {
			writeAttachmentError(w, r, logger, "create attachment upload intent", err)
			return
		}
		writeJSON(w, http.StatusCreated, attachmentUploadGrantResponse{
			IntentID: grant.Intent.ID.String(), ExpiresAt: grant.Intent.ExpiresAt,
			Upload: signedRequestResponse{URL: grant.Upload.URL, Method: grant.Upload.Method, Headers: grant.Upload.Headers, ExpiresAt: grant.Upload.ExpiresAt},
		})
	})

	mux.HandleFunc("POST /api/v1/attachment-upload-intents/{intent_id}/confirm", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := attachmentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseAttachmentID(r.PathValue("intent_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.ConfirmUpload(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeAttachmentError(w, r, logger, "confirm attachment upload", err)
			return
		}
		writeJSON(w, http.StatusCreated, attachmentFromDomain(value))
	})

	mux.HandleFunc("GET /api/v1/attachments", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := attachmentActor(w, r, authentication, service)
		if !ok {
			return
		}
		owner, problem := attachmentOwnerFromQuery(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		includeTrashed := false
		if raw := strings.TrimSpace(r.URL.Query().Get("include_trashed")); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Attachment trash filter is invalid"})
				return
			}
			includeTrashed = parsed
		}
		values, err := service.List(r.Context(), actor, owner, includeTrashed)
		if err != nil {
			writeAttachmentError(w, r, logger, "list attachments", err)
			return
		}
		response := attachmentListResponse{Attachments: make([]attachmentResponse, 0, len(values))}
		for _, value := range values {
			response.Attachments = append(response.Attachments, attachmentFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("POST /api/v1/attachments/{attachment_id}/download", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := attachmentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseAttachmentID(r.PathValue("attachment_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		signed, err := service.Download(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeAttachmentError(w, r, logger, "create attachment download", err)
			return
		}
		writeJSON(w, http.StatusOK, signedRequestResponse{URL: signed.URL, Method: signed.Method, Headers: signed.Headers, ExpiresAt: signed.ExpiresAt})
	})

	mux.HandleFunc("DELETE /api/v1/attachments/{attachment_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := attachmentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseAttachmentID(r.PathValue("attachment_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request attachmentMutationRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Trash(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context()))
		if err != nil {
			writeAttachmentError(w, r, logger, "trash attachment", err)
			return
		}
		writeJSON(w, http.StatusOK, attachmentFromDomain(value))
	})

	mux.HandleFunc("POST /api/v1/attachments/{attachment_id}/restore", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := attachmentActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := parseAttachmentID(r.PathValue("attachment_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request attachmentMutationRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Restore(r.Context(), actor, id, request.Version, requestIDFromContext(r.Context()))
		if err != nil {
			writeAttachmentError(w, r, logger, "restore attachment", err)
			return
		}
		writeJSON(w, http.StatusOK, attachmentFromDomain(value))
	})
}

func attachmentActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service attachmentService) (auth.Session, bool) {
	if service == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "Attachments are unavailable"})
		return auth.Session{}, false
	}
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	return actor, true
}

func (request attachmentOwnerRequest) domain() (attachment.OwnerReference, *Problem) {
	id, problem := parseAttachmentID(request.OwnerID)
	if problem != nil {
		return attachment.OwnerReference{}, problem
	}
	owner := attachment.OwnerReference{Kind: request.OwnerKind, ID: id, CustomTargetKind: request.CustomTargetKind}
	if strings.TrimSpace(request.FieldDefinitionID) != "" {
		fieldID, fieldProblem := parseAttachmentID(request.FieldDefinitionID)
		if fieldProblem != nil {
			return attachment.OwnerReference{}, fieldProblem
		}
		owner.FieldDefinitionID = fieldID
	}
	if !owner.Valid() {
		return attachment.OwnerReference{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Attachment owner is invalid"}
	}
	return owner, nil
}

func attachmentOwnerFromQuery(r *http.Request) (attachment.OwnerReference, *Problem) {
	return (attachmentOwnerRequest{
		OwnerKind:         attachment.OwnerKind(strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("owner_kind")))),
		OwnerID:           r.URL.Query().Get("owner_id"),
		CustomTargetKind:  attachment.CustomTargetKind(strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("custom_target_kind")))),
		FieldDefinitionID: r.URL.Query().Get("field_definition_id"),
	}).domain()
}

func parseAttachmentID(value string) (attachment.Identifier, *Problem) {
	id, err := attachment.ParseIdentifier(value)
	if err != nil {
		return attachment.Identifier{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Attachment identifier is invalid"}
	}
	return id, nil
}

func attachmentFromDomain(value attachment.Attachment) attachmentResponse {
	return attachmentResponse{
		ID: value.ID.String(), OwnerKind: value.Owner.Kind, OwnerID: value.Owner.ID.String(),
		CustomTargetKind: value.Owner.CustomTargetKind, FieldDefinitionID: optionalAttachmentID(value.Owner.FieldDefinitionID),
		OriginalFileName: value.OriginalFileName, DeclaredMIME: value.DeclaredMIME, DetectedMIME: value.DetectedMIME,
		ByteSize: value.ByteSize, SHA256: hex.EncodeToString(value.SHA256[:]), LifecycleState: value.LifecycleState,
		DeletedAt: value.DeletedAt, PurgeAfter: value.PurgeAfter, Version: value.Version,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func optionalAttachmentID(value attachment.Identifier) string {
	if value.IsZero() {
		return ""
	}
	return value.String()
}

func writeAttachmentError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	var validation *attachment.ValidationError
	switch {
	case errors.As(err, &validation):
		fields := make([]FieldProblem, 0, len(validation.Fields))
		for _, field := range validation.Fields {
			fields = append(fields, FieldProblem{Field: field.Field, Code: field.Code, Message: "Attachment field is invalid"})
		}
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "Attachment validation failed", FieldErrors: fields})
	case errors.Is(err, attachment.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "This attachment operation is not allowed"})
	case errors.Is(err, attachment.ErrUploadIntentNotFound), errors.Is(err, attachment.ErrUploadObjectNotFound), errors.Is(err, attachment.ErrAttachmentNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Attachment resource was not found"})
	case errors.Is(err, attachment.ErrConflict), errors.Is(err, attachment.ErrUploadIntentConsumed), errors.Is(err, attachment.ErrInvalidState):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Attachment changed or is no longer in the expected state"})
	case errors.Is(err, attachment.ErrUploadIntentExpired):
		writeProblem(w, r, Problem{Status: http.StatusGone, Code: ErrorCodeConflict, Message: "Attachment upload expired"})
	case errors.Is(err, attachment.ErrMIMEMismatch), errors.Is(err, attachment.ErrUnsupportedFile), errors.Is(err, attachment.ErrInvalidMIME), errors.Is(err, attachment.ErrInvalidSize), errors.Is(err, attachment.ErrInvalidOwner), errors.Is(err, attachment.ErrInvalidConfirmation), errors.Is(err, attachment.ErrInvalidInput):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Attachment request or file is invalid"})
	case errors.Is(err, attachment.ErrStorageUnavailable):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "Private file storage is unavailable"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Attachment operation failed"})
	}
}

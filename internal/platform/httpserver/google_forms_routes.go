package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/googleforms"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
)

type googleFormsService interface {
	Enabled() bool
	BeginOAuth(context.Context, auth.Session, string, string) (googleforms.OAuthStart, error)
	CompleteOAuth(context.Context, auth.Session, string, string, string) (googleforms.Connection, string, error)
	RejectOAuth(context.Context, auth.Session, string, string) (string, error)
	GetConnection(context.Context, auth.Session) (googleforms.Connection, error)
	Disconnect(context.Context, auth.Session, int64, string) (googleforms.Connection, error)
	CreateSource(context.Context, auth.Session, googleforms.CreateSourceInput, string) (googleforms.Source, error)
	GetSource(context.Context, auth.Session, googleforms.Identifier) (googleforms.Source, error)
	ListSources(context.Context, auth.Session, int, int) (googleforms.SourcePage, error)
	SaveMapping(context.Context, auth.Session, googleforms.Identifier, int64, []googleforms.MappingInput, string) (googleforms.Source, error)
	UpdateSource(context.Context, auth.Session, googleforms.Identifier, int64, googleforms.UpdateSourceInput, string) (googleforms.Source, error)
	RefreshSource(context.Context, auth.Session, googleforms.Identifier, string) (googleforms.Source, bool, error)
	RequestSync(context.Context, auth.Session, googleforms.Identifier, string, string) (googleforms.SyncRun, error)
	ListSyncRuns(context.Context, auth.Session, *googleforms.Identifier, int, int) (googleforms.SyncRunPage, error)
	CancelSync(context.Context, auth.Session, googleforms.Identifier, int64, string) (googleforms.SyncRun, error)
}

type googleFormsOAuthRequest struct {
	ReturnPath string `json:"return_path,omitempty"`
}

type googleFormsVersionRequest struct {
	Version int64 `json:"version"`
}

type googleFormsCreateSourceRequest struct {
	FormReference string            `json:"form_reference"`
	Module        operations.Module `json:"module"`
}

type googleFormsMappingRequest struct {
	Version int64                      `json:"version"`
	Mapping []googleforms.MappingInput `json:"mapping"`
}

type googleFormsUpdateSourceRequest struct {
	Version             int64                `json:"version"`
	SyncMode            googleforms.SyncMode `json:"sync_mode"`
	PollIntervalSeconds int                  `json:"poll_interval_seconds"`
	Enabled             bool                 `json:"enabled"`
}

type googleFormsSyncRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type googleFormsStatusResponse struct {
	Enabled    bool                           `json:"enabled"`
	Connected  bool                           `json:"connected"`
	Connection *googleFormsConnectionResponse `json:"connection,omitempty"`
}

type googleFormsOAuthResponse struct {
	AuthorizationURL string    `json:"authorization_url"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type googleFormsConnectionResponse struct {
	ID             string                      `json:"id"`
	State          googleforms.ConnectionState `json:"state"`
	GrantedScopes  []string                    `json:"granted_scopes"`
	ErrorCode      string                      `json:"error_code,omitempty"`
	ConnectedAt    *time.Time                  `json:"connected_at,omitempty"`
	DisconnectedAt *time.Time                  `json:"disconnected_at,omitempty"`
	Version        int64                       `json:"version"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

type googleFormsQuestionResponse struct {
	ID              string                 `json:"id"`
	Position        int                    `json:"position"`
	Title           string                 `json:"title"`
	AnswerKind      googleforms.AnswerKind `json:"answer_kind"`
	Required        bool                   `json:"required"`
	Supported       bool                   `json:"supported"`
	UnsupportedCode string                 `json:"unsupported_code,omitempty"`
	TargetField     string                 `json:"target_field,omitempty"`
}

type googleFormsSourceResponse struct {
	ID                  string                        `json:"id"`
	ProviderFormID      string                        `json:"provider_form_id"`
	Title               string                        `json:"title"`
	Module              operations.Module             `json:"module"`
	State               googleforms.SourceState       `json:"state"`
	SchemaRevision      string                        `json:"schema_revision"`
	SyncMode            googleforms.SyncMode          `json:"sync_mode"`
	PollIntervalSeconds int                           `json:"poll_interval_seconds"`
	CursorSubmittedAt   *time.Time                    `json:"cursor_submitted_at,omitempty"`
	PaginationPending   bool                          `json:"pagination_pending"`
	LastSyncedAt        *time.Time                    `json:"last_synced_at,omitempty"`
	NextSyncAt          *time.Time                    `json:"next_sync_at,omitempty"`
	ErrorCode           string                        `json:"error_code,omitempty"`
	Version             int64                         `json:"version"`
	CreatedAt           time.Time                     `json:"created_at"`
	UpdatedAt           time.Time                     `json:"updated_at"`
	Questions           []googleFormsQuestionResponse `json:"questions"`
}

type googleFormsSourcePageResponse struct {
	Sources []googleFormsSourceResponse `json:"sources"`
	Total   int                         `json:"total"`
	Limit   int                         `json:"limit"`
	Offset  int                         `json:"offset"`
}

type googleFormsSyncResponse struct {
	ID                string                  `json:"id"`
	SourceID          string                  `json:"source_id"`
	TriggerKind       googleforms.TriggerKind `json:"trigger_kind"`
	State             googleforms.SyncState   `json:"state"`
	OperationImportID string                  `json:"operation_import_id,omitempty"`
	ReceivedCount     int                     `json:"received_count"`
	StagedCount       int                     `json:"staged_count"`
	DuplicateCount    int                     `json:"duplicate_count"`
	ErrorCode         string                  `json:"error_code,omitempty"`
	StartedAt         *time.Time              `json:"started_at,omitempty"`
	CompletedAt       *time.Time              `json:"completed_at,omitempty"`
	Version           int64                   `json:"version"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

type googleFormsSyncPageResponse struct {
	Runs   []googleFormsSyncResponse `json:"runs"`
	Total  int                       `json:"total"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
}

func registerGoogleFormsRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service googleFormsService, applicationURL string) {
	mux.HandleFunc("GET /api/v1/google-forms/status", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil || !service.Enabled() {
			writeJSON(w, http.StatusOK, googleFormsStatusResponse{Enabled: false})
			return
		}
		connection, err := service.GetConnection(r.Context(), actor)
		if errors.Is(err, googleforms.ErrNotFound) {
			writeJSON(w, http.StatusOK, googleFormsStatusResponse{Enabled: true})
			return
		}
		if err != nil {
			writeGoogleFormsError(w, r, logger, "read Google Forms status", err)
			return
		}
		converted := googleFormsConnectionFromDomain(connection)
		writeJSON(w, http.StatusOK, googleFormsStatusResponse{Enabled: true, Connected: connection.State == googleforms.ConnectionActive, Connection: &converted})
	})

	mux.HandleFunc("POST /api/v1/google-forms/oauth/start", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := googleFormsActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request googleFormsOAuthRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		started, err := service.BeginOAuth(r.Context(), actor, request.ReturnPath, requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "start Google Forms OAuth", err)
			return
		}
		writeJSON(w, http.StatusCreated, googleFormsOAuthResponse{AuthorizationURL: started.AuthorizationURL, ExpiresAt: started.ExpiresAt})
	})

	mux.HandleFunc("GET /api/v1/google-forms/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := googleFormsActor(w, r, authentication, service)
		if !ok {
			return
		}
		state := r.URL.Query().Get("state")
		if r.URL.Query().Get("error") != "" {
			returnPath, err := service.RejectOAuth(r.Context(), actor, state, requestIDFromContext(r.Context()))
			if err != nil {
				writeGoogleFormsError(w, r, logger, "reject Google Forms OAuth", err)
				return
			}
			http.Redirect(w, r, googleFormsRedirect(applicationURL, returnPath, "denied"), http.StatusFound)
			return
		}
		_, returnPath, err := service.CompleteOAuth(r.Context(), actor, state, r.URL.Query().Get("code"), requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "complete Google Forms OAuth", err)
			return
		}
		http.Redirect(w, r, googleFormsRedirect(applicationURL, returnPath, "connected"), http.StatusFound)
	})

	mux.HandleFunc("DELETE /api/v1/google-forms/connection", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := googleFormsActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request googleFormsVersionRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		connection, err := service.Disconnect(r.Context(), actor, request.Version, requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "disconnect Google Forms", err)
			return
		}
		writeJSON(w, http.StatusOK, googleFormsConnectionFromDomain(connection))
	})

	registerGoogleFormsSourceRoutes(mux, logger, authentication, service)
	registerGoogleFormsSyncRoutes(mux, logger, authentication, service)
}

func registerGoogleFormsSourceRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service googleFormsService) {
	mux.HandleFunc("POST /api/v1/google-forms/sources", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := googleFormsActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request googleFormsCreateSourceRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.CreateSource(r.Context(), actor, googleforms.CreateSourceInput{FormReference: request.FormReference, Module: request.Module}, requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "create Google Forms source", err)
			return
		}
		writeJSON(w, http.StatusCreated, googleFormsSourceFromDomain(value))
	})

	mux.HandleFunc("GET /api/v1/google-forms/sources", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := googleFormsActor(w, r, authentication, service)
		if !ok {
			return
		}
		limit, offset, problem := googleFormsPagination(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListSources(r.Context(), actor, limit, offset)
		if err != nil {
			writeGoogleFormsError(w, r, logger, "list Google Forms sources", err)
			return
		}
		response := googleFormsSourcePageResponse{Total: page.Total, Limit: limit, Offset: offset, Sources: make([]googleFormsSourceResponse, 0, len(page.Sources))}
		for _, value := range page.Sources {
			response.Sources = append(response.Sources, googleFormsSourceFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /api/v1/google-forms/sources/{source_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := googleFormsSourceActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, err := service.GetSource(r.Context(), actor, id)
		if err != nil {
			writeGoogleFormsError(w, r, logger, "get Google Forms source", err)
			return
		}
		writeJSON(w, http.StatusOK, googleFormsSourceFromDomain(value))
	})

	mux.HandleFunc("PUT /api/v1/google-forms/sources/{source_id}/mapping", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := googleFormsSourceActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request googleFormsMappingRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.SaveMapping(r.Context(), actor, id, request.Version, request.Mapping, requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "save Google Forms mapping", err)
			return
		}
		writeJSON(w, http.StatusOK, googleFormsSourceFromDomain(value))
	})

	mux.HandleFunc("PATCH /api/v1/google-forms/sources/{source_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := googleFormsSourceActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request googleFormsUpdateSourceRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.UpdateSource(r.Context(), actor, id, request.Version, googleforms.UpdateSourceInput{
			SyncMode: request.SyncMode, PollInterval: time.Duration(request.PollIntervalSeconds) * time.Second, Enabled: request.Enabled,
		}, requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "update Google Forms source", err)
			return
		}
		writeJSON(w, http.StatusOK, googleFormsSourceFromDomain(value))
	})

	mux.HandleFunc("POST /api/v1/google-forms/sources/{source_id}/refresh", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := googleFormsSourceActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, drifted, err := service.RefreshSource(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "refresh Google Forms source", err)
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Source  googleFormsSourceResponse `json:"source"`
			Drifted bool                      `json:"drifted"`
		}{Source: googleFormsSourceFromDomain(value), Drifted: drifted})
	})
}

func registerGoogleFormsSyncRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service googleFormsService) {
	mux.HandleFunc("POST /api/v1/google-forms/sources/{source_id}/syncs", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := googleFormsSourceActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request googleFormsSyncRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.RequestSync(r.Context(), actor, id, request.IdempotencyKey, requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "request Google Forms sync", err)
			return
		}
		writeJSON(w, http.StatusAccepted, googleFormsSyncFromDomain(value))
	})

	mux.HandleFunc("GET /api/v1/google-forms/syncs", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := googleFormsActor(w, r, authentication, service)
		if !ok {
			return
		}
		limit, offset, problem := googleFormsPagination(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var sourceID *googleforms.Identifier
		if raw := strings.TrimSpace(r.URL.Query().Get("source_id")); raw != "" {
			parsed, err := googleforms.ParseIdentifier(raw)
			if err != nil {
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador de fonte inválido"})
				return
			}
			sourceID = &parsed
		}
		page, err := service.ListSyncRuns(r.Context(), actor, sourceID, limit, offset)
		if err != nil {
			writeGoogleFormsError(w, r, logger, "list Google Forms syncs", err)
			return
		}
		response := googleFormsSyncPageResponse{Total: page.Total, Limit: limit, Offset: offset, Runs: make([]googleFormsSyncResponse, 0, len(page.Runs))}
		for _, value := range page.Runs {
			response.Runs = append(response.Runs, googleFormsSyncFromDomain(value))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("POST /api/v1/google-forms/syncs/{sync_id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := googleFormsActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, err := googleforms.ParseIdentifier(r.PathValue("sync_id"))
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador de sincronização inválido"})
			return
		}
		var request googleFormsVersionRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.CancelSync(r.Context(), actor, id, request.Version, requestIDFromContext(r.Context()))
		if err != nil {
			writeGoogleFormsError(w, r, logger, "cancel Google Forms sync", err)
			return
		}
		writeJSON(w, http.StatusOK, googleFormsSyncFromDomain(value))
	})
}

func googleFormsActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service googleFormsService) (auth.Session, bool) {
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	if service == nil || !service.Enabled() {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "A integração com Google Forms não está configurada"})
		return auth.Session{}, false
	}
	return actor, true
}

func googleFormsSourceActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service googleFormsService) (auth.Session, googleforms.Identifier, bool) {
	actor, ok := googleFormsActor(w, r, authentication, service)
	if !ok {
		return auth.Session{}, googleforms.Identifier{}, false
	}
	id, err := googleforms.ParseIdentifier(r.PathValue("source_id"))
	if err != nil {
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador de fonte inválido"})
		return auth.Session{}, googleforms.Identifier{}, false
	}
	return actor, id, true
}

func googleFormsPagination(r *http.Request) (int, int, *Problem) {
	limit, offset := 25, 0
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
	if limit < 1 || limit > 100 || offset < 0 || offset > 10_000 {
		return 0, 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação fora dos limites"}
	}
	return limit, offset, nil
}

func googleFormsConnectionFromDomain(value googleforms.Connection) googleFormsConnectionResponse {
	return googleFormsConnectionResponse{
		ID: value.ID.String(), State: value.State, GrantedScopes: value.GrantedScopes,
		ErrorCode: value.ErrorCode, ConnectedAt: value.ConnectedAt, DisconnectedAt: value.DisconnectedAt,
		Version: value.Version, UpdatedAt: value.UpdatedAt,
	}
}

func googleFormsSourceFromDomain(value googleforms.Source) googleFormsSourceResponse {
	response := googleFormsSourceResponse{
		ID: value.ID.String(), ProviderFormID: value.ProviderFormID, Title: value.Title,
		Module: value.Module, State: value.State, SchemaRevision: value.SchemaRevision,
		SyncMode: value.SyncMode, PollIntervalSeconds: int(value.PollInterval / time.Second),
		CursorSubmittedAt: value.CursorSubmittedAt, PaginationPending: value.ResponsePageToken != "",
		LastSyncedAt: value.LastSyncedAt, NextSyncAt: value.NextSyncAt, ErrorCode: value.ErrorCode,
		Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Questions: make([]googleFormsQuestionResponse, 0, len(value.Questions)),
	}
	for _, question := range value.Questions {
		response.Questions = append(response.Questions, googleFormsQuestionResponse{
			ID: question.ID, Position: question.Position, Title: question.Title,
			AnswerKind: question.AnswerKind, Required: question.Required, Supported: question.Supported,
			UnsupportedCode: question.UnsupportedCode, TargetField: question.TargetField,
		})
	}
	return response
}

func googleFormsSyncFromDomain(value googleforms.SyncRun) googleFormsSyncResponse {
	response := googleFormsSyncResponse{
		ID: value.ID.String(), SourceID: value.SourceID.String(), TriggerKind: value.TriggerKind,
		State: value.State, ReceivedCount: value.ReceivedCount, StagedCount: value.StagedCount,
		DuplicateCount: value.DuplicateCount, ErrorCode: value.ErrorCode,
		StartedAt: value.StartedAt, CompletedAt: value.CompletedAt, Version: value.Version,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
	if value.OperationImportID != nil {
		response.OperationImportID = value.OperationImportID.String()
	}
	return response
}

func googleFormsRedirect(applicationURL, returnPath, result string) string {
	target, err := url.Parse(returnPath)
	if err != nil {
		target = &url.URL{Path: "/google-forms"}
	}
	query := target.Query()
	query.Set("google_forms", result)
	target.RawQuery = query.Encode()
	base, err := url.Parse(applicationURL)
	if err == nil && base.IsAbs() && base.Host != "" {
		return base.ResolveReference(target).String()
	}
	return target.String()
}

func writeGoogleFormsError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	switch {
	case errors.Is(err, googleforms.ErrDisabled):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "A integração com Google Forms está desativada"})
	case errors.Is(err, googleforms.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para gerenciar Google Forms"})
	case errors.Is(err, googleforms.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Recurso do Google Forms não encontrado"})
	case errors.Is(err, googleforms.ErrOAuthState):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeInvalidOAuthState, Message: "A autorização expirou ou não pertence a esta sessão"})
	case errors.Is(err, googleforms.ErrOAuthScopes), errors.Is(err, googleforms.ErrUnsupportedForm):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "O formulário ou as permissões concedidas não são compatíveis"})
	case errors.Is(err, googleforms.ErrNeedsReauth), errors.Is(err, googleforms.ErrSchemaDrift), errors.Is(err, googleforms.ErrInvalidState):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A conexão ou a fonte precisa ser revisada antes de continuar"})
	case errors.Is(err, googleforms.ErrConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O recurso mudou. Atualize os dados e tente novamente"})
	case errors.Is(err, googleforms.ErrRateLimited):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeRateLimited, Message: "O Google limitou temporariamente as solicitações"})
	case errors.Is(err, googleforms.ErrInvalidInput):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Os dados da integração são inválidos"})
	case errors.Is(err, googleforms.ErrProvider):
		logger.Warn(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusBadGateway, Code: ErrorCodeAuthProvider, Message: "O Google Forms não respondeu como esperado"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a integração"})
	}
}

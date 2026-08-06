package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	taskdomain "github.com/Pherlsz/Gymkhana-Database/internal/taskengine"
)

const taskStreamPollInterval = 250 * time.Millisecond
const taskStreamHeartbeatInterval = 10 * time.Second

type taskService interface {
	Capability() taskdomain.Capability
	Interpret(context.Context, auth.Session, string, string) (taskdomain.Proposal, error)
	CreateDraft(context.Context, auth.Session, taskdomain.TaskSpec, string) (taskdomain.Draft, error)
	ReviewDraft(context.Context, auth.Session, taskdomain.Identifier, taskdomain.TaskSpec, int64, string) (taskdomain.Draft, error)
	Draft(context.Context, auth.Session, taskdomain.Identifier, string) (taskdomain.Draft, error)
	StartJob(context.Context, auth.Session, taskdomain.Identifier, string, *taskdomain.Identifier, string) (taskdomain.Job, error)
	Jobs(context.Context, auth.Session, int, int, string) (taskdomain.JobPage, error)
	Job(context.Context, auth.Session, taskdomain.Identifier, string) (taskdomain.Job, error)
	CancelJob(context.Context, auth.Session, taskdomain.Identifier, string) (taskdomain.Job, error)
	Events(context.Context, auth.Session, taskdomain.Identifier, int64, int) (taskdomain.EventPage, error)
	Results(context.Context, auth.Session, taskdomain.Identifier, int, int, string) (taskdomain.ResultPage, error)
}

type taskInterpretRequest struct {
	TaskText string `json:"task_text"`
}
type taskDraftRequest struct {
	Spec taskdomain.TaskSpec `json:"spec"`
}
type taskReviewRequest struct {
	Spec    taskdomain.TaskSpec `json:"spec"`
	Version int64               `json:"version"`
}
type taskStartRequest struct {
	DraftID        string  `json:"draft_id"`
	IdempotencyKey string  `json:"idempotency_key"`
	RetryOfJobID   *string `json:"retry_of_job_id,omitempty"`
}

type taskCapabilityResponse struct {
	Enabled                   bool  `json:"enabled"`
	SemanticInterpretation    bool  `json:"semantic_interpretation"`
	DirectTypedSpecifications bool  `json:"direct_typed_specifications"`
	MaximumTaskTextRunes      int   `json:"maximum_task_text_runes"`
	MaximumRequirements       int   `json:"maximum_requirements"`
	MaximumRoles              int   `json:"maximum_roles"`
	MaximumConstraints        int   `json:"maximum_constraints"`
	MaximumCandidatesPerRole  int   `json:"maximum_candidates_per_role"`
	MaximumBranches           int   `json:"maximum_branches"`
	MaximumSolutions          int   `json:"maximum_solutions"`
	MaximumDurationSeconds    int64 `json:"maximum_duration_seconds"`
	MaximumRequests           int   `json:"maximum_requests_per_hour"`
	RetentionSeconds          int64 `json:"retention_seconds"`
}

type taskDraftResponse struct {
	ID             string                `json:"id"`
	CatalogVersion string                `json:"catalog_version"`
	State          taskdomain.DraftState `json:"state"`
	Spec           taskdomain.TaskSpec   `json:"spec"`
	Version        int64                 `json:"version"`
	ExpiresAt      time.Time             `json:"expires_at"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

type taskJobResponse struct {
	ID                string              `json:"id"`
	DraftID           string              `json:"draft_id"`
	RetryOfJobID      *string             `json:"retry_of_job_id,omitempty"`
	CatalogVersion    string              `json:"catalog_version"`
	State             taskdomain.JobState `json:"state"`
	AttemptCount      int                 `json:"attempt_count"`
	ProgressCurrent   int                 `json:"progress_current"`
	ProgressTotal     int                 `json:"progress_total"`
	CandidateCount    int                 `json:"candidate_count"`
	CompositionCount  int                 `json:"composition_count"`
	ErrorCode         string              `json:"error_code,omitempty"`
	CancelRequestedAt *time.Time          `json:"cancel_requested_at,omitempty"`
	StartedAt         *time.Time          `json:"started_at,omitempty"`
	CompletedAt       *time.Time          `json:"completed_at,omitempty"`
	ExpiresAt         time.Time           `json:"expires_at"`
	Version           int64               `json:"version"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

type taskJobPageResponse struct {
	Jobs   []taskJobResponse `json:"jobs"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}
type taskResultPageResponse struct {
	Job          taskJobResponse          `json:"job"`
	Compositions []taskdomain.Composition `json:"compositions"`
	Total        int                      `json:"total"`
	Limit        int                      `json:"limit"`
	Offset       int                      `json:"offset"`
}

func registerTaskRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service taskService) {
	mux.HandleFunc("GET /api/v1/tasks/capability", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		if _, problem := authenticatedSession(r, authentication); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		capability := taskdomain.DefaultCapability()
		if service != nil {
			capability = service.Capability()
		}
		writeJSON(w, http.StatusOK, taskCapabilityFromDomain(capability))
	}))
	mux.HandleFunc("POST /api/v1/tasks/interpret", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request taskInterpretRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Interpret(r.Context(), actor, request.TaskText, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "interpret task", err)
			return
		}
		writeJSON(w, http.StatusOK, value)
	}))
	mux.HandleFunc("POST /api/v1/tasks/drafts", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request taskDraftRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.CreateDraft(r.Context(), actor, request.Spec, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "create task draft", err)
			return
		}
		writeJSON(w, http.StatusCreated, taskDraftFromDomain(value))
	}))
	mux.HandleFunc("GET /api/v1/tasks/drafts/{draft_id}", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, ok := taskPathID(w, r, "draft_id")
		if !ok {
			return
		}
		value, err := service.Draft(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "read task draft", err)
			return
		}
		writeJSON(w, http.StatusOK, taskDraftFromDomain(value))
	}))
	mux.HandleFunc("PUT /api/v1/tasks/drafts/{draft_id}/review", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, ok := taskPathID(w, r, "draft_id")
		if !ok {
			return
		}
		var request taskReviewRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.ReviewDraft(r.Context(), actor, id, request.Spec, request.Version, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "review task draft", err)
			return
		}
		writeJSON(w, http.StatusOK, taskDraftFromDomain(value))
	}))
	mux.HandleFunc("GET /api/v1/tasks/jobs", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		limit, offset, problem := taskPagination(r, taskdomain.MaximumJobPage)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.Jobs(r.Context(), actor, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "list task jobs", err)
			return
		}
		response := taskJobPageResponse{Jobs: make([]taskJobResponse, 0, len(page.Jobs)), Total: page.Total, Limit: page.Limit, Offset: page.Offset}
		for _, job := range page.Jobs {
			response.Jobs = append(response.Jobs, taskJobFromDomain(job))
		}
		writeJSON(w, http.StatusOK, response)
	}))
	mux.HandleFunc("POST /api/v1/tasks/jobs", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request taskStartRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		draftID, err := taskdomain.IdentifierFromString(request.DraftID)
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador do rascunho inválido"})
			return
		}
		var retry *taskdomain.Identifier
		if request.RetryOfJobID != nil {
			parsed, parseErr := taskdomain.IdentifierFromString(*request.RetryOfJobID)
			if parseErr != nil {
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador do job anterior inválido"})
				return
			}
			retry = &parsed
		}
		value, err := service.StartJob(r.Context(), actor, draftID, request.IdempotencyKey, retry, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "start task job", err)
			return
		}
		writeJSON(w, http.StatusAccepted, taskJobFromDomain(value))
	}))
	mux.HandleFunc("GET /api/v1/tasks/jobs/{job_id}", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, ok := taskPathID(w, r, "job_id")
		if !ok {
			return
		}
		value, err := service.Job(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "read task job", err)
			return
		}
		writeJSON(w, http.StatusOK, taskJobFromDomain(value))
	}))
	mux.HandleFunc("POST /api/v1/tasks/jobs/{job_id}/cancel", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, ok := taskPathID(w, r, "job_id")
		if !ok {
			return
		}
		value, err := service.CancelJob(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "cancel task job", err)
			return
		}
		writeJSON(w, http.StatusOK, taskJobFromDomain(value))
	}))
	mux.HandleFunc("GET /api/v1/tasks/jobs/{job_id}/events", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, ok := taskPathID(w, r, "job_id")
		if !ok {
			return
		}
		after, problem := taskEventCursor(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		streamTaskEvents(w, r, logger, service, actor, id, after)
	}))
	mux.HandleFunc("GET /api/v1/tasks/jobs/{job_id}/results", requireCapability(auth.CapTasks, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredTaskActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, ok := taskPathID(w, r, "job_id")
		if !ok {
			return
		}
		limit, offset, problem := taskPagination(r, taskdomain.MaximumResultPage)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.Results(r.Context(), actor, id, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			writeTaskError(w, r, logger, "read task results", err)
			return
		}
		writeJSON(w, http.StatusOK, taskResultPageResponse{Job: taskJobFromDomain(page.Job), Compositions: page.Compositions, Total: page.Total, Limit: page.Limit, Offset: page.Offset})
	}))
}

func configuredTaskActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service taskService) (auth.Session, bool) {
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	if service == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "As tarefas avançadas não estão configuradas"})
		return auth.Session{}, false
	}
	return actor, true
}

func taskPathID(w http.ResponseWriter, r *http.Request, key string) (taskdomain.Identifier, bool) {
	id, err := taskdomain.IdentifierFromString(r.PathValue(key))
	if err != nil {
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador de tarefa inválido"})
		return taskdomain.Identifier{}, false
	}
	return id, true
}

func taskPagination(r *http.Request, maximum int) (int, int, *Problem) {
	limit, offset := maximum, 0
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
	if limit < 1 || limit > maximum || offset < 0 || offset > 10000 {
		return 0, 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação fora dos limites"}
	}
	return limit, offset, nil
}

func taskEventCursor(r *http.Request) (int64, *Problem) {
	raw := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("after"))
	}
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Cursor de eventos inválido"}
	}
	return value, nil
}

func streamTaskEvents(w http.ResponseWriter, r *http.Request, logger *slog.Logger, service taskService, actor auth.Session, id taskdomain.Identifier, after int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Streaming não suportado"})
		return
	}
	first, err := service.Events(r.Context(), actor, id, after, taskdomain.MaximumEventPage)
	if err != nil {
		writeTaskError(w, r, logger, "open task event stream", err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	cursor := after
	writePage := func(page taskdomain.EventPage) bool {
		for _, event := range page.Events {
			encoded, encodeErr := json.Marshal(event)
			if encodeErr != nil {
				return false
			}
			if _, writeErr := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Kind, encoded); writeErr != nil {
				return false
			}
			cursor = event.Sequence
		}
		flusher.Flush()
		return true
	}
	if !writePage(first) || first.Terminal {
		return
	}
	poll := time.NewTicker(taskStreamPollInterval)
	heartbeat := time.NewTicker(taskStreamHeartbeatInterval)
	defer poll.Stop()
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-poll.C:
			page, err := service.Events(r.Context(), actor, id, cursor, taskdomain.MaximumEventPage)
			if err != nil {
				return
			}
			if !writePage(page) || page.Terminal {
				return
			}
		}
	}
}

func taskCapabilityFromDomain(value taskdomain.Capability) taskCapabilityResponse {
	return taskCapabilityResponse{Enabled: value.Enabled, SemanticInterpretation: value.SemanticInterpretation, DirectTypedSpecifications: value.DirectTypedSpecifications, MaximumTaskTextRunes: value.MaximumTaskTextRunes, MaximumRequirements: value.MaximumRequirements, MaximumRoles: value.MaximumRoles, MaximumConstraints: value.MaximumConstraints, MaximumCandidatesPerRole: value.MaximumCandidatesPerRole, MaximumBranches: value.MaximumBranches, MaximumSolutions: value.MaximumSolutions, MaximumDurationSeconds: int64(value.MaximumDuration / time.Second), MaximumRequests: value.MaximumRequests, RetentionSeconds: int64(value.Retention / time.Second)}
}
func taskDraftFromDomain(value taskdomain.Draft) taskDraftResponse {
	return taskDraftResponse{ID: value.ID.String(), CatalogVersion: value.CatalogVersion, State: value.State, Spec: value.Spec, Version: value.Version, ExpiresAt: value.ExpiresAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func taskJobFromDomain(value taskdomain.Job) taskJobResponse {
	response := taskJobResponse{ID: value.ID.String(), DraftID: value.DraftID.String(), CatalogVersion: value.CatalogVersion, State: value.State, AttemptCount: value.AttemptCount, ProgressCurrent: value.ProgressCurrent, ProgressTotal: value.ProgressTotal, CandidateCount: value.CandidateCount, CompositionCount: value.CompositionCount, ErrorCode: value.ErrorCode, CancelRequestedAt: value.CancelRequestedAt, StartedAt: value.StartedAt, CompletedAt: value.CompletedAt, ExpiresAt: value.ExpiresAt, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if value.RetryOfJobID != nil {
		id := value.RetryOfJobID.String()
		response.RetryOfJobID = &id
	}
	return response
}

func writeTaskError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	var validation interface{ Error() string }
	_ = validation
	switch {
	case errors.Is(err, taskdomain.ErrInterpreterDisabled):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "A interpretação semântica está desativada; use uma especificação tipada"})
	case errors.Is(err, taskdomain.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para esta tarefa"})
	case errors.Is(err, taskdomain.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Tarefa não encontrada"})
	case errors.Is(err, taskdomain.ErrExpired):
		writeProblem(w, r, Problem{Status: http.StatusGone, Code: ErrorCodeQueryExpired, Message: "A tarefa expirou"})
	case errors.Is(err, taskdomain.ErrConflict), errors.Is(err, taskdomain.ErrUnresolved), errors.Is(err, taskdomain.ErrStaleCatalog):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Revise a tarefa e o catálogo antes de continuar"})
	case errors.Is(err, taskdomain.ErrRateLimited):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeRateLimited, Message: "Limite de tarefas atingido"})
	case errors.Is(err, taskdomain.ErrInvalidInput), errors.Is(err, taskdomain.ErrInvalidSpec), errors.Is(err, taskdomain.ErrTaskTooLarge):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "A especificação da tarefa é inválida"})
	case errors.Is(err, taskdomain.ErrUnavailable):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O processamento de tarefas está indisponível"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error_type", fmt.Sprintf("%T", err))
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a tarefa"})
	}
}

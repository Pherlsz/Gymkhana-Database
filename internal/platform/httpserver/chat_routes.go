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

	chatdomain "github.com/Pherlsz/Gymkhana-Database/internal/aichat"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const chatStreamPollInterval = 200 * time.Millisecond
const chatStreamHeartbeatInterval = 10 * time.Second

type chatService interface {
	Capability() chatdomain.Capability
	CreateThread(context.Context, auth.Session, string, string) (chatdomain.Thread, error)
	Threads(context.Context, auth.Session, int, int, string) (chatdomain.ThreadPage, error)
	Thread(context.Context, auth.Session, chatdomain.Identifier, string) (chatdomain.Thread, error)
	RenameThread(context.Context, auth.Session, chatdomain.Identifier, string, int64, string) (chatdomain.Thread, error)
	DeleteThread(context.Context, auth.Session, chatdomain.Identifier, string) error
	Messages(context.Context, auth.Session, chatdomain.Identifier, int, int, string) (chatdomain.MessagePage, error)
	StartTurn(context.Context, auth.Session, chatdomain.Identifier, string, string, *chatdomain.Identifier, string) (chatdomain.RunCreation, error)
	Run(context.Context, auth.Session, chatdomain.Identifier) (chatdomain.Run, error)
	CancelRun(context.Context, auth.Session, chatdomain.Identifier, string) (chatdomain.Run, error)
	Events(context.Context, auth.Session, chatdomain.Identifier, int64, int) (chatdomain.EventPage, error)
	ResultReference(context.Context, auth.Session, chatdomain.Identifier, string) (chatdomain.ResultReference, error)
	SetActiveResult(context.Context, auth.Session, chatdomain.Identifier, *chatdomain.Identifier, string) (chatdomain.Thread, error)
}

type chatResultReader interface {
	ReadResult(context.Context, auth.Session, chatdomain.Identifier, int, int, string) (chatdomain.ToolOutput, error)
}

type chatRunLauncher interface {
	Start(auth.Session, chatdomain.Identifier, string) bool
	Cancel(chatdomain.Identifier) bool
}

type chatCapabilityResponse struct {
	Enabled                bool  `json:"enabled"`
	MaximumToolCalls       int   `json:"maximum_tool_calls"`
	MaximumRows            int   `json:"maximum_rows"`
	MaximumResultBytes     int   `json:"maximum_result_bytes"`
	MaximumUsage           int64 `json:"maximum_usage"`
	MaximumDurationSeconds int64 `json:"maximum_duration_seconds"`
	MaximumMessageRunes    int   `json:"maximum_message_runes"`
}

type chatThreadResponse struct {
	ID                      string    `json:"id"`
	Title                   string    `json:"title"`
	ActiveResultReferenceID *string   `json:"active_result_reference_id,omitempty"`
	RetentionExpiresAt      time.Time `json:"retention_expires_at"`
	Version                 int64     `json:"version"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type chatThreadPageResponse struct {
	Threads []chatThreadResponse `json:"threads"`
	Total   int                  `json:"total"`
	Limit   int                  `json:"limit"`
	Offset  int                  `json:"offset"`
}

type chatMessageResponse struct {
	ID                 string                 `json:"id"`
	ThreadID           string                 `json:"thread_id"`
	RunID              string                 `json:"run_id"`
	Sequence           int64                  `json:"sequence"`
	Role               chatdomain.MessageRole `json:"role"`
	Content            string                 `json:"content"`
	ResultReferenceIDs []string               `json:"result_reference_ids"`
	CreatedAt          time.Time              `json:"created_at"`
}

type chatMessagePageResponse struct {
	Messages []chatMessageResponse `json:"messages"`
	Total    int                   `json:"total"`
	Limit    int                   `json:"limit"`
	Offset   int                   `json:"offset"`
}

type chatRunResponse struct {
	ID                string              `json:"id"`
	ThreadID          string              `json:"thread_id"`
	RetryOfRunID      *string             `json:"retry_of_run_id,omitempty"`
	State             chatdomain.RunState `json:"state"`
	ToolCallCount     int                 `json:"tool_call_count"`
	InputUsage        int64               `json:"input_usage"`
	OutputUsage       int64               `json:"output_usage"`
	ResultBytes       int64               `json:"result_bytes"`
	ErrorCode         string              `json:"error_code,omitempty"`
	CancelRequestedAt *time.Time          `json:"cancel_requested_at,omitempty"`
	StartedAt         *time.Time          `json:"started_at,omitempty"`
	CompletedAt       *time.Time          `json:"completed_at,omitempty"`
	Version           int64               `json:"version"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

type chatRunCreationResponse struct {
	Run         chatRunResponse     `json:"run"`
	UserMessage chatMessageResponse `json:"user_message"`
	Created     bool                `json:"created"`
}

type chatResultReferenceResponse struct {
	ID               string                         `json:"id"`
	ThreadID         string                         `json:"thread_id"`
	RunID            string                         `json:"run_id"`
	Kind             chatdomain.ResultReferenceKind `json:"kind"`
	QueryExecutionID *string                        `json:"query_execution_id,omitempty"`
	Label            string                         `json:"label"`
	RowCount         int                            `json:"row_count"`
	ColumnCount      int                            `json:"column_count"`
	ExpiresAt        time.Time                      `json:"expires_at"`
	CreatedAt        time.Time                      `json:"created_at"`
}

type chatReferenceResultResponse struct {
	Reference  chatResultReferenceResponse `json:"reference"`
	Data       json.RawMessage             `json:"data"`
	RowCount   int                         `json:"row_count"`
	FieldCount int                         `json:"field_count"`
}

type chatEventResponse struct {
	Sequence          int64                `json:"sequence"`
	Kind              chatdomain.EventKind `json:"kind"`
	TextDelta         string               `json:"text_delta,omitempty"`
	ToolStepID        *string              `json:"tool_step_id,omitempty"`
	ResultReferenceID *string              `json:"result_reference_id,omitempty"`
	ErrorCode         string               `json:"error_code,omitempty"`
	CreatedAt         time.Time            `json:"created_at"`
}

type chatCreateThreadRequest struct {
	Title string `json:"title,omitempty"`
}

type chatRenameThreadRequest struct {
	Title   string `json:"title"`
	Version int64  `json:"version"`
}

type chatStartTurnRequest struct {
	Content        string  `json:"content"`
	IdempotencyKey string  `json:"idempotency_key"`
	RetryOfRunID   *string `json:"retry_of_run_id,omitempty"`
}

type chatSetActiveResultRequest struct {
	ReferenceID json.RawMessage `json:"reference_id"`
}

func registerChatRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service chatService, results chatResultReader, launcher chatRunLauncher) {
	mux.HandleFunc("GET /api/v1/chat/capability", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		if _, problem := authenticatedSession(r, authentication); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil || results == nil || launcher == nil {
			writeJSON(w, http.StatusOK, chatCapabilityFromDomain(chatdomain.DefaultCapability()))
			return
		}
		writeJSON(w, http.StatusOK, chatCapabilityFromDomain(service.Capability()))
	}))

	mux.HandleFunc("GET /api/v1/chat/threads", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		limit, offset, problem := chatPagination(r, chatdomain.MaximumThreadsPage, 10_000)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.Threads(r.Context(), actor, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			writeChatError(w, r, logger, "list AI Chat threads", err)
			return
		}
		writeJSON(w, http.StatusOK, chatThreadPageFromDomain(page))
	}))

	mux.HandleFunc("POST /api/v1/chat/threads", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		var request chatCreateThreadRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		thread, err := service.CreateThread(r.Context(), actor, request.Title, requestIDFromContext(r.Context()))
		if err != nil {
			writeChatError(w, r, logger, "create AI Chat thread", err)
			return
		}
		writeJSON(w, http.StatusCreated, chatThreadFromDomain(thread))
	}))

	mux.HandleFunc("GET /api/v1/chat/threads/{thread_id}", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		id, ok := chatPathIdentifier(w, r, "thread_id")
		if !ok {
			return
		}
		thread, err := service.Thread(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeChatError(w, r, logger, "read AI Chat thread", err)
			return
		}
		writeJSON(w, http.StatusOK, chatThreadFromDomain(thread))
	}))

	mux.HandleFunc("PATCH /api/v1/chat/threads/{thread_id}", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		id, ok := chatPathIdentifier(w, r, "thread_id")
		if !ok {
			return
		}
		var request chatRenameThreadRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		thread, err := service.RenameThread(r.Context(), actor, id, request.Title, request.Version, requestIDFromContext(r.Context()))
		if err != nil {
			writeChatError(w, r, logger, "rename AI Chat thread", err)
			return
		}
		writeJSON(w, http.StatusOK, chatThreadFromDomain(thread))
	}))

	mux.HandleFunc("DELETE /api/v1/chat/threads/{thread_id}", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		id, ok := chatPathIdentifier(w, r, "thread_id")
		if !ok {
			return
		}
		if err := service.DeleteThread(r.Context(), actor, id, requestIDFromContext(r.Context())); err != nil {
			writeChatError(w, r, logger, "delete AI Chat thread", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("GET /api/v1/chat/threads/{thread_id}/messages", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		id, ok := chatPathIdentifier(w, r, "thread_id")
		if !ok {
			return
		}
		limit, offset, problem := chatPagination(r, chatdomain.MaximumMessagesPage, 100_000)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.Messages(r.Context(), actor, id, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			writeChatError(w, r, logger, "read AI Chat messages", err)
			return
		}
		writeJSON(w, http.StatusOK, chatMessagePageFromDomain(page))
	}))

	mux.HandleFunc("PUT /api/v1/chat/threads/{thread_id}/active-result", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		threadID, ok := chatPathIdentifier(w, r, "thread_id")
		if !ok {
			return
		}
		var request chatSetActiveResultRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		referenceID, problem := optionalChatIdentifier(request.ReferenceID)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		thread, err := service.SetActiveResult(r.Context(), actor, threadID, referenceID, requestIDFromContext(r.Context()))
		if err != nil {
			writeChatError(w, r, logger, "set AI Chat active result", err)
			return
		}
		writeJSON(w, http.StatusOK, chatThreadFromDomain(thread))
	}))

	mux.HandleFunc("POST /api/v1/chat/threads/{thread_id}/turns", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		threadID, ok := chatPathIdentifier(w, r, "thread_id")
		if !ok {
			return
		}
		var request chatStartTurnRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var retryID *chatdomain.Identifier
		if request.RetryOfRunID != nil {
			parsed, err := chatdomain.ParseIdentifier(*request.RetryOfRunID)
			if err != nil {
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador do run anterior inválido"})
				return
			}
			retryID = &parsed
		}
		requestID := requestIDFromContext(r.Context())
		creation, err := service.StartTurn(r.Context(), actor, threadID, request.Content, request.IdempotencyKey, retryID, requestID)
		if err != nil {
			writeChatError(w, r, logger, "start AI Chat turn", err)
			return
		}
		if creation.Created || creation.Run.State == chatdomain.RunQueued {
			launcher.Start(actor, creation.Run.ID, requestID)
		}
		status := http.StatusOK
		if creation.Created {
			status = http.StatusCreated
		}
		writeJSON(w, status, chatRunCreationFromDomain(creation))
	}))

	mux.HandleFunc("GET /api/v1/chat/runs/{run_id}", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		id, ok := chatPathIdentifier(w, r, "run_id")
		if !ok {
			return
		}
		run, err := service.Run(r.Context(), actor, id)
		if err != nil {
			writeChatError(w, r, logger, "read AI Chat run", err)
			return
		}
		writeJSON(w, http.StatusOK, chatRunFromDomain(run))
	}))

	mux.HandleFunc("POST /api/v1/chat/runs/{run_id}/cancel", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		id, ok := chatPathIdentifier(w, r, "run_id")
		if !ok {
			return
		}
		run, err := service.CancelRun(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeChatError(w, r, logger, "cancel AI Chat run", err)
			return
		}
		launcher.Cancel(id)
		writeJSON(w, http.StatusOK, chatRunFromDomain(run))
	}))

	mux.HandleFunc("GET /api/v1/chat/runs/{run_id}/events", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		id, ok := chatPathIdentifier(w, r, "run_id")
		if !ok {
			return
		}
		after, problem := chatEventCursor(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		streamChatEvents(w, r, logger, service, actor, id, after)
	}))

	mux.HandleFunc("GET /api/v1/chat/result-references/{reference_id}", requireCapability(auth.CapChat, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredChatActor(w, r, authentication, service, results, launcher)
		if !ok {
			return
		}
		id, ok := chatPathIdentifier(w, r, "reference_id")
		if !ok {
			return
		}
		limit, offset, problem := chatPagination(r, chatdomain.MaximumToolRows, 10_000)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		requestID := requestIDFromContext(r.Context())
		reference, err := service.ResultReference(r.Context(), actor, id, requestID)
		if err != nil {
			writeChatError(w, r, logger, "read AI Chat result reference", err)
			return
		}
		output, err := results.ReadResult(r.Context(), actor, id, limit, offset, requestID)
		if err != nil {
			writeChatError(w, r, logger, "reopen AI Chat result reference", err)
			return
		}
		writeJSON(w, http.StatusOK, chatReferenceResultResponse{Reference: chatResultReferenceFromDomain(reference), Data: output.Payload,
			RowCount: output.RowCount, FieldCount: output.FieldCount})
	}))
}

func configuredChatActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service chatService, results chatResultReader, launcher chatRunLauncher) (auth.Session, bool) {
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	if service == nil || results == nil || launcher == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeChatUnavailable,
			Message: "O Chat está desativado até que provedor, modelo e retenção sejam configurados"})
		return auth.Session{}, false
	}
	return actor, true
}

func streamChatEvents(w http.ResponseWriter, r *http.Request, logger *slog.Logger, service chatService, actor auth.Session, runID chatdomain.Identifier, after int64) {
	page, err := service.Events(r.Context(), actor, runID, after, chatdomain.MaximumEventsPage)
	if err != nil {
		writeChatError(w, r, logger, "open AI Chat event stream", err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeChatUnavailable, Message: "Streaming não está disponível neste servidor"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	controller := http.NewResponseController(w)
	refreshDeadline := func() { _ = controller.SetWriteDeadline(time.Now().Add(chatStreamHeartbeatInterval + 5*time.Second)) }
	refreshDeadline()
	heartbeat := time.NewTicker(chatStreamHeartbeatInterval)
	poll := time.NewTicker(chatStreamPollInterval)
	defer heartbeat.Stop()
	defer poll.Stop()
	for {
		for _, event := range page.Events {
			refreshDeadline()
			encoded, marshalErr := json.Marshal(chatEventFromDomain(event))
			if marshalErr != nil {
				logger.Error("encode AI Chat event", "request_id", requestIDFromContext(r.Context()), "error_code", "internal_error")
				return
			}
			if _, writeErr := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Kind, encoded); writeErr != nil {
				return
			}
			after = event.Sequence
		}
		flusher.Flush()
		if page.Terminal {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			refreshDeadline()
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-poll.C:
			page, err = service.Events(r.Context(), actor, runID, after, chatdomain.MaximumEventsPage)
			if err != nil {
				logger.Warn("close AI Chat event stream", "request_id", requestIDFromContext(r.Context()), "error_code", string(chatErrorCode(err)))
				return
			}
		}
	}
}

func chatPathIdentifier(w http.ResponseWriter, r *http.Request, name string) (chatdomain.Identifier, bool) {
	id, err := chatdomain.ParseIdentifier(r.PathValue(name))
	if err != nil {
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador do Chat inválido"})
		return chatdomain.Identifier{}, false
	}
	return id, true
}

func optionalChatIdentifier(raw json.RawMessage) (*chatdomain.Identifier, *Problem) {
	if len(raw) == 0 {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Informe reference_id como UUID ou null"}
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "reference_id deve ser um UUID ou null"}
	}
	id, err := chatdomain.ParseIdentifier(value)
	if err != nil {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Referência de resultado inválida"}
	}
	return &id, nil
}

func chatPagination(r *http.Request, maximumLimit, maximumOffset int) (int, int, *Problem) {
	limit, offset := maximumLimit, 0
	for key, destination := range map[string]*int{"limit": &limit, "offset": &offset} {
		raw := strings.TrimSpace(r.URL.Query().Get(key))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação do Chat inválida"}
		}
		*destination = value
	}
	if limit < 1 || limit > maximumLimit || offset < 0 || offset > maximumOffset {
		return 0, 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação do Chat fora dos limites"}
	}
	return limit, offset, nil
}

func chatEventCursor(r *http.Request) (int64, *Problem) {
	var cursor int64
	for _, raw := range []string{r.Header.Get("Last-Event-ID"), r.URL.Query().Get("after")} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Cursor de eventos inválido"}
		}
		if value > cursor {
			cursor = value
		}
	}
	return cursor, nil
}

func chatCapabilityFromDomain(value chatdomain.Capability) chatCapabilityResponse {
	return chatCapabilityResponse{Enabled: value.Enabled, MaximumToolCalls: value.MaximumToolCalls, MaximumRows: value.MaximumRows,
		MaximumResultBytes: value.MaximumBytes, MaximumUsage: value.MaximumUsage,
		MaximumDurationSeconds: int64(value.MaximumDuration / time.Second), MaximumMessageRunes: value.MaximumMessage}
}

func chatThreadFromDomain(value chatdomain.Thread) chatThreadResponse {
	return chatThreadResponse{ID: value.ID.String(), Title: value.Title, ActiveResultReferenceID: chatIdentifierString(value.ActiveResultReferenceID),
		RetentionExpiresAt: value.RetentionExpiresAt, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func chatThreadPageFromDomain(value chatdomain.ThreadPage) chatThreadPageResponse {
	response := chatThreadPageResponse{Threads: make([]chatThreadResponse, 0, len(value.Threads)), Total: value.Total, Limit: value.Limit, Offset: value.Offset}
	for _, thread := range value.Threads {
		response.Threads = append(response.Threads, chatThreadFromDomain(thread))
	}
	return response
}

func chatMessageFromDomain(value chatdomain.Message) chatMessageResponse {
	response := chatMessageResponse{ID: value.ID.String(), ThreadID: value.ThreadID.String(), RunID: value.RunID.String(), Sequence: value.Sequence,
		Role: value.Role, Content: value.Content, ResultReferenceIDs: make([]string, 0, len(value.ResultReferenceIDs)), CreatedAt: value.CreatedAt}
	for _, referenceID := range value.ResultReferenceIDs {
		response.ResultReferenceIDs = append(response.ResultReferenceIDs, referenceID.String())
	}
	return response
}

func chatMessagePageFromDomain(value chatdomain.MessagePage) chatMessagePageResponse {
	response := chatMessagePageResponse{Messages: make([]chatMessageResponse, 0, len(value.Messages)), Total: value.Total, Limit: value.Limit, Offset: value.Offset}
	for _, message := range value.Messages {
		response.Messages = append(response.Messages, chatMessageFromDomain(message))
	}
	return response
}

func chatRunFromDomain(value chatdomain.Run) chatRunResponse {
	return chatRunResponse{ID: value.ID.String(), ThreadID: value.ThreadID.String(), RetryOfRunID: chatIdentifierString(value.RetryOfRunID), State: value.State,
		ToolCallCount: value.ToolCallCount, InputUsage: value.InputUsage, OutputUsage: value.OutputUsage, ResultBytes: value.ResultBytes,
		ErrorCode: value.ErrorCode, CancelRequestedAt: value.CancelRequestedAt, StartedAt: value.StartedAt, CompletedAt: value.CompletedAt,
		Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func chatRunCreationFromDomain(value chatdomain.RunCreation) chatRunCreationResponse {
	return chatRunCreationResponse{Run: chatRunFromDomain(value.Run), UserMessage: chatMessageFromDomain(value.UserMessage), Created: value.Created}
}

func chatResultReferenceFromDomain(value chatdomain.ResultReference) chatResultReferenceResponse {
	return chatResultReferenceResponse{ID: value.ID.String(), ThreadID: value.ThreadID.String(), RunID: value.RunID.String(), Kind: value.Kind,
		QueryExecutionID: chatIdentifierString(value.QueryExecutionID), Label: value.Label, RowCount: value.RowCount, ColumnCount: value.ColumnCount,
		ExpiresAt: value.ExpiresAt, CreatedAt: value.CreatedAt}
}

func chatEventFromDomain(value chatdomain.RunEvent) chatEventResponse {
	return chatEventResponse{Sequence: value.Sequence, Kind: value.Kind, TextDelta: value.TextDelta, ToolStepID: chatIdentifierString(value.ToolStepID),
		ResultReferenceID: chatIdentifierString(value.ResultReferenceID), ErrorCode: value.ErrorCode, CreatedAt: value.CreatedAt}
}

func chatIdentifierString(value *chatdomain.Identifier) *string {
	if value == nil {
		return nil
	}
	encoded := value.String()
	return &encoded
}

func writeChatError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	code := chatErrorCode(err)
	switch {
	case errors.Is(err, chatdomain.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para usar este Chat"})
	case errors.Is(err, chatdomain.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Conversa, run ou referência não encontrada"})
	case errors.Is(err, chatdomain.ErrConflict), errors.Is(err, chatdomain.ErrInvalidState):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeChatBusy, Message: "A conversa já possui um run ativo ou mudou simultaneamente"})
	case errors.Is(err, chatdomain.ErrInvalidInput):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Revise os dados enviados ao Chat"})
	case errors.Is(err, chatdomain.ErrRateLimited):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeRateLimited, Message: "Limite de mensagens atingido. Aguarde antes de tentar novamente"})
	case errors.Is(err, chatdomain.ErrQuotaExceeded):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeChatQuota, Message: "O run atingiu um limite seguro de uso, ferramentas ou resultados"})
	case errors.Is(err, chatdomain.ErrStaleContext):
		writeProblem(w, r, Problem{Status: http.StatusGone, Code: ErrorCodeChatStaleContext, Message: "O contexto de resultado expirou ou não corresponde mais à conversa"})
	case errors.Is(err, chatdomain.ErrCancelled):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeChatCancelled, Message: "O run foi cancelado"})
	case errors.Is(err, chatdomain.ErrTimeout):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeChatTimeout, Message: "O run excedeu o tempo seguro"})
	case errors.Is(err, chatdomain.ErrUnavailable):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeChatUnavailable, Message: "O provedor do Chat está indisponível"})
	case errors.Is(err, chatdomain.ErrMalformedProvider), errors.Is(err, chatdomain.ErrToolFailed), errors.Is(err, chatdomain.ErrUnsafeResult):
		writeProblem(w, r, Problem{Status: http.StatusBadGateway, Code: code, Message: "O run foi encerrado porque uma resposta externa não atendeu ao contrato seguro"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error_code", string(code))
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a operação do Chat"})
	}
}

func chatErrorCode(err error) ErrorCode {
	switch {
	case errors.Is(err, chatdomain.ErrQuotaExceeded):
		return ErrorCodeChatQuota
	case errors.Is(err, chatdomain.ErrStaleContext):
		return ErrorCodeChatStaleContext
	case errors.Is(err, chatdomain.ErrCancelled):
		return ErrorCodeChatCancelled
	case errors.Is(err, chatdomain.ErrTimeout):
		return ErrorCodeChatTimeout
	case errors.Is(err, chatdomain.ErrUnavailable):
		return ErrorCodeChatUnavailable
	case errors.Is(err, chatdomain.ErrMalformedProvider):
		return ErrorCodeChatMalformed
	case errors.Is(err, chatdomain.ErrToolFailed):
		return ErrorCodeChatToolFailed
	case errors.Is(err, chatdomain.ErrUnsafeResult):
		return ErrorCodeChatUnsafeResult
	default:
		return ErrorCodeInternal
	}
}

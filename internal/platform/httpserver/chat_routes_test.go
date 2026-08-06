package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	chatdomain "github.com/Pherlsz/Gymkhana-Database/internal/aichat"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type fakeChatHTTPService struct {
	capability     chatdomain.Capability
	thread         chatdomain.Thread
	threadPage     chatdomain.ThreadPage
	messagePage    chatdomain.MessagePage
	creation       chatdomain.RunCreation
	run            chatdomain.Run
	events         []chatdomain.RunEvent
	reference      chatdomain.ResultReference
	actor          auth.Session
	requestID      string
	title          string
	version        int64
	content        string
	idempotencyKey string
	retryID        *chatdomain.Identifier
	activeID       *chatdomain.Identifier
	limit          int
	offset         int
	after          int64
	deleted        bool
	err            error
}

func (service *fakeChatHTTPService) Capability() chatdomain.Capability { return service.capability }

func (service *fakeChatHTTPService) CreateThread(_ context.Context, actor auth.Session, title, requestID string) (chatdomain.Thread, error) {
	service.actor, service.title, service.requestID = actor, title, requestID
	return service.thread, service.err
}

func (service *fakeChatHTTPService) Threads(_ context.Context, actor auth.Session, limit, offset int, requestID string) (chatdomain.ThreadPage, error) {
	service.actor, service.limit, service.offset, service.requestID = actor, limit, offset, requestID
	return service.threadPage, service.err
}

func (service *fakeChatHTTPService) Thread(_ context.Context, actor auth.Session, _ chatdomain.Identifier, requestID string) (chatdomain.Thread, error) {
	service.actor, service.requestID = actor, requestID
	return service.thread, service.err
}

func (service *fakeChatHTTPService) RenameThread(_ context.Context, actor auth.Session, _ chatdomain.Identifier, title string, version int64, requestID string) (chatdomain.Thread, error) {
	service.actor, service.title, service.version, service.requestID = actor, title, version, requestID
	return service.thread, service.err
}

func (service *fakeChatHTTPService) DeleteThread(_ context.Context, actor auth.Session, _ chatdomain.Identifier, requestID string) error {
	service.actor, service.requestID, service.deleted = actor, requestID, true
	return service.err
}

func (service *fakeChatHTTPService) Messages(_ context.Context, actor auth.Session, _ chatdomain.Identifier, limit, offset int, requestID string) (chatdomain.MessagePage, error) {
	service.actor, service.limit, service.offset, service.requestID = actor, limit, offset, requestID
	return service.messagePage, service.err
}

func (service *fakeChatHTTPService) StartTurn(_ context.Context, actor auth.Session, _ chatdomain.Identifier, content, idempotencyKey string, retry *chatdomain.Identifier, requestID string) (chatdomain.RunCreation, error) {
	service.actor, service.content, service.idempotencyKey, service.retryID, service.requestID = actor, content, idempotencyKey, retry, requestID
	return service.creation, service.err
}

func (service *fakeChatHTTPService) Run(_ context.Context, actor auth.Session, _ chatdomain.Identifier) (chatdomain.Run, error) {
	service.actor = actor
	return service.run, service.err
}

func (service *fakeChatHTTPService) CancelRun(_ context.Context, actor auth.Session, _ chatdomain.Identifier, requestID string) (chatdomain.Run, error) {
	service.actor, service.requestID = actor, requestID
	return service.run, service.err
}

func (service *fakeChatHTTPService) Events(_ context.Context, actor auth.Session, _ chatdomain.Identifier, after int64, limit int) (chatdomain.EventPage, error) {
	service.actor, service.after, service.limit = actor, after, limit
	values := make([]chatdomain.RunEvent, 0)
	for _, event := range service.events {
		if event.Sequence > after {
			values = append(values, event)
		}
	}
	last := after
	if len(values) > 0 {
		last = values[len(values)-1].Sequence
	}
	return chatdomain.EventPage{Events: values, LastSequence: last, Terminal: true}, service.err
}

func (service *fakeChatHTTPService) ResultReference(_ context.Context, actor auth.Session, _ chatdomain.Identifier, requestID string) (chatdomain.ResultReference, error) {
	service.actor, service.requestID = actor, requestID
	return service.reference, service.err
}

func (service *fakeChatHTTPService) SetActiveResult(_ context.Context, actor auth.Session, _ chatdomain.Identifier, reference *chatdomain.Identifier, requestID string) (chatdomain.Thread, error) {
	service.actor, service.activeID, service.requestID = actor, reference, requestID
	return service.thread, service.err
}

type fakeChatResultReader struct {
	output    chatdomain.ToolOutput
	actor     auth.Session
	reference chatdomain.Identifier
	limit     int
	offset    int
	err       error
}

func (reader *fakeChatResultReader) ReadResult(_ context.Context, actor auth.Session, reference chatdomain.Identifier, limit, offset int, _ string) (chatdomain.ToolOutput, error) {
	reader.actor, reader.reference, reader.limit, reader.offset = actor, reference, limit, offset
	return reader.output, reader.err
}

type fakeChatLauncher struct {
	started   chatdomain.Identifier
	cancelled chatdomain.Identifier
	actor     auth.Session
	requestID string
}

func (launcher *fakeChatLauncher) Start(actor auth.Session, run chatdomain.Identifier, requestID string) bool {
	launcher.actor, launcher.started, launcher.requestID = actor, run, requestID
	return true
}

func (launcher *fakeChatLauncher) Cancel(run chatdomain.Identifier) bool {
	launcher.cancelled = run
	return true
}

func TestChatRoutesExposeProtectedStrictLifecycleAndTypedReferences(t *testing.T) {
	service, reader, launcher, handler := chatHTTPFixture(t, authTestLogger(), "")
	threadID, runID, messageID, referenceID := chatHTTPIdentifiers(t)

	response := serveChatRequest(handler, http.MethodPost, "/api/v1/chat/threads", `{"title":"Minha conversa"}`, "")
	if response.Code != http.StatusCreated || service.title != "Minha conversa" || service.requestID == "" || service.actor.User.ID == (auth.Identifier{}) {
		t.Fatalf("create response = %d %s, service=%#v", response.Code, response.Body.String(), service)
	}
	response = serveChatRequest(handler, http.MethodPost, "/api/v1/chat/threads", `{"title":"x","unknown":true}`, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("strict create response = %d %s", response.Code, response.Body.String())
	}

	response = serveChatRequest(handler, http.MethodGet, "/api/v1/chat/threads?limit=20&offset=0", "", "")
	if response.Code != http.StatusOK || service.limit != 20 || !strings.Contains(response.Body.String(), threadID.String()) {
		t.Fatalf("list response = %d %s", response.Code, response.Body.String())
	}
	response = serveChatRequest(handler, http.MethodPatch, "/api/v1/chat/threads/"+threadID.String(), `{"title":"Renomeada","version":1}`, "")
	if response.Code != http.StatusOK || service.title != "Renomeada" || service.version != 1 {
		t.Fatalf("rename response = %d %s", response.Code, response.Body.String())
	}
	response = serveChatRequest(handler, http.MethodGet, "/api/v1/chat/threads/"+threadID.String()+"/messages?limit=25", "", "")
	var messagePage chatMessagePageResponse
	decodeErr := json.Unmarshal(response.Body.Bytes(), &messagePage)
	if response.Code != http.StatusOK || service.limit != 25 || decodeErr != nil || len(messagePage.Messages) != 1 || messagePage.Messages[0].Content != "<script>alert(1)</script>" ||
		len(messagePage.Messages[0].ResultReferenceIDs) != 1 || messagePage.Messages[0].ResultReferenceIDs[0] != referenceID.String() {
		t.Fatalf("messages response = %d %s", response.Code, response.Body.String())
	}

	response = serveChatRequest(handler, http.MethodPut, "/api/v1/chat/threads/"+threadID.String()+"/active-result", `{"reference_id":null}`, "")
	if response.Code != http.StatusOK || service.activeID != nil {
		t.Fatalf("clear context response = %d %s, active=%v", response.Code, response.Body.String(), service.activeID)
	}
	response = serveChatRequest(handler, http.MethodPut, "/api/v1/chat/threads/"+threadID.String()+"/active-result", `{}`, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing context response = %d %s", response.Code, response.Body.String())
	}

	response = serveChatRequest(handler, http.MethodPost, "/api/v1/chat/threads/"+threadID.String()+"/turns",
		`{"content":"Quem está em Recife?","idempotency_key":"http-chat-turn-01"}`, "")
	if response.Code != http.StatusCreated || service.content != "Quem está em Recife?" || service.idempotencyKey != "http-chat-turn-01" || launcher.started != runID {
		t.Fatalf("turn response = %d %s, launcher=%#v", response.Code, response.Body.String(), launcher)
	}
	response = serveChatRequest(handler, http.MethodGet, "/api/v1/chat/runs/"+runID.String(), "", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"QUEUED"`) {
		t.Fatalf("run response = %d %s", response.Code, response.Body.String())
	}
	response = serveChatRequest(handler, http.MethodPost, "/api/v1/chat/runs/"+runID.String()+"/cancel", "", "")
	if response.Code != http.StatusOK || launcher.cancelled != runID {
		t.Fatalf("cancel response = %d %s, launcher=%#v", response.Code, response.Body.String(), launcher)
	}

	response = serveChatRequest(handler, http.MethodGet, "/api/v1/chat/result-references/"+referenceID.String()+"?limit=10", "", "")
	if response.Code != http.StatusOK || reader.reference != referenceID || reader.limit != 10 || !strings.Contains(response.Body.String(), `"results"`) ||
		strings.Contains(response.Body.String(), "logical_request") {
		t.Fatalf("reference response = %d %s, reader=%#v", response.Code, response.Body.String(), reader)
	}
	response = serveChatRequest(handler, http.MethodDelete, "/api/v1/chat/threads/"+threadID.String(), "", "")
	if response.Code != http.StatusNoContent || !service.deleted {
		t.Fatalf("delete response = %d %s", response.Code, response.Body.String())
	}
	_ = messageID
}

func TestChatCapabilityAndDisabledRoutesAreExplicit(t *testing.T) {
	actorID, _ := auth.NewIdentifier()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Login: "member", Role: auth.RoleExternal, Active: true,
	}}}}
	handler := New(authTestLogger(), nil, Options{Auth: authentication})
	response := serveChatRequest(handler, http.MethodGet, "/api/v1/chat/capability", "", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":false`) ||
		!strings.Contains(response.Body.String(), `"maximum_usage":200000`) || !strings.Contains(response.Body.String(), `"maximum_duration_seconds":45`) {
		t.Fatalf("disabled capability = %d %s", response.Code, response.Body.String())
	}
	response = serveChatRequest(handler, http.MethodGet, "/api/v1/chat/threads", "", "")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"chat_unavailable"`) {
		t.Fatalf("disabled threads = %d %s", response.Code, response.Body.String())
	}

	handler = New(authTestLogger(), nil, Options{Auth: &fakeAdministrationService{}})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chat/capability", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated capability = %d %s", response.Code, response.Body.String())
	}
}

func TestChatSSEOrdersAndReplaysPersistedEventsWithoutDuplication(t *testing.T) {
	service, _, _, handler := chatHTTPFixture(t, authTestLogger(), "")
	_, runID, _, _ := chatHTTPIdentifiers(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chat/runs/"+runID.String()+"/events", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	request.Header.Set("Last-Event-ID", "1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/event-stream" || strings.Contains(body, "id: 1\n") ||
		!strings.Contains(body, "id: 2\nevent: TEXT_DELTA") || !strings.Contains(body, "id: 3\nevent: RUN_COMPLETED") || service.after != 1 {
		t.Fatalf("SSE response = %d %#v %s", response.Code, response.Header(), body)
	}
	if strings.Index(body, "id: 2") > strings.Index(body, "id: 3") {
		t.Fatalf("SSE events out of order: %s", body)
	}
}

func TestChatErrorsAreStableRedactedAndOriginProtected(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   ErrorCode
	}{
		{name: "forbidden", err: chatdomain.ErrForbidden, status: http.StatusForbidden, code: ErrorCodeForbidden},
		{name: "not found", err: chatdomain.ErrNotFound, status: http.StatusNotFound, code: ErrorCodeNotFound},
		{name: "busy", err: chatdomain.ErrConflict, status: http.StatusConflict, code: ErrorCodeChatBusy},
		{name: "rate", err: chatdomain.ErrRateLimited, status: http.StatusTooManyRequests, code: ErrorCodeRateLimited},
		{name: "quota", err: chatdomain.ErrQuotaExceeded, status: http.StatusTooManyRequests, code: ErrorCodeChatQuota},
		{name: "stale", err: chatdomain.ErrStaleContext, status: http.StatusGone, code: ErrorCodeChatStaleContext},
		{name: "cancelled", err: chatdomain.ErrCancelled, status: http.StatusConflict, code: ErrorCodeChatCancelled},
		{name: "timeout", err: chatdomain.ErrTimeout, status: http.StatusServiceUnavailable, code: ErrorCodeChatTimeout},
		{name: "unavailable", err: chatdomain.ErrUnavailable, status: http.StatusServiceUnavailable, code: ErrorCodeChatUnavailable},
		{name: "malformed", err: chatdomain.ErrMalformedProvider, status: http.StatusBadGateway, code: ErrorCodeChatMalformed},
		{name: "internal", err: errors.New("provider payload SELECT secret-token"), status: http.StatusInternalServerError, code: ErrorCodeInternal},
	} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			service, _, _, handler := chatHTTPFixture(t, slog.New(slog.NewJSONHandler(&logs, nil)), "")
			threadID, _, _, _ := chatHTTPIdentifiers(t)
			service.err = test.err
			response := serveChatRequest(handler, http.MethodGet, "/api/v1/chat/threads/"+threadID.String(), "", "")
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+string(test.code)+`"`) {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "SELECT") || strings.Contains(response.Body.String(), "secret-token") || strings.Contains(logs.String(), "secret-token") {
				t.Fatalf("sensitive provider detail leaked: body=%s logs=%s", response.Body.String(), logs.String())
			}
		})
	}

	service, reader, launcher, handler := chatHTTPFixture(t, authTestLogger(), "https://app.example")
	threadID, _, _, _ := chatHTTPIdentifiers(t)
	_ = service
	_ = reader
	_ = launcher
	response := serveChatRequest(handler, http.MethodPost, "/api/v1/chat/threads/"+threadID.String()+"/turns",
		`{"content":"x","idempotency_key":"origin-key-01"}`, "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing Origin mutation = %d %s", response.Code, response.Body.String())
	}
	response = serveChatRequest(handler, http.MethodPost, "/api/v1/chat/threads/"+threadID.String()+"/turns",
		`{"content":"x","idempotency_key":"origin-key-01"}`, "https://app.example")
	if response.Code != http.StatusCreated {
		t.Fatalf("trusted Origin mutation = %d %s", response.Code, response.Body.String())
	}
}

func chatHTTPFixture(t *testing.T, logger *slog.Logger, applicationURL string) (*fakeChatHTTPService, *fakeChatResultReader, *fakeChatLauncher, http.Handler) {
	t.Helper()
	actorID, _ := auth.NewIdentifier()
	threadID, runID, messageID, referenceID := chatHTTPIdentifiers(t)
	now := time.Date(2026, time.July, 18, 20, 0, 0, 0, time.UTC)
	thread := chatdomain.Thread{ID: threadID, Title: "Conversa", ActiveResultReferenceID: &referenceID, RetentionExpiresAt: now.Add(time.Hour),
		Version: 1, CreatedAt: now, UpdatedAt: now}
	message := chatdomain.Message{ID: messageID, ThreadID: threadID, RunID: runID, Sequence: 1, Role: chatdomain.MessageUser,
		Content: "<script>alert(1)</script>", CreatedAt: now}
	historyMessage := message
	historyMessage.Role = chatdomain.MessageAssistant
	historyMessage.ResultReferenceIDs = []chatdomain.Identifier{referenceID}
	run := chatdomain.Run{ID: runID, ThreadID: threadID, State: chatdomain.RunQueued, Version: 1, CreatedAt: now, UpdatedAt: now}
	service := &fakeChatHTTPService{
		capability: chatdomain.Capability{Enabled: true, MaximumToolCalls: 8, MaximumRows: 100, MaximumBytes: 262144, MaximumUsage: 1000,
			MaximumDuration: 45 * time.Second, MaximumMessage: 20000},
		thread: thread, threadPage: chatdomain.ThreadPage{Threads: []chatdomain.Thread{thread}, Total: 1, Limit: 100},
		messagePage: chatdomain.MessagePage{Messages: []chatdomain.Message{historyMessage}, Total: 1, Limit: 100},
		creation:    chatdomain.RunCreation{Run: run, UserMessage: message, Created: true}, run: run,
		events: []chatdomain.RunEvent{{RunID: runID, Sequence: 1, Kind: chatdomain.EventRunStarted, CreatedAt: now},
			{RunID: runID, Sequence: 2, Kind: chatdomain.EventTextDelta, TextDelta: "Olá", CreatedAt: now},
			{RunID: runID, Sequence: 3, Kind: chatdomain.EventRunCompleted, CreatedAt: now}},
		reference: chatdomain.ResultReference{ID: referenceID, ThreadID: threadID, RunID: runID, Kind: chatdomain.ResultReferenceSearch,
			Label: "Busca: Ana", RowCount: 1, ColumnCount: 1, ExpiresAt: now.Add(time.Hour), CreatedAt: now},
	}
	reader := &fakeChatResultReader{output: chatdomain.ToolOutput{Kind: chatdomain.ToolResult, Payload: json.RawMessage(`{"reference_id":"` + referenceID.String() + `","results":[]}`), ByteCount: 64}}
	launcher := &fakeChatLauncher{}
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Login: "member", Role: auth.RoleExternal, Active: true,
	}}}}
	return service, reader, launcher, New(logger, nil, Options{Auth: authentication, Chat: service, ChatResults: reader, ChatLauncher: launcher, ApplicationURL: applicationURL})
}

func chatHTTPIdentifiers(t *testing.T) (chatdomain.Identifier, chatdomain.Identifier, chatdomain.Identifier, chatdomain.Identifier) {
	t.Helper()
	values := []string{
		"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222",
		"33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444",
	}
	identifiers := make([]chatdomain.Identifier, len(values))
	for index, value := range values {
		identifier, err := chatdomain.ParseIdentifier(value)
		if err != nil {
			t.Fatalf("ParseIdentifier(%q) error = %v", value, err)
		}
		identifiers[index] = identifier
	}
	return identifiers[0], identifiers[1], identifiers[2], identifiers[3]
}

func serveChatRequest(handler http.Handler, method, path, body, origin string) *httptest.ResponseRecorder {
	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	}
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

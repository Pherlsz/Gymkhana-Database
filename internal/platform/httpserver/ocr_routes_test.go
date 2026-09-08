package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	ocrdomain "github.com/Pherlsz/Gymkhana-Database/internal/ocr"
)

type fakeOCRHTTPService struct {
	capability     ocrdomain.Capability
	job            ocrdomain.Job
	jobPage        ocrdomain.JobPage
	events         []ocrdomain.JobEvent
	suggestion     ocrdomain.Suggestion
	suggestionPage ocrdomain.SuggestionPage
	receipt        ocrdomain.ApplyReceipt
	actor          auth.Session
	attachmentID   attachment.Identifier
	retryID        *ocrdomain.Identifier
	idempotencyKey string
	selections     []ocrdomain.ApplySelection
	review         ocrdomain.ReviewInput
	requestID      string
	limit          int
	offset         int
	after          int64
	err            error
}

func (service *fakeOCRHTTPService) Capability() ocrdomain.Capability { return service.capability }

func (service *fakeOCRHTTPService) StartJob(_ context.Context, actor auth.Session, attachmentID attachment.Identifier, idempotencyKey string, retryID *ocrdomain.Identifier, requestID string) (ocrdomain.Job, error) {
	service.actor, service.attachmentID, service.idempotencyKey = actor, attachmentID, idempotencyKey
	service.retryID, service.requestID = retryID, requestID
	return service.job, service.err
}

func (service *fakeOCRHTTPService) Jobs(_ context.Context, actor auth.Session, limit, offset int, requestID string) (ocrdomain.JobPage, error) {
	service.actor, service.limit, service.offset, service.requestID = actor, limit, offset, requestID
	return service.jobPage, service.err
}

func (service *fakeOCRHTTPService) Job(_ context.Context, actor auth.Session, _ ocrdomain.Identifier, requestID string) (ocrdomain.Job, error) {
	service.actor, service.requestID = actor, requestID
	return service.job, service.err
}

func (service *fakeOCRHTTPService) CancelJob(_ context.Context, actor auth.Session, _ ocrdomain.Identifier, requestID string) (ocrdomain.Job, error) {
	service.actor, service.requestID = actor, requestID
	return service.job, service.err
}

func (service *fakeOCRHTTPService) Events(_ context.Context, actor auth.Session, _ ocrdomain.Identifier, after int64, limit int) (ocrdomain.EventPage, error) {
	service.actor, service.after, service.limit = actor, after, limit
	values := make([]ocrdomain.JobEvent, 0)
	for _, event := range service.events {
		if event.Sequence > after {
			values = append(values, event)
		}
	}
	last := after
	if len(values) > 0 {
		last = values[len(values)-1].Sequence
	}
	return ocrdomain.EventPage{Events: values, LastSequence: last, Terminal: true}, service.err
}

func (service *fakeOCRHTTPService) Suggestions(_ context.Context, actor auth.Session, _ ocrdomain.Identifier, limit, offset int, requestID string) (ocrdomain.SuggestionPage, error) {
	service.actor, service.limit, service.offset, service.requestID = actor, limit, offset, requestID
	return service.suggestionPage, service.err
}

func (service *fakeOCRHTTPService) ReviewSuggestion(_ context.Context, actor auth.Session, _ ocrdomain.Identifier, review ocrdomain.ReviewInput, requestID string) (ocrdomain.Suggestion, error) {
	service.actor, service.review, service.requestID = actor, review, requestID
	return service.suggestion, service.err
}

func (service *fakeOCRHTTPService) Apply(_ context.Context, actor auth.Session, _ ocrdomain.Identifier, selections []ocrdomain.ApplySelection, idempotencyKey, requestID string) (ocrdomain.ApplyReceipt, error) {
	service.actor, service.selections, service.idempotencyKey, service.requestID = actor, selections, idempotencyKey, requestID
	return service.receipt, service.err
}

func TestOCRRoutesExposeProtectedStrictReviewAndApplyLifecycle(t *testing.T) {
	service, handler, jobID, attachmentID, suggestionID := ocrHTTPFixture(t)

	response := serveChatRequest(handler, http.MethodPost, "/api/v1/ocr/jobs", `{"attachment_id":"`+attachmentID.String()+`","idempotency_key":"ocr-http-job-0001"}`, "")
	if response.Code != http.StatusAccepted || service.attachmentID != attachmentID || service.idempotencyKey != "ocr-http-job-0001" || service.requestID == "" {
		t.Fatalf("start response = %d %s, service=%#v", response.Code, response.Body.String(), service)
	}
	response = serveChatRequest(handler, http.MethodPost, "/api/v1/ocr/jobs", `{"attachment_id":"`+attachmentID.String()+`","idempotency_key":"ocr-http-job-0001","unknown":true}`, "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("strict start response = %d %s", response.Code, response.Body.String())
	}

	response = serveChatRequest(handler, http.MethodGet, "/api/v1/ocr/jobs?limit=20&offset=0", "", "")
	if response.Code != http.StatusOK || service.limit != 20 || !strings.Contains(response.Body.String(), jobID.String()) {
		t.Fatalf("list response = %d %s", response.Code, response.Body.String())
	}
	response = serveChatRequest(handler, http.MethodGet, "/api/v1/ocr/jobs/"+jobID.String(), "", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"attempt_count":1`) {
		t.Fatalf("job response = %d %s", response.Code, response.Body.String())
	}
	response = serveChatRequest(handler, http.MethodGet, "/api/v1/ocr/jobs/"+jobID.String()+"/suggestions?limit=25", "", "")
	if response.Code != http.StatusOK || service.limit != 25 || !strings.Contains(response.Body.String(), `"current_value":"Nome atual"`) {
		t.Fatalf("suggestions response = %d %s", response.Code, response.Body.String())
	}

	response = serveChatRequest(handler, http.MethodPatch, "/api/v1/ocr/suggestions/"+suggestionID.String(), `{"action":"ACCEPT","value":"Nome conferido","version":1}`, "")
	if response.Code != http.StatusOK || service.review.Action != ocrdomain.ReviewAccept || service.review.Value == nil || *service.review.Value != "Nome conferido" {
		t.Fatalf("review response = %d %s, review=%#v", response.Code, response.Body.String(), service.review)
	}
	response = serveChatRequest(handler, http.MethodPost, "/api/v1/ocr/jobs/"+jobID.String()+"/apply", `{"idempotency_key":"ocr-http-apply-0001","selections":[{"suggestion_id":"`+suggestionID.String()+`","version":2}]}`, "")
	if response.Code != http.StatusOK || len(service.selections) != 1 || service.selections[0].SuggestionID != suggestionID || service.selections[0].Version != 2 {
		t.Fatalf("apply response = %d %s, selections=%#v", response.Code, response.Body.String(), service.selections)
	}
}

func TestOCRCapabilityDisabledAndAuthenticationAreExplicit(t *testing.T) {
	actorID, _ := auth.NewIdentifier()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Email: "member", Role: auth.RoleExternal, Active: true,
	}}}}
	handler := New(authTestLogger(), nil, Options{Auth: authentication})
	response := serveChatRequest(handler, http.MethodGet, "/api/v1/ocr/capability", "", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":false`) ||
		!strings.Contains(response.Body.String(), `"maximum_pages":20`) || !strings.Contains(response.Body.String(), `"maximum_source_bytes":20971520`) {
		t.Fatalf("disabled capability = %d %s", response.Code, response.Body.String())
	}
	response = serveChatRequest(handler, http.MethodGet, "/api/v1/ocr/jobs", "", "")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"ocr_unavailable"`) {
		t.Fatalf("disabled jobs = %d %s", response.Code, response.Body.String())
	}

	unauthenticated := New(authTestLogger(), nil, Options{Auth: &fakeAdministrationService{}})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/capability", nil)
	response = httptest.NewRecorder()
	unauthenticated.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated capability = %d %s", response.Code, response.Body.String())
	}
}

func TestOCRSSEReplaysDurableEventsAndErrorsStayRedacted(t *testing.T) {
	service, handler, jobID, _, _ := ocrHTTPFixture(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ocr/jobs/"+jobID.String()+"/events?after=0", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	request.Header.Set("Last-Event-ID", "1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/event-stream" || response.Header().Get("Cache-Control") != "no-store" ||
		strings.Contains(body, "id: 1\n") || !strings.Contains(body, "id: 2\nevent: JOB_COMPLETED") || service.after != 1 {
		t.Fatalf("SSE response = %d %#v %s", response.Code, response.Header(), body)
	}

	service.err = errors.New("private-provider-secret: " + ocrdomain.ErrUnavailable.Error())
	response = serveChatRequest(handler, http.MethodGet, "/api/v1/ocr/jobs/"+jobID.String(), "", "")
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private-provider-secret") {
		t.Fatalf("redacted response = %d %s", response.Code, response.Body.String())
	}
	service.err = errors.Join(errors.New("private-provider-secret"), ocrdomain.ErrUnavailable)
	response = serveChatRequest(handler, http.MethodGet, "/api/v1/ocr/jobs/"+jobID.String(), "", "")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"code":"ocr_unavailable"`) || strings.Contains(response.Body.String(), "private-provider-secret") {
		t.Fatalf("stable wrapped response = %d %s", response.Code, response.Body.String())
	}
}

func ocrHTTPFixture(t *testing.T) (*fakeOCRHTTPService, http.Handler, ocrdomain.Identifier, attachment.Identifier, ocrdomain.Identifier) {
	t.Helper()
	jobID := ocrHTTPIdentifier(t, "11111111-1111-4111-8111-111111111111")
	suggestionID := ocrHTTPIdentifier(t, "22222222-2222-4222-8222-222222222222")
	targetID := ocrHTTPIdentifier(t, "33333333-3333-4333-8333-333333333333")
	attachmentID, err := attachment.ParseIdentifier("44444444-4444-4444-8444-444444444444")
	if err != nil {
		t.Fatalf("ParseIdentifier(attachment) error = %v", err)
	}
	receiptID := ocrHTTPIdentifier(t, "55555555-5555-4555-8555-555555555555")
	actorID, _ := auth.NewIdentifier()
	now := time.Date(2026, time.July, 18, 20, 0, 0, 0, time.UTC)
	completed := now
	job := ocrdomain.Job{
		ID: jobID, OwnerUserID: actorID, AttachmentID: attachmentID, SourceMIME: "image/png", SourceBytes: 1024,
		State: ocrdomain.JobCompleted, AttemptCount: 1, PageCount: 1, PixelCount: 20_000, SuggestionCount: 1,
		ProviderUsage: 22, CompletedAt: &completed, Version: 5, CreatedAt: now, UpdatedAt: now,
	}
	reviewed := "Nome conferido"
	suggestion := ocrdomain.Suggestion{
		ID: suggestionID, JobID: jobID, Ordinal: 1, Target: ocrdomain.TargetReference{Kind: ocrdomain.TargetBill, ID: targetID},
		TargetVersion: 3, FieldKey: "bill.printed_holder_name", FieldLabel: "Titular impresso", Kind: ocrdomain.ValueText,
		ProposedValue: "Nome reconhecido", Evidence: ocrdomain.Evidence{Page: 1, Excerpt: "evidência privada"},
		ReviewState: ocrdomain.ReviewAccepted, ReviewedValue: &reviewed, Version: 2, CreatedAt: now, UpdatedAt: now,
	}
	version := int64(4)
	receipt := ocrdomain.ApplyReceipt{
		ID: receiptID, JobID: jobID, OwnerUserID: actorID, State: ocrdomain.ApplyCompleted,
		Results:   []ocrdomain.ApplyResult{{ReceiptID: receiptID, SuggestionID: suggestionID, Outcome: ocrdomain.ApplyApplied, TargetVersion: &version, CreatedAt: now}},
		CreatedAt: now, UpdatedAt: now, CompletedAt: &completed,
	}
	service := &fakeOCRHTTPService{
		capability: ocrdomain.Capability{Enabled: true, SupportedMIMEs: ocrdomain.SupportedMIMEs(), MaximumSourceBytes: ocrdomain.MaximumSourceBytes,
			MaximumPages: ocrdomain.MaximumPages, MaximumPixels: ocrdomain.MaximumPixels, MaximumSuggestions: ocrdomain.MaximumSuggestions,
			MaximumDuration: 90 * time.Second, MaximumRequests: 10, MaximumProviderUsage: 500_000},
		job: job, jobPage: ocrdomain.JobPage{Jobs: []ocrdomain.Job{job}, Total: 1, Limit: 100}, suggestion: suggestion,
		suggestionPage: ocrdomain.SuggestionPage{Suggestions: []ocrdomain.SuggestionView{{Suggestion: suggestion, CurrentValue: "Nome atual", CurrentVersion: 3}}, Total: 1, Limit: 100},
		receipt:        receipt, events: []ocrdomain.JobEvent{
			{JobID: jobID, Sequence: 1, Kind: ocrdomain.EventJobAccepted, CreatedAt: now},
			{JobID: jobID, Sequence: 2, Kind: ocrdomain.EventJobCompleted, CreatedAt: now},
		},
	}
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Email: "member", Role: auth.RoleExternal, Active: true,
	}}}}
	return service, New(authTestLogger(), nil, Options{Auth: authentication, OCR: service}), jobID, attachmentID, suggestionID
}

func ocrHTTPIdentifier(t *testing.T, value string) ocrdomain.Identifier {
	t.Helper()
	id, err := ocrdomain.ParseIdentifier(value)
	if err != nil {
		t.Fatalf("ParseIdentifier(%q) error = %v", value, err)
	}
	return id
}

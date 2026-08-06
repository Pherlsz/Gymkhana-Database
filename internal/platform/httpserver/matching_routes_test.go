package httpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/matching"
)

type fakeMatchingHTTPService struct {
	catalog        []matching.EvidenceDefinition
	analysis       matching.Analysis
	page           matching.CasePage
	matchingCase   matching.Case
	preview        matching.MergePreview
	result         matching.MergeResult
	actor          auth.Session
	analysisID     matching.Identifier
	caseID         matching.Identifier
	analysisKey    string
	listOptions    matching.CaseListOptions
	dismissVersion int64
	previewInput   matching.MergePreviewInput
	mergeInput     matching.MergeInput
	lastRequestID  string
	err            error
}

func (service *fakeMatchingHTTPService) Catalog(actor auth.Session) ([]matching.EvidenceDefinition, error) {
	service.actor = actor
	return service.catalog, service.err
}

func (service *fakeMatchingHTTPService) StartAnalysis(_ context.Context, actor auth.Session, key, requestID string) (matching.Analysis, error) {
	service.actor, service.analysisKey, service.lastRequestID = actor, key, requestID
	return service.analysis, service.err
}

func (service *fakeMatchingHTTPService) Analysis(_ context.Context, actor auth.Session, id matching.Identifier) (matching.Analysis, error) {
	service.actor, service.analysisID = actor, id
	return service.analysis, service.err
}

func (service *fakeMatchingHTTPService) CancelAnalysis(_ context.Context, actor auth.Session, id matching.Identifier, requestID string) (matching.Analysis, error) {
	service.actor, service.analysisID, service.lastRequestID = actor, id, requestID
	return service.analysis, service.err
}

func (service *fakeMatchingHTTPService) ListCases(_ context.Context, actor auth.Session, options matching.CaseListOptions) (matching.CasePage, error) {
	service.actor, service.listOptions = actor, options
	return service.page, service.err
}

func (service *fakeMatchingHTTPService) Case(_ context.Context, actor auth.Session, id matching.Identifier, requestID string) (matching.Case, error) {
	service.actor, service.caseID, service.lastRequestID = actor, id, requestID
	return service.matchingCase, service.err
}

func (service *fakeMatchingHTTPService) DismissCase(_ context.Context, actor auth.Session, id matching.Identifier, version int64, requestID string) (matching.Case, error) {
	service.actor, service.caseID, service.dismissVersion, service.lastRequestID = actor, id, version, requestID
	return service.matchingCase, service.err
}

func (service *fakeMatchingHTTPService) PreviewMerge(_ context.Context, actor auth.Session, input matching.MergePreviewInput, requestID string) (matching.MergePreview, error) {
	service.actor, service.previewInput, service.lastRequestID = actor, input, requestID
	return service.preview, service.err
}

func (service *fakeMatchingHTTPService) Merge(_ context.Context, actor auth.Session, input matching.MergeInput, requestID string) (matching.MergeResult, error) {
	service.actor, service.mergeInput, service.lastRequestID = actor, input, requestID
	return service.result, service.err
}

func matchingHTTPFixture(t *testing.T, role auth.Role, service matchingService, logger *slog.Logger) http.Handler {
	t.Helper()
	actorID, err := auth.NewIdentifier()
	if err != nil {
		t.Fatalf("auth.NewIdentifier() error = %v", err)
	}
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{
		session: auth.Session{User: auth.User{ID: actorID, Email: "reviewer", Role: role, Active: true}},
	}}
	return New(logger, nil, Options{Auth: authentication, Matching: service})
}

func TestMatchingHTTPExposesExplainableReviewAndExplicitMerge(t *testing.T) {
	now := time.Date(2026, time.July, 17, 21, 0, 0, 0, time.UTC)
	analysisID, _ := matching.ParseIdentifier("11111111-1111-4111-8111-111111111111")
	caseID, _ := matching.ParseIdentifier("22222222-2222-4222-8222-222222222222")
	leftID, _ := matching.ParseIdentifier("33333333-3333-4333-8333-333333333333")
	rightID, _ := matching.ParseIdentifier("44444444-4444-4444-8444-444444444444")
	leftName, rightName := "Ana Silva", "Ana da Silva"
	matchingCase := matching.Case{
		ID: caseID, LeftProfileID: leftID, RightProfileID: rightID,
		LeftProfileVersion: 2, RightProfileVersion: 3, Score: 95, ScoreBand: matching.ScoreHigh,
		State: matching.CasePending, Evidence: []matching.Evidence{{Kind: matching.EvidenceEmailExact, Strength: 100, Contribution: 30}},
		Left:    &matching.ProfileSnapshot{ID: leftID, FullName: leftName, Email: "ana@example.org", Version: 2, UpdatedAt: now},
		Right:   &matching.ProfileSnapshot{ID: rightID, FullName: rightName, Email: "ana@example.org", Version: 3, UpdatedAt: now},
		Version: 4, CreatedAt: now, UpdatedAt: now,
	}
	dependencies := []matching.DependencyCount{
		{Kind: matching.DependencyDocumentOwner, Count: 1}, {Kind: matching.DependencyDocumentHolder},
		{Kind: matching.DependencyBillOwner, Count: 2}, {Kind: matching.DependencyBillHolder},
		{Kind: matching.DependencyCustomEntity}, {Kind: matching.DependencyCustomValue},
		{Kind: matching.DependencyAttachmentIntent}, {Kind: matching.DependencyAttachment, Count: 1},
	}
	fingerprint := sha256.Sum256([]byte("matching-preview"))
	service := &fakeMatchingHTTPService{
		catalog: matching.EvidenceCatalog(),
		analysis: matching.Analysis{ID: analysisID, State: matching.AnalysisCompleted, ProfilesScanned: 12,
			CandidateCount: 1, ExpiresAt: now.Add(24 * time.Hour), Version: 2, CreatedAt: now, UpdatedAt: now},
		page:         matching.CasePage{Cases: []matching.Case{matchingCase}, Total: 1, Limit: 25, Offset: 0},
		matchingCase: matchingCase,
		preview: matching.MergePreview{CaseID: caseID,
			Survivor: *matchingCase.Left, Source: *matchingCase.Right,
			Fields: []matching.MergeField{{Key: "full_name", Label: "Nome completo", Kind: "TEXT", SurvivorValue: &leftName, SourceValue: &rightName,
				Conflict: true, ChoiceRequired: true, SelectedSource: matching.FieldFromSurvivor}},
			Dependencies: dependencies, Conflicts: []matching.DependencyConflict{{Kind: matching.ConflictDocumentUnique, Count: 1}},
			UnresolvedFieldCount: 0, PreviewFingerprint: fingerprint, Confirmation: "MESCLAR Ana Silva", GeneratedAt: now},
		result: matching.MergeResult{CaseID: caseID, SurvivorProfileID: leftID, SourceProfileID: rightID,
			SurvivorVersion: 3, MovedDependencies: dependencies, MergedAt: now},
	}
	handler := matchingHTTPFixture(t, auth.RoleAdmin, service, authTestLogger())

	catalog := serveMatchingRequest(handler, http.MethodGet, "/api/v1/matching/catalog", "")
	if catalog.Code != http.StatusOK || !strings.Contains(catalog.Body.String(), `"kind":"CPF_EXACT"`) ||
		!strings.Contains(catalog.Body.String(), `"can_merge":true`) || strings.Contains(catalog.Body.String(), `"Kind"`) {
		t.Fatalf("catalog response = %d, %s", catalog.Code, catalog.Body.String())
	}
	started := serveMatchingRequest(handler, http.MethodPost, "/api/v1/matching/analyses", `{"idempotency_key":"matching-http-key"}`)
	if started.Code != http.StatusAccepted || service.analysisKey != "matching-http-key" || service.lastRequestID == "" {
		t.Fatalf("start response = %d, %s; key=%q request=%q", started.Code, started.Body.String(), service.analysisKey, service.lastRequestID)
	}
	listed := serveMatchingRequest(handler, http.MethodGet, "/api/v1/matching/cases?state=PENDING,STALE&score_band=HIGH&sort=updated_at&order=asc&limit=25&offset=50", "")
	if listed.Code != http.StatusOK || service.listOptions.Limit != 25 || service.listOptions.Offset != 50 ||
		len(service.listOptions.States) != 2 || service.listOptions.Bands[0] != matching.ScoreHigh ||
		service.listOptions.Sort != matching.CaseSortUpdatedAt || service.listOptions.Order != matching.SortAscending {
		t.Fatalf("list response/options = %d, %s / %#v", listed.Code, listed.Body.String(), service.listOptions)
	}
	if strings.Contains(listed.Body.String(), "address_street") {
		t.Fatalf("list unexpectedly exposed detailed address: %s", listed.Body.String())
	}
	detail := serveMatchingRequest(handler, http.MethodGet, "/api/v1/matching/cases/"+caseID.String(), "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"email":"ana@example.org"`) {
		t.Fatalf("case response = %d, %s", detail.Code, detail.Body.String())
	}
	dismissed := serveMatchingRequest(handler, http.MethodPost, "/api/v1/matching/cases/"+caseID.String()+"/dismiss", `{"version":4}`)
	if dismissed.Code != http.StatusOK || service.dismissVersion != 4 {
		t.Fatalf("dismiss response = %d, %s; version=%d", dismissed.Code, dismissed.Body.String(), service.dismissVersion)
	}
	previewBody := `{"survivor_profile_id":"` + leftID.String() + `","source_profile_id":"` + rightID.String() + `","survivor_version":2,"source_version":3,"choices":[{"field_key":"full_name","source":"SURVIVOR"}]}`
	preview := serveMatchingRequest(handler, http.MethodPost, "/api/v1/matching/cases/"+caseID.String()+"/merge-preview", previewBody)
	if preview.Code != http.StatusOK || service.previewInput.SurvivorID != leftID || len(service.previewInput.Choices) != 1 ||
		!strings.Contains(preview.Body.String(), `"dependencies":[{"kind":"DOCUMENT_OWNER","count":1}`) ||
		!strings.Contains(preview.Body.String(), `"conflicts":[{"kind":"DOCUMENT_UNIQUENESS","count":1}]`) ||
		strings.Contains(preview.Body.String(), `"Kind"`) {
		t.Fatalf("preview response/input = %d, %s / %#v", preview.Code, preview.Body.String(), service.previewInput)
	}
	mergeBody := strings.TrimSuffix(previewBody, "}") + `,"preview_fingerprint":"` + strings.Repeat("a", 64) + `","idempotency_key":"matching-merge-key","confirmation":"MESCLAR Ana Silva"}`
	merged := serveMatchingRequest(handler, http.MethodPost, "/api/v1/matching/cases/"+caseID.String()+"/merge", mergeBody)
	if merged.Code != http.StatusOK || service.mergeInput.IdempotencyKey != "matching-merge-key" || service.mergeInput.Confirmation != "MESCLAR Ana Silva" ||
		service.mergeInput.PreviewFingerprint == ([sha256.Size]byte{}) || strings.Contains(merged.Body.String(), `"Kind"`) {
		t.Fatalf("merge response/input = %d, %s / %#v", merged.Code, merged.Body.String(), service.mergeInput)
	}
}

func TestMatchingHTTPRequiresAuthenticationAndConfiguredService(t *testing.T) {
	handler := New(authTestLogger(), nil, Options{Auth: &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{
		sessionErr: auth.ErrUnauthenticated,
	}}, Matching: &fakeMatchingHTTPService{}})
	response := serveMatchingRequest(handler, http.MethodGet, "/api/v1/matching/catalog", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated response = %d, %s", response.Code, response.Body.String())
	}
	handler = matchingHTTPFixture(t, auth.RoleExternal, nil, authTestLogger())
	response = serveMatchingRequest(handler, http.MethodGet, "/api/v1/matching/catalog", "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured response = %d, %s", response.Code, response.Body.String())
	}
}

func TestMatchingHTTPRejectsInvalidInputsAndMapsStableErrors(t *testing.T) {
	service := &fakeMatchingHTTPService{}
	handler := matchingHTTPFixture(t, auth.RoleExternal, service, authTestLogger())
	for _, request := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/matching/analyses/not-a-uuid", ""},
		{http.MethodGet, "/api/v1/matching/cases?limit=101", ""},
		{http.MethodGet, "/api/v1/matching/cases?offset=-1", ""},
		{http.MethodGet, "/api/v1/matching/cases?sort=physical_column", ""},
		{http.MethodGet, "/api/v1/matching/cases?order=sideways", ""},
		{http.MethodPost, "/api/v1/matching/analyses", `{"idempotency_key":"matching-http-key","sql":"SELECT * FROM profiles"}`},
		{http.MethodPost, "/api/v1/matching/cases/not-a-uuid/dismiss", `{"version":1}`},
		{http.MethodPost, "/api/v1/matching/cases/22222222-2222-4222-8222-222222222222/dismiss", `{"version":1,"profile_email":"private@example.org"}`},
		{http.MethodPost, "/api/v1/matching/cases/22222222-2222-4222-8222-222222222222/merge", `{"survivor_profile_id":"33333333-3333-4333-8333-333333333333","source_profile_id":"44444444-4444-4444-8444-444444444444","survivor_version":1,"source_version":1,"choices":[],"preview_fingerprint":"invalid","idempotency_key":"matching-key","confirmation":"MESCLAR Ana"}`},
	} {
		response := serveMatchingRequest(handler, request.method, request.path, request.body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s = %d, %s", request.method, request.path, response.Code, response.Body.String())
		}
	}

	tests := []struct {
		err    error
		status int
		code   ErrorCode
	}{
		{matching.ErrForbidden, http.StatusForbidden, ErrorCodeForbidden},
		{matching.ErrNotFound, http.StatusNotFound, ErrorCodeNotFound},
		{matching.ErrRateLimited, http.StatusTooManyRequests, ErrorCodeRateLimited},
		{matching.ErrTimeout, http.StatusServiceUnavailable, ErrorCodeMatchingTimeout},
		{matching.ErrCancelled, http.StatusConflict, ErrorCodeMatchingCancelled},
		{matching.ErrStalePreview, http.StatusConflict, ErrorCodeMatchingStale},
		{matching.ErrDependencyConflict, http.StatusConflict, ErrorCodeMatchingConflict},
		{matching.ErrConflict, http.StatusConflict, ErrorCodeConflict},
		{matching.ErrInvalidInput, http.StatusBadRequest, ErrorCodeBadRequest},
		{errors.New("SELECT private_email FROM profiles"), http.StatusInternalServerError, ErrorCodeInternal},
	}
	for _, test := range tests {
		var logs bytes.Buffer
		service.err = test.err
		handler = matchingHTTPFixture(t, auth.RoleExternal, service, slog.New(slog.NewJSONHandler(&logs, nil)))
		response := serveMatchingRequest(handler, http.MethodGet, "/api/v1/matching/catalog", "")
		if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+string(test.code)+`"`) ||
			strings.Contains(response.Body.String(), "private_email") {
			t.Fatalf("error response = %d, %s", response.Code, response.Body.String())
		}
	}
}

func serveMatchingRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
	}
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestMatchingHTTPResponseShapesRemainLowercase(t *testing.T) {
	response := matchingCatalogResponse{Evidence: []matchingEvidenceDefinitionResponse{{Kind: matching.EvidenceCPFExact, Label: "CPF igual"}}}
	encoded, err := json.Marshal(response)
	if err != nil || strings.Contains(string(encoded), `"Kind"`) || !strings.Contains(string(encoded), `"kind"`) {
		t.Fatalf("encoded matching catalog = %s, error=%v", encoded, err)
	}
}

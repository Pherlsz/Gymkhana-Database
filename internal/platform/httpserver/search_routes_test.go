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

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	searchdomain "github.com/Pherlsz/Gymkhana-Database/internal/search"
)

type fakeSearchService struct {
	catalog searchdomain.Catalog
	page    searchdomain.Page
	query   searchdomain.Query
	actor   auth.Session
	err     error
}

func (service *fakeSearchService) Catalog(_ context.Context, actor auth.Session) (searchdomain.Catalog, error) {
	service.actor = actor
	return service.catalog, service.err
}

func (service *fakeSearchService) Search(_ context.Context, actor auth.Session, query searchdomain.Query) (searchdomain.Page, error) {
	service.actor = actor
	service.query = query
	return service.page, service.err
}

func searchHTTPFixture(t *testing.T, service *fakeSearchService, logger *slog.Logger) http.Handler {
	t.Helper()
	actorID, err := auth.NewIdentifier()
	if err != nil {
		t.Fatalf("auth.NewIdentifier() error = %v", err)
	}
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{
		session: auth.Session{User: auth.User{ID: actorID, Email: "member", Role: auth.RoleExternal, Active: true}},
	}}
	return New(logger, nil, Options{Auth: authentication, Search: service})
}

func TestSearchCatalogAndExecutionUseProtectedLogicalContracts(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	service := &fakeSearchService{
		catalog: searchdomain.Catalog{
			Modules: []searchdomain.ModuleDefinition{{Key: searchdomain.ModuleProfiles, Label: "Pessoas"}},
			Fields:  []searchdomain.FieldDefinition{{Key: "profile.full_name", Module: searchdomain.ModuleProfiles, Label: "Nome completo", Kind: "text"}},
			Limits: searchdomain.CatalogLimits{
				MaximumTerms: 5, MaximumPageSize: 100,
				MaximumResultCardinality: searchdomain.MaxResultCardinality,
			},
		},
		page: searchdomain.Page{
			Results: []searchdomain.Result{{
				Module: searchdomain.ModuleProfiles, EntityKind: "profile", EntityID: "11111111-1111-1111-1111-111111111111",
				ProfileID: "11111111-1111-1111-1111-111111111111", TargetKind: "profile", TargetID: "11111111-1111-1111-1111-111111111111",
				EntityLabel: "Ana", FieldKey: "profile.full_name", FieldLabel: "Nome completo", Preview: "Ana", Score: 1080, UpdatedAt: now,
			}},
			Total: 1, Limit: 50, Sort: searchdomain.SortRelevance, Order: searchdomain.SortDescending,
		},
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := searchHTTPFixture(t, service, logger)

	catalogRequest := httptest.NewRequest(http.MethodGet, "/api/v1/search/catalog", nil)
	catalogRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	catalogResponse := httptest.NewRecorder()
	handler.ServeHTTP(catalogResponse, catalogRequest)
	if catalogResponse.Code != http.StatusOK || strings.Contains(catalogResponse.Body.String(), "physical_table") {
		t.Fatalf("catalog response = %d, %s", catalogResponse.Code, catalogResponse.Body.String())
	}

	body := `{"terms":["secret-value"],"modules":["profiles"],"fields":["profile.full_name"],"limit":50,"offset":0,"sort":"relevance","order":"desc"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("search response = %d, %s", response.Code, response.Body.String())
	}
	if len(service.query.Terms) != 1 || service.query.Terms[0] != "secret-value" || service.query.Fields[0] != "profile.full_name" {
		t.Fatalf("query = %#v", service.query)
	}
	var payload searchPageResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Page.Total != 1 || len(payload.Results) != 1 {
		t.Fatalf("payload = %#v, error = %v", payload, err)
	}
	if strings.Contains(logs.String(), "secret-value") {
		t.Fatal("raw Search term was written to logs")
	}
}

func TestSearchRequiresAuthentication(t *testing.T) {
	handler := New(authTestLogger(), nil, Options{Auth: &fakeAdministrationService{}, Search: &fakeSearchService{}})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/search/catalog", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestSearchMapsStableLimitAndValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   ErrorCode
	}{
		{name: "validation", err: &searchdomain.ValidationError{Fields: []searchdomain.FieldError{{Field: "fields", Code: "unsupported"}}}, wantStatus: http.StatusUnprocessableEntity, wantCode: ErrorCodeValidation},
		{name: "rate", err: searchdomain.ErrRateLimited, wantStatus: http.StatusTooManyRequests, wantCode: ErrorCodeRateLimited},
		{name: "cost", err: searchdomain.ErrCostLimit, wantStatus: http.StatusUnprocessableEntity, wantCode: ErrorCodeQueryTooCostly},
		{name: "cardinality", err: searchdomain.ErrCardinalityLimit, wantStatus: http.StatusUnprocessableEntity, wantCode: ErrorCodeResultSetTooLarge},
		{name: "timeout", err: searchdomain.ErrQueryTimeout, wantStatus: http.StatusServiceUnavailable, wantCode: ErrorCodeSearchTimeout},
		{name: "forbidden", err: searchdomain.ErrForbidden, wantStatus: http.StatusForbidden, wantCode: ErrorCodeForbidden},
		{name: "invalid", err: searchdomain.ErrInvalidQuery, wantStatus: http.StatusBadRequest, wantCode: ErrorCodeBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeSearchService{err: test.err}
			handler := searchHTTPFixture(t, service, authTestLogger())
			request := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(`{"terms":["value"]}`))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			var payload errorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Error.Code != test.wantCode {
				t.Fatalf("payload = %#v, error = %v", payload, err)
			}
		})
	}
}

func TestSearchDoesNotEchoInternalErrors(t *testing.T) {
	service := &fakeSearchService{err: errors.New("query failed near physical_secret_table")}
	var logs bytes.Buffer
	handler := searchHTTPFixture(t, service, slog.New(slog.NewJSONHandler(&logs, nil)))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(`{"terms":["value"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "physical_secret_table") {
		t.Fatalf("response = %d, %s", response.Code, response.Body.String())
	}
	if strings.Contains(logs.String(), `"value"`) {
		t.Fatal("raw Search term was written to logs")
	}
}

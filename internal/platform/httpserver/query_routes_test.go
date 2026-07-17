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
	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

type fakeQueryHTTPService struct {
	catalog        querydomain.Catalog
	estimate       querydomain.PlanEstimate
	execution      querydomain.Execution
	page           querydomain.ResultPage
	plan           querydomain.QueryPlan
	idempotencyKey string
	executionID    querydomain.Identifier
	limit          int
	offset         int
	actor          auth.Session
	requestID      string
	err            error
}

func (service *fakeQueryHTTPService) Catalog(_ context.Context, actor auth.Session, requestID string) (querydomain.Catalog, error) {
	service.actor, service.requestID = actor, requestID
	return service.catalog, service.err
}

func (service *fakeQueryHTTPService) Validate(_ context.Context, actor auth.Session, plan querydomain.QueryPlan, requestID string) (querydomain.PlanEstimate, error) {
	service.actor, service.plan, service.requestID = actor, plan, requestID
	return service.estimate, service.err
}

func (service *fakeQueryHTTPService) Execute(_ context.Context, actor auth.Session, plan querydomain.QueryPlan, idempotencyKey, requestID string) (querydomain.Execution, error) {
	service.actor, service.plan, service.idempotencyKey, service.requestID = actor, plan, idempotencyKey, requestID
	return service.execution, service.err
}

func (service *fakeQueryHTTPService) Result(_ context.Context, actor auth.Session, id querydomain.Identifier, limit, offset int, requestID string) (querydomain.ResultPage, error) {
	service.actor, service.executionID, service.limit, service.offset, service.requestID = actor, id, limit, offset, requestID
	return service.page, service.err
}

func queryHTTPFixture(t *testing.T, service queryService, logger *slog.Logger) http.Handler {
	t.Helper()
	actorID, err := auth.NewIdentifier()
	if err != nil {
		t.Fatalf("auth.NewIdentifier() error = %v", err)
	}
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{
		session: auth.Session{User: auth.User{ID: actorID, Login: "member", Role: auth.RoleMember, Active: true}},
	}}
	return New(logger, nil, Options{Auth: authentication, Query: service})
}

func TestQueryRoutesExposeOnlyLogicalProtectedContracts(t *testing.T) {
	now := time.Date(2026, time.July, 17, 18, 0, 0, 0, time.UTC)
	executionID, _ := querydomain.ParseIdentifier("11111111-1111-4111-8111-111111111111")
	completed := now
	service := &fakeQueryHTTPService{
		catalog: querydomain.Catalog{Version: strings.Repeat("a", 64),
			Entities: []querydomain.EntityDefinition{{Key: "profiles", Label: "Pessoas", Kind: "profile", Navigable: true, DefaultSort: "profile.full_name"}},
			Fields:   []querydomain.FieldDefinition{{Key: "profile.full_name", Entity: "profiles", Label: "Nome completo", Kind: querydomain.ValueText, Projectable: true, Filterable: true}},
			Limits:   querydomain.CatalogLimits{MaximumRows: 500, MaximumPageSize: 100}},
		estimate: querydomain.PlanEstimate{Valid: true, Fingerprint: strings.Repeat("b", 64), Cost: 20,
			Columns: []querydomain.ResultColumn{{Position: 0, FieldKey: "profile.full_name", Label: "Nome completo", Kind: querydomain.ValueText}}},
		execution: querydomain.Execution{ID: executionID, State: querydomain.ExecutionCompleted, CatalogVersion: strings.Repeat("a", 64),
			RootEntity: "profiles", MaximumRows: 10, RowCount: 1, ColumnCount: 1, StartedAt: now, CompletedAt: &completed,
			ExpiresAt: now.Add(time.Hour), Version: 2},
	}
	cellValue := "Ana secreta"
	service.page = querydomain.ResultPage{Execution: service.execution, Columns: service.estimate.Columns,
		Rows: []querydomain.ResultRow{{Position: 0, EntityKind: "profile", EntityID: executionID.String(), EntityLabel: "Ana", UpdatedAt: now,
			Cells: []querydomain.ResultCell{{ColumnPosition: 0, Kind: querydomain.ValueText, TextValue: &cellValue}}}},
		Total: 1, Limit: 25, Offset: 0}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := queryHTTPFixture(t, service, logger)

	catalogResponse := serveQueryRequest(handler, http.MethodGet, "/api/v1/query/catalog", "")
	if catalogResponse.Code != http.StatusOK || strings.Contains(catalogResponse.Body.String(), "FROM profiles") || strings.Contains(catalogResponse.Body.String(), "physical") {
		t.Fatalf("catalog response = %d, %s", catalogResponse.Code, catalogResponse.Body.String())
	}
	planJSON := `{"version":"v1","catalog_version":"` + strings.Repeat("a", 64) + `","root_entity":"profiles","projections":["profile.full_name"],"filter":{"kind":"predicate","field":"profile.full_name","operator":"contains","values":["Ana secreta"]},"maximum_rows":10}`
	validateResponse := serveQueryRequest(handler, http.MethodPost, "/api/v1/query/validate", planJSON)
	if validateResponse.Code != http.StatusOK || service.plan.Filter == nil || service.plan.Filter.Values[0] != "Ana secreta" {
		t.Fatalf("validate response/plan = %d, %s / %#v", validateResponse.Code, validateResponse.Body.String(), service.plan)
	}
	executeResponse := serveQueryRequest(handler, http.MethodPost, "/api/v1/query/executions", `{"idempotency_key":"http-query-key","plan":`+planJSON+`}`)
	if executeResponse.Code != http.StatusOK || service.idempotencyKey != "http-query-key" || !strings.Contains(executeResponse.Body.String(), executionID.String()) {
		t.Fatalf("execute response = %d, %s, key=%q", executeResponse.Code, executeResponse.Body.String(), service.idempotencyKey)
	}
	resultResponse := serveQueryRequest(handler, http.MethodGet, "/api/v1/query/executions/"+executionID.String()+"/result?limit=25&offset=0", "")
	if resultResponse.Code != http.StatusOK || service.executionID != executionID || service.limit != 25 || service.offset != 0 {
		t.Fatalf("result response = %d, %s", resultResponse.Code, resultResponse.Body.String())
	}
	var result queryResultPageResponse
	if err := json.Unmarshal(resultResponse.Body.Bytes(), &result); err != nil || result.Execution.ID != executionID.String() || result.Rows[0].Cells[0].TextValue == nil {
		t.Fatalf("result payload = %#v, error=%v", result, err)
	}
	if strings.Contains(logs.String(), "Ana secreta") {
		t.Fatal("raw Query predicate or result was written to logs")
	}
	if service.actor.User.ID == (auth.Identifier{}) || service.requestID == "" {
		t.Fatalf("protected actor/request ID = %#v / %q", service.actor, service.requestID)
	}
}

func TestQueryRoutesRequireAuthenticationAndConfiguredService(t *testing.T) {
	configured := &fakeQueryHTTPService{}
	handler := New(authTestLogger(), nil, Options{Auth: &fakeAdministrationService{}, Query: configured})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/query/catalog", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated response = %d, %s", response.Code, response.Body.String())
	}
	handler = queryHTTPFixture(t, nil, authTestLogger())
	response = serveQueryRequest(handler, http.MethodGet, "/api/v1/query/catalog", "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured response = %d, %s", response.Code, response.Body.String())
	}
}

func TestQueryRouteErrorMappingDoesNotExposeInternals(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   ErrorCode
	}{
		{name: "validation", err: &querydomain.ValidationError{Fields: []querydomain.FieldError{{Field: "filter", Code: "invalid_shape"}}}, status: http.StatusUnprocessableEntity, code: ErrorCodeValidation},
		{name: "forbidden", err: querydomain.ErrForbidden, status: http.StatusForbidden, code: ErrorCodeForbidden},
		{name: "not found", err: querydomain.ErrNotFound, status: http.StatusNotFound, code: ErrorCodeNotFound},
		{name: "expired", err: querydomain.ErrExpired, status: http.StatusGone, code: ErrorCodeQueryExpired},
		{name: "stale", err: querydomain.ErrStaleCatalog, status: http.StatusConflict, code: ErrorCodeQueryCatalogStale},
		{name: "rate", err: querydomain.ErrRateLimited, status: http.StatusTooManyRequests, code: ErrorCodeRateLimited},
		{name: "cost", err: querydomain.ErrCostLimit, status: http.StatusUnprocessableEntity, code: ErrorCodeQueryTooCostly},
		{name: "timeout", err: querydomain.ErrTimeout, status: http.StatusServiceUnavailable, code: ErrorCodeQueryTimeout},
		{name: "cancelled", err: querydomain.ErrCancelled, status: http.StatusConflict, code: ErrorCodeQueryCancelled},
		{name: "internal", err: errors.New("SELECT secret FROM profiles"), status: http.StatusInternalServerError, code: ErrorCodeInternal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeQueryHTTPService{err: test.err}
			var logs bytes.Buffer
			handler := queryHTTPFixture(t, service, slog.New(slog.NewJSONHandler(&logs, nil)))
			response := serveQueryRequest(handler, http.MethodGet, "/api/v1/query/catalog", "")
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+string(test.code)+`"`) {
				t.Fatalf("response = %d, %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "SELECT secret") {
				t.Fatal("internal Query details leaked to response")
			}
		})
	}
}

func TestQueryResultRejectsInvalidIdentifierAndPagination(t *testing.T) {
	handler := queryHTTPFixture(t, &fakeQueryHTTPService{}, authTestLogger())
	for _, path := range []string{
		"/api/v1/query/executions/not-a-uuid/result",
		"/api/v1/query/executions/11111111-1111-4111-8111-111111111111/result?limit=101",
		"/api/v1/query/executions/11111111-1111-4111-8111-111111111111/result?offset=-1",
	} {
		response := serveQueryRequest(handler, http.MethodGet, path, "")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("GET %s = %d, %s", path, response.Code, response.Body.String())
		}
	}
}

func serveQueryRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
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

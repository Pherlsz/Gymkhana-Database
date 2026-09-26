package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/releaseinfo"
)

func TestLiveHealth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	request.Header.Set("X-Request-ID", "attacker-controlled-secret")
	response := httptest.NewRecorder()

	New(logger, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header is empty")
	}
	if response.Header().Get("X-Request-ID") == request.Header.Get("X-Request-ID") {
		t.Fatal("server trusted the inbound X-Request-ID")
	}
}

func TestHealthExposesOnlySafeReleaseMetadata(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(logger, nil, Options{Release: releaseinfo.Info{
		Version: "sha-304b760a3fff",
		Commit:  "304b760a3fff934cefaf4bd36cb71bcebac060a2",
	}})
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	var payload healthResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload.Release.Version != "sha-304b760a3fff" || payload.Release.Revision != "304b760a3fff" {
		t.Fatalf("release = %#v", payload.Release)
	}
	if strings.Contains(response.Body.String(), "304b760a3fff934cefaf4bd36cb71bcebac060a2") {
		t.Fatal("health response exposed the full commit identifier")
	}
}

func TestReadyHealthRequiresDatabase(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()

	New(logger, nil).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	var payload healthResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload.Database != "unavailable" {
		t.Fatalf("database = %q, want unavailable", payload.Database)
	}
}

func TestUnknownRouteUsesStableErrorContract(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	response := httptest.NewRecorder()

	New(logger, nil).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	var payload errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != ErrorCodeNotFound {
		t.Fatalf("error code = %q, want %q", payload.Error.Code, ErrorCodeNotFound)
	}
	if payload.RequestID == "" {
		t.Fatal("request_id is empty")
	}
}

func TestMutationRequiresConfiguredApplicationOrigin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(logger, nil, Options{ApplicationURL: "https://app.example/path"})

	tests := []struct {
		name       string
		origin     string
		wantStatus int
	}{
		{name: "missing", wantStatus: http.StatusForbidden},
		{name: "different", origin: "https://attacker.example", wantStatus: http.StatusForbidden},
		{name: "configured", origin: "https://app.example", wantStatus: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.origin == "https://app.example" {
				if response.Header().Get("Access-Control-Allow-Origin") != test.origin || response.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Fatalf("CORS headers = %#v", response.Header())
				}
			}
		})
	}
}

func TestTrustedPreflightReturnsCredentialedPolicy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodOptions, "/api/admin/users/example/access", nil)
	request.Header.Set("Origin", "https://app.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	request.Header.Set("Access-Control-Request-Headers", "content-type")
	response := httptest.NewRecorder()

	New(logger, nil, Options{ApplicationURL: "https://app.example"}).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("allow origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), http.MethodPatch) {
		t.Fatalf("allow methods = %q", response.Header().Get("Access-Control-Allow-Methods"))
	}
	if !strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Content-Type") {
		t.Fatalf("allow headers = %q", response.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestLoopbackApplicationOriginTrustsLocalViteHosts(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(logger, nil, Options{ApplicationURL: "http://localhost:5173"})

	allowed := []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://[::1]:5173"}
	for _, origin := range allowed {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		request.Header.Set("Origin", origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code == http.StatusForbidden {
			t.Fatalf("origin %s rejected with 403", origin)
		}
		if response.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Fatalf("allow origin for %s = %q", origin, response.Header().Get("Access-Control-Allow-Origin"))
		}
	}

	denied := []string{"http://localhost:5174", "http://127.0.0.1:8080", "https://localhost:5173", "http://evil.example:5173"}
	for _, origin := range denied {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		request.Header.Set("Origin", origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("origin %s status = %d, want 403", origin, response.Code)
		}
	}
}

func TestSafeMethodReceivesCorsOnlyForTrustedOrigin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(logger, nil, Options{ApplicationURL: "https://app.example"})

	trustedRequest := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	trustedRequest.Header.Set("Origin", "https://app.example")
	trustedResponse := httptest.NewRecorder()
	handler.ServeHTTP(trustedResponse, trustedRequest)
	if trustedResponse.Code != http.StatusOK || trustedResponse.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("trusted response = %d, %#v", trustedResponse.Code, trustedResponse.Header())
	}

	untrustedRequest := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	untrustedRequest.Header.Set("Origin", "https://attacker.example")
	untrustedResponse := httptest.NewRecorder()
	handler.ServeHTTP(untrustedResponse, untrustedRequest)
	if untrustedResponse.Code != http.StatusOK {
		t.Fatalf("untrusted safe status = %d", untrustedResponse.Code)
	}
	if untrustedResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("untrusted allow origin = %q", untrustedResponse.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestDecodeJSONEnforcesMediaTypeAndSingleValue(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantCode    ErrorCode
	}{
		{name: "media type", contentType: "text/plain", body: `{}`, wantCode: ErrorCodeUnsupportedMedia},
		{name: "unknown field", contentType: "application/json", body: `{"extra":true}`, wantCode: ErrorCodeInvalidJSON},
		{name: "multiple values", contentType: "application/json", body: `{} {}`, wantCode: ErrorCodeInvalidJSON},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			var destination struct {
				Name string `json:"name"`
			}
			problem := DecodeJSON(response, request, &destination)
			if problem == nil || problem.Code != test.wantCode {
				t.Fatalf("problem = %#v, want code %q", problem, test.wantCode)
			}
		})
	}
}

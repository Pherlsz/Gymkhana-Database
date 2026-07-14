package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveHealth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()

	New(logger, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID header is empty")
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

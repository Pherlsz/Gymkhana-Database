package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/modelprovider"
)

type fakeModelKeyService struct {
	status  modelprovider.KeyStatus
	secret  string
	model   string
	cleared bool
	err     error
}

func (service *fakeModelKeyService) Status(context.Context, auth.Session, string) (modelprovider.KeyStatus, error) {
	return service.status, service.err
}

func (service *fakeModelKeyService) Set(_ context.Context, _ auth.Session, provider, secret, model string) (modelprovider.KeyStatus, error) {
	service.secret, service.model = secret, model
	return modelprovider.KeyStatus{Provider: provider, Configured: true, Model: model}, service.err
}

func (service *fakeModelKeyService) Clear(context.Context, auth.Session, string) error {
	service.cleared = true
	return service.err
}

func modelKeyRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	return request
}

func TestModelKeyRoutesNeverEchoTheSecret(t *testing.T) {
	actorID, _ := auth.NewIdentifier()
	authentication := &fakeAdministrationService{fakeAuthenticationService: fakeAuthenticationService{session: auth.Session{User: auth.User{
		ID: actorID, Email: "owner", Role: auth.RoleAdmin, Active: true,
	}}}}
	keys := &fakeModelKeyService{status: modelprovider.KeyStatus{Provider: "google"}}
	handler := New(authTestLogger(), nil, Options{Auth: authentication, ModelKeys: keys})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, modelKeyRequest(http.MethodGet, "/api/admin/model-keys/google", ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"configured":false`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, modelKeyRequest(http.MethodPut, "/api/admin/model-keys/google", `{"secret":"AIzaSy-test-secret-0123456789abcdef","model":"gemini-2.5-flash"}`))
	if response.Code != http.StatusOK || keys.secret != "AIzaSy-test-secret-0123456789abcdef" || keys.model != "gemini-2.5-flash" {
		t.Fatalf("put status = %d, body = %s, stored = %q/%q", response.Code, response.Body.String(), keys.secret, keys.model)
	}
	if strings.Contains(response.Body.String(), "AIzaSy") {
		t.Fatalf("secret echoed in response: %s", response.Body.String())
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, modelKeyRequest(http.MethodDelete, "/api/admin/model-keys/google", ""))
	if response.Code != http.StatusNoContent || !keys.cleared {
		t.Fatalf("delete status = %d, cleared = %v", response.Code, keys.cleared)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, modelKeyRequest(http.MethodGet, "/api/admin/model-keys/openai", ""))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unsupported provider status = %d", response.Code)
	}

	keys.err = modelprovider.ErrForbidden
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, modelKeyRequest(http.MethodPut, "/api/admin/model-keys/google", `{"secret":"AIzaSy-test-secret-0123456789abcdef","model":"gemini"}`))
	if response.Code != http.StatusForbidden {
		t.Fatalf("forbidden status = %d", response.Code)
	}
}

func TestModelKeyRoutesReportUnconfiguredService(t *testing.T) {
	handler := New(authTestLogger(), nil, Options{Auth: &fakeAdministrationService{}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, modelKeyRequest(http.MethodGet, "/api/admin/model-keys/google", ""))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

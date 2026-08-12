package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func TestCapabilityGateFailsClosedWhenAuthenticationHasNoChecker(t *testing.T) {
	service := &fakeAuthenticationService{
		session: auth.Session{User: auth.User{Role: auth.RoleExternal, Active: true}},
	}
	handler := New(authTestLogger(), nil, Options{
		Auth:                   service,
		RequireCapabilityCheck: true,
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestCapabilityGateKeepsUnauthenticatedTestModeAvailable(t *testing.T) {
	nextCalled := false
	handler := requireCapability(auth.CapProfiles, nil, nil, func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/profiles", nil))

	if response.Code != http.StatusNoContent || !nextCalled {
		t.Fatalf("status = %d, nextCalled = %t", response.Code, nextCalled)
	}
}

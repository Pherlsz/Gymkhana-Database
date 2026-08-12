package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type fakeDevelopmentAuthenticationService struct {
	developmentLoginCalls int
}

func (service *fakeDevelopmentAuthenticationService) BeginLogin() (string, string, error) {
	return "oauth-state", "https://accounts.google.com/o/oauth2/auth", nil
}

func (service *fakeDevelopmentAuthenticationService) CompleteLogin(context.Context, string, string) (auth.LoginResult, error) {
	return auth.LoginResult{}, nil
}

func (service *fakeDevelopmentAuthenticationService) DevelopmentLogin(context.Context, string) (auth.LoginResult, error) {
	service.developmentLoginCalls++
	return auth.LoginResult{
		SessionValue: "development-session",
		ExpiresAt:    time.Now().Add(auth.SessionTTL),
	}, nil
}

func (service *fakeDevelopmentAuthenticationService) CurrentSession(context.Context, string) (auth.Session, error) {
	return auth.Session{}, auth.ErrUnauthenticated
}

func (service *fakeDevelopmentAuthenticationService) SignOut(context.Context, string, string) error {
	return nil
}

func TestDevelopmentLoginCreatesSessionWhenCookiesAreNotSecure(t *testing.T) {
	service := &fakeDevelopmentAuthenticationService{}
	handler := New(authTestLogger(), nil, Options{Auth: service, SecureCookies: false})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/dev-login", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("development login status = %d", response.Code)
	}
	if service.developmentLoginCalls != 1 {
		t.Fatalf("development login calls = %d", service.developmentLoginCalls)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.MaxAge > 0 {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("development login did not set a session cookie")
	}
	if sessionCookie.Secure {
		t.Fatal("development session cookie must not require HTTPS locally")
	}
	if !sessionCookie.HttpOnly {
		t.Fatal("development session cookie must remain HttpOnly")
	}
}

func TestDevelopmentLoginRouteIsAbsentWithSecureCookies(t *testing.T) {
	service := &fakeDevelopmentAuthenticationService{}
	handler := New(authTestLogger(), nil, Options{Auth: service, SecureCookies: true})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/auth/dev-login", nil))

	// The development route is not registered. Unknown /api paths are handled by
	// the server's generic API fallback, whose established contract is 405.
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("production development login status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if service.developmentLoginCalls != 0 {
		t.Fatalf("production development login calls = %d", service.developmentLoginCalls)
	}
}

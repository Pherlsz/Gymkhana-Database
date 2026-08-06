package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type fakeAuthenticationService struct {
	loginResult auth.LoginResult
	session     auth.Session
	sessionErr  error
	signedOut   string
}

func (service *fakeAuthenticationService) BeginLogin() (string, string, error) {
	return "oauth-state", "https://github.example/authorize", nil
}

func (service *fakeAuthenticationService) CompleteLogin(context.Context, string, string) (auth.LoginResult, error) {
	return service.loginResult, nil
}

func (service *fakeAuthenticationService) CurrentSession(context.Context, string) (auth.Session, error) {
	return service.session, service.sessionErr
}

func (service *fakeAuthenticationService) SignOut(_ context.Context, value, _ string) error {
	service.signedOut = value
	return nil
}

func authTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAuthenticationLoginCallbackAndProtectedSession(t *testing.T) {
	service := &fakeAuthenticationService{
		loginResult: auth.LoginResult{SessionValue: "session-value", ExpiresAt: time.Now().Add(auth.SessionTTL)},
		session:     auth.Session{User: auth.User{Email: "member", DisplayName: "Member Name", Role: auth.RoleExternal, Active: true}},
	}
	handler := New(authTestLogger(), nil, Options{Auth: service, ApplicationURL: "https://app.example"})

	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if loginResponse.Code != http.StatusFound || loginResponse.Header().Get("Location") != "https://github.example/authorize" {
		t.Fatalf("login response = %d, %q", loginResponse.Code, loginResponse.Header().Get("Location"))
	}
	stateCookie := loginResponse.Result().Cookies()[0]

	callbackRequest := httptest.NewRequest(http.MethodGet, "/auth/callback?code=code&state=oauth-state", nil)
	callbackRequest.AddCookie(stateCookie)
	callbackResponse := httptest.NewRecorder()
	handler.ServeHTTP(callbackResponse, callbackRequest)
	if callbackResponse.Code != http.StatusFound || callbackResponse.Header().Get("Location") != "https://app.example" {
		t.Fatalf("callback response = %d, %q", callbackResponse.Code, callbackResponse.Header().Get("Location"))
	}

	var sessionCookie *http.Cookie
	for _, cookie := range callbackResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.MaxAge > 0 {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly {
		t.Fatalf("session cookie = %#v", sessionCookie)
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionRequest.AddCookie(sessionCookie)
	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK {
		t.Fatalf("session status = %d", sessionResponse.Code)
	}
}

func TestAuthenticationRejectsInvalidStateAndMissingSession(t *testing.T) {
	handler := New(authTestLogger(), nil, Options{Auth: &fakeAuthenticationService{sessionErr: auth.ErrUnauthenticated}})

	callbackRequest := httptest.NewRequest(http.MethodGet, "/auth/callback?code=code&state=wrong", nil)
	callbackRequest.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "expected"})
	callbackResponse := httptest.NewRecorder()
	handler.ServeHTTP(callbackResponse, callbackRequest)
	if callbackResponse.Code != http.StatusBadRequest {
		t.Fatalf("callback status = %d", callbackResponse.Code)
	}

	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, httptest.NewRequest(http.MethodGet, "/api/auth/session", nil))
	if sessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("session status = %d", sessionResponse.Code)
	}
}

func TestAuthenticationLogoutIsIdempotent(t *testing.T) {
	service := &fakeAuthenticationService{}
	handler := New(authTestLogger(), nil, Options{Auth: service})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-value"})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent || service.signedOut != "session-value" {
		t.Fatalf("logout = %d, %q", response.Code, service.signedOut)
	}
}

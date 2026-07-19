package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGoogleProviderBuildsAuthorizationURLAndExchangesVerifiedIdentity(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("Content-Type = %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "oauth-code" {
			t.Fatalf("token form = %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "access-token", "token_type": "Bearer"})
	})
	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":            "google-subject-42",
			"email":          "Member@Example.com",
			"email_verified": true,
			"name":           "Member Name",
			"picture":        "https://example.test/avatar.png",
		})
	})

	provider, err := NewGoogleProvider(GoogleProviderOptions{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://database.example/auth/callback",
		HTTPClient:   server.Client(),
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
		UserInfoURL:  server.URL + "/userinfo",
	})
	if err != nil {
		t.Fatalf("NewGoogleProvider() error = %v", err)
	}

	authorizationURL, err := url.Parse(provider.AuthorizationURL("oauth-state"))
	if err != nil {
		t.Fatalf("parse authorization url: %v", err)
	}
	query := authorizationURL.Query()
	if query.Get("state") != "oauth-state" || query.Get("scope") != "openid email profile" || query.Get("prompt") != "select_account" {
		t.Fatalf("authorization query = %v", query)
	}

	identity, err := provider.Exchange(context.Background(), "oauth-code")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if identity.Subject != "google-subject-42" || identity.Email != "Member@Example.com" || !identity.EmailVerified || identity.DisplayName != "Member Name" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestGoogleProviderRejectsUnverifiedEmail(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "access-token"})
	})
	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": "subject", "email": "member@example.com", "email_verified": false})
	})
	provider, err := NewGoogleProvider(GoogleProviderOptions{
		ClientID: "client", ClientSecret: "secret", RedirectURL: "https://database.example/auth/callback",
		HTTPClient: server.Client(), AuthorizeURL: server.URL + "/authorize", TokenURL: server.URL + "/token", UserInfoURL: server.URL + "/userinfo",
	})
	if err != nil {
		t.Fatalf("NewGoogleProvider() error = %v", err)
	}
	if _, err := provider.Exchange(context.Background(), "code"); err == nil {
		t.Fatal("Exchange() error = nil")
	}
}

func TestGoogleProviderRejectsIncompleteConfiguration(t *testing.T) {
	if _, err := NewGoogleProvider(GoogleProviderOptions{}); err == nil {
		t.Fatal("NewGoogleProvider() error = nil")
	}
}

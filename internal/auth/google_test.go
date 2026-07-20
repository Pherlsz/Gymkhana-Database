package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGoogleProviderAuthorizationURL(t *testing.T) {
	provider, err := NewGoogleProvider(GoogleProviderOptions{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8080/auth/callback",
		AuthorizeURL: "https://accounts.example.test/authorize",
	})
	if err != nil {
		t.Fatalf("NewGoogleProvider returned error: %v", err)
	}

	authorizationURL, err := url.Parse(provider.AuthorizationURL("state-value"))
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	if authorizationURL.Host != "accounts.example.test" {
		t.Fatalf("unexpected authorization host: %s", authorizationURL.Host)
	}
	query := authorizationURL.Query()
	if query.Get("client_id") != "client-id" || query.Get("state") != "state-value" {
		t.Fatalf("authorization URL omitted required values: %v", query)
	}
	if query.Get("scope") != "openid email profile" {
		t.Fatalf("unexpected scope: %q", query.Get("scope"))
	}
}

func TestGoogleProviderExchangeReturnsVerifiedIdentity(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token form: %v", err)
		}
		if r.Form.Get("code") != "valid-code" || r.Form.Get("grant_type") != "authorization_code" {
			t.Fatalf("unexpected token request: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "access-token", "token_type": "Bearer"})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":            "google-subject",
			"email":          "User@Example.com",
			"email_verified": true,
			"name":           "Test User",
			"picture":        "https://example.test/avatar.png",
		})
	})

	provider, err := NewGoogleProvider(GoogleProviderOptions{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8080/auth/callback",
		TokenURL:     server.URL + "/token",
		UserInfoURL:  server.URL + "/userinfo",
	})
	if err != nil {
		t.Fatalf("NewGoogleProvider returned error: %v", err)
	}

	identity, err := provider.Exchange(context.Background(), "valid-code")
	if err != nil {
		t.Fatalf("Exchange returned error: %v", err)
	}
	if identity.Subject != "google-subject" || identity.Email != "User@Example.com" || !identity.EmailVerified {
		t.Fatalf("unexpected Google identity: %+v", identity)
	}
}

func TestGoogleProviderRejectsUnverifiedEmail(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer"}`))
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"subject","email":"user@example.com","email_verified":false}`))
	})

	provider, err := NewGoogleProvider(GoogleProviderOptions{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8080/auth/callback",
		TokenURL:     server.URL + "/token",
		UserInfoURL:  server.URL + "/userinfo",
	})
	if err != nil {
		t.Fatalf("NewGoogleProvider returned error: %v", err)
	}
	_, err = provider.Exchange(context.Background(), "valid-code")
	if err == nil || !strings.Contains(err.Error(), "verified email") {
		t.Fatalf("expected verified-email error, got %v", err)
	}
}

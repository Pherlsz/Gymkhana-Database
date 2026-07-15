package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGitHubProviderBuildsAuthorizationURLAndExchangesIdentity(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("Accept = %q", r.Header.Get("Accept"))
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "access-token"})
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         42,
			"login":      "member",
			"name":       "Member Name",
			"avatar_url": "https://example.test/avatar.png",
		})
	})

	provider, err := NewGitHubProvider(GitHubProviderOptions{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://database.example/auth/callback",
		HTTPClient:   server.Client(),
		AuthorizeURL: server.URL + "/authorize",
		TokenURL:     server.URL + "/token",
		UserURL:      server.URL + "/user",
	})
	if err != nil {
		t.Fatalf("NewGitHubProvider() error = %v", err)
	}

	authorizationURL, err := url.Parse(provider.AuthorizationURL("oauth-state"))
	if err != nil {
		t.Fatalf("parse authorization url: %v", err)
	}
	if authorizationURL.Query().Get("state") != "oauth-state" || authorizationURL.Query().Get("scope") != "read:user" {
		t.Fatalf("authorization query = %v", authorizationURL.Query())
	}

	identity, err := provider.Exchange(context.Background(), "oauth-code")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if identity.UserID != 42 || identity.Login != "member" || identity.DisplayName != "Member Name" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestGitHubProviderRejectsIncompleteConfiguration(t *testing.T) {
	if _, err := NewGitHubProvider(GitHubProviderOptions{}); err == nil {
		t.Fatal("NewGitHubProvider() error = nil")
	}
}

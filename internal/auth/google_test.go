package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGoogleProvider_Exchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			response := map[string]any{
				"access_token": "test-access-token",
				"id_token":     "test-id-token",
				"token_type":   "Bearer",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		if r.URL.Path == "/userinfo" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-access-token" {
				t.Errorf("expected Authorization: Bearer test-access-token, got %s", auth)
			}
			response := map[string]any{
				"id":      "123456789",
				"email":   "test@example.com",
				"name":    "Test User",
				"picture": "https://example.com/avatar.jpg",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := &GoogleProvider{
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
		redirectURL:  "http://localhost/callback",
		tokenURL:     server.URL + "/token",
		userURL:      server.URL + "/userinfo",
		httpClient:   server.Client(),
	}

	identity, err := provider.Exchange(context.Background(), "test-code")
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}

	if identity.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", identity.Email, "test@example.com")
	}
	if identity.DisplayName != "Test User" {
		t.Errorf("DisplayName = %q, want %q", identity.DisplayName, "Test User")
	}
	if identity.Subject != "123456789" {
		t.Errorf("Subject = %q, want %q", identity.Subject, "123456789")
	}
	if identity.AvatarURL != "https://example.com/avatar.jpg" {
		t.Errorf("AvatarURL = %q, want %q", identity.AvatarURL, "https://example.com/avatar.jpg")
	}
}

func TestGoogleProvider_AuthorizationURL(t *testing.T) {
	provider := &GoogleProvider{
		clientID:    "test-client-id",
		redirectURL: "http://localhost/callback",
		authorizeURL: "https://accounts.google.com/o/oauth2/v2/auth",
	}

	url := provider.AuthorizationURL("test-state")

	if url == "" {
		t.Fatal("AuthorizationURL() returned empty string")
	}

	expected := "https://accounts.google.com/o/oauth2/v2/auth?access_type=offline&client_id=test-client-id&redirect_uri=http%3A%2F%2Flocalhost%2Fcallback&response_type=code&scope=email+profile&state=test-state"
	if url != expected {
		t.Errorf("AuthorizationURL() = %q, want %q", url, expected)
	}
}

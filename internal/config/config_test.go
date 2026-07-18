package config

import (
	"bytes"
	"encoding/base64"
	"testing"
	"time"
)

var configurationKeys = []string{
	"APP_ENV",
	"HTTP_ADDRESS",
	"HTTP_MAX_BODY_BYTES",
	"DATABASE_URL",
	"LOG_LEVEL",
	"SHUTDOWN_TIMEOUT",
	"AUTH_ENABLED",
	"GITHUB_OAUTH_CLIENT_ID",
	"GITHUB_OAUTH_CLIENT_SECRET",
	"GITHUB_OAUTH_REDIRECT_URL",
	"AUTH_APPLICATION_URL",
	"AUTH_ALLOWED_GITHUB_LOGINS",
	"AUTH_SUPERADMIN_GITHUB_LOGIN",
	"GOOGLE_FORMS_ENABLED",
	"GOOGLE_FORMS_OAUTH_CLIENT_ID",
	"GOOGLE_FORMS_OAUTH_CLIENT_SECRET",
	"GOOGLE_FORMS_OAUTH_REDIRECT_URL",
	"GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY",
	"GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS",
	"GOOGLE_FORMS_TOKEN_KEY_VERSION",
	"GOOGLE_FORMS_SYNC_INTERVAL",
	"GOOGLE_FORMS_RESPONSE_PAGE_SIZE",
	"AI_CHAT_ENABLED",
	"AI_CHAT_PROVIDER",
	"AI_CHAT_MODEL",
	"AI_CHAT_RETENTION",
}

func setValidGoogleForms(t *testing.T) {
	t.Helper()
	setValidLocalAuthentication(t)
	t.Setenv("GOOGLE_FORMS_ENABLED", "true")
	t.Setenv("GOOGLE_FORMS_OAUTH_CLIENT_ID", "forms-client-id")
	t.Setenv("GOOGLE_FORMS_OAUTH_CLIENT_SECRET", "forms-client-secret")
	t.Setenv("GOOGLE_FORMS_OAUTH_REDIRECT_URL", "http://localhost:8080/api/v1/google-forms/oauth/callback")
	t.Setenv("GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)))
}

func clearConfiguration(t *testing.T) {
	t.Helper()
	for _, key := range configurationKeys {
		t.Setenv(key, "")
	}
}

func setValidLocalAuthentication(t *testing.T) {
	t.Helper()
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://localhost/gymkhana")
	t.Setenv("GITHUB_OAUTH_CLIENT_ID", "client-id")
	t.Setenv("GITHUB_OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("GITHUB_OAUTH_REDIRECT_URL", "http://localhost:8080/auth/callback")
	t.Setenv("AUTH_APPLICATION_URL", "http://localhost:5173")
	t.Setenv("AUTH_ALLOWED_GITHUB_LOGINS", " Pherlsz, member,PHERLSZ ")
	t.Setenv("AUTH_SUPERADMIN_GITHUB_LOGIN", "Pherlsz")
}

func TestLoadUsesSafeTypedDefaults(t *testing.T) {
	clearConfiguration(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment != EnvironmentLocal {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, EnvironmentLocal)
	}
	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("HTTPAddress = %q, want :8080", cfg.HTTPAddress)
	}
	if cfg.HTTPMaxBodyBytes != defaultHTTPMaxBodyBytes {
		t.Fatalf("HTTPMaxBodyBytes = %d, want %d", cfg.HTTPMaxBodyBytes, defaultHTTPMaxBodyBytes)
	}
	if cfg.LogLevel != LogLevelInfo {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, LogLevelInfo)
	}
	if cfg.Auth.Enabled {
		t.Fatal("authentication is enabled by default in local development")
	}
	if cfg.GoogleForms.Enabled || cfg.GoogleForms.SyncInterval != 15*time.Minute || cfg.GoogleForms.ResponsePageSize != 100 {
		t.Fatalf("GoogleForms defaults = %#v", cfg.GoogleForms)
	}
	if cfg.AIChat.Enabled || cfg.AIChat.Provider != "" || cfg.AIChat.Model != "" || cfg.AIChat.Retention != 0 {
		t.Fatalf("AIChat defaults = %#v", cfg.AIChat)
	}
}

func TestLoadRejectsInvalidTypedValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "environment", key: "APP_ENV", value: "preview"},
		{name: "address", key: "HTTP_ADDRESS", value: "8080"},
		{name: "body limit", key: "HTTP_MAX_BODY_BYTES", value: "0"},
		{name: "log level", key: "LOG_LEVEL", value: "verbose"},
		{name: "shutdown timeout", key: "SHUTDOWN_TIMEOUT", value: "10m"},
		{name: "auth enabled", key: "AUTH_ENABLED", value: "sometimes"},
		{name: "google forms enabled", key: "GOOGLE_FORMS_ENABLED", value: "sometimes"},
		{name: "google forms key version", key: "GOOGLE_FORMS_TOKEN_KEY_VERSION", value: "zero"},
		{name: "google forms interval", key: "GOOGLE_FORMS_SYNC_INTERVAL", value: "later"},
		{name: "google forms page size", key: "GOOGLE_FORMS_RESPONSE_PAGE_SIZE", value: "many"},
		{name: "AI Chat enabled", key: "AI_CHAT_ENABLED", value: "sometimes"},
		{name: "AI Chat retention", key: "AI_CHAT_RETENTION", value: "forever"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConfiguration(t)
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}
}

func TestLoadAllowsOnlyExplicitDeterministicTestChatConfiguration(t *testing.T) {
	clearConfiguration(t)
	setValidLocalAuthentication(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("AI_CHAT_ENABLED", "true")
	t.Setenv("AI_CHAT_PROVIDER", "fake")
	t.Setenv("AI_CHAT_MODEL", "deterministic-v1")
	t.Setenv("AI_CHAT_RETENTION", "24h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.AIChat.Enabled || cfg.AIChat.Provider != "fake" || cfg.AIChat.Model != "deterministic-v1" || cfg.AIChat.Retention != 24*time.Hour {
		t.Fatalf("AIChat = %#v", cfg.AIChat)
	}
}

func TestLoadKeepsProductionChatBlockedUntilOwnerActivationDecisions(t *testing.T) {
	for _, test := range []struct {
		name        string
		environment string
		provider    string
		model       string
		retention   string
	}{
		{name: "missing provider", environment: "test", model: "deterministic-v1", retention: "24h"},
		{name: "missing model", environment: "test", provider: "fake", retention: "24h"},
		{name: "missing retention", environment: "test", provider: "fake", model: "deterministic-v1"},
		{name: "short retention", environment: "test", provider: "fake", model: "deterministic-v1", retention: "30m"},
		{name: "unsupported local adapter", environment: "local", provider: "fake", model: "deterministic-v1", retention: "24h"},
	} {
		t.Run(test.name, func(t *testing.T) {
			clearConfiguration(t)
			setValidLocalAuthentication(t)
			t.Setenv("APP_ENV", test.environment)
			t.Setenv("AI_CHAT_ENABLED", "true")
			t.Setenv("AI_CHAT_PROVIDER", test.provider)
			t.Setenv("AI_CHAT_MODEL", test.model)
			t.Setenv("AI_CHAT_RETENTION", test.retention)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want AI Chat activation error")
			}
		})
	}

	clearConfiguration(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("AI_CHAT_ENABLED", "true")
	t.Setenv("AI_CHAT_PROVIDER", "fake")
	t.Setenv("AI_CHAT_MODEL", "deterministic-v1")
	t.Setenv("AI_CHAT_RETENTION", "24h")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want authentication dependency error")
	}
}

func TestLoadValidatesEnabledGoogleForms(t *testing.T) {
	clearConfiguration(t)
	setValidGoogleForms(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.GoogleForms.Enabled || cfg.GoogleForms.TokenKeyVersion != 1 {
		t.Fatalf("GoogleForms = %#v", cfg.GoogleForms)
	}
	if cfg.GoogleForms.TokenEncryptionKey == ([32]byte{}) {
		t.Fatal("Google Forms token key was not decoded")
	}
}

func TestLoadRejectsUnsafeGoogleFormsConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "missing client", key: "GOOGLE_FORMS_OAUTH_CLIENT_ID", value: ""},
		{name: "missing secret", key: "GOOGLE_FORMS_OAUTH_CLIENT_SECRET", value: ""},
		{name: "wrong redirect path", key: "GOOGLE_FORMS_OAUTH_REDIRECT_URL", value: "http://localhost:8080/forms/callback"},
		{name: "redirect query", key: "GOOGLE_FORMS_OAUTH_REDIRECT_URL", value: "http://localhost:8080/api/v1/google-forms/oauth/callback?code=test"},
		{name: "short key", key: "GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY", value: base64.StdEncoding.EncodeToString(make([]byte, 16))},
		{name: "zero key version", key: "GOOGLE_FORMS_TOKEN_KEY_VERSION", value: "0"},
		{name: "short interval", key: "GOOGLE_FORMS_SYNC_INTERVAL", value: "1m"},
		{name: "large page", key: "GOOGLE_FORMS_RESPONSE_PAGE_SIZE", value: "501"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConfiguration(t)
			setValidGoogleForms(t)
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want Google Forms validation error")
			}
		})
	}

	clearConfiguration(t)
	t.Setenv("GOOGLE_FORMS_ENABLED", "true")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want authentication dependency error")
	}
}

func TestDecodeEncryptionKeysRetainsPreviousVersions(t *testing.T) {
	current := [32]byte{1}
	previous := make([]byte, 32)
	previous[0] = 2
	keys, err := decodeEncryptionKeys("1:"+base64.StdEncoding.EncodeToString(previous), 2, current)
	if err != nil {
		t.Fatalf("decodeEncryptionKeys() error = %v", err)
	}
	if len(keys) != 2 || keys[2] != current || keys[1][0] != 2 {
		t.Fatalf("decodeEncryptionKeys() = %#v", keys)
	}
}

func TestLoadRequiresDatabaseOutsideLocalAndTest(t *testing.T) {
	for _, environment := range []string{"staging", "production"} {
		t.Run(environment, func(t *testing.T) {
			clearConfiguration(t)
			t.Setenv("APP_ENV", environment)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want DATABASE_URL validation error")
			}
		})
	}
}

func TestLoadValidatesEnabledAuthentication(t *testing.T) {
	clearConfiguration(t)
	setValidLocalAuthentication(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Auth.Enabled {
		t.Fatal("authentication is disabled")
	}
	if len(cfg.Auth.AllowedLogins) != 2 || cfg.Auth.AllowedLogins[0] != "pherlsz" {
		t.Fatalf("AllowedLogins = %#v", cfg.Auth.AllowedLogins)
	}
	if cfg.Auth.SuperadminLogin != "pherlsz" {
		t.Fatalf("SuperadminLogin = %q", cfg.Auth.SuperadminLogin)
	}
	if cfg.Auth.SecureCookies {
		t.Fatal("local cookies are unexpectedly secure")
	}
}

func TestLoadRejectsUnsafeAuthenticationURLs(t *testing.T) {
	tests := []struct {
		name           string
		redirectURL    string
		applicationURL string
	}{
		{name: "redirect scheme", redirectURL: "ftp://localhost/auth/callback"},
		{name: "redirect path", redirectURL: "http://localhost:8080/callback"},
		{name: "redirect credentials", redirectURL: "http://user:pass@localhost:8080/auth/callback"},
		{name: "redirect query", redirectURL: "http://localhost:8080/auth/callback?code=example"},
		{name: "redirect fragment", redirectURL: "http://localhost:8080/auth/callback#fragment"},
		{name: "application scheme", applicationURL: "file:///tmp/app"},
		{name: "application credentials", applicationURL: "http://user:pass@localhost:5173"},
		{name: "application query", applicationURL: "http://localhost:5173?mode=admin"},
		{name: "application fragment", applicationURL: "http://localhost:5173#admin"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearConfiguration(t)
			setValidLocalAuthentication(t)
			if test.redirectURL != "" {
				t.Setenv("GITHUB_OAUTH_REDIRECT_URL", test.redirectURL)
			}
			if test.applicationURL != "" {
				t.Setenv("AUTH_APPLICATION_URL", test.applicationURL)
			}
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want URL validation error")
			}
		})
	}
}

func TestLoadRequiresSecureCompleteAuthenticationOutsideDevelopment(t *testing.T) {
	clearConfiguration(t)
	setValidLocalAuthentication(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://database/gymkhana")
	t.Setenv("GITHUB_OAUTH_REDIRECT_URL", "https://api.database.example/auth/callback")
	t.Setenv("AUTH_APPLICATION_URL", "https://database.example")
	t.Setenv("AUTH_ALLOWED_GITHUB_LOGINS", "pherlsz")
	t.Setenv("AUTH_SUPERADMIN_GITHUB_LOGIN", "pherlsz")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Auth.Enabled || !cfg.Auth.SecureCookies {
		t.Fatalf("Auth = %#v", cfg.Auth)
	}

	t.Setenv("AUTH_ENABLED", "false")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want production authentication validation error")
	}
}

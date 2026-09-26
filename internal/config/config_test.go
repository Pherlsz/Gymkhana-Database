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
	"GOOGLE_OAUTH_CLIENT_ID",
	"GOOGLE_OAUTH_CLIENT_SECRET",
	"GOOGLE_OAUTH_REDIRECT_URL",
	"AUTH_APPLICATION_URL",
	"AUTH_ALLOWED_EMAILS",
	"AUTH_SUPERADMIN_EMAIL",
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
	"OCR_ENABLED",
	"OCR_PROVIDER",
	"OCR_MODEL",
	"OCR_TIMEOUT",
	"OCR_MAX_REQUESTS_PER_HOUR",
	"OCR_MAX_PROVIDER_USAGE_PER_HOUR",
	"OCR_MAX_SOURCE_BYTES",
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
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "http://localhost:8080/auth/callback")
	t.Setenv("AUTH_APPLICATION_URL", "http://localhost:5173")
	t.Setenv("AUTH_ALLOWED_EMAILS", " pedro@example.com, member@example.com ")
	t.Setenv("AUTH_SUPERADMIN_EMAIL", "pedro@example.com")
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
	if cfg.OCR.Enabled || cfg.OCR.Provider != "" || cfg.OCR.Model != "" || cfg.OCR.Timeout != 90*time.Second ||
		cfg.OCR.MaximumRequests != 10 || cfg.OCR.MaximumProviderUsage != 500_000 || cfg.OCR.MaximumSourceBytes != 20<<20 {
		t.Fatalf("OCR defaults = %#v", cfg.OCR)
	}
}

func TestLoadTreatsDevelopmentAsLocal(t *testing.T) {
	for _, value := range []string{"development", "dev", "DEVELOPMENT"} {
		t.Run(value, func(t *testing.T) {
			clearConfiguration(t)
			t.Setenv("APP_ENV", value)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Environment != EnvironmentLocal {
				t.Fatalf("Environment = %q, want %q", cfg.Environment, EnvironmentLocal)
			}
		})
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
		{name: "OCR enabled", key: "OCR_ENABLED", value: "sometimes"},
		{name: "OCR timeout", key: "OCR_TIMEOUT", value: "later"},
		{name: "OCR request limit", key: "OCR_MAX_REQUESTS_PER_HOUR", value: "many"},
		{name: "OCR usage limit", key: "OCR_MAX_PROVIDER_USAGE_PER_HOUR", value: "many"},
		{name: "OCR source limit", key: "OCR_MAX_SOURCE_BYTES", value: "many"},
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

func TestLoadAllowsOnlyExplicitDeterministicTestOCRConfiguration(t *testing.T) {
	clearConfiguration(t)
	setValidLocalAuthentication(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("OCR_ENABLED", "true")
	t.Setenv("OCR_PROVIDER", "fake")
	t.Setenv("OCR_MODEL", "deterministic-v1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.OCR.Enabled || cfg.OCR.Provider != "fake" || cfg.OCR.Model != "deterministic-v1" {
		t.Fatalf("OCR = %#v", cfg.OCR)
	}
}

func TestLoadKeepsProductionOCRBlockedUntilOwnerActivationDecision(t *testing.T) {
	for _, test := range []struct {
		name        string
		environment string
		provider    string
		model       string
	}{
		{name: "missing provider", environment: "test", model: "deterministic-v1"},
		{name: "missing model", environment: "test", provider: "fake"},
		{name: "unsupported local adapter", environment: "local", provider: "fake", model: "deterministic-v1"},
		{name: "unknown provider", environment: "production", provider: "vendor", model: "vision-v1"},
		{name: "google without sealing key", environment: "local", provider: "google", model: "gemini-2.5-flash"},
	} {
		t.Run(test.name, func(t *testing.T) {
			clearConfiguration(t)
			setValidLocalAuthentication(t)
			t.Setenv("APP_ENV", test.environment)
			t.Setenv("OCR_ENABLED", "true")
			t.Setenv("OCR_PROVIDER", test.provider)
			t.Setenv("OCR_MODEL", test.model)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want OCR activation error")
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
		{name: "unknown provider", environment: "local", provider: "openai", model: "gpt", retention: "24h"},
		{name: "google without sealing key", environment: "local", provider: "google", model: "gemini-2.5-flash", retention: "336h"},
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

func TestLoadAllowsGoogleChatProviderWithSealingKey(t *testing.T) {
	clearConfiguration(t)
	setValidLocalAuthentication(t)
	t.Setenv("APP_ENV", "local")
	t.Setenv("AI_CHAT_ENABLED", "true")
	t.Setenv("AI_CHAT_PROVIDER", "google")
	t.Setenv("AI_CHAT_MODEL", "gemini-2.5-flash")
	t.Setenv("AI_CHAT_RETENTION", "336h")
	t.Setenv("GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AIChat.Provider != "google" || cfg.AIChat.KeyVersion != 1 || cfg.AIChat.KeyEncryptionKeys[1] == ([32]byte{}) {
		t.Fatalf("AIChat = %#v", cfg.AIChat)
	}
}

func TestLoadAllowsGoogleOCRProviderWithSealingKey(t *testing.T) {
	clearConfiguration(t)
	setValidLocalAuthentication(t)
	t.Setenv("APP_ENV", "local")
	t.Setenv("OCR_ENABLED", "true")
	t.Setenv("OCR_PROVIDER", "google")
	t.Setenv("OCR_MODEL", "gemini-2.5-flash")
	t.Setenv("GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.OCR.Enabled || cfg.OCR.Provider != "google" || !cfg.UsesSharedModelKey() {
		t.Fatalf("OCR = %#v", cfg.OCR)
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
	if len(cfg.Auth.AllowedEmails) != 2 || cfg.Auth.AllowedEmails[0] != "pedro@example.com" {
		t.Fatalf("AllowedEmails = %#v", cfg.Auth.AllowedEmails)
	}
	if cfg.Auth.SuperadminEmail != "pedro@example.com" {
		t.Fatalf("SuperadminEmail = %q", cfg.Auth.SuperadminEmail)
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
				t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", test.redirectURL)
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
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "https://api.database.example/auth/callback")
	t.Setenv("AUTH_APPLICATION_URL", "https://database.example")
	t.Setenv("AUTH_ALLOWED_EMAILS", "admin@example.com")
	t.Setenv("AUTH_SUPERADMIN_EMAIL", "admin@example.com")

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

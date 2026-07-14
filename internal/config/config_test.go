package config

import "testing"

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
}

func clearConfiguration(t *testing.T) {
	t.Helper()
	for _, key := range configurationKeys {
		t.Setenv(key, "")
	}
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
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://localhost/gymkhana")
	t.Setenv("GITHUB_OAUTH_CLIENT_ID", "client-id")
	t.Setenv("GITHUB_OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("GITHUB_OAUTH_REDIRECT_URL", "http://localhost:8080/auth/callback")
	t.Setenv("AUTH_APPLICATION_URL", "http://localhost:5173")
	t.Setenv("AUTH_ALLOWED_GITHUB_LOGINS", " Pherlsz, member,PHERLSZ ")
	t.Setenv("AUTH_SUPERADMIN_GITHUB_LOGIN", "Pherlsz")

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

func TestLoadRequiresSecureCompleteAuthenticationOutsideDevelopment(t *testing.T) {
	clearConfiguration(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://database/gymkhana")
	t.Setenv("GITHUB_OAUTH_CLIENT_ID", "client-id")
	t.Setenv("GITHUB_OAUTH_CLIENT_SECRET", "client-secret")
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

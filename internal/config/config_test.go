package config

import "testing"

func TestLoadUsesSafeTypedDefaults(t *testing.T) {
	for _, key := range []string{"APP_ENV", "HTTP_ADDRESS", "HTTP_MAX_BODY_BYTES", "DATABASE_URL", "LOG_LEVEL", "SHUTDOWN_TIMEOUT"} {
		t.Setenv(key, "")
	}

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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, key := range []string{"APP_ENV", "HTTP_ADDRESS", "HTTP_MAX_BODY_BYTES", "DATABASE_URL", "LOG_LEVEL", "SHUTDOWN_TIMEOUT"} {
				t.Setenv(key, "")
			}
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
			t.Setenv("APP_ENV", environment)
			t.Setenv("DATABASE_URL", "")
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want DATABASE_URL validation error")
			}
		})
	}
}

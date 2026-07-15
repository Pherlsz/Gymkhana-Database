package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Environment string

const (
	EnvironmentLocal      Environment = "local"
	EnvironmentTest       Environment = "test"
	EnvironmentStaging    Environment = "staging"
	EnvironmentProduction Environment = "production"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

const defaultHTTPMaxBodyBytes int64 = 1 << 20

type AuthConfig struct {
	Enabled            bool
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string
	ApplicationURL     string
	AllowedLogins      []string
	SuperadminLogin    string
	SecureCookies      bool
}

type Config struct {
	Environment      Environment
	HTTPAddress      string
	HTTPMaxBodyBytes int64
	DatabaseURL      string
	LogLevel         LogLevel
	ShutdownTimeout  time.Duration
	Auth             AuthConfig
}

func Load() (Config, error) {
	environment := Environment(strings.ToLower(valueOrDefault("APP_ENV", string(EnvironmentLocal))))
	shutdownTimeout, err := time.ParseDuration(valueOrDefault("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}

	maxBodyBytes, err := strconv.ParseInt(valueOrDefault("HTTP_MAX_BODY_BYTES", strconv.FormatInt(defaultHTTPMaxBodyBytes, 10)), 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_MAX_BODY_BYTES: %w", err)
	}

	authEnabledDefault := environment == EnvironmentStaging || environment == EnvironmentProduction
	authEnabled, err := strconv.ParseBool(valueOrDefault("AUTH_ENABLED", strconv.FormatBool(authEnabledDefault)))
	if err != nil {
		return Config{}, fmt.Errorf("parse AUTH_ENABLED: %w", err)
	}

	cfg := Config{
		Environment:      environment,
		HTTPAddress:      valueOrDefault("HTTP_ADDRESS", ":8080"),
		HTTPMaxBodyBytes: maxBodyBytes,
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		LogLevel:         LogLevel(strings.ToLower(valueOrDefault("LOG_LEVEL", string(LogLevelInfo)))),
		ShutdownTimeout:  shutdownTimeout,
		Auth: AuthConfig{
			Enabled:            authEnabled,
			GitHubClientID:     strings.TrimSpace(os.Getenv("GITHUB_OAUTH_CLIENT_ID")),
			GitHubClientSecret: strings.TrimSpace(os.Getenv("GITHUB_OAUTH_CLIENT_SECRET")),
			GitHubRedirectURL:  strings.TrimSpace(os.Getenv("GITHUB_OAUTH_REDIRECT_URL")),
			ApplicationURL:     strings.TrimSpace(os.Getenv("AUTH_APPLICATION_URL")),
			AllowedLogins:      commaSeparatedValues(os.Getenv("AUTH_ALLOWED_GITHUB_LOGINS")),
			SuperadminLogin:    strings.ToLower(strings.TrimSpace(os.Getenv("AUTH_SUPERADMIN_GITHUB_LOGIN"))),
			SecureCookies:      environment == EnvironmentStaging || environment == EnvironmentProduction,
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg Config) validate() error {
	switch cfg.Environment {
	case EnvironmentLocal, EnvironmentTest, EnvironmentStaging, EnvironmentProduction:
	default:
		return fmt.Errorf("unsupported APP_ENV %q", cfg.Environment)
	}

	if cfg.HTTPAddress == "" {
		return errors.New("HTTP_ADDRESS cannot be empty")
	}
	if _, _, err := net.SplitHostPort(cfg.HTTPAddress); err != nil {
		return fmt.Errorf("invalid HTTP_ADDRESS %q: %w", cfg.HTTPAddress, err)
	}
	if cfg.HTTPMaxBodyBytes <= 0 {
		return errors.New("HTTP_MAX_BODY_BYTES must be positive")
	}
	if cfg.ShutdownTimeout <= 0 {
		return errors.New("SHUTDOWN_TIMEOUT must be positive")
	}
	if cfg.ShutdownTimeout > 5*time.Minute {
		return errors.New("SHUTDOWN_TIMEOUT cannot exceed 5m")
	}

	switch cfg.LogLevel {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
	default:
		return fmt.Errorf("unsupported LOG_LEVEL %q", cfg.LogLevel)
	}

	outsideDevelopment := cfg.Environment == EnvironmentStaging || cfg.Environment == EnvironmentProduction
	if outsideDevelopment && cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required outside local and test environments")
	}
	if outsideDevelopment && !cfg.Auth.Enabled {
		return errors.New("AUTH_ENABLED must be true outside local and test environments")
	}
	if !cfg.Auth.Enabled {
		return nil
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required when authentication is enabled")
	}
	if cfg.Auth.GitHubClientID == "" || cfg.Auth.GitHubClientSecret == "" {
		return errors.New("GitHub OAuth client credentials are required when authentication is enabled")
	}
	if cfg.Auth.GitHubRedirectURL == "" {
		return errors.New("GITHUB_OAUTH_REDIRECT_URL is required when authentication is enabled")
	}
	if cfg.Auth.ApplicationURL == "" {
		return errors.New("AUTH_APPLICATION_URL is required when authentication is enabled")
	}
	redirectURL, err := url.Parse(cfg.Auth.GitHubRedirectURL)
	if err != nil || !redirectURL.IsAbs() || redirectURL.Host == "" {
		return errors.New("GITHUB_OAUTH_REDIRECT_URL must be an absolute URL")
	}
	if outsideDevelopment && redirectURL.Scheme != "https" {
		return errors.New("GITHUB_OAUTH_REDIRECT_URL must use HTTPS outside local and test environments")
	}
	applicationURL, err := url.Parse(cfg.Auth.ApplicationURL)
	if err != nil || !applicationURL.IsAbs() || applicationURL.Host == "" {
		return errors.New("AUTH_APPLICATION_URL must be an absolute URL")
	}
	if outsideDevelopment && applicationURL.Scheme != "https" {
		return errors.New("AUTH_APPLICATION_URL must use HTTPS outside local and test environments")
	}
	if len(cfg.Auth.AllowedLogins) == 0 {
		return errors.New("AUTH_ALLOWED_GITHUB_LOGINS must contain at least one login")
	}
	if cfg.Auth.SuperadminLogin == "" {
		return errors.New("AUTH_SUPERADMIN_GITHUB_LOGIN is required when authentication is enabled")
	}
	allowed := false
	for _, login := range cfg.Auth.AllowedLogins {
		if login == cfg.Auth.SuperadminLogin {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("AUTH_SUPERADMIN_GITHUB_LOGIN must be included in AUTH_ALLOWED_GITHUB_LOGINS")
	}

	return nil
}

func valueOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func commaSeparatedValues(value string) []string {
	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		values = append(values, normalized)
	}
	return values
}

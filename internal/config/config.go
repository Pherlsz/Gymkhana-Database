package config

import (
	"encoding/base64"
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

type GoogleFormsConfig struct {
	Enabled             bool
	ClientID            string
	ClientSecret        string
	RedirectURL         string
	TokenEncryptionKey  [32]byte
	TokenEncryptionKeys map[uint16][32]byte
	TokenKeyVersion     uint16
	SyncInterval        time.Duration
	ResponsePageSize    int
}

type Config struct {
	Environment      Environment
	HTTPAddress      string
	HTTPMaxBodyBytes int64
	DatabaseURL      string
	LogLevel         LogLevel
	ShutdownTimeout  time.Duration
	Auth             AuthConfig
	GoogleForms      GoogleFormsConfig
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
	googleFormsEnabled, err := strconv.ParseBool(valueOrDefault("GOOGLE_FORMS_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("parse GOOGLE_FORMS_ENABLED: %w", err)
	}
	googleFormsKeyVersion, err := strconv.ParseUint(valueOrDefault("GOOGLE_FORMS_TOKEN_KEY_VERSION", "1"), 10, 16)
	if err != nil {
		return Config{}, fmt.Errorf("parse GOOGLE_FORMS_TOKEN_KEY_VERSION: %w", err)
	}
	googleFormsSyncInterval, err := time.ParseDuration(valueOrDefault("GOOGLE_FORMS_SYNC_INTERVAL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("parse GOOGLE_FORMS_SYNC_INTERVAL: %w", err)
	}
	googleFormsPageSize, err := strconv.Atoi(valueOrDefault("GOOGLE_FORMS_RESPONSE_PAGE_SIZE", "100"))
	if err != nil {
		return Config{}, fmt.Errorf("parse GOOGLE_FORMS_RESPONSE_PAGE_SIZE: %w", err)
	}
	googleFormsKey, err := decodeEncryptionKey(strings.TrimSpace(os.Getenv("GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY")))
	if err != nil {
		return Config{}, fmt.Errorf("parse GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY: %w", err)
	}
	googleFormsKeys, err := decodeEncryptionKeys(strings.TrimSpace(os.Getenv("GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS")), uint16(googleFormsKeyVersion), googleFormsKey)
	if err != nil {
		return Config{}, fmt.Errorf("parse GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS: %w", err)
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
		GoogleForms: GoogleFormsConfig{
			Enabled:             googleFormsEnabled,
			ClientID:            strings.TrimSpace(os.Getenv("GOOGLE_FORMS_OAUTH_CLIENT_ID")),
			ClientSecret:        strings.TrimSpace(os.Getenv("GOOGLE_FORMS_OAUTH_CLIENT_SECRET")),
			RedirectURL:         strings.TrimSpace(os.Getenv("GOOGLE_FORMS_OAUTH_REDIRECT_URL")),
			TokenEncryptionKey:  googleFormsKey,
			TokenEncryptionKeys: googleFormsKeys,
			TokenKeyVersion:     uint16(googleFormsKeyVersion),
			SyncInterval:        googleFormsSyncInterval,
			ResponsePageSize:    googleFormsPageSize,
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
	if cfg.GoogleForms.SyncInterval < 5*time.Minute || cfg.GoogleForms.SyncInterval > 24*time.Hour {
		return errors.New("GOOGLE_FORMS_SYNC_INTERVAL must be between 5m and 24h")
	}
	if cfg.GoogleForms.ResponsePageSize < 1 || cfg.GoogleForms.ResponsePageSize > 500 {
		return errors.New("GOOGLE_FORMS_RESPONSE_PAGE_SIZE must be between 1 and 500")
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
		if cfg.GoogleForms.Enabled {
			return errors.New("GOOGLE_FORMS_ENABLED requires authentication")
		}
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
	if err != nil || !validHTTPURL(redirectURL) {
		return errors.New("GITHUB_OAUTH_REDIRECT_URL must be an absolute HTTP(S) URL")
	}
	if redirectURL.User != nil || redirectURL.RawQuery != "" || redirectURL.Fragment != "" || redirectURL.Path != "/auth/callback" {
		return errors.New("GITHUB_OAUTH_REDIRECT_URL must contain only the /auth/callback path")
	}
	if outsideDevelopment && redirectURL.Scheme != "https" {
		return errors.New("GITHUB_OAUTH_REDIRECT_URL must use HTTPS outside local and test environments")
	}
	applicationURL, err := url.Parse(cfg.Auth.ApplicationURL)
	if err != nil || !validHTTPURL(applicationURL) {
		return errors.New("AUTH_APPLICATION_URL must be an absolute HTTP(S) URL")
	}
	if applicationURL.User != nil || applicationURL.RawQuery != "" || applicationURL.Fragment != "" {
		return errors.New("AUTH_APPLICATION_URL cannot contain credentials, query parameters, or a fragment")
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
	if !cfg.GoogleForms.Enabled {
		return nil
	}
	if cfg.GoogleForms.ClientID == "" || cfg.GoogleForms.ClientSecret == "" {
		return errors.New("google forms OAuth client credentials are required when Google Forms is enabled")
	}
	if cfg.GoogleForms.TokenKeyVersion == 0 {
		return errors.New("GOOGLE_FORMS_TOKEN_KEY_VERSION must be positive")
	}
	if cfg.GoogleForms.TokenEncryptionKey == ([32]byte{}) {
		return errors.New("GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY is required when Google Forms is enabled")
	}
	if key, exists := cfg.GoogleForms.TokenEncryptionKeys[cfg.GoogleForms.TokenKeyVersion]; !exists || key != cfg.GoogleForms.TokenEncryptionKey {
		return errors.New("GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS must retain the current encryption key")
	}
	formsRedirectURL, err := url.Parse(cfg.GoogleForms.RedirectURL)
	if err != nil || !validHTTPURL(formsRedirectURL) {
		return errors.New("GOOGLE_FORMS_OAUTH_REDIRECT_URL must be an absolute HTTP(S) URL")
	}
	if formsRedirectURL.User != nil || formsRedirectURL.RawQuery != "" || formsRedirectURL.Fragment != "" || formsRedirectURL.Path != "/api/v1/google-forms/oauth/callback" {
		return errors.New("GOOGLE_FORMS_OAUTH_REDIRECT_URL must contain only the /api/v1/google-forms/oauth/callback path")
	}
	if outsideDevelopment && formsRedirectURL.Scheme != "https" {
		return errors.New("GOOGLE_FORMS_OAUTH_REDIRECT_URL must use HTTPS outside local and test environments")
	}

	return nil
}

func decodeEncryptionKey(value string) ([32]byte, error) {
	if value == "" {
		return [32]byte{}, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, errors.New("must be base64-encoded 32 bytes")
	}
	var key [32]byte
	copy(key[:], decoded)
	return key, nil
}

func decodeEncryptionKeys(value string, currentVersion uint16, currentKey [32]byte) (map[uint16][32]byte, error) {
	result := make(map[uint16][32]byte)
	if currentVersion > 0 && currentKey != ([32]byte{}) {
		result[currentVersion] = currentKey
	}
	if value == "" {
		return result, nil
	}
	for _, item := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("must use version:base64 entries")
		}
		parsed, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 16)
		if err != nil || parsed == 0 {
			return nil, errors.New("contains an invalid key version")
		}
		key, err := decodeEncryptionKey(strings.TrimSpace(parts[1]))
		if err != nil || key == ([32]byte{}) {
			return nil, errors.New("contains an invalid encryption key")
		}
		version := uint16(parsed)
		if existing, duplicate := result[version]; duplicate && existing != key {
			return nil, errors.New("contains conflicting keys for one version")
		}
		result[version] = key
	}
	return result, nil
}

func validHTTPURL(value *url.URL) bool {
	if value == nil || !value.IsAbs() || value.Host == "" {
		return false
	}
	return value.Scheme == "http" || value.Scheme == "https"
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

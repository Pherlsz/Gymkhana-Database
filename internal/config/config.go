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
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	ApplicationURL     string
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

type AIChatConfig struct {
	Enabled   bool
	Provider  string
	Model     string
	Retention time.Duration
	// The shared Administração model key is sealed with the same versioned
	// AES-256-GCM material as Google Forms tokens (GOOGLE_FORMS_TOKEN_*).
	KeyEncryptionKeys map[uint16][32]byte
	KeyVersion        uint16
}

// AIChatProviders lists the model providers with a production adapter.
var AIChatProviders = map[string]bool{"google": true}

// OCRProviders lists extractors that use the shared Administração model key.
var OCRProviders = map[string]bool{"google": true}

// UsesSharedModelKey reports whether Assistente or OCR needs ai_model_keys.
func (cfg Config) UsesSharedModelKey() bool {
	return cfg.AIChat.Enabled && AIChatProviders[cfg.AIChat.Provider] ||
		cfg.OCR.Enabled && OCRProviders[cfg.OCR.Provider]
}

type OCRConfig struct {
	Enabled              bool
	Provider             string
	Model                string
	Timeout              time.Duration
	MaximumRequests      int
	MaximumProviderUsage int64
	MaximumSourceBytes   int64
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
	AIChat           AIChatConfig
	OCR              OCRConfig
}

func Load() (Config, error) {
	LoadDotenv()
	environment := parseEnvironment()
	shutdownTimeout, err := envDuration("SHUTDOWN_TIMEOUT", "10s")
	if err != nil {
		return Config{}, err
	}
	maxBodyBytes, err := envInt64("HTTP_MAX_BODY_BYTES", defaultHTTPMaxBodyBytes)
	if err != nil {
		return Config{}, err
	}

	authEnabledDefault := environment == EnvironmentStaging || environment == EnvironmentProduction
	authEnabled, err := envBool("AUTH_ENABLED", authEnabledDefault)
	if err != nil {
		return Config{}, err
	}
	googleFormsEnabled, err := envBool("GOOGLE_FORMS_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	googleFormsKeyVersion, err := envUint16("GOOGLE_FORMS_TOKEN_KEY_VERSION", 1)
	if err != nil {
		return Config{}, err
	}
	googleFormsSyncInterval, err := envDuration("GOOGLE_FORMS_SYNC_INTERVAL", "15m")
	if err != nil {
		return Config{}, err
	}
	googleFormsPageSize, err := envInt("GOOGLE_FORMS_RESPONSE_PAGE_SIZE", 100)
	if err != nil {
		return Config{}, err
	}
	googleFormsKey, err := decodeEncryptionKey(strings.TrimSpace(os.Getenv("GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY")))
	if err != nil {
		return Config{}, fmt.Errorf("parse GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY: %w", err)
	}
	googleFormsKeys, err := decodeEncryptionKeys(strings.TrimSpace(os.Getenv("GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS")), googleFormsKeyVersion, googleFormsKey)
	if err != nil {
		return Config{}, fmt.Errorf("parse GOOGLE_FORMS_TOKEN_DECRYPTION_KEYS: %w", err)
	}
	aiChatEnabled, err := envBool("AI_CHAT_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	var aiChatRetention time.Duration
	if value := strings.TrimSpace(os.Getenv("AI_CHAT_RETENTION")); value != "" {
		aiChatRetention, err = time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse AI_CHAT_RETENTION: %w", err)
		}
	}
	ocrEnabled, err := envBool("OCR_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	ocrTimeout, err := envDuration("OCR_TIMEOUT", "90s")
	if err != nil {
		return Config{}, err
	}
	ocrMaximumRequests, err := envInt("OCR_MAX_REQUESTS_PER_HOUR", 10)
	if err != nil {
		return Config{}, err
	}
	ocrMaximumProviderUsage, err := envInt64("OCR_MAX_PROVIDER_USAGE_PER_HOUR", 500000)
	if err != nil {
		return Config{}, err
	}
	ocrMaximumSourceBytes, err := envInt64("OCR_MAX_SOURCE_BYTES", 20971520)
	if err != nil {
		return Config{}, err
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
			GoogleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID")),
			GoogleClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")),
			GoogleRedirectURL:  strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_REDIRECT_URL")),
			ApplicationURL:     strings.TrimSpace(os.Getenv("AUTH_APPLICATION_URL")),
			SecureCookies:      environment == EnvironmentStaging || environment == EnvironmentProduction,
		},
		GoogleForms: GoogleFormsConfig{
			Enabled:             googleFormsEnabled,
			ClientID:            strings.TrimSpace(os.Getenv("GOOGLE_FORMS_OAUTH_CLIENT_ID")),
			ClientSecret:        strings.TrimSpace(os.Getenv("GOOGLE_FORMS_OAUTH_CLIENT_SECRET")),
			RedirectURL:         strings.TrimSpace(os.Getenv("GOOGLE_FORMS_OAUTH_REDIRECT_URL")),
			TokenEncryptionKey:  googleFormsKey,
			TokenEncryptionKeys: googleFormsKeys,
			TokenKeyVersion:     googleFormsKeyVersion,
			SyncInterval:        googleFormsSyncInterval,
			ResponsePageSize:    googleFormsPageSize,
		},
		AIChat: AIChatConfig{
			Enabled: aiChatEnabled, Provider: strings.ToLower(strings.TrimSpace(os.Getenv("AI_CHAT_PROVIDER"))),
			Model: strings.TrimSpace(os.Getenv("AI_CHAT_MODEL")), Retention: aiChatRetention,
			KeyEncryptionKeys: googleFormsKeys, KeyVersion: googleFormsKeyVersion,
		},
		OCR: OCRConfig{
			Enabled: ocrEnabled, Provider: strings.ToLower(strings.TrimSpace(os.Getenv("OCR_PROVIDER"))),
			Model: strings.TrimSpace(os.Getenv("OCR_MODEL")), Timeout: ocrTimeout,
			MaximumRequests: ocrMaximumRequests, MaximumProviderUsage: ocrMaximumProviderUsage,
			MaximumSourceBytes: ocrMaximumSourceBytes,
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
	if cfg.OCR.Timeout < time.Second || cfg.OCR.Timeout > 5*time.Minute {
		return errors.New("OCR_TIMEOUT must be between 1s and 5m")
	}
	if cfg.OCR.MaximumRequests < 1 || cfg.OCR.MaximumRequests > 1000 {
		return errors.New("OCR_MAX_REQUESTS_PER_HOUR must be between 1 and 1000")
	}
	if cfg.OCR.MaximumProviderUsage < 1 || cfg.OCR.MaximumProviderUsage > 100_000_000 {
		return errors.New("OCR_MAX_PROVIDER_USAGE_PER_HOUR must be between 1 and 100000000")
	}
	if cfg.OCR.MaximumSourceBytes < 1 || cfg.OCR.MaximumSourceBytes > 20<<20 {
		return errors.New("OCR_MAX_SOURCE_BYTES must be between 1 and 20971520")
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
		if cfg.AIChat.Enabled {
			return errors.New("AI_CHAT_ENABLED requires authentication")
		}
		if cfg.OCR.Enabled {
			return errors.New("OCR_ENABLED requires authentication")
		}
		return nil
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required when authentication is enabled")
	}
	if cfg.Auth.GoogleClientID == "" || cfg.Auth.GoogleClientSecret == "" {
		return errors.New("google OAuth client credentials are required when authentication is enabled")
	}
	if cfg.Auth.GoogleRedirectURL == "" {
		return errors.New("GOOGLE_OAUTH_REDIRECT_URL is required when authentication is enabled")
	}
	if cfg.Auth.ApplicationURL == "" {
		return errors.New("AUTH_APPLICATION_URL is required when authentication is enabled")
	}
	redirectURL, err := url.Parse(cfg.Auth.GoogleRedirectURL)
	if err != nil || !validHTTPURL(redirectURL) {
		return errors.New("GOOGLE_OAUTH_REDIRECT_URL must be an absolute HTTP(S) URL")
	}
	if redirectURL.User != nil || redirectURL.RawQuery != "" || redirectURL.Fragment != "" || redirectURL.Path != "/auth/callback" {
		return errors.New("GOOGLE_OAUTH_REDIRECT_URL must contain only the /auth/callback path")
	}
	if outsideDevelopment && redirectURL.Scheme != "https" {
		return errors.New("GOOGLE_OAUTH_REDIRECT_URL must use HTTPS outside local and test environments")
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
	if cfg.AIChat.Enabled {
		if cfg.AIChat.Provider == "" || cfg.AIChat.Model == "" || cfg.AIChat.Retention == 0 {
			return errors.New("AI_CHAT_PROVIDER, AI_CHAT_MODEL and AI_CHAT_RETENTION are required when AI Chat is enabled")
		}
		if len(cfg.AIChat.Model) > 120 {
			return errors.New("AI_CHAT_MODEL cannot exceed 120 characters")
		}
		if cfg.AIChat.Retention < time.Hour || cfg.AIChat.Retention > 365*24*time.Hour {
			return errors.New("AI_CHAT_RETENTION must be between 1h and 8760h")
		}
		switch {
		case cfg.AIChat.Provider == "fake":
			if cfg.Environment != EnvironmentTest {
				return errors.New("AI_CHAT_PROVIDER=fake is accepted only with APP_ENV=test")
			}
		case AIChatProviders[cfg.AIChat.Provider]:
			if cfg.AIChat.KeyEncryptionKeys[cfg.AIChat.KeyVersion] == ([32]byte{}) {
				return errors.New("GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY is required to seal the shared model key when AI Chat is enabled")
			}
		default:
			return fmt.Errorf("unsupported AI_CHAT_PROVIDER %q", cfg.AIChat.Provider)
		}
	}
	if cfg.OCR.Enabled {
		if cfg.OCR.Provider == "" || cfg.OCR.Model == "" {
			return errors.New("OCR_PROVIDER and OCR_MODEL are required when OCR is enabled")
		}
		if len(cfg.OCR.Model) > 120 {
			return errors.New("OCR_MODEL cannot exceed 120 characters")
		}
		switch {
		case cfg.OCR.Provider == "fake":
			if cfg.Environment != EnvironmentTest {
				return errors.New("OCR_PROVIDER=fake is accepted only with APP_ENV=test")
			}
		case OCRProviders[cfg.OCR.Provider]:
			if cfg.AIChat.KeyEncryptionKeys[cfg.AIChat.KeyVersion] == ([32]byte{}) {
				return errors.New("GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY is required to seal the shared model key when OCR is enabled")
			}
		default:
			return fmt.Errorf("unsupported OCR_PROVIDER %q", cfg.OCR.Provider)
		}
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

func parseEnvironment() Environment {
	raw := strings.ToLower(valueOrDefault("APP_ENV", string(EnvironmentLocal)))
	switch raw {
	case "development", "dev":
		return EnvironmentLocal
	default:
		return Environment(raw)
	}
}

func valueOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) (bool, error) {
	v, err := strconv.ParseBool(valueOrDefault(key, strconv.FormatBool(fallback)))
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}
	return v, nil
}

func envDuration(key, fallback string) (time.Duration, error) {
	v, err := time.ParseDuration(valueOrDefault(key, fallback))
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return v, nil
}

func envInt(key string, fallback int) (int, error) {
	v, err := strconv.Atoi(valueOrDefault(key, strconv.Itoa(fallback)))
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return v, nil
}

func envInt64(key string, fallback int64) (int64, error) {
	v, err := strconv.ParseInt(valueOrDefault(key, strconv.FormatInt(fallback, 10)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return v, nil
}

func envUint16(key string, fallback uint16) (uint16, error) {
	v, err := strconv.ParseUint(valueOrDefault(key, strconv.FormatUint(uint64(fallback), 10)), 10, 16)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return uint16(v), nil
}

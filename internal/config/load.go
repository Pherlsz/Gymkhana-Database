package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

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
	aiChatEnabled, err := strconv.ParseBool(valueOrDefault("AI_CHAT_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("parse AI_CHAT_ENABLED: %w", err)
	}
	var aiChatRetention time.Duration
	if value := strings.TrimSpace(os.Getenv("AI_CHAT_RETENTION")); value != "" {
		aiChatRetention, err = time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse AI_CHAT_RETENTION: %w", err)
		}
	}
	ocrEnabled, err := strconv.ParseBool(valueOrDefault("OCR_ENABLED", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("parse OCR_ENABLED: %w", err)
	}
	ocrTimeout, err := time.ParseDuration(valueOrDefault("OCR_TIMEOUT", "90s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse OCR_TIMEOUT: %w", err)
	}
	ocrMaximumRequests, err := strconv.Atoi(valueOrDefault("OCR_MAX_REQUESTS_PER_HOUR", "10"))
	if err != nil {
		return Config{}, fmt.Errorf("parse OCR_MAX_REQUESTS_PER_HOUR: %w", err)
	}
	ocrMaximumProviderUsage, err := strconv.ParseInt(valueOrDefault("OCR_MAX_PROVIDER_USAGE_PER_HOUR", "500000"), 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("parse OCR_MAX_PROVIDER_USAGE_PER_HOUR: %w", err)
	}
	ocrMaximumSourceBytes, err := strconv.ParseInt(valueOrDefault("OCR_MAX_SOURCE_BYTES", "20971520"), 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("parse OCR_MAX_SOURCE_BYTES: %w", err)
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
			GoogleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_LOGIN_OAUTH_CLIENT_ID")),
			GoogleClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_LOGIN_OAUTH_CLIENT_SECRET")),
			GoogleRedirectURL:  strings.TrimSpace(os.Getenv("GOOGLE_LOGIN_OAUTH_REDIRECT_URL")),
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
			TokenKeyVersion:     uint16(googleFormsKeyVersion),
			SyncInterval:        googleFormsSyncInterval,
			ResponsePageSize:    googleFormsPageSize,
		},
		AIChat: AIChatConfig{
			Enabled: aiChatEnabled,
			Provider: strings.ToLower(strings.TrimSpace(os.Getenv("AI_CHAT_PROVIDER"))),
			Model: strings.TrimSpace(os.Getenv("AI_CHAT_MODEL")),
			Retention: aiChatRetention,
		},
		OCR: OCRConfig{
			Enabled: ocrEnabled,
			Provider: strings.ToLower(strings.TrimSpace(os.Getenv("OCR_PROVIDER"))),
			Model: strings.TrimSpace(os.Getenv("OCR_MODEL")),
			Timeout: ocrTimeout,
			MaximumRequests: ocrMaximumRequests,
			MaximumProviderUsage: ocrMaximumProviderUsage,
			MaximumSourceBytes: ocrMaximumSourceBytes,
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

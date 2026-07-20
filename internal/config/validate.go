package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"
)

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
	if cfg.ShutdownTimeout <= 0 || cfg.ShutdownTimeout > 5*time.Minute {
		return errors.New("SHUTDOWN_TIMEOUT must be between 1ns and 5m")
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
		return errors.New("Google OAuth client credentials are required when authentication is enabled")
	}
	if cfg.Auth.GoogleRedirectURL == "" {
		return errors.New("GOOGLE_LOGIN_OAUTH_REDIRECT_URL is required when authentication is enabled")
	}
	if cfg.Auth.ApplicationURL == "" {
		return errors.New("AUTH_APPLICATION_URL is required when authentication is enabled")
	}
	redirectURL, err := url.Parse(cfg.Auth.GoogleRedirectURL)
	if err != nil || !validHTTPURL(redirectURL) {
		return errors.New("GOOGLE_LOGIN_OAUTH_REDIRECT_URL must be an absolute HTTP(S) URL")
	}
	if redirectURL.User != nil || redirectURL.RawQuery != "" || redirectURL.Fragment != "" || redirectURL.Path != "/auth/callback" {
		return errors.New("GOOGLE_LOGIN_OAUTH_REDIRECT_URL must contain only the /auth/callback path")
	}
	if outsideDevelopment && redirectURL.Scheme != "https" {
		return errors.New("GOOGLE_LOGIN_OAUTH_REDIRECT_URL must use HTTPS outside local and test environments")
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
		if cfg.Environment != EnvironmentTest || cfg.AIChat.Provider != "fake" {
			return errors.New("AI Chat has no production provider adapter; keep AI_CHAT_ENABLED=false until the owner selects and configures one")
		}
	}
	if cfg.OCR.Enabled {
		if cfg.OCR.Provider == "" || cfg.OCR.Model == "" {
			return errors.New("OCR_PROVIDER and OCR_MODEL are required when OCR is enabled")
		}
		if len(cfg.OCR.Model) > 120 {
			return errors.New("OCR_MODEL cannot exceed 120 characters")
		}
		if cfg.Environment != EnvironmentTest || cfg.OCR.Provider != "fake" {
			return errors.New("OCR has no production provider adapter; keep OCR_ENABLED=false until the owner selects and configures one")
		}
	}
	if !cfg.GoogleForms.Enabled {
		return nil
	}
	if cfg.GoogleForms.ClientID == "" || cfg.GoogleForms.ClientSecret == "" {
		return errors.New("Google Forms OAuth client credentials are required when Google Forms is enabled")
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

func validHTTPURL(value *url.URL) bool {
	if value == nil || !value.IsAbs() || value.Host == "" {
		return false
	}
	return value.Scheme == "http" || value.Scheme == "https"
}

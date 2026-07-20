package config

import "time"

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
}

type OCRConfig struct {
	Enabled              bool
	Provider             string
	Model                 string
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

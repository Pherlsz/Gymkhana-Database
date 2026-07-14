package config

import (
	"errors"
	"fmt"
	"net"
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

type Config struct {
	Environment      Environment
	HTTPAddress      string
	HTTPMaxBodyBytes int64
	DatabaseURL      string
	LogLevel         LogLevel
	ShutdownTimeout  time.Duration
}

func Load() (Config, error) {
	shutdownTimeout, err := time.ParseDuration(valueOrDefault("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}

	maxBodyBytes, err := strconv.ParseInt(valueOrDefault("HTTP_MAX_BODY_BYTES", strconv.FormatInt(defaultHTTPMaxBodyBytes, 10)), 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_MAX_BODY_BYTES: %w", err)
	}

	cfg := Config{
		Environment:      Environment(strings.ToLower(valueOrDefault("APP_ENV", string(EnvironmentLocal)))),
		HTTPAddress:      valueOrDefault("HTTP_ADDRESS", ":8080"),
		HTTPMaxBodyBytes: maxBodyBytes,
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		LogLevel:         LogLevel(strings.ToLower(valueOrDefault("LOG_LEVEL", string(LogLevelInfo)))),
		ShutdownTimeout:  shutdownTimeout,
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

	if (cfg.Environment == EnvironmentStaging || cfg.Environment == EnvironmentProduction) && cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required outside local and test environments")
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

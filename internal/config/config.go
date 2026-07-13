package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Environment     string
	HTTPAddress     string
	DatabaseURL     string
	LogLevel        string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	shutdownTimeout, err := time.ParseDuration(valueOrDefault("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}

	cfg := Config{
		Environment:     valueOrDefault("APP_ENV", "local"),
		HTTPAddress:     valueOrDefault("HTTP_ADDRESS", ":8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		LogLevel:        valueOrDefault("LOG_LEVEL", "info"),
		ShutdownTimeout: shutdownTimeout,
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg Config) validate() error {
	switch cfg.Environment {
	case "local", "test", "staging", "production":
	default:
		return fmt.Errorf("unsupported APP_ENV %q", cfg.Environment)
	}

	if cfg.HTTPAddress == "" {
		return errors.New("HTTP_ADDRESS cannot be empty")
	}
	if cfg.ShutdownTimeout <= 0 {
		return errors.New("SHUTDOWN_TIMEOUT must be positive")
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

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

func main() {
	if err := run(); err != nil {
		slog.Error("River migration command failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	action := flag.String("action", "migrate", "migrate or validate River's database schema")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required for River migrations")
	}
	ctx := context.Background()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if pool == nil {
		return errors.New("River migrations require a database connection")
	}
	defer pool.Close()
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("configure River migrator: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(*action)) {
	case "migrate":
		if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
			return fmt.Errorf("migrate River schema: %w", err)
		}
		return nil
	case "validate":
		result, err := migrator.Validate(ctx, nil)
		if err != nil {
			return fmt.Errorf("validate River schema: %w", err)
		}
		if !result.OK {
			return fmt.Errorf("River schema is not current: %s", strings.Join(result.Messages, "; "))
		}
		return nil
	default:
		return fmt.Errorf("unsupported River migration action %q", *action)
	}
}

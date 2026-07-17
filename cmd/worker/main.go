package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
)

func main() {
	drain := flag.Bool("drain", false, "process queued work and exit when the queue is empty")
	flag.Parse()
	if err := run(*drain); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(drain bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	storageCfg, err := config.LoadStorage()
	if err != nil {
		return err
	}
	logger := logging.New(string(cfg.LogLevel))
	slog.SetDefault(logger)
	if !drain {
		logger.Info("worker is configured for scheduled drain execution", "environment", cfg.Environment)
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runAttachmentCleanup(ctx, cfg, storageCfg, logger)
}

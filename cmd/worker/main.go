package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/logging"
)

func main() {
	drain := flag.Bool("drain", false, "process queued work and exit when the queue is empty")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}
	logger := logging.New(cfg.LogLevel)

	if *drain {
		logger.Info("worker drain completed", "registered_jobs", 0)
		return
	}

	logger.Info("worker bootstrap has no registered jobs", "environment", cfg.Environment)
}

package logging

import (
	"log/slog"
	"os"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/releaseinfo"
)

func New(level string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	build := releaseinfo.Current().Public()
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})).With(
		"build_version", build.Version,
		"build_revision", build.Revision,
	)
}

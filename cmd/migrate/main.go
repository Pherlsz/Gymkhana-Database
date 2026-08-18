package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Pherlsz/Gymkhana-Database/internal/platform/releaseinfo"
)

func main() {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	immutableRelease := environment == "staging" || environment == "production"
	if err := releaseinfo.Current().Validate(immutableRelease); err != nil {
		fmt.Fprintf(os.Stderr, "release identity validation failed: %v\n", err)
		os.Exit(1)
	}

	command := flag.String("command", "migrate", "tern command to execute")
	migrations := flag.String("migrations", "database/migrations", "migration directory")
	flag.Parse()

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ternPath, err := exec.LookPath("tern")
	if err != nil {
		fmt.Fprintln(os.Stderr, "tern executable was not found; run `make migrate` or install tern v2.4.1")
		os.Exit(1)
	}

	args := []string{*command, "--migrations", *migrations, "--conn-string", databaseURL}
	cmd := exec.CommandContext(ctx, ternPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "migration command failed: %v\n", err)
		os.Exit(1)
	}
}

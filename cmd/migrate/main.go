package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	command := flag.String("command", "migrate", "tern command to execute")
	migrations := flag.String("migrations", "database/migrations", "migration directory")
	configPath := flag.String("config", "database/tern.conf", "tern configuration file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ternPath, err := exec.LookPath("tern")
	if err != nil {
		fmt.Fprintln(os.Stderr, "tern executable was not found; run `make migrate` or install tern v2.4.1")
		os.Exit(1)
	}

	args := []string{*command, "--migrations", *migrations, "--config", *configPath}
	cmd := exec.CommandContext(ctx, ternPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "migration command failed: %v\n", err)
		os.Exit(1)
	}
}

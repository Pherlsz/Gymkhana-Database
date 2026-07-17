package main

import (
	"fmt"
	"os"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration invalid:", err)
		os.Exit(1)
	}
	storage, err := config.LoadStorage()
	if err != nil {
		fmt.Fprintln(os.Stderr, "storage configuration invalid:", err)
		os.Exit(1)
	}

	fmt.Printf(
		"configuration valid: environment=%s authentication_enabled=%t secure_cookies=%t allowed_logins=%d attachments_enabled=%t\n",
		cfg.Environment,
		cfg.Auth.Enabled,
		cfg.Auth.SecureCookies,
		len(cfg.Auth.AllowedLogins),
		storage.Enabled,
	)
}

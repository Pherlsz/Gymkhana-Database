package main

import (
	"fmt"
	"os"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/releaseinfo"
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
	build := releaseinfo.Current().Public()

	fmt.Printf(
		"configuration valid: environment=%s authentication_enabled=%t secure_cookies=%t attachments_enabled=%t build_version=%s build_revision=%s\n",
		cfg.Environment,
		cfg.Auth.Enabled,
		cfg.Auth.SecureCookies,
		storage.Enabled,
		build.Version,
		build.Revision,
	)
}

package config

import "os"

// The configuration struct still carries the original M2 field names so this
// migration can remain source-compatible with older tests and deployment code.
// Runtime configuration is Google-specific and the aliases below are applied
// before Load reads the environment. The sentinel values only satisfy the old
// configuration validator; authorization is enforced by pre-provisioned
// app_users rows in PostgreSQL.
func init() {
	aliasEnvironment("GITHUB_OAUTH_CLIENT_ID", "GOOGLE_LOGIN_OAUTH_CLIENT_ID")
	aliasEnvironment("GITHUB_OAUTH_CLIENT_SECRET", "GOOGLE_LOGIN_OAUTH_CLIENT_SECRET")
	aliasEnvironment("GITHUB_OAUTH_REDIRECT_URL", "GOOGLE_LOGIN_OAUTH_REDIRECT_URL")
	if os.Getenv("GITHUB_OAUTH_CLIENT_ID") != "" {
		if os.Getenv("AUTH_ALLOWED_GITHUB_LOGINS") == "" {
			_ = os.Setenv("AUTH_ALLOWED_GITHUB_LOGINS", "database-allowlist")
		}
		if os.Getenv("AUTH_SUPERADMIN_GITHUB_LOGIN") == "" {
			_ = os.Setenv("AUTH_SUPERADMIN_GITHUB_LOGIN", "database-allowlist")
		}
	}
}

func aliasEnvironment(legacyKey, googleKey string) {
	if os.Getenv(legacyKey) != "" {
		return
	}
	if value := os.Getenv(googleKey); value != "" {
		_ = os.Setenv(legacyKey, value)
	}
}

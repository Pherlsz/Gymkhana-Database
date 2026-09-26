package config

import (
	"os"
	"strings"
	"testing"
)

func TestParseDotenv(t *testing.T) {
	vars, err := parseDotenv(strings.NewReader(`
# comment
APP_ENV=local
export DATABASE_URL="postgres://localhost/gymkhana"
QUOTED='single'
EMPTY=
export SKIP_THIS
BROKEN LINE
 AUTH_ENABLED = true
`))
	if err != nil {
		t.Fatalf("parseDotenv() error = %v", err)
	}
	if vars["APP_ENV"] != "local" {
		t.Fatalf("APP_ENV = %q", vars["APP_ENV"])
	}
	if vars["DATABASE_URL"] != "postgres://localhost/gymkhana" {
		t.Fatalf("DATABASE_URL = %q", vars["DATABASE_URL"])
	}
	if vars["QUOTED"] != "single" {
		t.Fatalf("QUOTED = %q", vars["QUOTED"])
	}
	if vars["EMPTY"] != "" {
		t.Fatalf("EMPTY = %q", vars["EMPTY"])
	}
	if vars["AUTH_ENABLED"] != "true" {
		t.Fatalf("AUTH_ENABLED = %q", vars["AUTH_ENABLED"])
	}
	if _, ok := vars["SKIP_THIS"]; ok {
		t.Fatal("expected invalid export line to be ignored")
	}
}

func TestApplyDotenvSetsValues(t *testing.T) {
	t.Setenv("GYMKHANA_DOTENV_EXISTING", "from-process")
	t.Setenv("GYMKHANA_DOTENV_NEW", "")
	applyDotenv(map[string]string{
		"GYMKHANA_DOTENV_EXISTING": "from-file",
		"GYMKHANA_DOTENV_NEW":      "from-file",
	})
	if got := os.Getenv("GYMKHANA_DOTENV_EXISTING"); got != "from-file" {
		t.Fatalf("GYMKHANA_DOTENV_EXISTING = %q", got)
	}
	if got := os.Getenv("GYMKHANA_DOTENV_NEW"); got != "from-file" {
		t.Fatalf("GYMKHANA_DOTENV_NEW = %q", got)
	}
}

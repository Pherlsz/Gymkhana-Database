package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LoadDotenv reads the repository `.env` into the process environment.
// When the file exists it is the local source of truth (including Cursor Cloud
// on this workspace). Tests skip the file so fixtures stay isolated.
func LoadDotenv() {
	if testing.Testing() {
		return
	}
	path, ok := findDotenv()
	if !ok {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	vars, err := parseDotenv(file)
	if err != nil {
		return
	}
	applyDotenv(vars)
}

func findDotenv() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			path := filepath.Join(dir, ".env")
			if _, err := os.Stat(path); err == nil {
				return path, true
			}
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func parseDotenv(r io.Reader) (map[string]string, error) {
	vars := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, " \t") {
			continue
		}
		vars[key] = unquoteDotenvValue(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return vars, nil
}

func applyDotenv(vars map[string]string) {
	for key, value := range vars {
		_ = os.Setenv(key, value)
	}
}

func unquoteDotenvValue(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

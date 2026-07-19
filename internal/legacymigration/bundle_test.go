package legacymigration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestValidateBundleAcceptsDeterministicLegacySnapshot(t *testing.T) {
	directory := writeTestBundle(t, map[string][]any{
		"contexts.jsonl": {
			map[string]any{"id": "c1", "slug": "dados-pessoais", "name": "Dados pessoais"},
			map[string]any{"id": "c2", "slug": "conta-luz", "name": "Conta de luz"},
		},
		"columns.jsonl": {
			map[string]any{"id": "col1", "context_id": "c1", "key": "name"},
			map[string]any{"id": "col2", "context_id": "c2", "key": "uc"},
		},
		"records.jsonl": {
			map[string]any{"id": "r1", "context_id": "c1", "person_id": nil, "data": map[string]any{"name": "Pessoa Teste"}},
			map[string]any{"id": "r2", "context_id": "c2", "person_id": "r1", "data": map[string]any{"uc": "123"}},
		},
		"attachments.jsonl": {},
	})

	report, err := ValidateBundle(directory)
	if err != nil {
		t.Fatalf("ValidateBundle() error = %v", err)
	}
	if !report.Valid || len(report.Errors) != 0 || len(report.Warnings) != 0 {
		t.Fatalf("report = %#v", report)
	}
	if report.Targets["profile"] != 1 || report.Targets["bill"] != 1 {
		t.Fatalf("targets = %#v", report.Targets)
	}
}

func TestValidateBundleRejectsMissingDocumentOwner(t *testing.T) {
	directory := writeTestBundle(t, map[string][]any{
		"contexts.jsonl": {
			map[string]any{"id": "c1", "slug": "doc-rg", "name": "RG"},
		},
		"columns.jsonl": {
			map[string]any{"id": "col1", "context_id": "c1", "key": "rg_number"},
		},
		"records.jsonl": {
			map[string]any{"id": "r1", "context_id": "c1", "person_id": nil, "data": map[string]any{"rg_number": "123"}},
		},
		"attachments.jsonl": {},
	})

	report, err := ValidateBundle(directory)
	if err != nil {
		t.Fatalf("ValidateBundle() error = %v", err)
	}
	if report.Valid || !hasFinding(report.Errors, "owner_profile_missing") {
		t.Fatalf("report = %#v", report)
	}
}

func TestValidateBundleDetectsTampering(t *testing.T) {
	directory := writeTestBundle(t, map[string][]any{
		"contexts.jsonl": {
			map[string]any{"id": "c1", "slug": "dados-pessoais", "name": "Dados pessoais"},
		},
		"columns.jsonl": {
			map[string]any{"id": "col1", "context_id": "c1", "key": "name"},
		},
		"records.jsonl": {
			map[string]any{"id": "r1", "context_id": "c1", "person_id": nil, "data": map[string]any{"name": "Pessoa Teste"}},
		},
		"attachments.jsonl": {},
	})
	path := filepath.Join(directory, "records.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "Pessoa Teste", "Pessoa Xeste", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := ValidateBundle(directory)
	if err != nil {
		t.Fatalf("ValidateBundle() error = %v", err)
	}
	if report.Valid || !hasFinding(report.Errors, "manifest_file_mismatch") {
		t.Fatalf("report = %#v", report)
	}
}

func writeTestBundle(t *testing.T, datasets map[string][]any) string {
	t.Helper()
	directory := t.TempDir()
	files := map[string]FileDigest{}
	for _, name := range bundleFiles {
		values := datasets[name]
		var data []byte
		for _, value := range values {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			data = append(data, encoded...)
			data = append(data, '\n')
		}
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		files[name] = FileDigest{Rows: len(values), SHA256: hex.EncodeToString(digest[:])}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	fingerprint := sha256.New()
	for _, name := range names {
		_, _ = fmt.Fprintf(fingerprint, "%s:%s\n", name, files[name].SHA256)
	}
	manifest := Manifest{
		FormatVersion: FormatVersion,
		SourceSchema:  "prisma-sqlite-record-v1",
		ExportedAt:    "2026-07-19T00:00:00Z",
		Files:         files,
		Fingerprint:   hex.EncodeToString(fingerprint.Sum(nil)),
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func hasFinding(findings []Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

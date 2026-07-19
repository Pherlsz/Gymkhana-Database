package legacymigration

import "encoding/json"

const FormatVersion = "gymkhana-legacy-sqlite-v1"

var bundleFiles = []string{"attachments.jsonl", "columns.jsonl", "contexts.jsonl", "records.jsonl"}

type FileDigest struct {
	Rows   int    `json:"rows"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	FormatVersion string                `json:"format_version"`
	SourceSchema  string                `json:"source_schema"`
	ExportedAt    string                `json:"exported_at"`
	Files         map[string]FileDigest `json:"files"`
	Fingerprint   string                `json:"fingerprint"`
}

type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Entity   string `json:"entity,omitempty"`
	ID       string `json:"id,omitempty"`
}

type Report struct {
	FormatVersion string         `json:"format_version"`
	Fingerprint   string         `json:"fingerprint"`
	Valid         bool           `json:"valid"`
	Counts        map[string]int `json:"counts"`
	Targets       map[string]int `json:"targets"`
	Errors        []Finding      `json:"errors"`
	Warnings      []Finding      `json:"warnings"`
}

type legacyContext struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type legacyColumn struct {
	ID        string `json:"id"`
	ContextID string `json:"context_id"`
	Key       string `json:"key"`
}

type legacyRecord struct {
	ID        string                     `json:"id"`
	ContextID string                     `json:"context_id"`
	PersonID  *string                    `json:"person_id"`
	Data      map[string]json.RawMessage `json:"data"`
}

type legacyAttachment struct {
	ID       string  `json:"id"`
	RecordID *string `json:"record_id"`
	FileName string  `json:"file_name"`
	FilePath string  `json:"file_path"`
}

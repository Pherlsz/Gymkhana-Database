package legacymigration

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
)

func classifyContext(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	switch {
	case slug == "dados-pessoais":
		return "profile"
	case strings.HasPrefix(slug, "doc-"):
		return "document"
	case strings.HasPrefix(slug, "conta-"):
		return "bill"
	default:
		return "custom_entity"
	}
}

func nonEmptyJSONString(value json.RawMessage) bool {
	if len(value) == 0 {
		return false
	}
	var text string
	return json.Unmarshal(value, &text) == nil && strings.TrimSpace(text) != ""
}

func (report *Report) addError(code, entity, id string) {
	report.Errors = append(report.Errors, Finding{Severity: "error", Code: code, Entity: entity, ID: id})
}

func (report *Report) addWarning(code, entity, id string) {
	report.Warnings = append(report.Warnings, Finding{Severity: "warning", Code: code, Entity: entity, ID: id})
}

func (report *Report) sortFindings() {
	less := func(findings []Finding) {
		sort.Slice(findings, func(i, j int) bool {
			left, right := findings[i], findings[j]
			return strings.Join([]string{left.Code, left.Entity, left.ID}, "\x00") < strings.Join([]string{right.Code, right.Entity, right.ID}, "\x00")
		})
	}
	less(report.Errors)
	less(report.Warnings)
}

func WriteReport(path string, report Report) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("report path cannot be empty")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

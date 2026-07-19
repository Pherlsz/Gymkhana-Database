package legacymigration

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ValidateBundle(directory string) (Report, error) {
	manifest, err := readManifest(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return Report{}, err
	}
	report := Report{
		FormatVersion: manifest.FormatVersion,
		Fingerprint:   manifest.Fingerprint,
		Counts:        map[string]int{},
		Targets:       map[string]int{},
		Errors:        []Finding{},
		Warnings:      []Finding{},
	}
	if manifest.FormatVersion != FormatVersion {
		return Report{}, fmt.Errorf("unsupported legacy bundle format %q", manifest.FormatVersion)
	}
	if manifest.SourceSchema != "prisma-sqlite-record-v1" {
		return Report{}, fmt.Errorf("unsupported legacy source schema %q", manifest.SourceSchema)
	}
	if err := validateDigests(directory, manifest, &report); err != nil {
		return Report{}, err
	}

	contexts, err := readJSONLines[legacyContext](filepath.Join(directory, "contexts.jsonl"))
	if err != nil {
		return Report{}, err
	}
	columns, err := readJSONLines[legacyColumn](filepath.Join(directory, "columns.jsonl"))
	if err != nil {
		return Report{}, err
	}
	records, err := readJSONLines[legacyRecord](filepath.Join(directory, "records.jsonl"))
	if err != nil {
		return Report{}, err
	}
	attachments, err := readJSONLines[legacyAttachment](filepath.Join(directory, "attachments.jsonl"))
	if err != nil {
		return Report{}, err
	}

	contextByID := make(map[string]legacyContext, len(contexts))
	contextBySlug := make(map[string]string, len(contexts))
	columnsByContext := make(map[string]map[string]struct{})
	for _, context := range contexts {
		if strings.TrimSpace(context.ID) == "" || strings.TrimSpace(context.Slug) == "" {
			report.addError("invalid_context", "context", context.ID)
			continue
		}
		if _, exists := contextByID[context.ID]; exists {
			report.addError("duplicate_id", "context", context.ID)
			continue
		}
		normalizedSlug := strings.ToLower(strings.TrimSpace(context.Slug))
		if existingID, exists := contextBySlug[normalizedSlug]; exists {
			report.addError("duplicate_context_slug", "context", existingID+":"+context.ID)
		} else {
			contextBySlug[normalizedSlug] = context.ID
		}
		contextByID[context.ID] = context
		columnsByContext[context.ID] = map[string]struct{}{}
	}
	for _, column := range columns {
		keys, exists := columnsByContext[column.ContextID]
		if !exists {
			report.addError("unknown_context", "column", column.ID)
			continue
		}
		if strings.TrimSpace(column.Key) == "" {
			report.addError("invalid_column_key", "column", column.ID)
			continue
		}
		if _, exists := keys[column.Key]; exists {
			report.addError("duplicate_column_key", "column", column.ID)
		}
		keys[column.Key] = struct{}{}
	}

	if _, exists := contextBySlug["dados-pessoais"]; !exists {
		report.addError("profile_context_missing", "bundle", "dados-pessoais")
	}
	recordByID := make(map[string]legacyRecord, len(records))
	profileIDs := map[string]struct{}{}
	for _, record := range records {
		context, exists := contextByID[record.ContextID]
		if !exists {
			report.addError("unknown_context", "record", record.ID)
			continue
		}
		if _, exists := recordByID[record.ID]; exists {
			report.addError("duplicate_id", "record", record.ID)
			continue
		}
		recordByID[record.ID] = record
		if record.Data == nil {
			report.addError("record_data_missing", "record", record.ID)
		}
		target := classifyContext(context.Slug)
		report.Targets[target]++
		if target == "profile" {
			profileIDs[record.ID] = struct{}{}
			if !nonEmptyJSONString(record.Data["name"]) {
				report.addError("profile_name_missing", "record", record.ID)
			}
			if record.PersonID != nil && strings.TrimSpace(*record.PersonID) != "" {
				report.addWarning("profile_has_person_link", "record", record.ID)
			}
		}
		for key := range record.Data {
			if _, exists := columnsByContext[record.ContextID][key]; !exists {
				report.addWarning("data_key_without_definition", "record", record.ID)
			}
		}
	}
	if len(profileIDs) == 0 {
		report.addError("profile_records_missing", "bundle", "records.jsonl")
	}
	for _, record := range records {
		context, exists := contextByID[record.ContextID]
		if !exists {
			continue
		}
		target := classifyContext(context.Slug)
		if target != "document" && target != "bill" {
			continue
		}
		if record.PersonID == nil || strings.TrimSpace(*record.PersonID) == "" {
			report.addError("owner_profile_missing", "record", record.ID)
			continue
		}
		if _, exists := profileIDs[*record.PersonID]; !exists {
			report.addError("owner_profile_unknown", "record", record.ID)
		}
	}

	for _, attachment := range attachments {
		if strings.TrimSpace(attachment.FileName) == "" || strings.TrimSpace(attachment.FilePath) == "" {
			report.addError("attachment_file_missing", "attachment", attachment.ID)
		}
		if attachment.RecordID == nil || strings.TrimSpace(*attachment.RecordID) == "" {
			report.addWarning("attachment_orphaned", "attachment", attachment.ID)
			continue
		}
		if _, exists := recordByID[*attachment.RecordID]; !exists {
			report.addError("attachment_record_unknown", "attachment", attachment.ID)
		}
	}

	report.Counts["contexts"] = len(contexts)
	report.Counts["columns"] = len(columns)
	report.Counts["records"] = len(records)
	report.Counts["attachments"] = len(attachments)
	report.sortFindings()
	report.Valid = len(report.Errors) == 0
	return report, nil
}

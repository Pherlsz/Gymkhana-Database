package ocr

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Core/civiltime"
	"github.com/Pherlsz/Gymkhana-Core/fingerprint"
	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

type providerCandidate struct {
	FieldKey string
	Value    string
	Evidence Evidence
}

var (
	pdfPagePattern = regexp.MustCompile(`/Type[[:space:]]*/Page([^sA-Za-z]|$)`)
	decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]{0,15})(\.[0-9]{1,6})?$`)
)

type ValidatedSource struct {
	Bytes      []byte
	PageCount  int
	PixelCount int64
}

func SupportedMIMEs() []string {
	return []string{"application/pdf", "image/jpeg", "image/png"}
}

func SupportedMIME(value string) bool {
	for _, candidate := range SupportedMIMEs() {
		if value == candidate {
			return true
		}
	}
	return false
}

func ValidateSource(reader io.Reader, mime string, expectedSize int64, expectedSHA256 [32]byte, maximumBytes int64) (ValidatedSource, error) {
	if reader == nil || !SupportedMIME(mime) || expectedSize < 1 || maximumBytes < 1 || expectedSize > maximumBytes {
		return ValidatedSource{}, ErrUnsafeSource
	}
	content, err := io.ReadAll(io.LimitReader(reader, maximumBytes+1))
	if err != nil {
		return ValidatedSource{}, fmt.Errorf("read OCR source: %w", err)
	}
	if int64(len(content)) != expectedSize || int64(len(content)) > maximumBytes || [32]byte(fingerprint.Sum(content)) != expectedSHA256 {
		return ValidatedSource{}, ErrUnsafeSource
	}
	if mime == "application/pdf" {
		return validatePDF(content)
	}
	return validateImage(content, mime)
}

func validateImage(content []byte, expectedMIME string) (ValidatedSource, error) {
	configuration, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || configuration.Width < 1 || configuration.Height < 1 {
		return ValidatedSource{}, ErrUnsafeSource
	}
	detected := map[string]string{"jpeg": "image/jpeg", "png": "image/png"}[format]
	if detected != expectedMIME {
		return ValidatedSource{}, ErrUnsafeSource
	}
	pixels := int64(configuration.Width) * int64(configuration.Height)
	if pixels < 1 || pixels > MaximumPixels {
		return ValidatedSource{}, ErrUnsafeSource
	}
	// Decode the complete bounded image after inspecting its dimensions. DecodeConfig
	// alone accepts some truncated payloads because it only needs the header.
	if _, decodedFormat, err := image.Decode(bytes.NewReader(content)); err != nil || decodedFormat != format {
		return ValidatedSource{}, ErrUnsafeSource
	}
	return ValidatedSource{Bytes: content, PageCount: 1, PixelCount: pixels}, nil
}

func validatePDF(content []byte) (ValidatedSource, error) {
	if !bytes.HasPrefix(content, []byte("%PDF-")) || bytes.Contains(content, []byte("/Encrypt")) {
		return ValidatedSource{}, ErrUnsafeSource
	}
	tailStart := len(content) - 4096
	if tailStart < 0 {
		tailStart = 0
	}
	if !bytes.Contains(content[tailStart:], []byte("%%EOF")) {
		return ValidatedSource{}, ErrUnsafeSource
	}
	pageCount := len(pdfPagePattern.FindAll(content, MaximumPages+1))
	if pageCount < 1 || pageCount > MaximumPages {
		return ValidatedSource{}, ErrUnsafeSource
	}
	return ValidatedSource{Bytes: content, PageCount: pageCount}, nil
}

func normalizeProviderSuggestions(candidates []providerCandidate, catalog Catalog, pageCount int) ([]CompleteSuggestionInput, error) {
	if len(candidates) > MaximumSuggestions || pageCount < 1 || pageCount > MaximumPages {
		return nil, ErrMalformedProvider
	}
	fields := make(map[string]FieldSchema, len(catalog.Fields))
	for _, field := range catalog.Fields {
		if _, duplicate := fields[field.Key]; duplicate || !validFieldSchema(field) {
			return nil, ErrInvalidSetup
		}
		fields[field.Key] = field
	}
	result := make([]CompleteSuggestionInput, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for index, raw := range candidates {
		field, exists := fields[raw.FieldKey]
		if !exists {
			return nil, ErrMalformedProvider
		}
		if _, duplicate := seen[raw.FieldKey]; duplicate {
			return nil, ErrMalformedProvider
		}
		seen[raw.FieldKey] = struct{}{}
		value, err := normalizeValue(field.Kind, raw.Value, false)
		if err != nil || value == "" {
			return nil, ErrMalformedProvider
		}
		evidence, err := normalizeEvidence(raw.Evidence, pageCount)
		if err != nil {
			return nil, ErrMalformedProvider
		}
		id, err := NewIdentifier()
		if err != nil {
			return nil, fmt.Errorf("generate OCR suggestion identifier: %w", err)
		}
		result = append(result, CompleteSuggestionInput{
			ID: id, Ordinal: index + 1, Field: field, ProposedValue: value, Evidence: evidence,
		})
	}
	return result, nil
}

func normalizeEvidence(value Evidence, pageCount int) (Evidence, error) {
	if value.Page < 1 || value.Page > pageCount {
		return Evidence{}, ErrInvalidInput
	}
	if value.Region != nil && !value.Region.Valid() {
		return Evidence{}, ErrInvalidInput
	}
	value.Excerpt = strings.TrimSpace(strings.ToValidUTF8(value.Excerpt, ""))
	if utf8.RuneCountInString(value.Excerpt) > MaximumEvidenceRunes || containsUnsafeControl(value.Excerpt, true) {
		return Evidence{}, ErrInvalidInput
	}
	if value.Confidence != nil && (*value.Confidence < 0 || *value.Confidence > MaximumConfidence) {
		return Evidence{}, ErrInvalidInput
	}
	return value, nil
}

func normalizeValue(kind ValueKind, value string, allowEmpty bool) (string, error) {
	value = strings.TrimSpace(strings.ToValidUTF8(value, ""))
	if (!allowEmpty && value == "") || !kind.Valid() || utf8.RuneCountInString(value) > MaximumValueRunes || containsUnsafeControl(value, kind == ValueLongText) {
		return "", ErrInvalidInput
	}
	if value == "" {
		return "", nil
	}
	switch kind {
	case ValueText:
		return normalize.DisplayText(value), nil
	case ValueLongText:
		return value, nil
	case ValueInteger:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", ErrInvalidInput
		}
		return strconv.FormatInt(parsed, 10), nil
	case ValueDecimal:
		if !decimalPattern.MatchString(value) {
			return "", ErrInvalidInput
		}
		return canonicalDecimal(value), nil
	case ValueBoolean:
		value = strings.ToLower(value)
		if value != "true" && value != "false" {
			return "", ErrInvalidInput
		}
		return value, nil
	case ValueCivilDate:
		parsed, err := civiltime.ParseCivilDate(value)
		if err != nil {
			return "", ErrInvalidInput
		}
		return parsed.String(), nil
	case ValueCivilMonth:
		parsed, err := civiltime.ParseYearMonth(value)
		if err != nil {
			return "", ErrInvalidInput
		}
		return parsed.String(), nil
	case ValueEmail:
		normalized, err := normalize.CanonicalEmail(value)
		if err != nil {
			return "", ErrInvalidInput
		}
		return strings.ToLower(normalized), nil
	case ValuePhone:
		normalized, err := normalize.CanonicalBrazilPhone(value)
		if err != nil {
			return "", ErrInvalidInput
		}
		return normalized, nil
	default:
		return "", ErrInvalidInput
	}
}

func canonicalDecimal(value string) string {
	negative := strings.HasPrefix(value, "-")
	value = strings.TrimPrefix(value, "-")
	parts := strings.SplitN(value, ".", 2)
	whole := strings.TrimLeft(parts[0], "0")
	if whole == "" {
		whole = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if negative && (whole != "0" || fraction != "") {
		whole = "-" + whole
	}
	if fraction == "" {
		return whole
	}
	return whole + "." + fraction
}

func containsUnsafeControl(value string, allowNewlines bool) bool {
	for _, character := range value {
		if !unicode.IsControl(character) {
			continue
		}
		if allowNewlines && (character == '\n' || character == '\r' || character == '\t') {
			continue
		}
		return true
	}
	return false
}

func validFieldSchema(field FieldSchema) bool {
	return validLogicalKey(field.Key) && strings.TrimSpace(field.Label) == field.Label && field.Label != "" &&
		utf8.RuneCountInString(field.Label) <= 160 && field.Kind.Valid() && field.Target.Valid() && field.TargetVersion > 0
}

func validLogicalKey(value string) bool {
	if len(value) < 2 || len(value) > 120 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value {
		if unicode.IsLower(character) || unicode.IsDigit(character) || character == '_' || character == '.' || character == '-' {
			continue
		}
		return false
	}
	return true
}

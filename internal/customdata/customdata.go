package customdata

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxTechnicalKeyLength = 64
	MaxLabelLength        = 120
	MaxRegexLength        = 500
	MaxTextLength         = 5000
	MaxOptionKeyLength    = 64
)

var ErrInvalidIdentifier = errors.New("invalid custom data identifier")

type Identifier [16]byte

func NewIdentifier() (Identifier, error) {
	var value Identifier
	if _, err := rand.Read(value[:]); err != nil {
		return Identifier{}, err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return value, nil
}

func ParseIdentifier(value string) (Identifier, error) {
	compact := strings.ReplaceAll(strings.TrimSpace(value), "-", "")
	if len(compact) != 32 {
		return Identifier{}, ErrInvalidIdentifier
	}
	decoded, err := hex.DecodeString(compact)
	if err != nil {
		return Identifier{}, ErrInvalidIdentifier
	}
	var identifier Identifier
	copy(identifier[:], decoded)
	return identifier, nil
}

func (identifier Identifier) String() string {
	encoded := hex.EncodeToString(identifier[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func (identifier Identifier) IsZero() bool {
	return identifier == Identifier{}
}

type TargetKind string

const (
	TargetProfile          TargetKind = "PROFILE"
	TargetDocumentType     TargetKind = "DOCUMENT_TYPE"
	TargetBillType         TargetKind = "BILL_TYPE"
	TargetCustomEntityType TargetKind = "CUSTOM_ENTITY_TYPE"
)

func (kind TargetKind) Valid() bool {
	return kind == TargetProfile || kind == TargetDocumentType || kind == TargetBillType || kind == TargetCustomEntityType
}

type FieldKind string

const (
	FieldText         FieldKind = "TEXT"
	FieldLongText     FieldKind = "LONG_TEXT"
	FieldInteger      FieldKind = "INTEGER"
	FieldDecimal      FieldKind = "DECIMAL"
	FieldBoolean      FieldKind = "BOOLEAN"
	FieldCivilDate    FieldKind = "CIVIL_DATE"
	FieldCivilMonth   FieldKind = "CIVIL_MONTH"
	FieldEmail        FieldKind = "EMAIL"
	FieldPhone        FieldKind = "PHONE"
	FieldSingleSelect FieldKind = "SINGLE_SELECT"
	FieldMultiSelect  FieldKind = "MULTI_SELECT"
)

func (kind FieldKind) Valid() bool {
	switch kind {
	case FieldText, FieldLongText, FieldInteger, FieldDecimal, FieldBoolean, FieldCivilDate, FieldCivilMonth, FieldEmail, FieldPhone, FieldSingleSelect, FieldMultiSelect:
		return true
	default:
		return false
	}
}

func (kind FieldKind) IsTextual() bool {
	return kind == FieldText || kind == FieldLongText || kind == FieldEmail || kind == FieldPhone
}

type ProfileCardinality string

const (
	CardinalityOnePerProfile  ProfileCardinality = "ONE_PER_PROFILE"
	CardinalityManyPerProfile ProfileCardinality = "MANY_PER_PROFILE"
)

func (cardinality ProfileCardinality) Valid() bool {
	return cardinality == "" || cardinality == CardinalityOnePerProfile || cardinality == CardinalityManyPerProfile
}

type FieldDefinitionValues struct {
	TargetKind      TargetKind
	TargetID        Identifier
	TechnicalKey    string
	Label           string
	Kind            FieldKind
	Required        bool
	Active          bool
	MinimumLength   int
	MaximumLength   int
	ValidationRegex string
	MinimumDecimal  string
	MaximumDecimal  string
}

type FieldDefinition struct {
	ID        Identifier
	Values    FieldDefinitionValues
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OptionValues struct {
	TechnicalKey string
	Label        string
	Active       bool
	SortOrder    int
}

type Option struct {
	ID                Identifier
	FieldDefinitionID Identifier
	Values            OptionValues
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type EntityTypeValues struct {
	TechnicalKey       string
	Label              string
	Active             bool
	ProfileCardinality ProfileCardinality
}

type EntityType struct {
	ID        Identifier
	Values    EntityTypeValues
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ValueInput struct {
	FieldDefinitionID Identifier
	Kind              FieldKind
	Text              string
	Integer           *int64
	Decimal           string
	Boolean           *bool
	CivilDate         string
	CivilMonth        string
	OptionIDs         []Identifier
}

type FieldError struct {
	Field string
	Code  string
}

type ValidationError struct {
	Fields []FieldError
}

func (err *ValidationError) Error() string {
	return "custom data validation failed"
}

func NormalizeFieldDefinition(values FieldDefinitionValues) (FieldDefinitionValues, error) {
	normalized := FieldDefinitionValues{
		TargetKind:      values.TargetKind,
		TargetID:        values.TargetID,
		TechnicalKey:    normalizeTechnicalKey(values.TechnicalKey),
		Label:           normalizeDisplayText(values.Label),
		Kind:            values.Kind,
		Required:        values.Required,
		Active:          values.Active,
		MinimumLength:   values.MinimumLength,
		MaximumLength:   values.MaximumLength,
		ValidationRegex: strings.TrimSpace(strings.ToValidUTF8(values.ValidationRegex, "")),
		MinimumDecimal:  strings.TrimSpace(values.MinimumDecimal),
		MaximumDecimal:  strings.TrimSpace(values.MaximumDecimal),
	}
	validation := &ValidationError{}
	validateTechnicalKey(validation, "technical_key", normalized.TechnicalKey, MaxTechnicalKeyLength)
	validateRequiredText(validation, "label", normalized.Label, MaxLabelLength)
	if !normalized.TargetKind.Valid() {
		validation.add("target_kind", "invalid_value")
	} else if normalized.TargetKind == TargetProfile {
		if !normalized.TargetID.IsZero() {
			validation.add("target_id", "unexpected")
		}
	} else if normalized.TargetID.IsZero() {
		validation.add("target_id", "required")
	}
	if !normalized.Kind.Valid() {
		validation.add("field_kind", "invalid_value")
	}
	validateFieldRules(validation, &normalized)
	if len(validation.Fields) > 0 {
		return FieldDefinitionValues{}, validation
	}
	return normalized, nil
}

func NormalizeOption(values OptionValues) (OptionValues, error) {
	normalized := OptionValues{
		TechnicalKey: normalizeTechnicalKey(values.TechnicalKey),
		Label:        normalizeDisplayText(values.Label),
		Active:       values.Active,
		SortOrder:    values.SortOrder,
	}
	validation := &ValidationError{}
	validateTechnicalKey(validation, "technical_key", normalized.TechnicalKey, MaxOptionKeyLength)
	validateRequiredText(validation, "label", normalized.Label, MaxLabelLength)
	if normalized.SortOrder < 0 {
		validation.add("sort_order", "invalid_value")
	}
	if len(validation.Fields) > 0 {
		return OptionValues{}, validation
	}
	return normalized, nil
}

func NormalizeEntityType(values EntityTypeValues) (EntityTypeValues, error) {
	normalized := EntityTypeValues{
		TechnicalKey:       normalizeTechnicalKey(values.TechnicalKey),
		Label:              normalizeDisplayText(values.Label),
		Active:             values.Active,
		ProfileCardinality: values.ProfileCardinality,
	}
	validation := &ValidationError{}
	validateTechnicalKey(validation, "technical_key", normalized.TechnicalKey, MaxTechnicalKeyLength)
	validateRequiredText(validation, "label", normalized.Label, MaxLabelLength)
	if !normalized.ProfileCardinality.Valid() {
		validation.add("profile_cardinality", "invalid_value")
	}
	if len(validation.Fields) > 0 {
		return EntityTypeValues{}, validation
	}
	return normalized, nil
}

func NormalizeValue(input ValueInput, definition FieldDefinition) (ValueInput, error) {
	normalized := ValueInput{
		FieldDefinitionID: input.FieldDefinitionID,
		Kind:              input.Kind,
		Text:              strings.TrimSpace(strings.ToValidUTF8(input.Text, "")),
		Integer:           input.Integer,
		Decimal:           strings.TrimSpace(input.Decimal),
		Boolean:           input.Boolean,
		CivilDate:         strings.TrimSpace(input.CivilDate),
		CivilMonth:        strings.TrimSpace(input.CivilMonth),
		OptionIDs:         append([]Identifier(nil), input.OptionIDs...),
	}
	validation := &ValidationError{}
	if normalized.FieldDefinitionID.IsZero() || normalized.FieldDefinitionID != definition.ID {
		validation.add("field_definition_id", "invalid_value")
	}
	if normalized.Kind == "" {
		normalized.Kind = definition.Values.Kind
	}
	if normalized.Kind != definition.Values.Kind {
		validation.add("field_kind", "invalid_value")
	}
	if !definition.Values.Active {
		validation.add("field_definition_id", "inactive")
	}
	validateNoUnexpectedValues(validation, normalized)
	if len(validation.Fields) == 0 {
		normalizeTypedValue(validation, &normalized, definition.Values)
	}
	if len(validation.Fields) > 0 {
		return ValueInput{}, validation
	}
	return normalized, nil
}

func validateFieldRules(validation *ValidationError, values *FieldDefinitionValues) {
	if values.MinimumLength < 0 {
		validation.add("minimum_length", "invalid_value")
	}
	if values.MaximumLength < 0 || values.MaximumLength > MaxTextLength {
		validation.add("maximum_length", "invalid_value")
	}
	if values.MaximumLength > 0 && values.MinimumLength > values.MaximumLength {
		validation.add("minimum_length", "invalid_value")
	}
	if values.ValidationRegex != "" {
		if utf8.RuneCountInString(values.ValidationRegex) > MaxRegexLength {
			validation.add("validation_regex", "too_long")
		} else if _, err := regexp.Compile(values.ValidationRegex); err != nil {
			validation.add("validation_regex", "invalid_format")
		}
	}
	if values.Kind.Valid() && !values.Kind.IsTextual() {
		if values.MinimumLength != 0 {
			validation.add("minimum_length", "unexpected")
		}
		if values.MaximumLength != 0 {
			validation.add("maximum_length", "unexpected")
		}
		if values.ValidationRegex != "" {
			validation.add("validation_regex", "unexpected")
		}
	}
	if values.Kind != FieldDecimal {
		if values.MinimumDecimal != "" {
			validation.add("minimum_decimal", "unexpected")
		}
		if values.MaximumDecimal != "" {
			validation.add("maximum_decimal", "unexpected")
		}
		return
	}
	minimum, minimumOK := canonicalDecimal(values.MinimumDecimal)
	maximum, maximumOK := canonicalDecimal(values.MaximumDecimal)
	if values.MinimumDecimal != "" && !minimumOK {
		validation.add("minimum_decimal", "invalid_format")
	} else if minimumOK {
		values.MinimumDecimal = minimum
	}
	if values.MaximumDecimal != "" && !maximumOK {
		validation.add("maximum_decimal", "invalid_format")
	} else if maximumOK {
		values.MaximumDecimal = maximum
	}
	if minimumOK && maximumOK && compareDecimal(minimum, maximum) > 0 {
		validation.add("minimum_decimal", "invalid_value")
	}
}

func validateNoUnexpectedValues(validation *ValidationError, value ValueInput) {
	hasText := value.Text != ""
	hasInteger := value.Integer != nil
	hasDecimal := value.Decimal != ""
	hasBoolean := value.Boolean != nil
	hasDate := value.CivilDate != ""
	hasMonth := value.CivilMonth != ""
	hasOptions := len(value.OptionIDs) > 0
	allowed := map[FieldKind]map[string]bool{
		FieldText: {"text": true}, FieldLongText: {"text": true}, FieldEmail: {"text": true}, FieldPhone: {"text": true},
		FieldInteger: {"integer": true}, FieldDecimal: {"decimal": true}, FieldBoolean: {"boolean": true},
		FieldCivilDate: {"date": true}, FieldCivilMonth: {"month": true},
		FieldSingleSelect: {"options": true}, FieldMultiSelect: {"options": true},
	}[value.Kind]
	for key, present := range map[string]bool{"text": hasText, "integer": hasInteger, "decimal": hasDecimal, "boolean": hasBoolean, "date": hasDate, "month": hasMonth, "options": hasOptions} {
		if present && !allowed[key] {
			validation.add(valueFieldName(key), "unexpected")
		}
	}
}

func normalizeTypedValue(validation *ValidationError, value *ValueInput, definition FieldDefinitionValues) {
	switch definition.Kind {
	case FieldText, FieldLongText:
		validateTextValue(validation, value.Text, definition)
	case FieldEmail:
		value.Text = strings.ToLower(value.Text)
		validateTextValue(validation, value.Text, definition)
		if value.Text != "" && !emailPattern.MatchString(value.Text) {
			validation.add("text_value", "invalid_format")
		}
	case FieldPhone:
		value.Text = normalizePhone(value.Text)
		validateTextValue(validation, value.Text, definition)
		if value.Text != "" && !phonePattern.MatchString(value.Text) {
			validation.add("text_value", "invalid_format")
		}
	case FieldInteger:
		if value.Integer == nil && definition.Required {
			validation.add("integer_value", "required")
		}
	case FieldDecimal:
		if value.Decimal == "" {
			if definition.Required {
				validation.add("decimal_value", "required")
			}
			return
		}
		canonical, ok := canonicalDecimal(value.Decimal)
		if !ok {
			validation.add("decimal_value", "invalid_format")
			return
		}
		value.Decimal = canonical
		if definition.MinimumDecimal != "" && compareDecimal(canonical, definition.MinimumDecimal) < 0 {
			validation.add("decimal_value", "too_small")
		}
		if definition.MaximumDecimal != "" && compareDecimal(canonical, definition.MaximumDecimal) > 0 {
			validation.add("decimal_value", "too_large")
		}
	case FieldBoolean:
		if value.Boolean == nil && definition.Required {
			validation.add("boolean_value", "required")
		}
	case FieldCivilDate:
		if value.CivilDate == "" {
			if definition.Required {
				validation.add("civil_date_value", "required")
			}
		} else if _, err := time.Parse("2006-01-02", value.CivilDate); err != nil {
			validation.add("civil_date_value", "invalid_format")
		}
	case FieldCivilMonth:
		if value.CivilMonth == "" {
			if definition.Required {
				validation.add("civil_month_value", "required")
			}
		} else if !civilMonthPattern.MatchString(value.CivilMonth) {
			validation.add("civil_month_value", "invalid_format")
		}
	case FieldSingleSelect:
		value.OptionIDs = uniqueSortedIdentifiers(value.OptionIDs)
		if len(value.OptionIDs) == 0 && definition.Required {
			validation.add("option_ids", "required")
		} else if len(value.OptionIDs) > 1 {
			validation.add("option_ids", "too_many")
		}
	case FieldMultiSelect:
		value.OptionIDs = uniqueSortedIdentifiers(value.OptionIDs)
		if len(value.OptionIDs) == 0 && definition.Required {
			validation.add("option_ids", "required")
		}
	}
}

func validateTextValue(validation *ValidationError, value string, definition FieldDefinitionValues) {
	length := utf8.RuneCountInString(value)
	if value == "" && definition.Required {
		validation.add("text_value", "required")
		return
	}
	if value == "" {
		return
	}
	if definition.MinimumLength > 0 && length < definition.MinimumLength {
		validation.add("text_value", "too_short")
	}
	if definition.MaximumLength > 0 && length > definition.MaximumLength {
		validation.add("text_value", "too_long")
	}
	if definition.MaximumLength == 0 && length > MaxTextLength {
		validation.add("text_value", "too_long")
	}
	if definition.ValidationRegex != "" {
		pattern, err := regexp.Compile(definition.ValidationRegex)
		if err != nil || !pattern.MatchString(value) {
			validation.add("text_value", "invalid_format")
		}
	}
}

func canonicalDecimal(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if !decimalPattern.MatchString(value) {
		return "", false
	}
	negative := strings.HasPrefix(value, "-")
	if negative {
		value = strings.TrimPrefix(value, "-")
	}
	parts := strings.SplitN(value, ".", 2)
	whole := strings.TrimLeft(parts[0], "0")
	if whole == "" {
		whole = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if len(whole) > 28 || len(fraction) > 10 {
		return "", false
	}
	result := whole
	if fraction != "" {
		result += "." + fraction
	}
	if negative && result != "0" {
		result = "-" + result
	}
	return result, true
}

func compareDecimal(left, right string) int {
	leftValue, leftOK := new(big.Rat).SetString(left)
	rightValue, rightOK := new(big.Rat).SetString(right)
	if !leftOK || !rightOK {
		return 0
	}
	return leftValue.Cmp(rightValue)
}

func uniqueSortedIdentifiers(values []Identifier) []Identifier {
	unique := make(map[Identifier]struct{}, len(values))
	for _, value := range values {
		if !value.IsZero() {
			unique[value] = struct{}{}
		}
	}
	result := make([]Identifier, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func normalizeTechnicalKey(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.ToValidUTF8(value, "")))
}

func normalizeDisplayText(value string) string {
	return strings.Join(strings.Fields(strings.ToValidUTF8(value, "")), " ")
}

func normalizePhone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	prefix := ""
	if strings.HasPrefix(value, "+") {
		prefix = "+"
	}
	var digits strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			digits.WriteRune(character)
		}
	}
	return prefix + digits.String()
}

func validateTechnicalKey(validation *ValidationError, field, value string, maximum int) {
	if value == "" {
		validation.add(field, "required")
		return
	}
	if utf8.RuneCountInString(value) > maximum || !technicalKeyPattern.MatchString(value) {
		validation.add(field, "invalid_format")
	}
}

func validateRequiredText(validation *ValidationError, field, value string, maximum int) {
	if value == "" {
		validation.add(field, "required")
		return
	}
	if utf8.RuneCountInString(value) > maximum {
		validation.add(field, "too_long")
	}
}

func valueFieldName(key string) string {
	switch key {
	case "text":
		return "text_value"
	case "integer":
		return "integer_value"
	case "decimal":
		return "decimal_value"
	case "boolean":
		return "boolean_value"
	case "date":
		return "civil_date_value"
	case "month":
		return "civil_month_value"
	default:
		return "option_ids"
	}
}

func (err *ValidationError) add(field, code string) {
	err.Fields = append(err.Fields, FieldError{Field: field, Code: code})
}

var (
	technicalKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	decimalPattern      = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]{1,10})?$`)
	civilMonthPattern   = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
	emailPattern        = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	phonePattern        = regexp.MustCompile(`^\+?[0-9]{8,15}$`)
)

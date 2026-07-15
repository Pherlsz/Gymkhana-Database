package document

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

const (
	MaxTechnicalKeyLength = 64
	MaxLabelLength        = 120
	MaxRegexLength        = 500
	MaxIdentifierLength   = 500
	MaxNotesLength        = 5000
)

var ErrInvalidIdentifier = errors.New("invalid document identifier")

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

type UniquenessPolicy string

const (
	UniquenessNone         UniquenessPolicy = "NONE"
	UniquenessPerProfile   UniquenessPolicy = "PER_PROFILE"
	UniquenessGlobalByType UniquenessPolicy = "GLOBAL_BY_TYPE"
)

func (policy UniquenessPolicy) Valid() bool {
	return policy == UniquenessNone || policy == UniquenessPerProfile || policy == UniquenessGlobalByType
}

type RecordState string

const (
	RecordCurrent  RecordState = "CURRENT"
	RecordReplaced RecordState = "REPLACED"
	RecordExpired  RecordState = "EXPIRED"
	RecordArchived RecordState = "ARCHIVED"
)

func (state RecordState) Valid() bool {
	return state == RecordCurrent || state == RecordReplaced || state == RecordExpired || state == RecordArchived
}

type Status string

const (
	StatusAvailable Status = "AVAILABLE"
	StatusInUse     Status = "IN_USE"
)

type TypeValues struct {
	TechnicalKey     string
	Label            string
	Active           bool
	UniquenessPolicy UniquenessPolicy
	ValidationRegex  string
	DateRequired     bool
}

type TypeDefinition struct {
	ID        Identifier
	Values    TypeValues
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Values struct {
	OwnerProfileID profile.Identifier
	TypeID         Identifier
	Identifier     string
	DocumentDate   string
	Notes          string
	RecordState    RecordState
}

type CurrentUse struct {
	HolderProfileID profile.Identifier
	AssignedAt      time.Time
}

type Document struct {
	ID         Identifier
	Values     Values
	Type       TypeDefinition
	Status     Status
	CurrentUse *CurrentUse
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type FieldError struct {
	Field string
	Code  string
}

type ValidationError struct {
	Fields []FieldError
}

func (err *ValidationError) Error() string {
	return "document validation failed"
}

func NormalizeType(values TypeValues) (TypeValues, error) {
	normalized := TypeValues{
		TechnicalKey:     strings.ToLower(strings.TrimSpace(values.TechnicalKey)),
		Label:            normalize.DisplayText(values.Label),
		Active:           values.Active,
		UniquenessPolicy: values.UniquenessPolicy,
		ValidationRegex:  strings.TrimSpace(strings.ToValidUTF8(values.ValidationRegex, "")),
		DateRequired:     values.DateRequired,
	}
	validation := &ValidationError{}
	if normalized.TechnicalKey == "" {
		validation.add("technical_key", "required")
	} else if utf8.RuneCountInString(normalized.TechnicalKey) > MaxTechnicalKeyLength || !technicalKeyPattern.MatchString(normalized.TechnicalKey) {
		validation.add("technical_key", "invalid_format")
	}
	validateRequiredText(validation, "label", normalized.Label, MaxLabelLength)
	if !normalized.UniquenessPolicy.Valid() {
		validation.add("uniqueness_policy", "invalid_value")
	}
	if normalized.ValidationRegex != "" {
		if utf8.RuneCountInString(normalized.ValidationRegex) > MaxRegexLength {
			validation.add("validation_regex", "too_long")
		} else if _, err := regexp.Compile(normalized.ValidationRegex); err != nil {
			validation.add("validation_regex", "invalid_format")
		}
	}
	if len(validation.Fields) > 0 {
		return TypeValues{}, validation
	}
	return normalized, nil
}

func Normalize(values Values, definition TypeDefinition) (Values, error) {
	normalized := Values{
		OwnerProfileID: values.OwnerProfileID,
		TypeID:         values.TypeID,
		Identifier:     strings.TrimSpace(strings.ToValidUTF8(values.Identifier, "")),
		DocumentDate:   strings.TrimSpace(values.DocumentDate),
		Notes:          strings.TrimSpace(strings.ToValidUTF8(values.Notes, "")),
		RecordState:    values.RecordState,
	}
	if normalized.RecordState == "" {
		normalized.RecordState = RecordCurrent
	}
	validation := &ValidationError{}
	if normalized.OwnerProfileID == (profile.Identifier{}) {
		validation.add("owner_profile_id", "required")
	}
	if normalized.TypeID.IsZero() || normalized.TypeID != definition.ID {
		validation.add("document_type_id", "invalid_value")
	}
	validateRequiredText(validation, "identifier_value", normalized.Identifier, MaxIdentifierLength)
	if definition.Values.ValidationRegex != "" && normalized.Identifier != "" {
		pattern, err := regexp.Compile(definition.Values.ValidationRegex)
		if err != nil || !pattern.MatchString(normalized.Identifier) {
			validation.add("identifier_value", "invalid_format")
		}
	}
	if normalized.DocumentDate == "" {
		if definition.Values.DateRequired {
			validation.add("document_date", "required")
		}
	} else if _, err := time.Parse("2006-01-02", normalized.DocumentDate); err != nil {
		validation.add("document_date", "invalid_format")
	}
	if normalized.Notes != "" && utf8.RuneCountInString(normalized.Notes) > MaxNotesLength {
		validation.add("notes", "too_long")
	}
	if !normalized.RecordState.Valid() {
		validation.add("record_state", "invalid_value")
	}
	if len(validation.Fields) > 0 {
		return Values{}, validation
	}
	return normalized, nil
}

func SameRules(left, right TypeValues) bool {
	return left.UniquenessPolicy == right.UniquenessPolicy && left.ValidationRegex == right.ValidationRegex && left.DateRequired == right.DateRequired
}

func (err *ValidationError) add(field, code string) {
	err.Fields = append(err.Fields, FieldError{Field: field, Code: code})
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

var technicalKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

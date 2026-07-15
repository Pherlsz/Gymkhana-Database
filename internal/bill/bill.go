package bill

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

const (
	MaxTechnicalKeyLength = 64
	MaxLabelLength        = 120
	MaxHolderNameLength   = 200
	MaxPrintedAddress     = 500
	MaxReferenceLength    = 500
	MaxNotesLength        = 5000
)

var ErrInvalidIdentifier = errors.New("invalid bill identifier")

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
	TechnicalKey       string
	Label              string
	Active             bool
	SupportsCurrentUse bool
}

type TypeDefinition struct {
	ID        Identifier
	Values    TypeValues
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Values struct {
	OwnerProfileID    profile.Identifier
	TypeID            Identifier
	PrintedHolderName string
	PrintedAddress    string
	Reference         string
	Competence        string
	Amount            string
	Currency          string
	Notes             string
	RecordState       RecordState
}

type CurrentUse struct {
	HolderProfileID profile.Identifier
	AssignedAt      time.Time
}

type Bill struct {
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
	return "bill validation failed"
}

func NormalizeType(values TypeValues) (TypeValues, error) {
	normalized := TypeValues{
		TechnicalKey:       strings.ToLower(strings.TrimSpace(values.TechnicalKey)),
		Label:              normalize.DisplayText(values.Label),
		Active:             values.Active,
		SupportsCurrentUse: values.SupportsCurrentUse,
	}
	validation := &ValidationError{}
	if normalized.TechnicalKey == "" {
		validation.add("technical_key", "required")
	} else if utf8.RuneCountInString(normalized.TechnicalKey) > MaxTechnicalKeyLength || !technicalKeyPattern.MatchString(normalized.TechnicalKey) {
		validation.add("technical_key", "invalid_format")
	}
	validateRequiredText(validation, "label", normalized.Label, MaxLabelLength)
	if len(validation.Fields) > 0 {
		return TypeValues{}, validation
	}
	return normalized, nil
}

func Normalize(values Values, definition TypeDefinition) (Values, error) {
	normalized := Values{
		OwnerProfileID:    values.OwnerProfileID,
		TypeID:            values.TypeID,
		PrintedHolderName: normalize.DisplayText(values.PrintedHolderName),
		PrintedAddress:    normalize.DisplayText(values.PrintedAddress),
		Reference:         strings.TrimSpace(strings.ToValidUTF8(values.Reference, "")),
		Competence:        strings.TrimSpace(values.Competence),
		Amount:            strings.TrimSpace(values.Amount),
		Currency:          strings.ToUpper(strings.TrimSpace(values.Currency)),
		Notes:             strings.TrimSpace(strings.ToValidUTF8(values.Notes, "")),
		RecordState:       values.RecordState,
	}
	if normalized.RecordState == "" {
		normalized.RecordState = RecordCurrent
	}
	validation := &ValidationError{}
	if normalized.OwnerProfileID == (profile.Identifier{}) {
		validation.add("owner_profile_id", "required")
	}
	if normalized.TypeID.IsZero() || normalized.TypeID != definition.ID {
		validation.add("bill_type_id", "invalid_value")
	}
	validateOptionalText(validation, "printed_holder_name", normalized.PrintedHolderName, MaxHolderNameLength)
	validateOptionalText(validation, "printed_address", normalized.PrintedAddress, MaxPrintedAddress)
	validateOptionalText(validation, "reference_value", normalized.Reference, MaxReferenceLength)
	if normalized.Competence != "" && !competencePattern.MatchString(normalized.Competence) {
		validation.add("competence", "invalid_format")
	}
	if normalized.Amount == "" {
		if normalized.Currency != "" {
			validation.add("currency", "unexpected")
		}
	} else {
		amount, ok := canonicalAmount(normalized.Amount)
		if !ok {
			validation.add("amount", "invalid_format")
		} else {
			normalized.Amount = amount
		}
		if !currencyPattern.MatchString(normalized.Currency) {
			validation.add("currency", "invalid_format")
		}
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

func canonicalAmount(value string) (string, bool) {
	if !amountPattern.MatchString(value) {
		return "", false
	}
	parts := strings.SplitN(value, ".", 2)
	whole := strings.TrimLeft(parts[0], "0")
	if whole == "" {
		whole = "0"
	}
	if len(whole) > 16 {
		return "", false
	}
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
	}
	if _, err := strconv.ParseUint(whole, 10, 64); err != nil {
		return "", false
	}
	return whole + "." + fraction, true
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

func validateOptionalText(validation *ValidationError, field, value string, maximum int) {
	if value != "" && utf8.RuneCountInString(value) > maximum {
		validation.add(field, "too_long")
	}
}

var (
	technicalKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	competencePattern   = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
	currencyPattern     = regexp.MustCompile(`^[A-Z]{3}$`)
	amountPattern       = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,2})?$`)
)

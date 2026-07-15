package profile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

const (
	MaxNameLength         = 200
	MaxEmailLength        = 320
	MaxStreetLength       = 200
	MaxAddressNumber      = 30
	MaxComplementLength   = 100
	MaxNeighborhoodLength = 100
	MaxCityLength         = 100
	MaxNotesLength        = 5000
)

var ErrInvalidIdentifier = errors.New("invalid profile identifier")

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

type Address struct {
	Street       string
	Number       string
	Complement   string
	Neighborhood string
	City         string
	State        string
	PostalCode   string
}

type Values struct {
	FullName      string
	SocialName    string
	CPF           string
	Email         string
	MobilePhone   string
	LandlinePhone string
	Address       Address
	Notes         string
}

type Profile struct {
	ID        Identifier
	Values    Values
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FieldError struct {
	Field string
	Code  string
}

type ValidationError struct {
	Fields []FieldError
}

func (err *ValidationError) Error() string {
	return "profile validation failed"
}

func Normalize(values Values) (Values, error) {
	normalized := Values{
		FullName:   normalize.DisplayText(values.FullName),
		SocialName: normalize.DisplayText(values.SocialName),
		Address: Address{
			Street:       normalize.DisplayText(values.Address.Street),
			Number:       normalize.DisplayText(values.Address.Number),
			Complement:   normalize.DisplayText(values.Address.Complement),
			Neighborhood: normalize.DisplayText(values.Address.Neighborhood),
			City:         normalize.DisplayText(values.Address.City),
			State:        strings.ToUpper(normalize.DisplayText(values.Address.State)),
		},
		Notes: strings.TrimSpace(strings.ToValidUTF8(values.Notes, "")),
	}

	validation := &ValidationError{}
	validateRequiredText(validation, "full_name", normalized.FullName, MaxNameLength)
	validateOptionalText(validation, "social_name", normalized.SocialName, MaxNameLength)
	validateOptionalText(validation, "address.street", normalized.Address.Street, MaxStreetLength)
	validateOptionalText(validation, "address.number", normalized.Address.Number, MaxAddressNumber)
	validateOptionalText(validation, "address.complement", normalized.Address.Complement, MaxComplementLength)
	validateOptionalText(validation, "address.neighborhood", normalized.Address.Neighborhood, MaxNeighborhoodLength)
	validateOptionalText(validation, "address.city", normalized.Address.City, MaxCityLength)
	if normalized.Notes != "" && utf8.RuneCountInString(normalized.Notes) > MaxNotesLength {
		validation.add("notes", "too_long")
	}

	if strings.TrimSpace(values.CPF) != "" {
		cpf, err := normalize.CanonicalCPF(values.CPF)
		if err != nil {
			validation.add("cpf", normalizationCode(err))
		} else {
			normalized.CPF = cpf
		}
	}
	if strings.TrimSpace(values.Email) != "" {
		email, err := normalize.CanonicalEmail(values.Email)
		if err != nil {
			validation.add("email", normalizationCode(err))
		} else {
			normalized.Email = strings.ToLower(email)
			if utf8.RuneCountInString(normalized.Email) > MaxEmailLength {
				validation.add("email", "too_long")
			}
		}
	}
	if strings.TrimSpace(values.MobilePhone) != "" {
		phone, err := normalize.CanonicalBrazilPhone(values.MobilePhone)
		if err != nil {
			validation.add("mobile_phone", normalizationCode(err))
		} else if len(phone) != 14 {
			validation.add("mobile_phone", "not_mobile")
		} else {
			normalized.MobilePhone = phone
		}
	}
	if strings.TrimSpace(values.LandlinePhone) != "" {
		phone, err := normalize.CanonicalBrazilPhone(values.LandlinePhone)
		if err != nil {
			validation.add("landline_phone", normalizationCode(err))
		} else if len(phone) != 13 {
			validation.add("landline_phone", "not_landline")
		} else {
			normalized.LandlinePhone = phone
		}
	}

	if normalized.Address.State != "" && !validState(normalized.Address.State) {
		validation.add("address.state", "invalid_format")
	}
	if strings.TrimSpace(values.Address.PostalCode) != "" {
		postalCode, ok := canonicalPostalCode(values.Address.PostalCode)
		if !ok {
			validation.add("address.postal_code", "invalid_format")
		} else {
			normalized.Address.PostalCode = postalCode
		}
	}

	if len(validation.Fields) > 0 {
		return Values{}, validation
	}
	return normalized, nil
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

func normalizationCode(err error) string {
	var validationError *normalize.ValidationError
	if errors.As(err, &validationError) {
		return string(validationError.Code)
	}
	return "invalid_format"
}

func validState(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func canonicalPostalCode(value string) (string, bool) {
	for _, character := range value {
		if unicode.IsSpace(character) || character == '-' || (character >= '0' && character <= '9') {
			continue
		}
		return "", false
	}
	digits := normalize.Digits(value)
	return digits, len(digits) == 8
}

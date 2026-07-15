package auth

import (
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidIdentifier = errors.New("invalid identifier")

func ParseIdentifier(value string) (Identifier, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if len(normalized) != 36 ||
		normalized[8] != '-' ||
		normalized[13] != '-' ||
		normalized[18] != '-' ||
		normalized[23] != '-' {
		return Identifier{}, ErrInvalidIdentifier
	}

	raw := strings.ReplaceAll(normalized, "-", "")
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != len(Identifier{}) {
		return Identifier{}, ErrInvalidIdentifier
	}

	var identifier Identifier
	copy(identifier[:], decoded)
	return identifier, nil
}

func (identifier Identifier) String() string {
	raw := hex.EncodeToString(identifier[:])
	return raw[0:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32]
}

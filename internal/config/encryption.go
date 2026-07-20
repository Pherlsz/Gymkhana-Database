package config

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

func decodeEncryptionKey(value string) ([32]byte, error) {
	if value == "" {
		return [32]byte{}, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil || len(decoded) != 32 {
		return [32]byte{}, errors.New("must be base64-encoded 32 bytes")
	}
	var key [32]byte
	copy(key[:], decoded)
	return key, nil
}

func decodeEncryptionKeys(value string, currentVersion uint16, currentKey [32]byte) (map[uint16][32]byte, error) {
	result := make(map[uint16][32]byte)
	if currentVersion > 0 && currentKey != ([32]byte{}) {
		result[currentVersion] = currentKey
	}
	if value == "" {
		return result, nil
	}
	for _, item := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("must use version:base64 entries")
		}
		parsed, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 16)
		if err != nil || parsed == 0 {
			return nil, errors.New("contains an invalid key version")
		}
		key, err := decodeEncryptionKey(strings.TrimSpace(parts[1]))
		if err != nil || key == ([32]byte{}) {
			return nil, errors.New("contains an invalid encryption key")
		}
		version := uint16(parsed)
		if existing, duplicate := result[version]; duplicate && existing != key {
			return nil, errors.New("contains conflicting keys for one version")
		}
		result[version] = key
	}
	return result, nil
}

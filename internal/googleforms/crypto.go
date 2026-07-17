package googleforms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

type Ciphertext struct {
	Data       []byte
	Nonce      []byte
	KeyVersion uint16
}

type TokenCipher struct {
	currentVersion uint16
	keys           map[uint16][32]byte
	random         io.Reader
}

func NewTokenCipher(currentVersion uint16, keys map[uint16][32]byte) (*TokenCipher, error) {
	if currentVersion == 0 || len(keys) == 0 {
		return nil, ErrInvalidInput
	}
	copied := make(map[uint16][32]byte, len(keys))
	for version, key := range keys {
		if version == 0 || key == ([32]byte{}) {
			return nil, ErrInvalidInput
		}
		copied[version] = key
	}
	if _, ok := copied[currentVersion]; !ok {
		return nil, ErrInvalidInput
	}
	return &TokenCipher{currentVersion: currentVersion, keys: copied, random: rand.Reader}, nil
}

func (value *TokenCipher) Seal(plaintext []byte, additionalData string) (Ciphertext, error) {
	if value == nil || len(plaintext) == 0 || additionalData == "" {
		return Ciphertext{}, ErrInvalidInput
	}
	aead, err := value.aead(value.currentVersion)
	if err != nil {
		return Ciphertext{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(value.random, nonce); err != nil {
		return Ciphertext{}, fmt.Errorf("generate token nonce: %w", err)
	}
	sealed := aead.Seal(nil, nonce, plaintext, []byte(additionalData))
	return Ciphertext{Data: sealed, Nonce: nonce, KeyVersion: value.currentVersion}, nil
}

func (value *TokenCipher) Open(encrypted Ciphertext, additionalData string) ([]byte, error) {
	if value == nil || len(encrypted.Data) == 0 || len(encrypted.Nonce) == 0 || additionalData == "" {
		return nil, ErrInvalidInput
	}
	aead, err := value.aead(encrypted.KeyVersion)
	if err != nil {
		return nil, err
	}
	if len(encrypted.Nonce) != aead.NonceSize() {
		return nil, ErrInvalidInput
	}
	plaintext, err := aead.Open(nil, encrypted.Nonce, encrypted.Data, []byte(additionalData))
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", ErrInvalidState)
	}
	return plaintext, nil
}

func (value *TokenCipher) aead(version uint16) (cipher.AEAD, error) {
	key, ok := value.keys[version]
	if !ok {
		return nil, fmt.Errorf("unknown token key version: %w", ErrInvalidState)
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("configure token cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("configure token aead: %w", err)
	}
	return aead, nil
}

package googleforms

import (
	"bytes"
	"testing"
)

func TestTokenCipherRoundTripAndTamperResistance(t *testing.T) {
	key := [32]byte{1, 2, 3, 4}
	cipher, err := NewTokenCipher(7, map[uint16][32]byte{7: key})
	if err != nil {
		t.Fatalf("NewTokenCipher() error = %v", err)
	}
	cipher.random = bytes.NewReader(bytes.Repeat([]byte{9}, 64))
	encrypted, err := cipher.Seal([]byte("refresh-token"), "connection:owner")
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if encrypted.KeyVersion != 7 || bytes.Contains(encrypted.Data, []byte("refresh-token")) {
		t.Fatalf("encrypted = %#v", encrypted)
	}
	plaintext, err := cipher.Open(encrypted, "connection:owner")
	if err != nil || string(plaintext) != "refresh-token" {
		t.Fatalf("Open() = %q, %v", plaintext, err)
	}

	tampered := encrypted
	tampered.Data = append([]byte(nil), encrypted.Data...)
	tampered.Data[0] ^= 1
	if _, err := cipher.Open(tampered, "connection:owner"); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
	if _, err := cipher.Open(encrypted, "connection:another-owner"); err == nil {
		t.Fatal("ciphertext with wrong additional data was accepted")
	}
}

func TestTokenCipherSupportsVersionedDecryption(t *testing.T) {
	oldKey := [32]byte{1}
	newKey := [32]byte{2}
	oldCipher, err := NewTokenCipher(1, map[uint16][32]byte{1: oldKey})
	if err != nil {
		t.Fatal(err)
	}
	oldCipher.random = bytes.NewReader(bytes.Repeat([]byte{3}, 32))
	encrypted, err := oldCipher.Seal([]byte("secret"), "owner")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := NewTokenCipher(2, map[uint16][32]byte{1: oldKey, 2: newKey})
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := rotated.Open(encrypted, "owner")
	if err != nil || string(plaintext) != "secret" {
		t.Fatalf("Open() = %q, %v", plaintext, err)
	}
	withoutOldKey, err := NewTokenCipher(2, map[uint16][32]byte{2: newKey})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := withoutOldKey.Open(encrypted, "owner"); err == nil {
		t.Fatal("unknown historical key version was accepted")
	}
}

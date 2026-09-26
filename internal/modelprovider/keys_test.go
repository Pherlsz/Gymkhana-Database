package modelprovider

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/googleforms"
)

type memoryKeyStore struct{ records map[string]KeyRecord }

func (store *memoryKeyStore) Load(_ context.Context, provider string) (KeyRecord, bool, error) {
	record, ok := store.records[provider]
	return record, ok, nil
}

func (store *memoryKeyStore) Save(_ context.Context, record KeyRecord) error {
	if store.records == nil {
		store.records = map[string]KeyRecord{}
	}
	store.records[record.Provider] = record
	return nil
}

func (store *memoryKeyStore) Delete(_ context.Context, provider string) error {
	delete(store.records, provider)
	return nil
}

func newKeyFixture(t *testing.T) (*KeyService, *memoryKeyStore) {
	t.Helper()
	var key [32]byte
	copy(key[:], bytes.Repeat([]byte{9}, 32))
	cipher, err := googleforms.NewTokenCipher(1, map[uint16][32]byte{1: key})
	if err != nil {
		t.Fatalf("NewTokenCipher() error = %v", err)
	}
	store := &memoryKeyStore{}
	service, err := NewKeyService(store, cipher, func() time.Time { return time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("NewKeyService() error = %v", err)
	}
	return service, store
}

func adminSession() auth.Session {
	id, _ := auth.NewIdentifier()
	return auth.Session{User: auth.User{ID: id, Email: "admin@example.test", Role: auth.RoleAdmin, Active: true}}
}

func TestKeyServiceSealsResolvesAndClears(t *testing.T) {
	service, store := newKeyFixture(t)
	ctx := context.Background()
	admin := adminSession()
	secret := "AIzaSy-test-secret-0123456789abcdef"

	status, err := service.Set(ctx, admin, ProviderGoogle, "  "+secret+"  ", " gemini-2.5-flash ")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !status.Configured || status.Model != "gemini-2.5-flash" || status.UpdatedAt == nil {
		t.Fatalf("Set() status = %#v", status)
	}
	if stored := store.records[ProviderGoogle]; bytes.Contains(stored.Secret.Data, []byte(secret)) || stored.Secret.KeyVersion != 1 {
		t.Fatalf("secret persisted in clear or without key version: %#v", stored.Secret)
	}

	plaintext, model, err := service.Resolve(ctx, ProviderGoogle)
	if err != nil || plaintext != secret || model != "gemini-2.5-flash" {
		t.Fatalf("Resolve() = %q, %q, %v", plaintext, model, err)
	}
	if !service.Configured(ctx, ProviderGoogle) {
		t.Fatal("Configured() = false after Set")
	}

	if err := service.Clear(ctx, admin, ProviderGoogle); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, _, err := service.Resolve(ctx, ProviderGoogle); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Resolve() after Clear error = %v, want ErrNotConfigured", err)
	}
	status, err = service.Status(ctx, admin, ProviderGoogle)
	if err != nil || status.Configured || status.Model != "" {
		t.Fatalf("Status() after Clear = %#v, %v", status, err)
	}
}

func TestKeyServiceRejectsExternalAndInvalidInput(t *testing.T) {
	service, _ := newKeyFixture(t)
	ctx := context.Background()
	external := adminSession()
	external.User.Role = auth.RoleExternal

	if _, err := service.Set(ctx, external, ProviderGoogle, "AIzaSy-test-secret-0123456789abcdef", "gemini"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Set() by EXTERNAL error = %v, want ErrForbidden", err)
	}
	if _, err := service.Status(ctx, external, ProviderGoogle); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Status() by EXTERNAL error = %v, want ErrForbidden", err)
	}
	admin := adminSession()
	for name, input := range map[string][3]string{
		"unknown provider":  {"openai", "AIzaSy-test-secret-0123456789abcdef", "gpt"},
		"short secret":      {ProviderGoogle, "short", "gemini"},
		"secret with space": {ProviderGoogle, "AIzaSy test secret 0123456789abcdef", "gemini"},
		"empty model":       {ProviderGoogle, "AIzaSy-test-secret-0123456789abcdef", "   "},
	} {
		if _, err := service.Set(ctx, admin, input[0], input[1], input[2]); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Set(%s) error = %v, want ErrInvalidInput", name, err)
		}
	}
}

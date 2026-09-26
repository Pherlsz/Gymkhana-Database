// Package modelprovider owns the shared model-provider key managed in
// Administração and the concrete provider adapters that consume it.
package modelprovider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/googleforms"
)

const (
	ProviderGoogle = "google"

	minimumSecretLength = 20
	maximumSecretLength = 512
	maximumModelLength  = 120
)

var (
	ErrForbidden     = errors.New("model key administration is forbidden")
	ErrInvalidInput  = errors.New("model key input is invalid")
	ErrNotConfigured = errors.New("model key is not configured")
)

// KeyRecord is the persisted sealed secret. The plaintext never leaves Resolve.
type KeyRecord struct {
	Provider  string
	Secret    googleforms.Ciphertext
	Model     string
	UpdatedBy auth.Identifier
	UpdatedAt time.Time
}

type KeyStore interface {
	Load(context.Context, string) (KeyRecord, bool, error)
	Save(context.Context, KeyRecord) error
	Delete(context.Context, string) error
}

// KeyStatus is the admin-visible state of one provider key.
type KeyStatus struct {
	Provider   string
	Configured bool
	Model      string
	UpdatedAt  *time.Time
}

type KeyService struct {
	store  KeyStore
	cipher *googleforms.TokenCipher
	now    func() time.Time
}

// NewKeyService reuses the application token cipher (AES-256-GCM, versioned)
// already used for Google Forms refresh tokens.
func NewKeyService(store KeyStore, cipher *googleforms.TokenCipher, now func() time.Time) (*KeyService, error) {
	if store == nil || cipher == nil {
		return nil, ErrInvalidInput
	}
	if now == nil {
		now = time.Now
	}
	return &KeyService{store: store, cipher: cipher, now: now}, nil
}

// NewSealedKeyService wires the PostgreSQL store to the shared token cipher.
func NewSealedKeyService(pool *pgxpool.Pool, version uint16, keys map[uint16][32]byte, now func() time.Time) (*KeyService, error) {
	if pool == nil {
		return nil, ErrInvalidInput
	}
	cipher, err := googleforms.NewTokenCipher(version, keys)
	if err != nil {
		return nil, err
	}
	return NewKeyService(NewPostgresKeyStore(pool), cipher, now)
}

func SupportedProvider(provider string) bool {
	return provider == ProviderGoogle
}

// Status is admin-only and never includes the secret.
func (service *KeyService) Status(ctx context.Context, actor auth.Session, provider string) (KeyStatus, error) {
	if !actor.User.Role.CanManageUsers() {
		return KeyStatus{}, ErrForbidden
	}
	if !SupportedProvider(provider) {
		return KeyStatus{}, ErrInvalidInput
	}
	record, found, err := service.store.Load(ctx, provider)
	if err != nil {
		return KeyStatus{}, err
	}
	status := KeyStatus{Provider: provider, Configured: found}
	if found {
		status.Model = record.Model
		updatedAt := record.UpdatedAt
		status.UpdatedAt = &updatedAt
	}
	return status, nil
}

// Set seals and stores the provider secret. Replacing an existing key is the
// rotation path; there is no versioned history by design.
func (service *KeyService) Set(ctx context.Context, actor auth.Session, provider, secret, model string) (KeyStatus, error) {
	if !actor.User.Role.CanManageUsers() {
		return KeyStatus{}, ErrForbidden
	}
	secret = strings.TrimSpace(secret)
	model = strings.TrimSpace(model)
	if !SupportedProvider(provider) || !validSecret(secret) || !validModel(model) {
		return KeyStatus{}, ErrInvalidInput
	}
	sealed, err := service.cipher.Seal([]byte(secret), sealContext(provider))
	if err != nil {
		return KeyStatus{}, fmt.Errorf("seal model key: %w", err)
	}
	now := service.now().UTC()
	if err := service.store.Save(ctx, KeyRecord{Provider: provider, Secret: sealed, Model: model, UpdatedBy: actor.User.ID, UpdatedAt: now}); err != nil {
		return KeyStatus{}, err
	}
	return KeyStatus{Provider: provider, Configured: true, Model: model, UpdatedAt: &now}, nil
}

func (service *KeyService) Clear(ctx context.Context, actor auth.Session, provider string) error {
	if !actor.User.Role.CanManageUsers() {
		return ErrForbidden
	}
	if !SupportedProvider(provider) {
		return ErrInvalidInput
	}
	return service.store.Delete(ctx, provider)
}

// Resolve returns the plaintext secret and configured model for internal use by
// provider adapters. It has no actor: the key is shared, and callers are already
// authorized by the chat capability.
func (service *KeyService) Resolve(ctx context.Context, provider string) (secret, model string, err error) {
	if !SupportedProvider(provider) {
		return "", "", ErrInvalidInput
	}
	record, found, err := service.store.Load(ctx, provider)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", ErrNotConfigured
	}
	plaintext, err := service.cipher.Open(record.Secret, sealContext(provider))
	if err != nil {
		return "", "", fmt.Errorf("open model key: %w", err)
	}
	return string(plaintext), record.Model, nil
}

// Configured reports whether the provider can be used right now.
func (service *KeyService) Configured(ctx context.Context, provider string) bool {
	_, found, err := service.store.Load(ctx, provider)
	return err == nil && found
}

func sealContext(provider string) string {
	return "ai_model_keys:" + provider
}

func validSecret(value string) bool {
	if len(value) < minimumSecretLength || len(value) > maximumSecretLength || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if r > unicode.MaxASCII || unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func validModel(value string) bool {
	if value == "" || utf8.RuneCountInString(value) > maximumModelLength || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

type PostgresKeyStore struct{ pool *pgxpool.Pool }

func NewPostgresKeyStore(pool *pgxpool.Pool) *PostgresKeyStore { return &PostgresKeyStore{pool: pool} }

func (store *PostgresKeyStore) Load(ctx context.Context, provider string) (KeyRecord, bool, error) {
	var record KeyRecord
	var keyVersion int32
	var updatedBy pgtype.UUID
	err := store.pool.QueryRow(ctx, `SELECT provider, secret_ciphertext, secret_nonce, key_version, model, updated_by, updated_at
		FROM ai_model_keys WHERE provider=$1`, provider).
		Scan(&record.Provider, &record.Secret.Data, &record.Secret.Nonce, &keyVersion, &record.Model, &updatedBy, &record.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return KeyRecord{}, false, nil
	}
	if err != nil {
		return KeyRecord{}, false, fmt.Errorf("load model key: %w", err)
	}
	record.Secret.KeyVersion = uint16(keyVersion)
	if updatedBy.Valid {
		record.UpdatedBy = auth.Identifier(updatedBy.Bytes)
	}
	return record, true, nil
}

func (store *PostgresKeyStore) Save(ctx context.Context, record KeyRecord) error {
	updatedBy := pgtype.UUID{Bytes: [16]byte(record.UpdatedBy), Valid: record.UpdatedBy != (auth.Identifier{})}
	_, err := store.pool.Exec(ctx, `INSERT INTO ai_model_keys (provider, secret_ciphertext, secret_nonce, key_version, model, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$7)
		ON CONFLICT (provider) DO UPDATE SET secret_ciphertext=EXCLUDED.secret_ciphertext, secret_nonce=EXCLUDED.secret_nonce,
			key_version=EXCLUDED.key_version, model=EXCLUDED.model, updated_by=EXCLUDED.updated_by, updated_at=EXCLUDED.updated_at`,
		record.Provider, record.Secret.Data, record.Secret.Nonce, int32(record.Secret.KeyVersion), record.Model, updatedBy, record.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save model key: %w", err)
	}
	return nil
}

func (store *PostgresKeyStore) Delete(ctx context.Context, provider string) error {
	if _, err := store.pool.Exec(ctx, `DELETE FROM ai_model_keys WHERE provider=$1`, provider); err != nil {
		return fmt.Errorf("delete model key: %w", err)
	}
	return nil
}

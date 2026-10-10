package operations

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) GetActor(ctx context.Context, id auth.Identifier) (auth.Session, error) {
	var user auth.User
	var userID pgtype.UUID
	var avatar pgtype.Text
	if err := store.pool.QueryRow(ctx, `SELECT id, email, display_name, avatar_url, role, active
 FROM app_users WHERE id=$1`, authDatabaseUUID(id)).Scan(&userID, &user.Email, &user.DisplayName, &avatar, &user.Role, &user.Active); errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, ErrForbidden
	} else if err != nil {
		return auth.Session{}, fmt.Errorf("get operation actor: %w", err)
	}
	user.ID = authIdentifierFromUUID(userID)
	if avatar.Valid {
		user.AvatarURL = avatar.String
	}
	if !user.Active || !user.Role.Valid() {
		return auth.Session{}, ErrForbidden
	}
	return auth.Session{User: user}, nil
}

func (store *PostgresStore) DocumentType(ctx context.Context, id document.Identifier) (document.TypeDefinition, error) {
	var value document.TypeDefinition
	var databaseID pgtype.UUID
	var validation pgtype.Text
	if err := store.pool.QueryRow(ctx, `SELECT id, technical_key, label, active,
  uniqueness_policy, validation_regex, date_required, version, created_at, updated_at
 FROM document_types WHERE id=$1`, databaseUUID(Identifier(id))).Scan(&databaseID,
		&value.Values.TechnicalKey, &value.Values.Label, &value.Values.Active,
		&value.Values.UniquenessPolicy, &validation, &value.Values.DateRequired,
		&value.Version, &value.CreatedAt, &value.UpdatedAt); errors.Is(err, pgx.ErrNoRows) {
		return document.TypeDefinition{}, ErrInvalidInput
	} else if err != nil {
		return document.TypeDefinition{}, fmt.Errorf("get document type for import: %w", err)
	}
	value.ID = document.Identifier(databaseID.Bytes)
	if validation.Valid {
		value.Values.ValidationRegex = validation.String
	}
	return value, nil
}

func (store *PostgresStore) BillType(ctx context.Context, id bill.Identifier) (bill.TypeDefinition, error) {
	var value bill.TypeDefinition
	var databaseID pgtype.UUID
	if err := store.pool.QueryRow(ctx, `SELECT id, technical_key, label, active,
  version, created_at, updated_at
 FROM bill_types WHERE id=$1`, databaseUUID(Identifier(id))).Scan(&databaseID,
		&value.Values.TechnicalKey, &value.Values.Label, &value.Values.Active,
		&value.Version, &value.CreatedAt, &value.UpdatedAt); errors.Is(err, pgx.ErrNoRows) {
		return bill.TypeDefinition{}, ErrInvalidInput
	} else if err != nil {
		return bill.TypeDefinition{}, fmt.Errorf("get bill type for import: %w", err)
	}
	value.ID = bill.Identifier(databaseID.Bytes)
	return value, nil
}

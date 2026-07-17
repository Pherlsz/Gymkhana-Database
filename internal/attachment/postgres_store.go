package attachment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (store *PostgresStore) CreateUploadIntent(ctx context.Context, value UploadIntent) (UploadIntent, error) {
	owner := ownerDatabaseValues(value.Owner)
	row := store.pool.QueryRow(ctx, `INSERT INTO attachment_upload_intents
(id, actor_user_id, owner_kind, document_id, bill_id, custom_target_kind, custom_profile_id,
 custom_document_id, custom_bill_id, custom_entity_id, field_definition_id, original_filename,
 declared_mime, expected_size, object_key, expires_at, created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
RETURNING id, actor_user_id, owner_kind, document_id, bill_id, custom_target_kind, custom_profile_id,
 custom_document_id, custom_bill_id, custom_entity_id, field_definition_id, original_filename,
 declared_mime, expected_size, object_key, expires_at, consumed_at, created_at`,
		databaseUUID(value.ID), authDatabaseUUID(value.ActorUserID), string(value.Owner.Kind), owner.documentID,
		owner.billID, owner.customTargetKind, owner.customProfileID, owner.customDocumentID, owner.customBillID,
		owner.customEntityID, owner.fieldDefinitionID, value.OriginalFileName, value.DeclaredMIME,
		value.ExpectedSize, value.ObjectKey, value.ExpiresAt, value.CreatedAt)
	created, err := scanUploadIntent(row)
	if err != nil {
		return UploadIntent{}, mapPostgresError("create attachment upload intent", err)
	}
	return created, nil
}

func (store *PostgresStore) GetUploadIntent(ctx context.Context, id Identifier) (UploadIntent, error) {
	row := store.pool.QueryRow(ctx, `SELECT id, actor_user_id, owner_kind, document_id, bill_id, custom_target_kind,
 custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id, field_definition_id,
 original_filename, declared_mime, expected_size, object_key, expires_at, consumed_at, created_at
FROM attachment_upload_intents WHERE id=$1`, databaseUUID(id))
	value, err := scanUploadIntent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return UploadIntent{}, ErrUploadIntentNotFound
	}
	if err != nil {
		return UploadIntent{}, fmt.Errorf("get attachment upload intent: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ConfirmUploadIntent(ctx context.Context, intentID Identifier, actorID auth.Identifier, attachmentID Identifier, verified VerifiedObject, now time.Time) (Attachment, error) {
	var result Attachment
	err := pgx.BeginTxFunc(ctx, store.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `SELECT id, actor_user_id, owner_kind, document_id, bill_id, custom_target_kind,
 custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id, field_definition_id,
 original_filename, declared_mime, expected_size, object_key, expires_at, consumed_at, created_at
FROM attachment_upload_intents WHERE id=$1 FOR UPDATE`, databaseUUID(intentID))
		intent, err := scanUploadIntent(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUploadIntentNotFound
		}
		if err != nil {
			return fmt.Errorf("lock attachment upload intent: %w", err)
		}
		if intent.ActorUserID != actorID {
			return ErrForbidden
		}
		if intent.ConsumedAt != nil {
			return ErrUploadIntentConsumed
		}
		if !intent.ExpiresAt.After(now) {
			return ErrUploadIntentExpired
		}
		if verified.ByteSize != intent.ExpectedSize {
			return ErrInvalidSize
		}
		owner := ownerDatabaseValues(intent.Owner)
		attachmentRow := tx.QueryRow(ctx, `INSERT INTO attachments
(id, owner_kind, document_id, bill_id, custom_target_kind, custom_profile_id, custom_document_id,
 custom_bill_id, custom_entity_id, field_definition_id, original_filename, declared_mime, detected_mime,
 byte_size, sha256, object_key, lifecycle_state, version, created_at, updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,'ACTIVE',1,$17,$17)
RETURNING id, owner_kind, document_id, bill_id, custom_target_kind, custom_profile_id, custom_document_id,
 custom_bill_id, custom_entity_id, field_definition_id, original_filename, declared_mime, detected_mime,
 byte_size, sha256, object_key, lifecycle_state, deleted_at, purge_after, version, created_at, updated_at`,
			databaseUUID(attachmentID), string(intent.Owner.Kind), owner.documentID, owner.billID,
			owner.customTargetKind, owner.customProfileID, owner.customDocumentID, owner.customBillID,
			owner.customEntityID, owner.fieldDefinitionID, intent.OriginalFileName, intent.DeclaredMIME,
			verified.DetectedMIME, verified.ByteSize, verified.SHA256[:], intent.ObjectKey, now)
		result, err = scanAttachment(attachmentRow)
		if err != nil {
			return mapPostgresError("insert attachment", err)
		}
		command, err := tx.Exec(ctx, `UPDATE attachment_upload_intents
SET consumed_at=$2
WHERE id=$1 AND consumed_at IS NULL`, databaseUUID(intentID), now)
		if err != nil {
			return fmt.Errorf("consume attachment upload intent: %w", err)
		}
		if command.RowsAffected() != 1 {
			return ErrUploadIntentConsumed
		}
		return nil
	})
	if err != nil {
		return Attachment{}, err
	}
	return result, nil
}

func (store *PostgresStore) List(ctx context.Context, owner OwnerReference, includeTrashed bool) ([]Attachment, error) {
	condition, arguments, err := ownerCondition(owner, 1)
	if err != nil {
		return nil, err
	}
	stateCondition := " AND lifecycle_state='ACTIVE'"
	if includeTrashed {
		stateCondition = ""
	}
	rows, err := store.pool.Query(ctx, `SELECT id, owner_kind, document_id, bill_id, custom_target_kind,
 custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id, field_definition_id,
 original_filename, declared_mime, detected_mime, byte_size, sha256, object_key, lifecycle_state,
 deleted_at, purge_after, version, created_at, updated_at
FROM attachments WHERE `+condition+stateCondition+` ORDER BY created_at DESC, id`, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()
	values := make([]Attachment, 0)
	for rows.Next() {
		value, scanErr := scanAttachment(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan attachment: %w", scanErr)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}
	return values, nil
}

func (store *PostgresStore) Get(ctx context.Context, id Identifier) (Attachment, error) {
	row := store.pool.QueryRow(ctx, `SELECT id, owner_kind, document_id, bill_id, custom_target_kind,
 custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id, field_definition_id,
 original_filename, declared_mime, detected_mime, byte_size, sha256, object_key, lifecycle_state,
 deleted_at, purge_after, version, created_at, updated_at
FROM attachments WHERE id=$1`, databaseUUID(id))
	value, err := scanAttachment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrAttachmentNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("get attachment: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) Trash(ctx context.Context, id Identifier, version int64, deletedAt, purgeAfter time.Time) (Attachment, error) {
	row := store.pool.QueryRow(ctx, `UPDATE attachments
SET lifecycle_state='TRASHED', deleted_at=$3, purge_after=$4, version=version+1, updated_at=$3
WHERE id=$1 AND version=$2 AND lifecycle_state='ACTIVE'
RETURNING id, owner_kind, document_id, bill_id, custom_target_kind, custom_profile_id, custom_document_id,
 custom_bill_id, custom_entity_id, field_definition_id, original_filename, declared_mime, detected_mime,
 byte_size, sha256, object_key, lifecycle_state, deleted_at, purge_after, version, created_at, updated_at`,
		databaseUUID(id), version, deletedAt, purgeAfter)
	value, err := scanAttachment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, store.classifyMutation(ctx, id, version, LifecycleActive)
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("trash attachment: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) Restore(ctx context.Context, id Identifier, version int64, now time.Time) (Attachment, error) {
	row := store.pool.QueryRow(ctx, `UPDATE attachments
SET lifecycle_state='ACTIVE', deleted_at=NULL, purge_after=NULL, version=version+1, updated_at=$3
WHERE id=$1 AND version=$2 AND lifecycle_state='TRASHED' AND purge_after>$3
RETURNING id, owner_kind, document_id, bill_id, custom_target_kind, custom_profile_id, custom_document_id,
 custom_bill_id, custom_entity_id, field_definition_id, original_filename, declared_mime, detected_mime,
 byte_size, sha256, object_key, lifecycle_state, deleted_at, purge_after, version, created_at, updated_at`,
		databaseUUID(id), version, now)
	value, err := scanAttachment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, store.classifyMutation(ctx, id, version, LifecycleTrashed)
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("restore attachment: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ListExpiredUploadIntents(ctx context.Context, now time.Time, limit int) ([]UploadIntent, error) {
	rows, err := store.pool.Query(ctx, `SELECT id, actor_user_id, owner_kind, document_id, bill_id, custom_target_kind,
 custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id, field_definition_id,
 original_filename, declared_mime, expected_size, object_key, expires_at, consumed_at, created_at
FROM attachment_upload_intents
WHERE consumed_at IS NULL AND expires_at<=$1
ORDER BY expires_at, id LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired attachment upload intents: %w", err)
	}
	defer rows.Close()
	values := make([]UploadIntent, 0)
	for rows.Next() {
		value, scanErr := scanUploadIntent(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan expired attachment upload intent: %w", scanErr)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired attachment upload intents: %w", err)
	}
	return values, nil
}

func (store *PostgresStore) DeleteExpiredUploadIntent(ctx context.Context, id Identifier, now time.Time) error {
	_, err := store.pool.Exec(ctx, `DELETE FROM attachment_upload_intents
WHERE id=$1 AND consumed_at IS NULL AND expires_at<=$2`, databaseUUID(id), now)
	if err != nil {
		return fmt.Errorf("delete expired attachment upload intent: %w", err)
	}
	return nil
}

func (store *PostgresStore) ListPurgeDue(ctx context.Context, now time.Time, limit int) ([]Attachment, error) {
	rows, err := store.pool.Query(ctx, `SELECT id, owner_kind, document_id, bill_id, custom_target_kind,
 custom_profile_id, custom_document_id, custom_bill_id, custom_entity_id, field_definition_id,
 original_filename, declared_mime, detected_mime, byte_size, sha256, object_key, lifecycle_state,
 deleted_at, purge_after, version, created_at, updated_at
FROM attachments
WHERE lifecycle_state='TRASHED' AND purge_after<=$1
ORDER BY purge_after, id LIMIT $2`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list attachments due for purge: %w", err)
	}
	defer rows.Close()
	values := make([]Attachment, 0)
	for rows.Next() {
		value, scanErr := scanAttachment(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan attachment due for purge: %w", scanErr)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments due for purge: %w", err)
	}
	return values, nil
}

func (store *PostgresStore) DeletePurged(ctx context.Context, id Identifier, version int64) error {
	_, err := store.pool.Exec(ctx, `DELETE FROM attachments
WHERE id=$1 AND version=$2 AND lifecycle_state='TRASHED'`, databaseUUID(id), version)
	if err != nil {
		return fmt.Errorf("delete purged attachment metadata: %w", err)
	}
	return nil
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	var actor any
	if event.ActorUserID != nil {
		actor = authDatabaseUUID(*event.ActorUserID)
	}
	var attachmentID, uploadIntentID, ownerID, ownerKind any
	if event.AttachmentID != nil {
		attachmentID = databaseUUID(*event.AttachmentID)
	}
	if event.UploadIntentID != nil {
		uploadIntentID = databaseUUID(*event.UploadIntentID)
	}
	if event.Owner != nil {
		ownerID = databaseUUID(event.Owner.ID)
		ownerKind = string(event.Owner.Kind)
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO attachment_audit_events
(id, actor_user_id, attachment_id, upload_intent_id, owner_kind, owner_id, event_type, outcome, request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, databaseUUID(event.ID), actor, attachmentID,
		uploadIntentID, ownerKind, ownerID, string(event.EventType), string(event.Outcome), event.RequestID)
	if err != nil {
		return fmt.Errorf("record attachment audit event: %w", err)
	}
	return nil
}

func (store *PostgresStore) classifyMutation(ctx context.Context, id Identifier, version int64, expected LifecycleState) error {
	var currentVersion int64
	var currentState LifecycleState
	err := store.pool.QueryRow(ctx, `SELECT version, lifecycle_state
FROM attachments WHERE id=$1`, databaseUUID(id)).Scan(&currentVersion, &currentState)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAttachmentNotFound
	}
	if err != nil {
		return fmt.Errorf("classify attachment mutation: %w", err)
	}
	if currentVersion != version {
		return ErrConflict
	}
	if currentState != expected {
		return ErrInvalidState
	}
	return ErrConflict
}

type ownerValues struct {
	documentID        any
	billID            any
	customTargetKind  any
	customProfileID   any
	customDocumentID  any
	customBillID      any
	customEntityID    any
	fieldDefinitionID any
}

func ownerDatabaseValues(owner OwnerReference) ownerValues {
	result := ownerValues{}
	switch owner.Kind {
	case OwnerDocument:
		result.documentID = databaseUUID(owner.ID)
	case OwnerBill:
		result.billID = databaseUUID(owner.ID)
	case OwnerCustomField:
		result.customTargetKind = string(owner.CustomTargetKind)
		result.fieldDefinitionID = databaseUUID(owner.FieldDefinitionID)
		switch owner.CustomTargetKind {
		case CustomTargetProfile:
			result.customProfileID = databaseUUID(owner.ID)
		case CustomTargetDocument:
			result.customDocumentID = databaseUUID(owner.ID)
		case CustomTargetBill:
			result.customBillID = databaseUUID(owner.ID)
		case CustomTargetCustomEntity:
			result.customEntityID = databaseUUID(owner.ID)
		}
	}
	return result
}

func ownerCondition(owner OwnerReference, position int) (string, []any, error) {
	if !owner.Valid() {
		return "", nil, ErrInvalidOwner
	}
	placeholder := fmt.Sprintf("$%d", position)
	switch owner.Kind {
	case OwnerDocument:
		return "owner_kind='DOCUMENT' AND document_id=" + placeholder, []any{databaseUUID(owner.ID)}, nil
	case OwnerBill:
		return "owner_kind='BILL' AND bill_id=" + placeholder, []any{databaseUUID(owner.ID)}, nil
	case OwnerCustomField:
		column := map[CustomTargetKind]string{
			CustomTargetProfile:      "custom_profile_id",
			CustomTargetDocument:     "custom_document_id",
			CustomTargetBill:         "custom_bill_id",
			CustomTargetCustomEntity: "custom_entity_id",
		}[owner.CustomTargetKind]
		if column == "" {
			return "", nil, ErrInvalidOwner
		}
		return "owner_kind='CUSTOM_FIELD' AND " + column + "=" + placeholder +
				" AND field_definition_id=" + fmt.Sprintf("$%d", position+1),
			[]any{databaseUUID(owner.ID), databaseUUID(owner.FieldDefinitionID)}, nil
	default:
		return "", nil, ErrInvalidOwner
	}
}

type scanner interface {
	Scan(...any) error
}

func scanUploadIntent(row scanner) (UploadIntent, error) {
	var id, actorID pgtype.UUID
	var ownerKind string
	var documentID, billID, customProfileID, customDocumentID, customBillID, customEntityID, fieldDefinitionID pgtype.UUID
	var customTargetKind pgtype.Text
	var fileName, declaredMIME, objectKey string
	var expectedSize int64
	var expiresAt, consumedAt, createdAt pgtype.Timestamptz
	if err := row.Scan(&id, &actorID, &ownerKind, &documentID, &billID, &customTargetKind, &customProfileID,
		&customDocumentID, &customBillID, &customEntityID, &fieldDefinitionID, &fileName, &declaredMIME,
		&expectedSize, &objectKey, &expiresAt, &consumedAt, &createdAt); err != nil {
		return UploadIntent{}, err
	}
	owner, err := ownerFromDatabase(ownerKind, customTargetKind, documentID, billID, customProfileID, customDocumentID, customBillID, customEntityID, fieldDefinitionID)
	if err != nil {
		return UploadIntent{}, err
	}
	value := UploadIntent{
		ID:               identifierFromUUID(id),
		ActorUserID:      authIdentifierFromUUID(actorID),
		Owner:            owner,
		OriginalFileName: fileName,
		DeclaredMIME:     declaredMIME,
		ExpectedSize:     expectedSize,
		ObjectKey:        objectKey,
		ExpiresAt:        expiresAt.Time,
		CreatedAt:        createdAt.Time,
	}
	if consumedAt.Valid {
		value.ConsumedAt = &consumedAt.Time
	}
	return value, nil
}

func scanAttachment(row scanner) (Attachment, error) {
	var id pgtype.UUID
	var ownerKind string
	var documentID, billID, customProfileID, customDocumentID, customBillID, customEntityID, fieldDefinitionID pgtype.UUID
	var customTargetKind pgtype.Text
	var fileName, declaredMIME, detectedMIME, objectKey string
	var byteSize, version int64
	var digest []byte
	var lifecycleState LifecycleState
	var deletedAt, purgeAfter, createdAt, updatedAt pgtype.Timestamptz
	if err := row.Scan(&id, &ownerKind, &documentID, &billID, &customTargetKind, &customProfileID,
		&customDocumentID, &customBillID, &customEntityID, &fieldDefinitionID, &fileName, &declaredMIME,
		&detectedMIME, &byteSize, &digest, &objectKey, &lifecycleState, &deletedAt, &purgeAfter,
		&version, &createdAt, &updatedAt); err != nil {
		return Attachment{}, err
	}
	owner, err := ownerFromDatabase(ownerKind, customTargetKind, documentID, billID, customProfileID, customDocumentID, customBillID, customEntityID, fieldDefinitionID)
	if err != nil {
		return Attachment{}, err
	}
	if len(digest) != 32 {
		return Attachment{}, errors.New("invalid stored attachment digest")
	}
	value := Attachment{
		ID:               identifierFromUUID(id),
		Owner:            owner,
		OriginalFileName: fileName,
		DeclaredMIME:     declaredMIME,
		DetectedMIME:     detectedMIME,
		ByteSize:         byteSize,
		ObjectKey:        objectKey,
		LifecycleState:   lifecycleState,
		Version:          version,
		CreatedAt:        createdAt.Time,
		UpdatedAt:        updatedAt.Time,
	}
	copy(value.SHA256[:], digest)
	if deletedAt.Valid {
		value.DeletedAt = &deletedAt.Time
	}
	if purgeAfter.Valid {
		value.PurgeAfter = &purgeAfter.Time
	}
	return value, nil
}

func ownerFromDatabase(ownerKind string, customTargetKind pgtype.Text, documentID, billID, customProfileID, customDocumentID, customBillID, customEntityID, fieldDefinitionID pgtype.UUID) (OwnerReference, error) {
	owner := OwnerReference{Kind: OwnerKind(ownerKind)}
	switch owner.Kind {
	case OwnerDocument:
		owner.ID = identifierFromUUID(documentID)
	case OwnerBill:
		owner.ID = identifierFromUUID(billID)
	case OwnerCustomField:
		owner.CustomTargetKind = CustomTargetKind(customTargetKind.String)
		owner.FieldDefinitionID = identifierFromUUID(fieldDefinitionID)
		switch owner.CustomTargetKind {
		case CustomTargetProfile:
			owner.ID = identifierFromUUID(customProfileID)
		case CustomTargetDocument:
			owner.ID = identifierFromUUID(customDocumentID)
		case CustomTargetBill:
			owner.ID = identifierFromUUID(customBillID)
		case CustomTargetCustomEntity:
			owner.ID = identifierFromUUID(customEntityID)
		}
	}
	if !owner.Valid() {
		return OwnerReference{}, ErrInvalidOwner
	}
	return owner, nil
}

func mapPostgresError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23503", "23514":
			return fmt.Errorf("%w: %s", ErrInvalidOwner, operation)
		case "23505":
			return fmt.Errorf("%w: %s", ErrConflict, operation)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func databaseUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}

func authDatabaseUUID(value auth.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: value != auth.Identifier{}}
}

func identifierFromUUID(value pgtype.UUID) Identifier {
	if !value.Valid {
		return Identifier{}
	}
	return Identifier(value.Bytes)
}

func authIdentifierFromUUID(value pgtype.UUID) auth.Identifier {
	if !value.Valid {
		return auth.Identifier{}
	}
	return auth.Identifier(value.Bytes)
}

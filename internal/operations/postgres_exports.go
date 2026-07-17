package operations

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (store *PostgresStore) CreateExport(ctx context.Context, value Export, limits Limits) (Export, error) {
	if store == nil || store.pool == nil || value.ID.IsZero() || !value.Module.Valid() || value.ActorUserID == (auth.Identifier{}) {
		return Export{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Export{}, fmt.Errorf("begin export creation: %w", err)
	}
	defer tx.Rollback(ctx)
	actorID := authDatabaseUUID(value.ActorUserID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 8091))`, actorID); err != nil {
		return Export{}, fmt.Errorf("lock export actor: %w", err)
	}
	existing, err := scanExport(tx.QueryRow(ctx, exportSelect+` WHERE actor_user_id=$1 AND idempotency_key=$2`, actorID, value.IdempotencyKey))
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Export{}, fmt.Errorf("find idempotent export: %w", err)
	}
	if err := reserveOperationLimit(ctx, tx, actorID, limits); err != nil {
		return Export{}, err
	}
	created, err := scanExport(tx.QueryRow(ctx, `INSERT INTO operation_exports (
  id, actor_user_id, module, idempotency_key, state, object_key, filename,
  river_job_id, expires_at, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)
RETURNING id, actor_user_id, module, idempotency_key, state, object_key,
          filename, row_count, byte_size, content_sha256, river_job_id, error_code,
          expires_at, completed_at, version, created_at, updated_at`, databaseUUID(value.ID), actorID,
		value.Module, value.IdempotencyKey, value.State, value.ObjectKey, value.Filename,
		nullablePositiveInt64(value.RiverJobID), value.ExpiresAt, value.CreatedAt))
	if err != nil {
		return Export{}, mapPostgresError("create export", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Export{}, fmt.Errorf("commit export creation: %w", err)
	}
	return created, nil
}

func (store *PostgresStore) GetExport(ctx context.Context, id Identifier, actorID auth.Identifier) (Export, error) {
	value, err := scanExport(store.pool.QueryRow(ctx, exportSelect+` WHERE id=$1 AND actor_user_id=$2`, databaseUUID(id), authDatabaseUUID(actorID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, ErrNotFound
	}
	if err != nil {
		return Export{}, fmt.Errorf("get export: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) getExportTrusted(ctx context.Context, id Identifier) (Export, error) {
	value, err := scanExport(store.pool.QueryRow(ctx, exportSelect+` WHERE id=$1`, databaseUUID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, ErrNotFound
	}
	if err != nil {
		return Export{}, fmt.Errorf("get trusted export: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetExportForWorker(ctx context.Context, id Identifier) (Export, error) {
	return store.getExportTrusted(ctx, id)
}

func (store *PostgresStore) ListExports(ctx context.Context, actorID auth.Identifier, options ListOptions) (ExportPage, error) {
	options = normalizeListOptions(options)
	if options.Limit == 0 {
		return ExportPage{}, ErrInvalidInput
	}
	var total int64
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM operation_exports WHERE actor_user_id=$1`, authDatabaseUUID(actorID)).Scan(&total); err != nil {
		return ExportPage{}, fmt.Errorf("count exports: %w", err)
	}
	rows, err := store.pool.Query(ctx, exportSelect+` WHERE actor_user_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`, authDatabaseUUID(actorID), options.Limit, options.Offset)
	if err != nil {
		return ExportPage{}, fmt.Errorf("list exports: %w", err)
	}
	defer rows.Close()
	page := ExportPage{Total: total, Limit: options.Limit, Offset: options.Offset}
	for rows.Next() {
		value, scanErr := scanExport(rows)
		if scanErr != nil {
			return ExportPage{}, fmt.Errorf("scan export: %w", scanErr)
		}
		page.Exports = append(page.Exports, value)
	}
	if err := rows.Err(); err != nil {
		return ExportPage{}, fmt.Errorf("iterate exports: %w", err)
	}
	return page, nil
}

func (store *PostgresStore) BeginExport(ctx context.Context, id Identifier, now time.Time) (Export, error) {
	value, err := scanExport(store.pool.QueryRow(ctx, `UPDATE operation_exports
   SET state='RUNNING', version=version+1, updated_at=$2
 WHERE id=$1 AND state IN ('QUEUED','RUNNING') AND expires_at>$2
RETURNING id, actor_user_id, module, idempotency_key, state, object_key,
          filename, row_count, byte_size, content_sha256, river_job_id, error_code,
          expires_at, completed_at, version, created_at, updated_at`, databaseUUID(id), now))
	if errors.Is(err, pgx.ErrNoRows) {
		value, getErr := store.getExportTrusted(ctx, id)
		if getErr == nil && value.State == ExportCompleted {
			return value, nil
		}
		if getErr == nil && !value.ExpiresAt.After(now) {
			return Export{}, ErrExpired
		}
		return Export{}, ErrInvalidState
	}
	if err != nil {
		return Export{}, fmt.Errorf("begin export: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) ExportDataset(ctx context.Context, module Module) (ExportDataset, error) {
	switch module {
	case ModuleProfiles:
		rows, err := store.pool.Query(ctx, `SELECT id::text, version::text, full_name,
  COALESCE(social_name,''), COALESCE(cpf,''), COALESCE(email,''),
  COALESCE(mobile_phone,''), COALESCE(landline_phone,''), COALESCE(address_street,''),
  COALESCE(address_number,''), COALESCE(address_complement,''), COALESCE(address_neighborhood,''),
  COALESCE(address_city,''), COALESCE(address_state,''), COALESCE(address_postal_code,''), COALESCE(notes,'')
 FROM profiles ORDER BY id`)
		if err != nil {
			return ExportDataset{}, fmt.Errorf("query profile export: %w", err)
		}
		dataset, err := collectDataset(rows, []string{"record_id", "version", "full_name", "social_name", "cpf", "email", "mobile_phone", "landline_phone", "address_street", "address_number", "address_complement", "address_neighborhood", "address_city", "address_state", "address_postal_code", "notes"})
		if err != nil {
			return ExportDataset{}, err
		}
		return store.appendProfileCustomFieldExport(ctx, dataset)
	case ModuleDocuments:
		rows, err := store.pool.Query(ctx, `SELECT id::text, version::text, owner_profile_id::text,
  document_type_id::text, identifier_value, COALESCE(document_date::text,''),
  COALESCE(notes,''), record_state FROM documents ORDER BY id`)
		if err != nil {
			return ExportDataset{}, fmt.Errorf("query document export: %w", err)
		}
		return collectDataset(rows, []string{"record_id", "version", "owner_profile_id", "document_type_id", "identifier_value", "document_date", "notes", "record_state"})
	case ModuleBills:
		rows, err := store.pool.Query(ctx, `SELECT id::text, version::text, owner_profile_id::text,
  bill_type_id::text, COALESCE(printed_holder_name,''), COALESCE(printed_address,''),
  COALESCE(reference_value,''), COALESCE(competence,''), COALESCE(amount::text,''),
  COALESCE(currency,''), COALESCE(notes,''), record_state FROM bills ORDER BY id`)
		if err != nil {
			return ExportDataset{}, fmt.Errorf("query bill export: %w", err)
		}
		return collectDataset(rows, []string{"record_id", "version", "owner_profile_id", "bill_type_id", "printed_holder_name", "printed_address", "reference_value", "competence", "amount", "currency", "notes", "record_state"})
	default:
		return ExportDataset{}, ErrInvalidInput
	}
}

func (store *PostgresStore) appendProfileCustomFieldExport(ctx context.Context, dataset ExportDataset) (ExportDataset, error) {
	fields, err := store.CustomFields(ctx, ModuleProfiles)
	if err != nil {
		return ExportDataset{}, err
	}
	if len(fields) == 0 {
		return dataset, nil
	}
	positions := make(map[string]int, len(fields))
	for _, field := range fields {
		positions[field.Definition.ID.String()] = len(dataset.Headers)
		dataset.Headers = append(dataset.Headers, field.Field.ID)
	}
	values := make(map[string]map[int]string)
	rows, err := store.pool.Query(ctx, `SELECT value.profile_id::text, value.field_definition_id::text,
       value.field_kind, value.text_value, value.integer_value, value.decimal_value::text,
       value.boolean_value, value.civil_date_value, value.civil_month_value
  FROM custom_field_values value
  JOIN custom_field_definitions definition ON definition.id=value.field_definition_id
 WHERE value.profile_id IS NOT NULL AND definition.target_kind='PROFILE' AND definition.active=true
   AND definition.field_kind IN ('TEXT','LONG_TEXT','INTEGER','DECIMAL','BOOLEAN','CIVIL_DATE','CIVIL_MONTH','EMAIL','PHONE')
 ORDER BY value.profile_id, value.field_definition_id`)
	if err != nil {
		return ExportDataset{}, fmt.Errorf("query profile custom field export: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var profileID, fieldID string
		var kind string
		var textValue, decimalValue, civilMonth pgtype.Text
		var integerValue pgtype.Int8
		var booleanValue pgtype.Bool
		var civilDate pgtype.Date
		if err := rows.Scan(&profileID, &fieldID, &kind, &textValue, &integerValue, &decimalValue, &booleanValue, &civilDate, &civilMonth); err != nil {
			return ExportDataset{}, fmt.Errorf("scan profile custom field export: %w", err)
		}
		position, ok := positions[fieldID]
		if !ok {
			continue
		}
		if values[profileID] == nil {
			values[profileID] = make(map[int]string)
		}
		switch kind {
		case "TEXT", "LONG_TEXT", "EMAIL", "PHONE":
			values[profileID][position] = textValue.String
		case "INTEGER":
			values[profileID][position] = fmt.Sprint(integerValue.Int64)
		case "DECIMAL":
			values[profileID][position] = decimalValue.String
		case "BOOLEAN":
			values[profileID][position] = fmt.Sprint(booleanValue.Bool)
		case "CIVIL_DATE":
			values[profileID][position] = civilDate.Time.Format("2006-01-02")
		case "CIVIL_MONTH":
			values[profileID][position] = civilMonth.String
		}
	}
	if err := rows.Err(); err != nil {
		return ExportDataset{}, fmt.Errorf("iterate profile custom field export: %w", err)
	}
	for index := range dataset.Rows {
		row := make([]string, len(dataset.Headers))
		copy(row, dataset.Rows[index])
		if len(row) > 0 {
			for position, value := range values[row[0]] {
				row[position] = value
			}
		}
		dataset.Rows[index] = row
	}
	return dataset, nil
}

func collectDataset(rows pgx.Rows, headers []string) (ExportDataset, error) {
	defer rows.Close()
	dataset := ExportDataset{Headers: headers}
	for rows.Next() {
		if len(dataset.Rows) >= MaximumExportRows {
			return ExportDataset{}, ErrWorkbookLimit
		}
		values, err := rows.Values()
		if err != nil {
			return ExportDataset{}, fmt.Errorf("read export row: %w", err)
		}
		row := make([]string, len(values))
		for index, value := range values {
			switch typed := value.(type) {
			case string:
				row[index] = typed
			case []byte:
				row[index] = string(typed)
			default:
				row[index] = fmt.Sprint(typed)
			}
		}
		dataset.Rows = append(dataset.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return ExportDataset{}, fmt.Errorf("iterate export rows: %w", err)
	}
	return dataset, nil
}

func (store *PostgresStore) CompleteExport(ctx context.Context, id Identifier, rowCount int, byteSize int64, sha [32]byte, now time.Time) (Export, error) {
	value, err := scanExport(store.pool.QueryRow(ctx, `UPDATE operation_exports
   SET state='COMPLETED', row_count=$2, byte_size=$3, content_sha256=$4,
       completed_at=$5, version=version+1, updated_at=$5
 WHERE id=$1 AND state='RUNNING'
RETURNING id, actor_user_id, module, idempotency_key, state, object_key,
          filename, row_count, byte_size, content_sha256, river_job_id, error_code,
          expires_at, completed_at, version, created_at, updated_at`, databaseUUID(id), rowCount, byteSize, sha[:], now))
	if errors.Is(err, pgx.ErrNoRows) {
		value, getErr := store.getExportTrusted(ctx, id)
		if getErr == nil && value.State == ExportCompleted {
			return value, nil
		}
		return Export{}, ErrInvalidState
	}
	if err != nil {
		return Export{}, fmt.Errorf("complete export: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) FailExport(ctx context.Context, id Identifier, expected ExportState, code string, now time.Time) (bool, error) {
	command, err := store.pool.Exec(ctx, `UPDATE operation_exports
	   SET state='FAILED', error_code=$2, version=version+1, updated_at=$3
 WHERE id=$1 AND state=$4`, databaseUUID(id), code, now, expected)
	if err != nil {
		return false, fmt.Errorf("fail export: %w", err)
	}
	return command.RowsAffected() == 1, nil
}

func (store *PostgresStore) ClaimCleanupCandidates(ctx context.Context, now time.Time, limit int) ([]CleanupCandidate, error) {
	if limit < 1 || limit > 1000 {
		return nil, ErrInvalidInput
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin operation cleanup claim: %w", err)
	}
	defer tx.Rollback(ctx)
	result := make([]CleanupCandidate, 0, limit)
	rows, err := tx.Query(ctx, `WITH candidates AS (
  SELECT id FROM operation_imports
   WHERE expires_at<=$1 AND object_deleted_at IS NULL
	 AND (state NOT IN ('PARSING','RUNNING') OR updated_at<=$3)
     AND (cleanup_claimed_at IS NULL OR cleanup_claimed_at<=$3)
   ORDER BY expires_at, id
   FOR UPDATE SKIP LOCKED
   LIMIT $2
)
UPDATE operation_imports value
   SET state='EXPIRED', stage='CLEANUP', cleanup_claimed_at=$1,
       version=CASE WHEN value.state='EXPIRED' THEN value.version ELSE value.version+1 END,
       updated_at=$1
  FROM candidates
 WHERE value.id=candidates.id
RETURNING value.id, value.actor_user_id, value.module, value.object_key`, now, limit, now.Add(-CleanupClaimTTL))
	if err != nil {
		return nil, fmt.Errorf("claim expired imports: %w", err)
	}
	for rows.Next() {
		value := CleanupCandidate{Kind: "IMPORT"}
		var id, actorID pgtype.UUID
		if err := rows.Scan(&id, &actorID, &value.Module, &value.ObjectKey); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan expired import: %w", err)
		}
		value.ID = identifierFromUUID(id)
		value.ActorUserID = authIdentifierFromUUID(actorID)
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate expired imports: %w", err)
	}
	rows.Close()
	remaining := limit - len(result)
	if remaining > 0 {
		rows, err = tx.Query(ctx, `WITH candidates AS (
  SELECT id FROM operation_exports
   WHERE expires_at<=$1 AND object_deleted_at IS NULL
	 AND (state<>'RUNNING' OR updated_at<=$3)
     AND (cleanup_claimed_at IS NULL OR cleanup_claimed_at<=$3)
   ORDER BY expires_at, id
   FOR UPDATE SKIP LOCKED
   LIMIT $2
)
UPDATE operation_exports value
   SET state='EXPIRED', cleanup_claimed_at=$1,
       version=CASE WHEN value.state='EXPIRED' THEN value.version ELSE value.version+1 END,
       updated_at=$1
  FROM candidates
 WHERE value.id=candidates.id
RETURNING value.id, value.actor_user_id, value.module, value.object_key`, now, remaining, now.Add(-CleanupClaimTTL))
		if err != nil {
			return nil, fmt.Errorf("claim expired exports: %w", err)
		}
		for rows.Next() {
			value := CleanupCandidate{Kind: "EXPORT"}
			var id, actorID pgtype.UUID
			if err := rows.Scan(&id, &actorID, &value.Module, &value.ObjectKey); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan expired export: %w", err)
			}
			value.ID = identifierFromUUID(id)
			value.ActorUserID = authIdentifierFromUUID(actorID)
			result = append(result, value)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("iterate expired exports: %w", err)
		}
		rows.Close()
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit operation cleanup claim: %w", err)
	}
	return result, nil
}

func (store *PostgresStore) MarkObjectDeleted(ctx context.Context, candidate CleanupCandidate, now time.Time) error {
	var query string
	switch candidate.Kind {
	case "IMPORT":
		query = `UPDATE operation_imports SET object_deleted_at=$2, cleanup_claimed_at=NULL, updated_at=$2 WHERE id=$1 AND state='EXPIRED' AND object_deleted_at IS NULL`
	case "EXPORT":
		query = `UPDATE operation_exports SET object_deleted_at=$2, cleanup_claimed_at=NULL, updated_at=$2 WHERE id=$1 AND state='EXPIRED' AND object_deleted_at IS NULL`
	default:
		return ErrInvalidInput
	}
	if _, err := store.pool.Exec(ctx, query, databaseUUID(candidate.ID), now); err != nil {
		return fmt.Errorf("mark operation object deleted: %w", err)
	}
	return nil
}

func (store *PostgresStore) ReleaseCleanupCandidate(ctx context.Context, candidate CleanupCandidate) error {
	var query string
	switch candidate.Kind {
	case "IMPORT":
		query = `UPDATE operation_imports SET cleanup_claimed_at=NULL WHERE id=$1 AND object_deleted_at IS NULL`
	case "EXPORT":
		query = `UPDATE operation_exports SET cleanup_claimed_at=NULL WHERE id=$1 AND object_deleted_at IS NULL`
	default:
		return ErrInvalidInput
	}
	if _, err := store.pool.Exec(ctx, query, databaseUUID(candidate.ID)); err != nil {
		return fmt.Errorf("release operation cleanup claim: %w", err)
	}
	return nil
}

func (store *PostgresStore) BulkDelete(ctx context.Context, actorID auth.Identifier, module Module, items []BulkItem, requestID string, now time.Time) (int, error) {
	if !module.Valid() || len(items) == 0 || len(items) > MaximumBulkSelection || requestID == "" || len(requestID) > 128 {
		return 0, ErrInvalidInput
	}
	items = append([]BulkItem(nil), items...)
	sort.Slice(items, func(left, right int) bool { return items[left].ID.String() < items[right].ID.String() })
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin bulk delete: %w", err)
	}
	defer tx.Rollback(ctx)
	var actorActive bool
	var actorRole auth.Role
	if err := tx.QueryRow(ctx, `SELECT active, role FROM app_users WHERE id=$1 FOR SHARE`, authDatabaseUUID(actorID)).Scan(&actorActive, &actorRole); errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrForbidden
	} else if err != nil {
		return 0, fmt.Errorf("authorize operation bulk delete: %w", err)
	}
	catalog, ok := moduleCatalog(actorRole, module)
	if !actorActive || !ok || !catalog.CanBulkDelete {
		return 0, ErrForbidden
	}
	seen := make(map[Identifier]struct{}, len(items))
	for _, item := range items {
		if item.ID.IsZero() || item.Version <= 0 {
			return 0, ErrInvalidInput
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return 0, ErrInvalidInput
		}
		seen[item.ID] = struct{}{}
		if err := deleteCanonicalItem(ctx, tx, actorID, module, item, requestID, now); err != nil {
			return 0, err
		}
	}
	auditID, err := NewIdentifier()
	if err != nil {
		return 0, err
	}
	count := len(items)
	if _, err := tx.Exec(ctx, `INSERT INTO operation_audit_events (
  id, actor_user_id, module, event_type, outcome, affected_count, request_id, created_at
) VALUES ($1,$2,$3,'BULK_DELETE','SUCCESS',$4,$5,$6)`, databaseUUID(auditID), authDatabaseUUID(actorID), module, count, requestID, now); err != nil {
		return 0, fmt.Errorf("audit bulk delete: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit bulk delete: %w", err)
	}
	return count, nil
}

func deleteCanonicalItem(ctx context.Context, tx pgx.Tx, actorID auth.Identifier, module Module, item BulkItem, requestID string, now time.Time) error {
	var table, auditTable, idColumn, eventType string
	switch module {
	case ModuleProfiles:
		table, auditTable, idColumn, eventType = "profiles", "profile_audit_events", "profile_id", "PROFILE_DELETED"
	case ModuleDocuments:
		table, auditTable, idColumn, eventType = "documents", "document_audit_events", "document_id", "DOCUMENT_DELETED"
	case ModuleBills:
		table, auditTable, idColumn, eventType = "bills", "bill_audit_events", "bill_id", "BILL_DELETED"
	default:
		return ErrInvalidInput
	}
	command, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE id=$1 AND version=$2`, databaseUUID(item.ID), item.Version)
	if err != nil {
		return mapPostgresError("bulk delete canonical record", err)
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	auditID, err := NewIdentifier()
	if err != nil {
		return err
	}
	query := `INSERT INTO ` + auditTable + ` (id, actor_user_id, ` + idColumn + `, event_type, outcome, request_id, occurred_at)
VALUES ($1,$2,$3,$4,'SUCCESS',$5,$6)`
	if _, err := tx.Exec(ctx, query, databaseUUID(auditID), authDatabaseUUID(actorID), databaseUUID(item.ID), eventType, requestID, now); err != nil {
		return fmt.Errorf("audit canonical bulk delete: %w", err)
	}
	return nil
}

func (store *PostgresStore) GetActor(ctx context.Context, id auth.Identifier) (auth.Session, error) {
	var user auth.User
	var userID pgtype.UUID
	var avatar pgtype.Text
	if err := store.pool.QueryRow(ctx, `SELECT id, github_user_id, github_login, display_name, avatar_url, role, active
 FROM app_users WHERE id=$1`, authDatabaseUUID(id)).Scan(&userID, &user.GitHubUserID, &user.Login, &user.DisplayName, &avatar, &user.Role, &user.Active); errors.Is(err, pgx.ErrNoRows) {
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
  supports_current_use, version, created_at, updated_at
 FROM bill_types WHERE id=$1`, databaseUUID(Identifier(id))).Scan(&databaseID,
		&value.Values.TechnicalKey, &value.Values.Label, &value.Values.Active,
		&value.Values.SupportsCurrentUse, &value.Version, &value.CreatedAt, &value.UpdatedAt); errors.Is(err, pgx.ErrNoRows) {
		return bill.TypeDefinition{}, ErrInvalidInput
	} else if err != nil {
		return bill.TypeDefinition{}, fmt.Errorf("get bill type for import: %w", err)
	}
	value.ID = bill.Identifier(databaseID.Bytes)
	return value, nil
}

func (store *PostgresStore) RecordAuditEvent(ctx context.Context, event AuditEvent) error {
	if event.ID.IsZero() || event.EventType == "" || !validAuditOutcome(event.Outcome) {
		return ErrInvalidInput
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO operation_audit_events (
  id, actor_user_id, import_id, export_id, module, event_type, outcome,
  affected_count, request_id, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, databaseUUID(event.ID), optionalAuthIdentifier(event.ActorUserID), optionalIdentifier(event.ImportID), optionalIdentifier(event.ExportID), nullableModule(event.Module), event.EventType, event.Outcome, optionalInt(event.AffectedCount), event.RequestID, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("record operation audit event: %w", err)
	}
	return nil
}

func validAuditOutcome(outcome auth.AuditOutcome) bool {
	return outcome == auth.AuditOutcomeSuccess || outcome == auth.AuditOutcomeFailure || outcome == auth.AuditOutcomeDenied
}

func optionalAuthIdentifier(value *auth.Identifier) any {
	if value == nil {
		return nil
	}
	return authDatabaseUUID(*value)
}

func optionalInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableModule(value Module) any {
	if value == "" {
		return nil
	}
	return value
}

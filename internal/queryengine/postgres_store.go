package queryengine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

func (store *PostgresStore) CurrentUser(ctx context.Context, id auth.Identifier) (auth.User, error) {
	if store == nil || store.pool == nil || id == (auth.Identifier{}) {
		return auth.User{}, ErrForbidden
	}
	var value auth.User
	var databaseID pgtype.UUID
	var avatar pgtype.Text
	err := store.pool.QueryRow(ctx, `SELECT id, email, display_name, avatar_url, role, active
FROM app_users WHERE id=$1`, queryAuthUUID(id)).Scan(
		&databaseID, &value.Email, &value.DisplayName, &avatar, &value.Role, &value.Active,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, ErrForbidden
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("load current query user: %w", err)
	}
	value.ID = auth.Identifier(databaseID.Bytes)
	if avatar.Valid {
		value.AvatarURL = avatar.String
	}
	return value, nil
}

func (store *PostgresStore) CatalogDefinitions(ctx context.Context) (CatalogDefinitions, error) {
	if store == nil || store.pool == nil {
		return CatalogDefinitions{}, ErrInvalidSetup
	}
	rows, err := store.pool.Query(ctx, `SELECT id::text, technical_key, label, COALESCE(profile_cardinality, '')
FROM custom_entity_types
WHERE active=true
ORDER BY technical_key, id`)
	if err != nil {
		return CatalogDefinitions{}, fmt.Errorf("list query entity definitions: %w", err)
	}
	definitions := CatalogDefinitions{Entities: make([]DynamicEntityDefinition, 0), Fields: make([]DynamicFieldDefinition, 0)}
	for rows.Next() {
		var value DynamicEntityDefinition
		if err := rows.Scan(&value.ID, &value.TechnicalKey, &value.Label, &value.ProfileCardinality); err != nil {
			rows.Close()
			return CatalogDefinitions{}, fmt.Errorf("scan query entity definition: %w", err)
		}
		definitions.Entities = append(definitions.Entities, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CatalogDefinitions{}, fmt.Errorf("iterate query entity definitions: %w", err)
	}
	rows.Close()

	rows, err = store.pool.Query(ctx, `SELECT definition.id::text,
       definition.target_kind,
       COALESCE(definition.document_type_id::text, ''),
       COALESCE(definition.bill_type_id::text, ''),
       COALESCE(definition.custom_entity_type_id::text, ''),
       definition.technical_key,
       definition.label,
       definition.field_kind
FROM custom_field_definitions AS definition
LEFT JOIN document_types AS document_type ON document_type.id=definition.document_type_id
LEFT JOIN bill_types AS bill_type ON bill_type.id=definition.bill_type_id
LEFT JOIN custom_entity_types AS entity_type ON entity_type.id=definition.custom_entity_type_id
WHERE definition.active=true
  AND definition.field_kind <> 'ATTACHMENT'
  AND (
    definition.target_kind='PROFILE' OR
    (definition.target_kind='DOCUMENT_TYPE' AND document_type.active=true) OR
    (definition.target_kind='BILL_TYPE' AND bill_type.active=true) OR
    (definition.target_kind='CUSTOM_ENTITY_TYPE' AND entity_type.active=true)
  )
ORDER BY definition.target_kind, definition.technical_key, definition.id`)
	if err != nil {
		return CatalogDefinitions{}, fmt.Errorf("list query field definitions: %w", err)
	}
	fieldIndex := make(map[string]int)
	for rows.Next() {
		var value DynamicFieldDefinition
		if err := rows.Scan(&value.ID, &value.TargetKind, &value.DocumentTypeID, &value.BillTypeID,
			&value.CustomEntityTypeID, &value.TechnicalKey, &value.Label, &value.FieldKind); err != nil {
			rows.Close()
			return CatalogDefinitions{}, fmt.Errorf("scan query field definition: %w", err)
		}
		fieldIndex[value.ID] = len(definitions.Fields)
		definitions.Fields = append(definitions.Fields, value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CatalogDefinitions{}, fmt.Errorf("iterate query field definitions: %w", err)
	}
	rows.Close()

	rows, err = store.pool.Query(ctx, `SELECT option_value.field_definition_id::text, option_value.technical_key, option_value.label
FROM custom_field_options AS option_value
JOIN custom_field_definitions AS definition ON definition.id=option_value.field_definition_id
WHERE option_value.active=true AND definition.active=true
ORDER BY option_value.field_definition_id, option_value.sort_order, lower(option_value.label), option_value.id`)
	if err != nil {
		return CatalogDefinitions{}, fmt.Errorf("list query field options: %w", err)
	}
	for rows.Next() {
		var fieldID string
		var option OptionDefinition
		if err := rows.Scan(&fieldID, &option.Key, &option.Label); err != nil {
			rows.Close()
			return CatalogDefinitions{}, fmt.Errorf("scan query field option: %w", err)
		}
		if index, ok := fieldIndex[fieldID]; ok {
			definitions.Fields[index].Options = append(definitions.Fields[index].Options, option)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CatalogDefinitions{}, fmt.Errorf("iterate query field options: %w", err)
	}
	rows.Close()
	return definitions, nil
}

const executionColumns = `id, owner_user_id, state, idempotency_key, plan_fingerprint,
       catalog_version, root_entity, maximum_rows, row_count, column_count, error_code,
       started_at, completed_at, expires_at, version, created_at, updated_at`

type queryRowScanner interface {
	Scan(...any) error
}

func scanExecution(row queryRowScanner) (Execution, error) {
	var value Execution
	var id, ownerID pgtype.UUID
	var state string
	var fingerprint []byte
	var maximumRows, rowCount, columnCount int32
	var errorCode pgtype.Text
	var completedAt pgtype.Timestamptz
	err := row.Scan(&id, &ownerID, &state, &value.IdempotencyKey, &fingerprint,
		&value.CatalogVersion, &value.RootEntity, &maximumRows, &rowCount, &columnCount, &errorCode,
		&value.StartedAt, &completedAt, &value.ExpiresAt, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return Execution{}, err
	}
	if !id.Valid || !ownerID.Valid || len(fingerprint) != len(value.PlanFingerprint) {
		return Execution{}, ErrUnsafeResult
	}
	value.ID = Identifier(id.Bytes)
	value.OwnerUserID = auth.Identifier(ownerID.Bytes)
	value.State = ExecutionState(state)
	copy(value.PlanFingerprint[:], fingerprint)
	value.MaximumRows = int(maximumRows)
	value.RowCount = int(rowCount)
	value.ColumnCount = int(columnCount)
	if errorCode.Valid {
		value.ErrorCode = errorCode.String
	}
	if completedAt.Valid {
		completed := completedAt.Time
		value.CompletedAt = &completed
	}
	return value, nil
}

func (store *PostgresStore) CreateExecution(ctx context.Context, input ExecutionInput, window time.Time, maximumRequests int) (Execution, bool, error) {
	if store == nil || store.pool == nil || input.ID.IsZero() || input.OwnerUserID == (auth.Identifier{}) ||
		input.IdempotencyKey == "" || input.CatalogVersion == "" || input.RootEntity == "" ||
		input.MaximumRows < 1 || input.MaximumRows > MaximumRows || !input.ExpiresAt.After(input.StartedAt) ||
		window.IsZero() || maximumRequests < 1 {
		return Execution{}, false, ErrInvalidPlan
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Execution{}, false, fmt.Errorf("begin query execution creation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	ownerID := queryAuthUUID(input.OwnerUserID)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 1010))`, ownerID); err != nil {
		return Execution{}, false, fmt.Errorf("lock query owner: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM query_executions
WHERE owner_user_id=$1 AND expires_at <= $2 AND state <> 'RUNNING'`, ownerID, input.StartedAt); err != nil {
		return Execution{}, false, fmt.Errorf("delete expired idempotency records: %w", err)
	}
	existing, err := scanExecution(tx.QueryRow(ctx, `SELECT `+executionColumns+`
FROM query_executions WHERE owner_user_id=$1 AND idempotency_key=$2`, ownerID, input.IdempotencyKey))
	if err == nil {
		if existing.PlanFingerprint != input.PlanFingerprint {
			return Execution{}, false, ErrConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Execution{}, false, fmt.Errorf("commit idempotent query lookup: %w", err)
		}
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Execution{}, false, fmt.Errorf("find idempotent query execution: %w", err)
	}
	var reserved int
	err = tx.QueryRow(ctx, `INSERT INTO query_rate_limits(actor_user_id, window_started_at, request_count, updated_at)
VALUES($1,$2,1,$3)
ON CONFLICT (actor_user_id) DO UPDATE SET
  window_started_at=CASE WHEN query_rate_limits.window_started_at < EXCLUDED.window_started_at THEN EXCLUDED.window_started_at ELSE query_rate_limits.window_started_at END,
  request_count=CASE WHEN query_rate_limits.window_started_at < EXCLUDED.window_started_at THEN 1 ELSE query_rate_limits.request_count + 1 END,
  updated_at=EXCLUDED.updated_at
WHERE query_rate_limits.window_started_at < EXCLUDED.window_started_at
   OR query_rate_limits.request_count < $4
RETURNING request_count`, ownerID, window, input.StartedAt, maximumRequests).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return Execution{}, false, ErrRateLimited
	}
	if err != nil {
		return Execution{}, false, fmt.Errorf("reserve query rate limit: %w", err)
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM query_executions WHERE owner_user_id=$1 AND state='RUNNING'`, ownerID).Scan(&active); err != nil {
		return Execution{}, false, fmt.Errorf("count active query executions: %w", err)
	}
	if active > 0 {
		return Execution{}, false, ErrConflict
	}
	created, err := scanExecution(tx.QueryRow(ctx, `INSERT INTO query_executions(
  id, owner_user_id, state, idempotency_key, plan_fingerprint, catalog_version,
  root_entity, maximum_rows, started_at, expires_at, created_at, updated_at
) VALUES($1,$2,'RUNNING',$3,$4,$5,$6,$7,$8,$9,$8,$8)
RETURNING `+executionColumns,
		queryUUID(input.ID), ownerID, input.IdempotencyKey, input.PlanFingerprint[:], input.CatalogVersion,
		input.RootEntity, input.MaximumRows, input.StartedAt, input.ExpiresAt))
	if err != nil {
		return Execution{}, false, mapQueryPostgresError("create query execution", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Execution{}, false, mapQueryPostgresError("commit query execution creation", err)
	}
	return created, true, nil
}

func (store *PostgresStore) CountReadOnly(ctx context.Context, query string, arguments []any, timeout time.Duration) (int64, error) {
	query = strings.TrimSpace(query)
	if store == nil || store.pool == nil || !strings.HasPrefix(query, "SELECT ") || strings.Contains(query, ";") ||
		timeout <= 0 || timeout > 10*time.Second || len(arguments) > 64 {
		return 0, ErrReadOnlyRequired
	}
	tx, err := store.beginReadOnly(ctx, timeout)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var count int64
	if err := tx.QueryRow(ctx, query, arguments...).Scan(&count); err != nil {
		return 0, normalizeQueryExecutionError(err)
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return 0, normalizeQueryExecutionError(err)
	}
	return count, nil
}

func (store *PostgresStore) ScanTexts(ctx context.Context, query string, arguments []any, columns, limit int, timeout time.Duration) ([][]string, error) {
	query = strings.TrimSpace(query)
	if store == nil || store.pool == nil || !strings.HasPrefix(query, "SELECT ") || strings.Contains(query, ";") ||
		timeout <= 0 || timeout > 10*time.Second || len(arguments) > 64 ||
		columns < 2 || columns > MaximumProjections+2 || limit < 1 || limit > MaximumSequenceScan {
		return nil, ErrReadOnlyRequired
	}
	tx, err := store.beginReadOnly(ctx, timeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, query, arguments...)
	if err != nil {
		return nil, normalizeQueryExecutionError(err)
	}
	defer rows.Close()
	result := make([][]string, 0)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("read query scan values: %w", err)
		}
		if len(values) != columns || len(result) >= limit {
			return nil, ErrUnsafeResult
		}
		record := make([]string, columns)
		for index, value := range values {
			if value == nil {
				continue
			}
			text, ok := queryText(value)
			if !ok {
				return nil, ErrUnsafeResult
			}
			record[index] = text
		}
		if strings.TrimSpace(record[0]) == "" {
			return nil, ErrUnsafeResult
		}
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, normalizeQueryExecutionError(err)
	}
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return nil, normalizeQueryExecutionError(err)
	}
	return result, nil
}

func (store *PostgresStore) beginReadOnly(ctx context.Context, timeout time.Duration) (pgx.Tx, error) {
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin read-only query execution: %w", err)
	}
	timeoutMilliseconds := timeout.Milliseconds()
	if timeoutMilliseconds < 1 {
		timeoutMilliseconds = 1
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, strconv.FormatInt(timeoutMilliseconds, 10)); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, fmt.Errorf("configure query statement timeout: %w", err)
	}
	var readOnly string
	if err := tx.QueryRow(ctx, `SHOW transaction_read_only`).Scan(&readOnly); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, fmt.Errorf("verify read-only query transaction: %w", err)
	}
	if readOnly != "on" {
		_ = tx.Rollback(context.Background())
		return nil, ErrReadOnlyRequired
	}
	return tx, nil
}

func (store *PostgresStore) ExecuteReadOnly(ctx context.Context, plan CompiledPlan, timeout time.Duration) ([]RawResultRow, error) {
	if store == nil || store.pool == nil || strings.TrimSpace(plan.SQL) == "" || !strings.HasPrefix(strings.TrimSpace(plan.SQL), "SELECT ") ||
		strings.Contains(plan.SQL, ";") || timeout <= 0 || timeout > 10*time.Second || plan.MaximumRows < 1 ||
		plan.MaximumRows > MaximumRows || len(plan.Columns) == 0 || len(plan.Columns) > MaximumProjections {
		return nil, ErrReadOnlyRequired
	}
	tx, err := store.beginReadOnly(ctx, timeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, plan.SQL, plan.Arguments...)
	if err != nil {
		return nil, normalizeQueryExecutionError(err)
	}
	defer rows.Close()
	result := make([]RawResultRow, 0, plan.MaximumRows)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("read query row values: %w", err)
		}
		if len(values) != len(plan.Columns)+3 || len(result) >= plan.MaximumRows {
			return nil, ErrUnsafeResult
		}
		entityID, ok := queryText(values[0])
		if !ok {
			return nil, ErrUnsafeResult
		}
		entityLabel, ok := queryText(values[1])
		if !ok {
			return nil, ErrUnsafeResult
		}
		updatedAt, ok := queryTime(values[2])
		if !ok {
			return nil, ErrUnsafeResult
		}
		value := RawResultRow{EntityKind: plan.EntityKind, EntityID: entityID, EntityLabel: entityLabel, UpdatedAt: updatedAt, Values: make([]*string, 0, len(plan.Columns))}
		for _, raw := range values[3:] {
			if raw == nil {
				value.Values = append(value.Values, nil)
				continue
			}
			textValue, ok := queryText(raw)
			if !ok {
				return nil, ErrUnsafeResult
			}
			copyValue := textValue
			value.Values = append(value.Values, &copyValue)
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, normalizeQueryExecutionError(err)
	}
	if len(rows.FieldDescriptions()) != len(plan.Columns)+3 {
		return nil, ErrUnsafeResult
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return nil, normalizeQueryExecutionError(err)
	}
	return result, nil
}

func (store *PostgresStore) CompleteExecution(ctx context.Context, id Identifier, columns []ResultColumn, rows []ResultRow, completedAt time.Time) (Execution, error) {
	if store == nil || store.pool == nil || id.IsZero() || completedAt.IsZero() || len(columns) == 0 ||
		len(columns) > MaximumProjections || len(rows) > MaximumRows || !validStoredResult(columns, rows) {
		return Execution{}, ErrUnsafeResult
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Execution{}, fmt.Errorf("begin query result completion: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var state string
	var maximumRows int
	if err := tx.QueryRow(ctx, `SELECT state, maximum_rows FROM query_executions WHERE id=$1 FOR UPDATE`, queryUUID(id)).Scan(&state, &maximumRows); errors.Is(err, pgx.ErrNoRows) {
		return Execution{}, ErrNotFound
	} else if err != nil {
		return Execution{}, fmt.Errorf("lock query execution completion: %w", err)
	}
	if state != string(ExecutionRunning) || len(rows) > maximumRows {
		return Execution{}, ErrConflict
	}
	columnValues := make([][]any, 0, len(columns))
	for _, column := range columns {
		columnValues = append(columnValues, []any{queryUUID(id), column.Position, column.FieldKey, column.Label, string(column.Kind)})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"query_result_columns"},
		[]string{"execution_id", "position", "field_key", "label", "value_kind"}, pgx.CopyFromRows(columnValues)); err != nil {
		return Execution{}, mapQueryPostgresError("store query result columns", err)
	}
	rowValues := make([][]any, 0, len(rows))
	cellValues := make([][]any, 0, len(rows)*len(columns))
	for _, row := range rows {
		rowValues = append(rowValues, []any{queryUUID(id), row.Position, row.EntityKind, row.EntityID, row.EntityLabel, row.UpdatedAt})
		for _, cell := range row.Cells {
			stored, err := storedCellValues(cell)
			if err != nil {
				return Execution{}, err
			}
			cellValues = append(cellValues, append([]any{queryUUID(id), row.Position, cell.ColumnPosition, string(cell.Kind), cell.IsNull}, stored...))
		}
	}
	if len(rowValues) > 0 {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"query_result_rows"},
			[]string{"execution_id", "position", "entity_kind", "entity_id", "entity_label", "updated_at"}, pgx.CopyFromRows(rowValues)); err != nil {
			return Execution{}, mapQueryPostgresError("store query result rows", err)
		}
	}
	if len(cellValues) > 0 {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"query_result_cells"},
			[]string{"execution_id", "row_position", "column_position", "value_kind", "is_null", "text_value", "integer_value", "decimal_value", "boolean_value", "civil_date_value", "timestamp_value"},
			pgx.CopyFromRows(cellValues)); err != nil {
			return Execution{}, mapQueryPostgresError("store query result cells", err)
		}
	}
	completed, err := scanExecution(tx.QueryRow(ctx, `UPDATE query_executions SET
  state='COMPLETED', row_count=$2, column_count=$3, completed_at=$4, updated_at=$4, version=version+1
WHERE id=$1 AND state='RUNNING'
RETURNING `+executionColumns, queryUUID(id), len(rows), len(columns), completedAt))
	if errors.Is(err, pgx.ErrNoRows) {
		return Execution{}, ErrConflict
	}
	if err != nil {
		return Execution{}, mapQueryPostgresError("complete query execution", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Execution{}, mapQueryPostgresError("commit query result completion", err)
	}
	return completed, nil
}

func (store *PostgresStore) FailExecution(ctx context.Context, id Identifier, code string, state ExecutionState, completedAt time.Time) error {
	if store == nil || store.pool == nil || id.IsZero() || completedAt.IsZero() ||
		(state != ExecutionFailed && state != ExecutionCancelled) || code == "" || len(code) > 80 {
		return ErrInvalidPlan
	}
	command, err := store.pool.Exec(ctx, `UPDATE query_executions SET
  state=$2, error_code=$3, completed_at=$4, updated_at=$4, version=version+1
WHERE id=$1 AND state='RUNNING'`, queryUUID(id), string(state), code, completedAt)
	if err != nil {
		return mapQueryPostgresError("fail query execution", err)
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (store *PostgresStore) GetExecution(ctx context.Context, id Identifier, ownerID auth.Identifier) (Execution, error) {
	if store == nil || store.pool == nil || id.IsZero() || ownerID == (auth.Identifier{}) {
		return Execution{}, ErrNotFound
	}
	value, err := scanExecution(store.pool.QueryRow(ctx, `SELECT `+executionColumns+`
FROM query_executions WHERE id=$1 AND owner_user_id=$2`, queryUUID(id), queryAuthUUID(ownerID)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Execution{}, ErrNotFound
	}
	if err != nil {
		return Execution{}, fmt.Errorf("get query execution: %w", err)
	}
	return value, nil
}

func (store *PostgresStore) GetResultPage(ctx context.Context, id Identifier, ownerID auth.Identifier, limit, offset int) (ResultPage, error) {
	execution, err := store.GetExecution(ctx, id, ownerID)
	if err != nil {
		return ResultPage{}, err
	}
	columnRows, err := store.pool.Query(ctx, `SELECT position, field_key, label, value_kind
FROM query_result_columns WHERE execution_id=$1 ORDER BY position`, queryUUID(id))
	if err != nil {
		return ResultPage{}, fmt.Errorf("list query result columns: %w", err)
	}
	columns := make([]ResultColumn, 0, execution.ColumnCount)
	for columnRows.Next() {
		var value ResultColumn
		if err := columnRows.Scan(&value.Position, &value.FieldKey, &value.Label, &value.Kind); err != nil {
			columnRows.Close()
			return ResultPage{}, fmt.Errorf("scan query result column: %w", err)
		}
		columns = append(columns, value)
	}
	if err := columnRows.Err(); err != nil {
		columnRows.Close()
		return ResultPage{}, fmt.Errorf("iterate query result columns: %w", err)
	}
	columnRows.Close()

	resultRows, err := store.pool.Query(ctx, `SELECT position, entity_kind, entity_id, entity_label, updated_at
FROM query_result_rows
WHERE execution_id=$1 AND position >= $2
ORDER BY position LIMIT $3`, queryUUID(id), offset, limit)
	if err != nil {
		return ResultPage{}, fmt.Errorf("list query result rows: %w", err)
	}
	rows := make([]ResultRow, 0, limit)
	rowByPosition := make(map[int]int, limit)
	for resultRows.Next() {
		var value ResultRow
		if err := resultRows.Scan(&value.Position, &value.EntityKind, &value.EntityID, &value.EntityLabel, &value.UpdatedAt); err != nil {
			resultRows.Close()
			return ResultPage{}, fmt.Errorf("scan query result row: %w", err)
		}
		value.Cells = make([]ResultCell, 0, len(columns))
		rowByPosition[value.Position] = len(rows)
		rows = append(rows, value)
	}
	if err := resultRows.Err(); err != nil {
		resultRows.Close()
		return ResultPage{}, fmt.Errorf("iterate query result rows: %w", err)
	}
	resultRows.Close()
	if len(rows) > 0 {
		cellRows, err := store.pool.Query(ctx, `SELECT row_position, column_position, value_kind, is_null,
       text_value, integer_value, decimal_value::text, boolean_value, civil_date_value::text, timestamp_value
FROM query_result_cells
WHERE execution_id=$1 AND row_position >= $2 AND row_position < $3
ORDER BY row_position, column_position`, queryUUID(id), offset, offset+limit)
		if err != nil {
			return ResultPage{}, fmt.Errorf("list query result cells: %w", err)
		}
		for cellRows.Next() {
			var rowPosition int
			var cell ResultCell
			var textValue, decimalValue, dateValue pgtype.Text
			var integerValue pgtype.Int8
			var booleanValue pgtype.Bool
			var timestampValue pgtype.Timestamptz
			if err := cellRows.Scan(&rowPosition, &cell.ColumnPosition, &cell.Kind, &cell.IsNull,
				&textValue, &integerValue, &decimalValue, &booleanValue, &dateValue, &timestampValue); err != nil {
				cellRows.Close()
				return ResultPage{}, fmt.Errorf("scan query result cell: %w", err)
			}
			if textValue.Valid {
				value := textValue.String
				cell.TextValue = &value
			}
			if integerValue.Valid {
				value := integerValue.Int64
				cell.IntegerValue = &value
			}
			if decimalValue.Valid {
				value := decimalValue.String
				cell.DecimalValue = &value
			}
			if booleanValue.Valid {
				value := booleanValue.Bool
				cell.BooleanValue = &value
			}
			if dateValue.Valid {
				value := dateValue.String
				cell.CivilDateValue = &value
			}
			if timestampValue.Valid {
				value := timestampValue.Time
				cell.TimestampValue = &value
			}
			rowIndex, ok := rowByPosition[rowPosition]
			if !ok {
				cellRows.Close()
				return ResultPage{}, ErrUnsafeResult
			}
			rows[rowIndex].Cells = append(rows[rowIndex].Cells, cell)
		}
		if err := cellRows.Err(); err != nil {
			cellRows.Close()
			return ResultPage{}, fmt.Errorf("iterate query result cells: %w", err)
		}
		cellRows.Close()
	}
	return ResultPage{Execution: execution, Columns: columns, Rows: rows, Total: execution.RowCount, Limit: limit, Offset: offset}, nil
}

func (store *PostgresStore) DeleteExpired(ctx context.Context, now time.Time, limit int) (int, error) {
	if store == nil || store.pool == nil || now.IsZero() || limit < 1 || limit > 1000 {
		return 0, ErrInvalidPlan
	}
	var deleted int
	err := store.pool.QueryRow(ctx, `WITH expired AS (
  SELECT id FROM query_executions WHERE expires_at <= $1 ORDER BY expires_at, id LIMIT $2
), removed AS (
  DELETE FROM query_executions WHERE id IN (SELECT id FROM expired) RETURNING 1
)
SELECT count(*) FROM removed`, now, limit).Scan(&deleted)
	if err != nil {
		return 0, fmt.Errorf("delete expired query results: %w", err)
	}
	_, _ = store.pool.Exec(ctx, `DELETE FROM query_rate_limits WHERE updated_at < $1`, now.Add(-24*time.Hour))
	return deleted, nil
}

func (store *PostgresStore) SaveAudit(ctx context.Context, event AuditEvent) error {
	if store == nil || store.pool == nil || event.ID.IsZero() || event.EventType == "" || event.Outcome == "" || event.CreatedAt.IsZero() {
		return ErrInvalidPlan
	}
	_, err := store.pool.Exec(ctx, `INSERT INTO query_audit_events(
  id, actor_user_id, execution_id, event_type, outcome, affected_count, request_id, created_at
) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, queryUUID(event.ID), optionalQueryAuthUUID(event.ActorUserID), optionalQueryUUID(event.ExecutionID),
		string(event.EventType), event.Outcome, event.AffectedCount, event.RequestID, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("save query audit event: %w", err)
	}
	return nil
}

func validStoredResult(columns []ResultColumn, rows []ResultRow) bool {
	for position, column := range columns {
		if column.Position != position || column.FieldKey == "" || column.Label == "" || !column.Kind.Valid() {
			return false
		}
	}
	for position, row := range rows {
		if row.Position != position || len(row.Cells) != len(columns) {
			return false
		}
		for cellPosition, cell := range row.Cells {
			if cell.ColumnPosition != cellPosition || cell.Kind != columns[cellPosition].Kind || !validMaterializedCell(cell) {
				return false
			}
		}
	}
	return true
}

func storedCellValues(cell ResultCell) ([]any, error) {
	values := []any{nil, nil, nil, nil, nil, nil}
	if cell.IsNull {
		return values, nil
	}
	switch cell.Kind {
	case ValueText, ValueLongText, ValueIdentifier, ValueCivilMonth, ValueEnum:
		values[0] = valueOrEmpty(cell.TextValue)
	case ValueInteger:
		values[1] = *cell.IntegerValue
	case ValueDecimal:
		var numeric pgtype.Numeric
		if err := numeric.Scan(valueOrEmpty(cell.DecimalValue)); err != nil {
			return nil, ErrUnsafeResult
		}
		values[2] = numeric
	case ValueBoolean:
		values[3] = *cell.BooleanValue
	case ValueCivilDate:
		parsed, err := time.Parse("2006-01-02", valueOrEmpty(cell.CivilDateValue))
		if err != nil {
			return nil, ErrUnsafeResult
		}
		values[4] = pgtype.Date{Time: parsed, Valid: true}
	case ValueTimestamp:
		values[5] = *cell.TimestampValue
	default:
		return nil, ErrUnsafeResult
	}
	return values, nil
}

func queryText(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case []byte:
		return string(typed), true
	case pgtype.Text:
		return typed.String, typed.Valid
	default:
		return "", false
	}
}

func queryTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, !typed.IsZero()
	case pgtype.Timestamptz:
		return typed.Time, typed.Valid
	default:
		return time.Time{}, false
	}
}

func normalizeQueryExecutionError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "57014":
			return ErrTimeout
		case "25006":
			return ErrReadOnlyRequired
		}
	}
	return err
}

func mapQueryPostgresError(action string, err error) error {
	if errors.Is(err, pgx.ErrTxCommitRollback) {
		return fmt.Errorf("%s: %w", action, ErrConflict)
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23503", "23505", "23514", "40001":
			return fmt.Errorf("%s: %w", action, ErrConflict)
		}
	}
	return fmt.Errorf("%s: %w", action, err)
}

func queryUUID(value Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: !value.IsZero()}
}

func queryAuthUUID(value auth.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(value), Valid: value != (auth.Identifier{})}
}

func optionalQueryUUID(value *Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return queryUUID(*value)
}

func optionalQueryAuthUUID(value *auth.Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return queryAuthUUID(*value)
}

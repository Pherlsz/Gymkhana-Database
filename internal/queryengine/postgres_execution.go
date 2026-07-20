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
	"github.com/jackc/pgx/v5/pgtype"
)

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

func (store *PostgresStore) ExecuteReadOnly(ctx context.Context, plan CompiledPlan, timeout time.Duration) ([]RawResultRow, error) {
	if store == nil || store.pool == nil || strings.TrimSpace(plan.SQL) == "" || !strings.HasPrefix(strings.TrimSpace(plan.SQL), "SELECT ") ||
		strings.Contains(plan.SQL, ";") || timeout <= 0 || timeout > 10*time.Second || plan.MaximumRows < 1 ||
		plan.MaximumRows > MaximumRows || len(plan.Columns) == 0 || len(plan.Columns) > MaximumProjections {
		return nil, ErrReadOnlyRequired
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin read-only query execution: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	timeoutMilliseconds := timeout.Milliseconds()
	if timeoutMilliseconds < 1 {
		timeoutMilliseconds = 1
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true)`, strconv.FormatInt(timeoutMilliseconds, 10)); err != nil {
		return nil, fmt.Errorf("configure query statement timeout: %w", err)
	}
	var readOnly string
	if err := tx.QueryRow(ctx, `SHOW transaction_read_only`).Scan(&readOnly); err != nil {
		return nil, fmt.Errorf("verify read-only query transaction: %w", err)
	}
	if readOnly != "on" {
		return nil, ErrReadOnlyRequired
	}
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

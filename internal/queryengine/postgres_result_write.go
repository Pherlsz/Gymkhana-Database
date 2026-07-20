package queryengine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

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

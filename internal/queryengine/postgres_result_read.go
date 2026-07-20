package queryengine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

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

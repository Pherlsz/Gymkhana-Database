package queryengine

import (
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

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

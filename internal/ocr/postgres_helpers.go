package ocr

import (
	"context"
	"errors"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func lockJob(ctx context.Context, tx pgx.Tx, id Identifier) (Job, int64, error) {
	value, err := scanJob(tx.QueryRow(ctx, `SELECT `+jobColumns+` FROM ocr_jobs WHERE id=$1 FOR UPDATE`, databaseUUID(id)))
	if err != nil {
		return Job{}, 0, err
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT next_event_sequence FROM ocr_jobs WHERE id=$1`, databaseUUID(id)).Scan(&sequence); err != nil {
		return Job{}, 0, err
	}
	return value, sequence, nil
}

func normalizePostgresError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(err, pgx.ErrTxCommitRollback) {
		return ErrConflict
	}
	databaseCode := ""
	var databaseError interface{ SQLState() string }
	if errors.As(err, &databaseError) {
		databaseCode = databaseError.SQLState()
	}
	if databaseCode == "" {
		for _, code := range []string{"57014", "23505", "40001", "40P01", "23503", "23514", "23P01"} {
			if strings.Contains(err.Error(), "(SQLSTATE "+code+")") {
				databaseCode = code
				break
			}
		}
	}
	switch databaseCode {
	case "57014":
		return ErrTimeout
	case "23505", "40001", "40P01":
		return ErrConflict
	case "23503", "23P01":
		return ErrInvalidState
	case "23514":
		return ErrInvalidInput
	default:
		return err
	}
}

func optionalAuthUUID(value *auth.Identifier) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return authDatabaseUUID(*value)
}

func optionalText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func optionalInteger(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

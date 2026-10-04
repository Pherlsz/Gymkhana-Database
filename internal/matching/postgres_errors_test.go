package matching

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizePostgresMatchingErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: ErrTimeout},
		{name: "statement timeout", err: fmt.Errorf("query: %w", &pgconn.PgError{Code: "57014"}), want: ErrTimeout},
		{name: "serialization", err: fmt.Errorf("merge: %w", &pgconn.PgError{Code: "40001"}), want: ErrConflict},
		{name: "opaque serialization wrapper", err: errors.New("driver wrapper (SQLSTATE 40001)"), want: ErrConflict},
		{name: "deadlock", err: fmt.Errorf("merge: %w", &pgconn.PgError{Code: "40P01"}), want: ErrConflict},
		{name: "commit rollback", err: pgx.ErrTxCommitRollback, want: ErrConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := normalizePostgresError(test.err); !errors.Is(err, test.want) {
				t.Fatalf("normalizePostgresError(%v) = %v, want %v", test.err, err, test.want)
			}
		})
	}
}

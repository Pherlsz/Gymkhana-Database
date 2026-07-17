package search

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeExecutionErrorMapsPostgresCancellationToStableTimeout(t *testing.T) {
	err := normalizeExecutionError(&pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"})
	if !errors.Is(err, ErrQueryTimeout) {
		t.Fatalf("normalizeExecutionError() = %v, want ErrQueryTimeout", err)
	}

	original := errors.New("database unavailable")
	if got := normalizeExecutionError(original); !errors.Is(got, original) {
		t.Fatalf("normalizeExecutionError() = %v, want original error", got)
	}
}

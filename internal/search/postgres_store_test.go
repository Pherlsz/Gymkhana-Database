package search

import (
	"errors"
	"strings"
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

func TestSearchSQLPlaceholdersStartAtOne(t *testing.T) {
	if !strings.Contains(searchSQL, "ANY($1::text[])") || !strings.Contains(searchSQL, "$8::jsonb") {
		t.Fatal("search SQL must bind modules as $1 and include specs as $8")
	}
	if strings.Contains(searchBody, "$11") || strings.Contains(searchSQL, "$11") || strings.Contains(searchIDSQL, "$12") {
		t.Fatal("unused $1/$2-style gaps make PostgreSQL reject the query with 42P18")
	}
}

func TestSearchIDQueriesTypeUnusedParameters(t *testing.T) {
	queries := map[string]string{
		"profiles": searchProfileIDSQL,
		"records":  searchIDSQL,
		"hits":     searchProfileHitSQL,
	}
	for name, sql := range queries {
		for _, typed := range []string{"$3::int", "$4::int", "$5::text", "$6::text"} {
			if !strings.Contains(sql, typed) {
				t.Errorf("%s id query missing %s; an untyped middle parameter is SQLSTATE 42P18", name, typed)
			}
		}
	}
}

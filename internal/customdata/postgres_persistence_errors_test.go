package customdata

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapPersistenceError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		postgres   *pgconn.PgError
		want       error
		wrappedMsg string
	}{
		{
			name: "one per profile conflict",
			postgres: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "custom_entities_one_per_profile",
			},
			want: ErrCardinalityConflict,
		},
		{
			name:     "technical key conflict",
			postgres: &pgconn.PgError{Code: "23505", ConstraintName: "custom_field_definitions_technical_key_key"},
			want:     ErrTechnicalKeyConflict,
		},
		{
			name:     "missing reference",
			postgres: &pgconn.PgError{Code: "23503"},
			want:     ErrReferenceNotFound,
		},
		{
			name:     "inactive entity type",
			postgres: &pgconn.PgError{Code: "23514", Message: "custom entity type is inactive"},
			want:     ErrEntityTypeInactive,
		},
		{
			name:     "other check violation",
			postgres: &pgconn.PgError{Code: "23514", Message: "target contract mismatch"},
			want:     ErrInvalidTarget,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := mapPersistenceError("save custom data", test.postgres)
			if !errors.Is(got, test.want) {
				t.Fatalf("mapPersistenceError() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestMapKnownOrPersistencePreservesDomainErrors(t *testing.T) {
	t.Parallel()

	wrapped := errors.Join(errors.New("repository"), ErrDefinitionChangeUnsafe)
	if got := mapKnownOrPersistence("update definition", wrapped); !errors.Is(got, ErrDefinitionChangeUnsafe) {
		t.Fatalf("mapKnownOrPersistence() = %v, want %v", got, ErrDefinitionChangeUnsafe)
	}
}

func TestMapPersistenceErrorWrapsUnknownFailures(t *testing.T) {
	t.Parallel()

	original := errors.New("connection reset")
	got := mapPersistenceError("replace values", original)
	if !errors.Is(got, original) {
		t.Fatalf("mapPersistenceError() did not preserve original error: %v", got)
	}
	if got.Error() != "replace values: connection reset" {
		t.Fatalf("mapPersistenceError() = %q, want operation context", got)
	}
}

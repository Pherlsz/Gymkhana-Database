//go:build integration

package profile

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresStoreSortsFullNameWithPortugueseCollation(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Profile PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	key := "sort_" + strings.ReplaceAll(mustIdentifier(t).String(), "-", "")[:20]
	agataID, anaID, zeliaID, zuraicaID := mustIdentifier(t), mustIdentifier(t), mustIdentifier(t), mustIdentifier(t)
	ids := []Identifier{agataID, anaID, zeliaID, zuraicaID}
	names := []string{"Ágata " + key, "Ana " + key, "Zélia " + key, "Zuraica " + key}
	for index, id := range ids {
		if _, err := pool.Exec(ctx, `INSERT INTO profiles(id, full_name) VALUES($1,$2)`, id.String(), names[index]); err != nil {
			t.Fatalf("insert profile %s: %v", names[index], err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM profiles WHERE id IN ($1,$2,$3,$4)`,
			agataID.String(), anaID.String(), zeliaID.String(), zuraicaID.String())
	})

	store := NewPostgresStore(pool)
	descending, err := store.List(ctx, ListOptions{
		Limit: 10, SortField: SortFullName, SortOrder: SortDescending, Filters: Filters{FullName: key},
	})
	if err != nil {
		t.Fatalf("List(desc) error = %v", err)
	}
	if got, want := listedNames(descending), []string{"Zuraica " + key, "Zélia " + key, "Ana " + key, "Ágata " + key}; !sameNames(got, want) {
		t.Fatalf("Z→A names = %#v, want %#v", got, want)
	}

	ascending, err := store.List(ctx, ListOptions{
		Limit: 10, SortField: SortFullName, SortOrder: SortAscending, Filters: Filters{FullName: key},
	})
	if err != nil {
		t.Fatalf("List(asc) error = %v", err)
	}
	if got, want := listedNames(ascending), []string{"Ágata " + key, "Ana " + key, "Zélia " + key, "Zuraica " + key}; !sameNames(got, want) {
		t.Fatalf("A→Z names = %#v, want %#v", got, want)
	}
}

func mustIdentifier(t *testing.T) Identifier {
	t.Helper()
	id, err := NewIdentifier()
	if err != nil {
		t.Fatalf("NewIdentifier() error = %v", err)
	}
	return id
}

func listedNames(profiles []Profile) []string {
	names := make([]string, 0, len(profiles))
	for _, value := range profiles {
		names = append(names, value.Values.FullName)
	}
	return names
}

func sameNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

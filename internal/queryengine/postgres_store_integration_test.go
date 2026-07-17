//go:build integration

package queryengine

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresQueryEngineLifecycleRelationsTypingOwnershipAndLimits(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Query Engine PostgreSQL integration tests")
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

	actorID, _ := auth.NewIdentifier()
	otherActorID, _ := auth.NewIdentifier()
	profileID, _ := auth.NewIdentifier()
	otherProfileID, _ := auth.NewIdentifier()
	documentTypeID, _ := auth.NewIdentifier()
	documentID, _ := auth.NewIdentifier()
	fieldID, _ := auth.NewIdentifier()
	valueID, _ := auth.NewIdentifier()
	key := "query_" + strings.ReplaceAll(actorID.String(), "-", "")[:20]
	githubID := time.Now().UnixNano()
	if githubID < 0 {
		githubID = -githubID
	}
	if githubID < 2 {
		githubID = 2
	}
	insertQueryActor(t, ctx, pool, actorID, githubID, key)
	insertQueryActor(t, ctx, pool, otherActorID, githubID+1, key+"_other")
	if _, err := pool.Exec(ctx, `INSERT INTO profiles(id, full_name, email, address_city)
VALUES($1,'Ana Query','ana.query@example.org','Recife'),($2,'Zed Query','zed.query@example.org','Olinda')`, profileID.String(), otherProfileID.String()); err != nil {
		t.Fatalf("insert query profiles: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO document_types(id, technical_key, label, active, uniqueness_policy, date_required)
VALUES($1,$2,'Documento Query',true,'NONE',false)`, documentTypeID.String(), key+"_doc"); err != nil {
		t.Fatalf("insert query document type: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO documents(id, owner_profile_id, document_type_id, identifier_value, uniqueness_policy)
VALUES($1,$2,$3,'RG%_42','NONE')`, documentID.String(), profileID.String(), documentTypeID.String()); err != nil {
		t.Fatalf("insert query document: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO custom_field_definitions
(id, target_kind, technical_key, label, field_kind, required, active)
VALUES($1,'PROFILE',$2,'Pontuação Query','DECIMAL',false,true)`, fieldID.String(), key+"_score"); err != nil {
		t.Fatalf("insert query field definition: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO custom_field_values
(id, field_definition_id, profile_id, field_kind, decimal_value)
VALUES($1,$2,$3,'DECIMAL',12.34)`, valueID.String(), fieldID.String(), profileID.String()); err != nil {
		t.Fatalf("insert query field value: %v", err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM query_audit_events WHERE actor_user_id IN ($1,$2)`, actorID.String(), otherActorID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM query_executions WHERE owner_user_id IN ($1,$2)`, actorID.String(), otherActorID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM query_rate_limits WHERE actor_user_id IN ($1,$2)`, actorID.String(), otherActorID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM custom_field_values WHERE id=$1`, valueID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM custom_field_definitions WHERE id=$1`, fieldID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM documents WHERE id=$1`, documentID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM document_types WHERE id=$1`, documentTypeID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM profiles WHERE id IN ($1,$2)`, profileID.String(), otherProfileID.String())
		_, _ = pool.Exec(cleanup, `DELETE FROM app_users WHERE id IN ($1,$2)`, actorID.String(), otherActorID.String())
	}()

	now := time.Now().UTC()
	store := NewPostgresStore(pool)
	service, err := NewService(store, ServiceOptions{Now: func() time.Time { return now }, RateLimit: 100, ResultRetention: time.Hour})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	actor := auth.Session{User: auth.User{ID: actorID, Role: auth.RoleMember, Active: true}}
	otherActor := auth.Session{User: auth.User{ID: otherActorID, Role: auth.RoleMember, Active: true}}
	catalog, err := service.Catalog(ctx, actor, "integration-catalog")
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	customField := "custom." + fieldID.String()
	if !catalogHasField(catalog, customField, ValueDecimal) || !catalogHasRelation(catalog, "profile.documents") {
		t.Fatalf("Catalog() misses dynamic field or relation: %#v", catalog)
	}
	plan := QueryPlan{
		CatalogVersion: catalog.Version, RootEntity: "profiles",
		Projections: []string{"profile.full_name", customField}, MaximumRows: 10,
		Filter: &FilterNode{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
			{Kind: FilterGroup, Conjunction: ConjunctionOr, Children: []FilterNode{
				{Kind: FilterPredicate, Field: "profile.address_city", Operator: OperatorEqual, Values: []string{"Recife"}},
				{Kind: FilterPredicate, Field: "profile.address_city", Operator: OperatorEqual, Values: []string{"Olinda"}},
			}},
			{Kind: FilterPredicate, Field: customField, Operator: OperatorGreater, Values: []string{"10"}},
			{Kind: FilterRelation, Relation: "profile.documents", Children: []FilterNode{{Kind: FilterPredicate, Field: "document.identifier", Operator: OperatorContains, Values: []string{"%_"}}}},
			{Kind: FilterNot, Children: []FilterNode{{Kind: FilterPredicate, Field: "profile.full_name", Operator: OperatorStartsWith, Values: []string{"Zed"}}}},
		}},
		Sort: []Sort{{Field: "profile.full_name", Direction: SortAscending}},
	}
	execution, err := service.Execute(ctx, actor, plan, "integration-query-key", "integration-execute")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if execution.State != ExecutionCompleted || execution.RowCount != 1 || execution.ColumnCount != 2 {
		t.Fatalf("Execute() = %#v", execution)
	}
	replayed, err := service.Execute(ctx, actor, plan, "integration-query-key", "integration-replay")
	if err != nil || replayed.ID != execution.ID {
		t.Fatalf("idempotent Execute() = %#v, error=%v", replayed, err)
	}
	page, err := service.Result(ctx, actor, execution.ID, 25, 0, "integration-result")
	if err != nil {
		t.Fatalf("Result() error = %v", err)
	}
	if page.Total != 1 || len(page.Rows) != 1 || page.Rows[0].EntityID != profileID.String() ||
		page.Rows[0].Cells[0].TextValue == nil || *page.Rows[0].Cells[0].TextValue != "Ana Query" ||
		page.Rows[0].Cells[1].DecimalValue == nil || *page.Rows[0].Cells[1].DecimalValue != "12.3400000000" {
		t.Fatalf("typed Result() = %#v", page)
	}
	if _, err := service.Result(ctx, otherActor, execution.ID, 25, 0, "integration-other-owner"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other-owner Result() error = %v", err)
	}
	different := plan
	different.MaximumRows = 9
	if _, err := service.Execute(ctx, actor, different, "integration-query-key", "integration-conflict"); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed idempotent Execute() error = %v", err)
	}
	if _, err := store.ExecuteReadOnly(ctx, CompiledPlan{SQL: "SELECT 1; UPDATE profiles SET full_name='x'", MaximumRows: 1,
		Columns: []ResultColumn{{Position: 0, FieldKey: "profile.id", Label: "ID", Kind: ValueIdentifier}}}, time.Second); !errors.Is(err, ErrReadOnlyRequired) {
		t.Fatalf("unsafe ExecuteReadOnly() error = %v", err)
	}

	expiredService, err := NewService(store, ServiceOptions{Now: func() time.Time { return now.Add(2 * time.Hour) }})
	if err != nil {
		t.Fatalf("NewService(expired) error = %v", err)
	}
	if _, err := expiredService.Result(ctx, actor, execution.ID, 25, 0, "integration-expired"); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired Result() error = %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE query_executions SET expires_at=created_at + interval '1 second' WHERE id=$1`, execution.ID.String()); err != nil {
		t.Fatalf("expire query execution: %v", err)
	}
	deleted, err := expiredService.CleanupExpired(ctx)
	if err != nil || deleted < 1 {
		t.Fatalf("CleanupExpired() = %d, error=%v", deleted, err)
	}
}

func insertQueryActor(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id auth.Identifier, githubID int64, login string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `INSERT INTO app_users
(id, github_user_id, github_login, display_name, role, active)
VALUES($1,$2,$3,'Query integration actor','MEMBER',true)`, id.String(), githubID, login); err != nil {
		t.Fatalf("insert query actor: %v", err)
	}
}

func catalogHasField(catalog Catalog, key string, kind ValueKind) bool {
	for _, field := range catalog.Fields {
		if field.Key == key && field.Kind == kind {
			return true
		}
	}
	return false
}

func catalogHasRelation(catalog Catalog, key string) bool {
	for _, relation := range catalog.Relations {
		if relation.Key == key {
			return true
		}
	}
	return false
}

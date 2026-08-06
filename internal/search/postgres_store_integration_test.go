//go:build integration

package search

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresSearchAcrossRelationsDynamicFieldsAttachmentsAndRateLimit(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for Search PostgreSQL integration tests")
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
	profileID, _ := auth.NewIdentifier()
	documentTypeID, _ := auth.NewIdentifier()
	documentID, _ := auth.NewIdentifier()
	billTypeID, _ := auth.NewIdentifier()
	billID, _ := auth.NewIdentifier()
	fieldID, _ := auth.NewIdentifier()
	valueID, _ := auth.NewIdentifier()
	attachmentID, _ := auth.NewIdentifier()
	key := "search_" + strings.ReplaceAll(profileID.String(), "-", "")[:20]
	githubID := time.Now().UnixNano()
	if githubID < 0 {
		githubID = -githubID
	}
	if githubID == 0 {
		githubID = 1
	}

	if _, err := pool.Exec(ctx, `INSERT INTO app_users
(id, github_user_id, github_login, display_name, role, active)
VALUES($1,$2,$3,$4,'EXTERNAL',true)`, actorID.String(), githubID, key, "Search test actor"); err != nil {
		t.Fatalf("insert app user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profiles
(id, full_name, address_city, address_postal_code)
VALUES($1,$2,$3,$4)`, profileID.String(), "Ana Percentual", "Recife", "00123456"); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO document_types
(id, technical_key, label, active, uniqueness_policy, date_required)
VALUES($1,$2,$3,true,'NONE',false)`, documentTypeID.String(), key+"_doc", "RG de teste"); err != nil {
		t.Fatalf("insert document type: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO documents
(id, owner_profile_id, document_type_id, identifier_value, uniqueness_policy)
VALUES($1,$2,$3,$4,'NONE')`, documentID.String(), profileID.String(), documentTypeID.String(), `00%_ABC`); err != nil {
		t.Fatalf("insert document: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO bill_types
(id, technical_key, label, active, supports_current_use)
VALUES($1,$2,$3,true,false)`, billTypeID.String(), key+"_bill", "Energia de teste"); err != nil {
		t.Fatalf("insert bill type: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO bills
(id, owner_profile_id, bill_type_id, printed_holder_name, printed_address, reference_value)
VALUES($1,$2,$3,$4,$5,$6)`, billID.String(), profileID.String(), billTypeID.String(), "Ana Percentual", "Rua do Sol 100", "UC-42"); err != nil {
		t.Fatalf("insert bill: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO custom_field_definitions
(id, target_kind, technical_key, label, field_kind, required, active)
VALUES($1,'PROFILE',$2,$3,'TEXT',false,true)`, fieldID.String(), key+"_field", "Codinome"); err != nil {
		t.Fatalf("insert custom field: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO custom_field_values
(id, field_definition_id, profile_id, field_kind, text_value)
VALUES($1,$2,$3,'TEXT',$4)`, valueID.String(), fieldID.String(), profileID.String(), "Fênix 001"); err != nil {
		t.Fatalf("insert custom value: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO attachments
(id, owner_kind, document_id, original_filename, declared_mime, detected_mime, byte_size, sha256,
 object_key, lifecycle_state)
VALUES($1,'DOCUMENT',$2,$3,'application/pdf','application/pdf',128,$4,$5,'ACTIVE')`,
		attachmentID.String(), documentID.String(), "prova_final.pdf", bytes.Repeat([]byte{7}, 32), "integration/search/"+attachmentID.String()); err != nil {
		t.Fatalf("insert attachment: %v", err)
	}
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, "DELETE FROM search_rate_limits WHERE actor_user_id=$1", actorID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM attachments WHERE id=$1", attachmentID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM custom_field_values WHERE id=$1", valueID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM custom_field_definitions WHERE id=$1", fieldID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM bills WHERE id=$1", billID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM bill_types WHERE id=$1", billTypeID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM documents WHERE id=$1", documentID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM document_types WHERE id=$1", documentTypeID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM profiles WHERE id=$1", profileID.String())
		_, _ = pool.Exec(cleanup, "DELETE FROM app_users WHERE id=$1", actorID.String())
	}()

	store := NewPostgresStore(pool)
	service, err := NewService(store, ServiceOptions{RateLimit: 100})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	actor := auth.Session{User: auth.User{ID: actorID, Role: auth.RoleExternal, Active: true}}

	relational, err := service.Search(ctx, actor, Query{
		Terms:   []string{"Ana", `%_`},
		Modules: []Module{ModuleProfiles, ModuleDocuments},
		Limit:   20,
	})
	if err != nil {
		t.Fatalf("relational Search() error = %v", err)
	}
	if relational.Total < 2 || !containsModule(relational.Results, ModuleProfiles) || !containsModule(relational.Results, ModuleDocuments) {
		t.Fatalf("relational results = %#v", relational)
	}

	custom, err := service.Search(ctx, actor, Query{
		Terms:  []string{"Fênix 001"},
		Fields: []string{"custom." + fieldID.String()},
	})
	if err != nil || custom.Total != 1 || custom.Results[0].Module != ModuleCustomData {
		t.Fatalf("custom Search() = %#v, error = %v", custom, err)
	}

	attachments, err := service.Search(ctx, actor, Query{
		Terms:   []string{"prova_final"},
		Modules: []Module{ModuleAttachments},
	})
	if err != nil || attachments.Total != 1 || attachments.Results[0].TargetKind != "document" {
		t.Fatalf("attachment Search() = %#v, error = %v", attachments, err)
	}
	if strings.Contains(attachments.Results[0].Preview, "integration/search") {
		t.Fatal("attachment result exposed the private object key")
	}

	filtered, err := service.Search(ctx, actor, Query{
		Terms: []string{`%_`}, Modules: []Module{ModuleDocuments},
		Fields: []string{"document.identifier"},
	})
	if err != nil || filtered.Total != 1 || !onlyModule(filtered.Results, ModuleDocuments) {
		t.Fatalf("filtered Search() = %#v, error = %v", filtered, err)
	}

	unauthorized := actor
	unauthorized.User.Role = auth.Role("UNKNOWN")
	if _, err := service.Search(ctx, unauthorized, Query{Terms: []string{"Ana"}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unauthorized Search() error = %v", err)
	}
	if _, err := service.Search(ctx, actor, Query{Terms: []string{"Ana"}, Offset: MaxOffset + 1}); err == nil {
		t.Fatal("over-limit Search() unexpectedly succeeded")
	}

	firstPage, err := service.Search(ctx, actor, Query{Terms: []string{"Ana"}, Limit: 1})
	if err != nil {
		t.Fatalf("first deterministic Search() error = %v", err)
	}
	repeatedPage, err := service.Search(ctx, actor, Query{Terms: []string{"Ana"}, Limit: 1})
	if err != nil || firstPage.Results[0] != repeatedPage.Results[0] {
		t.Fatalf("pagination is not deterministic: first=%#v repeated=%#v error=%v", firstPage, repeatedPage, err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM search_rate_limits WHERE actor_user_id=$1", actorID.String()); err != nil {
		t.Fatalf("reset rate limit: %v", err)
	}
	window := time.Now().UTC().Truncate(time.Minute)
	const attempts = 12
	const maximum = 5
	reservations := make(chan error, attempts)
	for range attempts {
		go func() { reservations <- store.ReserveRateLimit(ctx, actorID, window, maximum) }()
	}
	succeeded := 0
	limited := 0
	for range attempts {
		err := <-reservations
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrRateLimited):
			limited++
		default:
			t.Fatalf("concurrent ReserveRateLimit() error = %v", err)
		}
	}
	if succeeded != maximum || limited != attempts-maximum {
		t.Fatalf("concurrent reservations: succeeded=%d limited=%d", succeeded, limited)
	}
}

func containsModule(results []Result, module Module) bool {
	for _, result := range results {
		if result.Module == module {
			return true
		}
	}
	return false
}

func onlyModule(results []Result, module Module) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if result.Module != module {
			return false
		}
	}
	return true
}

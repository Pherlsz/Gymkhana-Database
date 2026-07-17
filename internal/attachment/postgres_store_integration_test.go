//go:build integration

package attachment

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

func TestPostgresStoreUploadLimitsCleanupLeaseAndDeleteConflicts(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for attachment PostgreSQL integration tests")
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

	actorID, _ := NewIdentifier()
	profileID, _ := NewIdentifier()
	typeID, _ := NewIdentifier()
	documentID, _ := NewIdentifier()
	intentID, _ := NewIdentifier()
	attachmentID, _ := NewIdentifier()
	technicalKey := "attachment_test_" + strings.ReplaceAll(typeID.String(), "-", "")[:20]
	githubID := time.Now().UnixNano()
	if githubID < 0 {
		githubID = -githubID
	}
	if githubID == 0 {
		githubID = 1
	}

	if _, err := pool.Exec(ctx, `INSERT INTO app_users
(id, github_user_id, github_login, display_name, role, active)
VALUES($1,$2,$3,$4,'MEMBER',true)`, databaseUUID(actorID), githubID, technicalKey, "Attachment test actor"); err != nil {
		t.Fatalf("insert app user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profiles(id, full_name) VALUES($1,$2)`, databaseUUID(profileID), "Attachment Test Profile"); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO document_types
(id, technical_key, label, active, uniqueness_policy, date_required)
VALUES($1,$2,$3,true,'NONE',false)`, databaseUUID(typeID), technicalKey, "Attachment test type"); err != nil {
		t.Fatalf("insert document type: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO documents
(id, owner_profile_id, document_type_id, identifier_value, uniqueness_policy)
VALUES($1,$2,$3,$4,'NONE')`, databaseUUID(documentID), databaseUUID(profileID), databaseUUID(typeID), technicalKey); err != nil {
		t.Fatalf("insert document: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM attachment_upload_intents WHERE actor_user_id=$1", databaseUUID(actorID))
		_, _ = pool.Exec(context.Background(), "DELETE FROM attachments WHERE document_id=$1", databaseUUID(documentID))
		_, _ = pool.Exec(context.Background(), "DELETE FROM documents WHERE id=$1", databaseUUID(documentID))
		_, _ = pool.Exec(context.Background(), "DELETE FROM document_types WHERE id=$1", databaseUUID(typeID))
		_, _ = pool.Exec(context.Background(), "DELETE FROM profiles WHERE id=$1", databaseUUID(profileID))
		_, _ = pool.Exec(context.Background(), "DELETE FROM app_users WHERE id=$1", databaseUUID(actorID))
	})

	store := NewPostgresStore(pool)
	now := time.Now().UTC()
	intent := UploadIntent{
		ID: intentID, ActorUserID: auth.Identifier(actorID), Owner: OwnerReference{Kind: OwnerDocument, ID: documentID},
		OriginalFileName: "proof.pdf", DeclaredMIME: "application/pdf", ExpectedSize: 100,
		ObjectKey: "integration/" + intentID.String(), CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
	}
	if _, err := store.CreateUploadIntent(ctx, intent, UploadLimits{RateWindowStart: now.Add(-time.Minute), MaximumIntents: 1, MaximumTotalBytes: 1000}); err != nil {
		t.Fatalf("CreateUploadIntent() error = %v", err)
	}

	second := intent
	second.ID, _ = NewIdentifier()
	second.ObjectKey = "integration/" + second.ID.String()
	if _, err := store.CreateUploadIntent(ctx, second, UploadLimits{RateWindowStart: now.Add(-time.Minute), MaximumIntents: 1, MaximumTotalBytes: 1000}); !errors.Is(err, ErrUploadRateLimited) {
		t.Fatalf("rate-limited CreateUploadIntent() error = %v", err)
	}
	if _, err := store.CreateUploadIntent(ctx, second, UploadLimits{RateWindowStart: now.Add(-time.Minute), MaximumIntents: 100, MaximumTotalBytes: 150}); !errors.Is(err, ErrStorageQuotaExceeded) {
		t.Fatalf("quota CreateUploadIntent() error = %v", err)
	}

	firstLease, acquired, err := store.AcquireCleanupLease(ctx)
	if err != nil || !acquired {
		t.Fatalf("first AcquireCleanupLease() = %t, %v", acquired, err)
	}
	if secondLease, acquired, err := store.AcquireCleanupLease(ctx); err != nil || acquired || secondLease != nil {
		t.Fatalf("second AcquireCleanupLease() = %#v, %t, %v", secondLease, acquired, err)
	}
	if err := firstLease.Release(ctx); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	reacquired, acquired, err := store.AcquireCleanupLease(ctx)
	if err != nil || !acquired {
		t.Fatalf("reacquire cleanup lease = %t, %v", acquired, err)
	}
	if err := reacquired.Release(ctx); err != nil {
		t.Fatalf("second Release() error = %v", err)
	}

	if err := store.DeleteExpiredUploadIntent(ctx, intentID, now.Add(11*time.Minute)); err != nil {
		t.Fatalf("DeleteExpiredUploadIntent() error = %v", err)
	}
	if err := store.DeleteExpiredUploadIntent(ctx, intentID, now.Add(11*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("second DeleteExpiredUploadIntent() error = %v", err)
	}

	deletedAt := now.Add(-2 * time.Minute)
	purgeAfter := now.Add(-time.Minute)
	if _, err := pool.Exec(ctx, `INSERT INTO attachments
(id, owner_kind, document_id, original_filename, declared_mime, detected_mime, byte_size, sha256,
 object_key, lifecycle_state, deleted_at, purge_after, version, created_at, updated_at)
VALUES($1,'DOCUMENT',$2,'proof.pdf','application/pdf','application/pdf',100,$3,$4,'TRASHED',$5,$6,2,$5,$5)`,
		databaseUUID(attachmentID), databaseUUID(documentID), bytes.Repeat([]byte{1}, 32),
		"integration/"+attachmentID.String(), deletedAt, purgeAfter); err != nil {
		t.Fatalf("insert trashed attachment: %v", err)
	}
	if err := store.DeletePurged(ctx, attachmentID, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong-version DeletePurged() error = %v", err)
	}
	if err := store.DeletePurged(ctx, attachmentID, 2); err != nil {
		t.Fatalf("DeletePurged() error = %v", err)
	}
	if err := store.DeletePurged(ctx, attachmentID, 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("second DeletePurged() error = %v", err)
	}
}

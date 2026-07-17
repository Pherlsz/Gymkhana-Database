package attachment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func TestServiceUploadConfirmDownloadTrashAndRestore(t *testing.T) {
	now := time.Date(2026, time.July, 17, 3, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	objects := newMemoryObjects()
	service := newTestService(t, store, objects, &now)
	actor := testActor(1)
	owner := OwnerReference{Kind: OwnerDocument, ID: testIdentifier(10)}
	payload := []byte("%PDF-1.7\nprivate document")

	grant, err := service.CreateUploadIntent(context.Background(), actor, CreateUploadIntentInput{
		Owner: owner, OriginalFileName: " documento.pdf ", DeclaredMIME: "application/pdf", ExpectedSize: int64(len(payload)),
	}, "request-create")
	if err != nil {
		t.Fatalf("CreateUploadIntent() error = %v", err)
	}
	if grant.Intent.OriginalFileName != "documento.pdf" || grant.Upload.Method != "PUT" {
		t.Fatalf("grant = %#v", grant)
	}
	objects.values[grant.Intent.ObjectKey] = payload
	created, err := service.ConfirmUpload(context.Background(), actor, grant.Intent.ID, "request-confirm")
	if err != nil {
		t.Fatalf("ConfirmUpload() error = %v", err)
	}
	if created.Owner != owner || created.DetectedMIME != "application/pdf" || created.Version != 1 {
		t.Fatalf("created = %#v", created)
	}
	if store.intents[grant.Intent.ID].ConsumedAt == nil {
		t.Fatal("upload intent was not consumed")
	}
	signed, err := service.Download(context.Background(), actor, created.ID, "request-download")
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if signed.Method != "GET" || signed.URL == "" {
		t.Fatalf("signed download = %#v", signed)
	}
	trashed, err := service.Trash(context.Background(), actor, created.ID, created.Version, DeleteConfirmation, "request-trash")
	if err != nil {
		t.Fatalf("Trash() error = %v", err)
	}
	if trashed.LifecycleState != LifecycleTrashed || trashed.PurgeAfter == nil {
		t.Fatalf("trashed = %#v", trashed)
	}
	restored, err := service.Restore(context.Background(), actor, trashed.ID, trashed.Version, "request-restore")
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.LifecycleState != LifecycleActive || restored.DeletedAt != nil || restored.PurgeAfter != nil {
		t.Fatalf("restored = %#v", restored)
	}
	if len(store.audits) < 5 {
		t.Fatalf("audit events = %d", len(store.audits))
	}
}

func TestServiceUploadIntentIsBoundToActorAndExpiry(t *testing.T) {
	now := time.Date(2026, time.July, 17, 3, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	objects := newMemoryObjects()
	service := newTestService(t, store, objects, &now)
	owner := OwnerReference{Kind: OwnerBill, ID: testIdentifier(20)}
	payload := []byte("%PDF-1.7\nprivate bill")
	grant, err := service.CreateUploadIntent(context.Background(), testActor(1), CreateUploadIntentInput{
		Owner: owner, OriginalFileName: "bill.pdf", DeclaredMIME: "application/pdf", ExpectedSize: int64(len(payload)),
	}, "request-create")
	if err != nil {
		t.Fatalf("CreateUploadIntent() error = %v", err)
	}
	objects.values[grant.Intent.ObjectKey] = payload
	if _, err := service.ConfirmUpload(context.Background(), testActor(2), grant.Intent.ID, "request-other"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other actor error = %v, want ErrForbidden", err)
	}
	now = now.Add(11 * time.Minute)
	if _, err := service.ConfirmUpload(context.Background(), testActor(1), grant.Intent.ID, "request-expired"); !errors.Is(err, ErrUploadIntentExpired) {
		t.Fatalf("expired error = %v, want ErrUploadIntentExpired", err)
	}
}

func TestServiceCleanupIsRetrySafeForExpiredAndTrashedObjects(t *testing.T) {
	now := time.Date(2026, time.July, 17, 3, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	objects := newMemoryObjects()
	service := newTestService(t, store, objects, &now)
	actor := testActor(1)
	payload := []byte("%PDF-1.7\nprivate document")
	grant, err := service.CreateUploadIntent(context.Background(), actor, CreateUploadIntentInput{
		Owner: OwnerReference{Kind: OwnerDocument, ID: testIdentifier(30)}, OriginalFileName: "expired.pdf", DeclaredMIME: "application/pdf", ExpectedSize: int64(len(payload)),
	}, "request-expired-create")
	if err != nil {
		t.Fatalf("CreateUploadIntent() error = %v", err)
	}
	objects.values[grant.Intent.ObjectKey] = payload
	now = now.Add(11 * time.Minute)
	result, err := service.Cleanup(context.Background(), "worker-drain")
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if result.ExpiredUploads != 1 || len(store.intents) != 0 {
		t.Fatalf("cleanup result = %#v, intents = %d", result, len(store.intents))
	}

	attachmentID := testIdentifier(31)
	deletedAt := now.Add(-8 * 24 * time.Hour)
	purgeAfter := deletedAt.Add(7 * 24 * time.Hour)
	store.attachments[attachmentID] = Attachment{
		ID: attachmentID, Owner: OwnerReference{Kind: OwnerDocument, ID: testIdentifier(30)}, OriginalFileName: "trash.pdf",
		DeclaredMIME: "application/pdf", DetectedMIME: "application/pdf", ByteSize: int64(len(payload)), ObjectKey: "attachments/trash",
		LifecycleState: LifecycleTrashed, DeletedAt: &deletedAt, PurgeAfter: &purgeAfter, Version: 2, CreatedAt: deletedAt, UpdatedAt: deletedAt,
	}
	objects.values["attachments/trash"] = payload
	result, err = service.Cleanup(context.Background(), "worker-drain")
	if err != nil {
		t.Fatalf("Cleanup() purge error = %v", err)
	}
	if result.Purged != 1 || len(store.attachments) != 0 {
		t.Fatalf("purge result = %#v, attachments = %d", result, len(store.attachments))
	}
	result, err = service.Cleanup(context.Background(), "worker-drain")
	if err != nil || result.Purged != 0 || result.ExpiredUploads != 0 {
		t.Fatalf("retry result = %#v, error = %v", result, err)
	}
}

func newTestService(t *testing.T, store *memoryStore, objects *memoryObjects, now *time.Time) *Service {
	t.Helper()
	service, err := NewService(store, objects, ServiceOptions{
		UploadTTL: 10 * time.Minute, DownloadTTL: 5 * time.Minute, TrashRetention: 7 * 24 * time.Hour,
		MaximumFileSize: 1 << 20, CleanupBatch: 100, Now: func() time.Time { return *now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func testActor(seed byte) auth.Session {
	return auth.Session{User: auth.User{ID: auth.Identifier{seed}, Role: auth.RoleMember, Active: true}}
}

func testIdentifier(seed byte) Identifier {
	return Identifier{seed}
}

type memoryObjects struct {
	values  map[string][]byte
	deleted map[string]int
}

func newMemoryObjects() *memoryObjects {
	return &memoryObjects{values: map[string][]byte{}, deleted: map[string]int{}}
}

func (objects *memoryObjects) PresignUpload(_ context.Context, key string, ttl time.Duration) (SignedRequest, error) {
	return SignedRequest{URL: "https://upload.invalid/" + key, Method: "PUT", ExpiresAt: time.Now().Add(ttl)}, nil
}

func (objects *memoryObjects) PresignDownload(_ context.Context, key, _, _ string, ttl time.Duration) (SignedRequest, error) {
	return SignedRequest{URL: "https://download.invalid/" + key, Method: "GET", ExpiresAt: time.Now().Add(ttl)}, nil
}

func (objects *memoryObjects) Open(_ context.Context, key string) (io.ReadCloser, error) {
	value, ok := objects.values[key]
	if !ok {
		return nil, ErrUploadObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(value)), nil
}

func (objects *memoryObjects) Delete(_ context.Context, key string) error {
	delete(objects.values, key)
	objects.deleted[key]++
	return nil
}

type memoryStore struct {
	intents     map[Identifier]UploadIntent
	attachments map[Identifier]Attachment
	audits      []AuditEvent
}

func newMemoryStore() *memoryStore {
	return &memoryStore{intents: map[Identifier]UploadIntent{}, attachments: map[Identifier]Attachment{}}
}

func (store *memoryStore) CreateUploadIntent(_ context.Context, value UploadIntent) (UploadIntent, error) {
	store.intents[value.ID] = value
	return value, nil
}

func (store *memoryStore) GetUploadIntent(_ context.Context, id Identifier) (UploadIntent, error) {
	value, ok := store.intents[id]
	if !ok {
		return UploadIntent{}, ErrUploadIntentNotFound
	}
	return value, nil
}

func (store *memoryStore) ConfirmUploadIntent(_ context.Context, intentID Identifier, actorID auth.Identifier, attachmentID Identifier, verified VerifiedObject, now time.Time) (Attachment, error) {
	intent, ok := store.intents[intentID]
	if !ok {
		return Attachment{}, ErrUploadIntentNotFound
	}
	if intent.ActorUserID != actorID {
		return Attachment{}, ErrForbidden
	}
	if intent.ConsumedAt != nil {
		return Attachment{}, ErrUploadIntentConsumed
	}
	intent.ConsumedAt = &now
	store.intents[intentID] = intent
	value := Attachment{
		ID: attachmentID, Owner: intent.Owner, OriginalFileName: intent.OriginalFileName, DeclaredMIME: intent.DeclaredMIME,
		DetectedMIME: verified.DetectedMIME, ByteSize: verified.ByteSize, SHA256: verified.SHA256,
		ObjectKey: intent.ObjectKey, LifecycleState: LifecycleActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	store.attachments[value.ID] = value
	return value, nil
}

func (store *memoryStore) List(_ context.Context, owner OwnerReference, includeTrashed bool) ([]Attachment, error) {
	values := make([]Attachment, 0)
	for _, value := range store.attachments {
		if value.Owner == owner && (includeTrashed || value.LifecycleState == LifecycleActive) {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID.String() < values[j].ID.String() })
	return values, nil
}

func (store *memoryStore) Get(_ context.Context, id Identifier) (Attachment, error) {
	value, ok := store.attachments[id]
	if !ok {
		return Attachment{}, ErrAttachmentNotFound
	}
	return value, nil
}

func (store *memoryStore) Trash(_ context.Context, id Identifier, version int64, deletedAt, purgeAfter time.Time) (Attachment, error) {
	value, ok := store.attachments[id]
	if !ok {
		return Attachment{}, ErrAttachmentNotFound
	}
	if value.Version != version {
		return Attachment{}, ErrConflict
	}
	value.LifecycleState = LifecycleTrashed
	value.DeletedAt = &deletedAt
	value.PurgeAfter = &purgeAfter
	value.Version++
	value.UpdatedAt = deletedAt
	store.attachments[id] = value
	return value, nil
}

func (store *memoryStore) Restore(_ context.Context, id Identifier, version int64, now time.Time) (Attachment, error) {
	value, ok := store.attachments[id]
	if !ok {
		return Attachment{}, ErrAttachmentNotFound
	}
	if value.Version != version {
		return Attachment{}, ErrConflict
	}
	if value.PurgeAfter == nil || !value.PurgeAfter.After(now) {
		return Attachment{}, ErrInvalidState
	}
	value.LifecycleState = LifecycleActive
	value.DeletedAt = nil
	value.PurgeAfter = nil
	value.Version++
	value.UpdatedAt = now
	store.attachments[id] = value
	return value, nil
}

func (store *memoryStore) ListExpiredUploadIntents(_ context.Context, now time.Time, limit int) ([]UploadIntent, error) {
	values := make([]UploadIntent, 0)
	for _, value := range store.intents {
		if value.ConsumedAt == nil && !value.ExpiresAt.After(now) {
			values = append(values, value)
		}
	}
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (store *memoryStore) DeleteExpiredUploadIntent(_ context.Context, id Identifier, now time.Time) error {
	value, ok := store.intents[id]
	if ok && value.ConsumedAt == nil && !value.ExpiresAt.After(now) {
		delete(store.intents, id)
	}
	return nil
}

func (store *memoryStore) ListPurgeDue(_ context.Context, now time.Time, limit int) ([]Attachment, error) {
	values := make([]Attachment, 0)
	for _, value := range store.attachments {
		if value.LifecycleState == LifecycleTrashed && value.PurgeAfter != nil && !value.PurgeAfter.After(now) {
			values = append(values, value)
		}
	}
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (store *memoryStore) DeletePurged(_ context.Context, id Identifier, version int64) error {
	value, ok := store.attachments[id]
	if ok && value.Version == version && value.LifecycleState == LifecycleTrashed {
		delete(store.attachments, id)
	}
	return nil
}

func (store *memoryStore) RecordAuditEvent(_ context.Context, event AuditEvent) error {
	store.audits = append(store.audits, event)
	return nil
}

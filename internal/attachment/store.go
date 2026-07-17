package attachment

import (
	"context"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type CleanupLease interface {
	Release(context.Context) error
}

type Store interface {
	CreateUploadIntent(context.Context, UploadIntent, UploadLimits) (UploadIntent, error)
	AcquireCleanupLease(context.Context) (CleanupLease, bool, error)
	GetUploadIntent(context.Context, Identifier) (UploadIntent, error)
	ConfirmUploadIntent(context.Context, Identifier, auth.Identifier, Identifier, VerifiedObject, time.Time) (Attachment, error)
	List(context.Context, OwnerReference, bool) ([]Attachment, error)
	Get(context.Context, Identifier) (Attachment, error)
	Trash(context.Context, Identifier, int64, time.Time, time.Time) (Attachment, error)
	Restore(context.Context, Identifier, int64, time.Time) (Attachment, error)
	ListExpiredUploadIntents(context.Context, time.Time, int) ([]UploadIntent, error)
	DeleteExpiredUploadIntent(context.Context, Identifier, time.Time) error
	ListPurgeDue(context.Context, time.Time, int) ([]Attachment, error)
	DeletePurged(context.Context, Identifier, int64) error
}

type AuditStore interface {
	RecordAuditEvent(context.Context, AuditEvent) error
}

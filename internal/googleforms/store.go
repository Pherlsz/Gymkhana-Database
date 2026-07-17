package googleforms

import (
	"context"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
)

type StageResult struct {
	Import         *operations.Import
	ReceivedCount  int
	StagedCount    int
	DuplicateCount int
	Cursor         *time.Time
}

type Store interface {
	SaveOAuthState(context.Context, OAuthState) error
	ConsumeOAuthState(context.Context, [32]byte, auth.Identifier, auth.Identifier, time.Time) (OAuthState, error)
	UpsertConnection(context.Context, Connection) (Connection, error)
	GetConnection(context.Context, auth.Identifier) (Connection, error)
	DisconnectConnection(context.Context, auth.Identifier, int64, time.Time) (Connection, error)
	MarkConnectionNeedsReauth(context.Context, auth.Identifier, string, time.Time) error
	CreateSource(context.Context, Source) (Source, error)
	GetSource(context.Context, Identifier, auth.Identifier) (Source, error)
	ListSources(context.Context, auth.Identifier, int, int) (SourcePage, error)
	SaveMapping(context.Context, Identifier, auth.Identifier, int64, []MappingInput, time.Time) (Source, error)
	UpdateSource(context.Context, Identifier, auth.Identifier, int64, UpdateSourceInput, time.Time) (Source, error)
	RefreshSourceSchema(context.Context, Identifier, auth.Identifier, Form, time.Time) (Source, bool, error)
	CreateSyncRun(context.Context, SyncRun) (SyncRun, error)
	SetSyncJob(context.Context, Identifier, int64, time.Time) error
	BeginSync(context.Context, Identifier, time.Time) (SyncRun, Source, Connection, auth.Session, error)
	StageResponses(context.Context, SyncRun, Source, []Response, time.Time, time.Time) (StageResult, error)
	CompleteSync(context.Context, Identifier, *time.Time, string, int, int, int, time.Time) (SyncRun, error)
	FailSync(context.Context, Identifier, string, time.Time) (bool, error)
	CancelSync(context.Context, Identifier, auth.Identifier, int64, time.Time) (SyncRun, error)
	ListSyncRuns(context.Context, auth.Identifier, *Identifier, int, int) (SyncRunPage, error)
	DueSources(context.Context, time.Time, int) ([]Source, error)
	GetActor(context.Context, auth.Identifier) (auth.Session, error)
	RecordAuditEvent(context.Context, AuditEvent) error
}

type Jobs interface {
	EnqueueSync(context.Context, Identifier) (int64, error)
	EnqueueDue(context.Context, time.Time) (int64, error)
	Cancel(context.Context, int64) error
}

type Operations interface {
	Catalog(context.Context, auth.Session) ([]operations.ModuleCatalog, error)
	GetImport(context.Context, auth.Session, operations.Identifier) (operations.Import, error)
	Preview(context.Context, auth.Session, operations.Identifier, int64, string) (operations.Import, error)
}

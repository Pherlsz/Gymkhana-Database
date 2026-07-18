package aichat

import (
	"context"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type CreateThreadInput struct {
	ID                 Identifier
	OwnerUserID        auth.Identifier
	Title              string
	RetentionExpiresAt time.Time
	Now                time.Time
}

type CreateRunInput struct {
	ID                      Identifier
	MessageID               Identifier
	ThreadID                Identifier
	OwnerUserID             auth.Identifier
	RetryOfRunID            *Identifier
	ActiveResultReferenceID *Identifier
	IdempotencyKey          string
	RequestFingerprint      [32]byte
	Content                 string
	Now                     time.Time
}

type BeginToolInput struct {
	ID                   Identifier
	RunID                Identifier
	OwnerUserID          auth.Identifier
	Kind                 ToolKind
	ArgumentsFingerprint [32]byte
	Now                  time.Time
}

type CompleteToolInput struct {
	StepID          Identifier
	RunID           Identifier
	OwnerUserID     auth.Identifier
	ResultReference *CreateResultReferenceInput
	RowCount        int
	ResultBytes     int
	Now             time.Time
}

type CreateResultReferenceInput struct {
	ID                 Identifier
	ThreadID           Identifier
	RunID              Identifier
	OwnerUserID        auth.Identifier
	Kind               ResultReferenceKind
	QueryExecutionID   *Identifier
	LogicalRequest     []byte
	ContextFingerprint [32]byte
	Label              string
	RowCount           int
	ColumnCount        int
	ExpiresAt          time.Time
	Now                time.Time
}

type CompleteRunInput struct {
	MessageID   Identifier
	RunID       Identifier
	OwnerUserID auth.Identifier
	Content     string
	InputUsage  int64
	OutputUsage int64
	Now         time.Time
}

type Store interface {
	CurrentUser(context.Context, auth.Identifier) (auth.User, error)
	CreateThread(context.Context, CreateThreadInput) (Thread, error)
	ListThreads(context.Context, auth.Identifier, time.Time, int, int) (ThreadPage, error)
	GetThread(context.Context, Identifier, auth.Identifier) (Thread, error)
	RenameThread(context.Context, Identifier, auth.Identifier, string, int64, time.Time) (Thread, error)
	DeleteThread(context.Context, Identifier, auth.Identifier) error
	ListMessages(context.Context, Identifier, auth.Identifier, int, int) (MessagePage, error)
	CreateRun(context.Context, CreateRunInput, time.Time, int, int64) (RunCreation, error)
	GetRun(context.Context, Identifier, auth.Identifier) (Run, error)
	RequestCancellation(context.Context, Identifier, auth.Identifier, time.Time) (Run, error)
	StartRun(context.Context, Identifier, auth.Identifier, time.Time) (Run, error)
	AppendTextDelta(context.Context, Identifier, auth.Identifier, string, time.Time) (RunEvent, error)
	AddRunUsage(context.Context, Identifier, auth.Identifier, int64, int64, int64, time.Time) (Run, bool, error)
	BeginTool(context.Context, BeginToolInput) (ToolStep, error)
	CompleteTool(context.Context, CompleteToolInput) (ToolStep, *ResultReference, error)
	FailTool(context.Context, Identifier, Identifier, auth.Identifier, string, time.Time) (ToolStep, error)
	CompleteRun(context.Context, CompleteRunInput) (Run, Message, error)
	FailRun(context.Context, Identifier, auth.Identifier, string, time.Time) (Run, error)
	FailStaleRuns(context.Context, time.Time, time.Time, int) (int, error)
	ListRunEvents(context.Context, Identifier, auth.Identifier, int64, int) (EventPage, error)
	GetResultReference(context.Context, Identifier, auth.Identifier) (ResultReference, error)
	SetActiveResultReference(context.Context, Identifier, auth.Identifier, *Identifier, time.Time) (Thread, error)
	CleanupExpired(context.Context, time.Time, int) (int, error)
	SaveAudit(context.Context, AuditEvent) error
}

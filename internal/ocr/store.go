package ocr

import (
	"context"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type CreateJobInput struct {
	ID                 Identifier
	OwnerUserID        auth.Identifier
	AttachmentID       attachment.Identifier
	RetryOfJobID       *Identifier
	IdempotencyKey     string
	RequestFingerprint [32]byte
	SourceSHA256       [32]byte
	CatalogFingerprint [32]byte
	SourceMIME         string
	SourceBytes        int64
	Now                time.Time
}

type CompleteSuggestionInput struct {
	ID            Identifier
	Ordinal       int
	Field         FieldSchema
	ProposedValue string
	Evidence      Evidence
}

type CompleteJobInput struct {
	JobID       Identifier
	PageCount   int
	PixelCount  int64
	Suggestions []CompleteSuggestionInput
	Now         time.Time
}

type ReviewSuggestionInput struct {
	SuggestionID  Identifier
	OwnerUserID   auth.Identifier
	Action        ReviewAction
	Value         *string
	TargetVersion int64
	Version       int64
	Now           time.Time
}

type ApplySelection struct {
	SuggestionID Identifier
	Version      int64
}

type CreateApplyReceiptInput struct {
	ID                 Identifier
	JobID              Identifier
	OwnerUserID        auth.Identifier
	IdempotencyKey     string
	RequestFingerprint [32]byte
	Now                time.Time
}

type ApplyResultInput struct {
	SuggestionID  Identifier
	Outcome       ApplyOutcome
	TargetVersion *int64
	ErrorCode     string
}

type Store interface {
	CurrentUser(context.Context, auth.Identifier) (auth.User, error)
	CreateJob(context.Context, CreateJobInput, time.Time, int, int64) (Job, bool, error)
	AttachRiverJob(context.Context, Identifier, auth.Identifier, int64, time.Time) (Job, error)
	ListJobs(context.Context, auth.Identifier, int, int) (JobPage, error)
	GetJob(context.Context, Identifier, auth.Identifier) (Job, error)
	GetJobForWorker(context.Context, Identifier) (Job, error)
	ClaimJob(context.Context, Identifier, time.Time) (Job, bool, error)
	RecordSourceValidated(context.Context, Identifier, int, int64, time.Time) (Job, error)
	AddProviderUsage(context.Context, Identifier, auth.Identifier, int64, int64, time.Time) (Job, bool, error)
	CompleteJob(context.Context, CompleteJobInput) (Job, error)
	FailJob(context.Context, Identifier, string, JobState, time.Time) (Job, error)
	RequestCancellation(context.Context, Identifier, auth.Identifier, time.Time) (Job, error)
	ListEvents(context.Context, Identifier, auth.Identifier, int64, int) (EventPage, error)
	ListSuggestions(context.Context, Identifier, auth.Identifier, int, int) ([]Suggestion, int, error)
	GetSuggestion(context.Context, Identifier, auth.Identifier) (Suggestion, Job, error)
	ReviewSuggestion(context.Context, ReviewSuggestionInput) (Suggestion, error)
	GetSuggestionsForApply(context.Context, Identifier, auth.Identifier, []ApplySelection) ([]Suggestion, error)
	CreateApplyReceipt(context.Context, CreateApplyReceiptInput) (ApplyReceipt, bool, error)
	ResumeApplyReceipt(context.Context, Identifier, auth.Identifier, time.Time, time.Time) (ApplyReceipt, bool, error)
	SaveApplyResults(context.Context, Identifier, []ApplyResultInput, time.Time) ([]ApplyResult, error)
	CompleteApplyReceipt(context.Context, Identifier, auth.Identifier, time.Time) (ApplyReceipt, error)
	RecoverStaleJobs(context.Context, time.Time, time.Time, int) (int, error)
	SaveAudit(context.Context, AuditEvent) error
}

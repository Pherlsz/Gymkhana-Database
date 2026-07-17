package matching

import (
	"context"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type CreateAnalysisInput struct {
	ID             Identifier
	ActorUserID    auth.Identifier
	IdempotencyKey string
	ExpiresAt      time.Time
}

type Store interface {
	CreateAnalysis(context.Context, CreateAnalysisInput, time.Time, int) (Analysis, bool, error)
	AttachAnalysisJob(context.Context, Identifier, auth.Identifier, int64) (Analysis, error)
	GetAnalysis(context.Context, Identifier, auth.Identifier) (Analysis, error)
	RequestAnalysisCancellation(context.Context, Identifier, auth.Identifier, time.Time) (Analysis, error)
	ClaimAnalysis(context.Context, Identifier, time.Time) (Analysis, bool, error)
	GenerateCandidates(context.Context, Identifier, int, time.Duration, time.Time) (AnalysisStats, error)
	FailAnalysis(context.Context, Identifier, string, AnalysisState, time.Time) error
	ListCases(context.Context, CaseListOptions) (CasePage, error)
	GetCase(context.Context, Identifier) (Case, error)
	DismissCase(context.Context, Identifier, auth.Identifier, int64, string, time.Time) (Case, error)
	PreviewMerge(context.Context, MergePreviewInput, time.Time) (MergePreview, error)
	Merge(context.Context, auth.Identifier, MergeInput, string, time.Time) (MergeResult, bool, error)
	CleanupAnalyses(context.Context, time.Time, int) (int, error)
	RecordAudit(context.Context, AuditEvent) error
}

type Jobs interface {
	EnqueueAnalysis(context.Context, Identifier) (int64, error)
	Cancel(context.Context, int64) error
}

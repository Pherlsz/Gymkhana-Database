package taskengine

import (
	"context"
	"errors"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

type Identifier = queryengine.Identifier

const (
	MaximumJobPage       = 100
	MaximumEventPage     = 200
	MaximumResultPage    = 100
	MaximumAttempts      = 3
	MinimumIdempotency   = 8
	MaximumIdempotency   = 128
	DefaultRetention     = 24 * time.Hour
	DefaultTaskTimeout   = 2 * time.Minute
	DefaultMaximumRate   = 10
	DefaultRecoveryBatch = 100
)

var (
	ErrInvalidInput  = errors.New("invalid task input")
	ErrInvalidSetup  = errors.New("invalid task setup")
	ErrInvalidState  = errors.New("invalid task state")
	ErrForbidden     = errors.New("task access forbidden")
	ErrNotFound      = errors.New("task resource not found")
	ErrConflict      = errors.New("task conflict")
	ErrRateLimited   = errors.New("task rate limited")
	ErrUnavailable   = errors.New("task service unavailable")
	ErrExpired       = errors.New("task result expired")
	ErrCancelled     = errors.New("task cancelled")
	ErrTimeout       = errors.New("task timed out")
	ErrUnsafeResult  = errors.New("unsafe task result")
	ErrQuotaExceeded = errors.New("task quota exceeded")
)

type DraftState string

const (
	DraftProposed DraftState = "PROPOSED"
	DraftReviewed DraftState = "REVIEWED"
)

type Draft struct {
	ID              Identifier      `json:"id"`
	OwnerUserID     auth.Identifier `json:"-"`
	CatalogVersion  string          `json:"catalog_version"`
	State           DraftState      `json:"state"`
	Spec            TaskSpec        `json:"spec"`
	SpecFingerprint [32]byte        `json:"-"`
	Version         int64           `json:"version"`
	ExpiresAt       time.Time       `json:"expires_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type JobState string

const (
	JobQueued     JobState = "QUEUED"
	JobRunning    JobState = "RUNNING"
	JobCompleted  JobState = "COMPLETED"
	JobIncomplete JobState = "INCOMPLETE"
	JobFailed     JobState = "FAILED"
	JobCancelled  JobState = "CANCELLED"
)

func (state JobState) Terminal() bool {
	return state == JobCompleted || state == JobIncomplete || state == JobFailed || state == JobCancelled
}

type Job struct {
	ID                 Identifier      `json:"id"`
	OwnerUserID        auth.Identifier `json:"-"`
	DraftID            Identifier      `json:"draft_id"`
	RetryOfJobID       *Identifier     `json:"retry_of_job_id,omitempty"`
	IdempotencyKey     string          `json:"-"`
	RequestFingerprint [32]byte        `json:"-"`
	CatalogVersion     string          `json:"catalog_version"`
	State              JobState        `json:"state"`
	RiverJobID         int64           `json:"-"`
	AttemptCount       int             `json:"attempt_count"`
	ProgressCurrent    int             `json:"progress_current"`
	ProgressTotal      int             `json:"progress_total"`
	CandidateCount     int             `json:"candidate_count"`
	CompositionCount   int             `json:"composition_count"`
	ErrorCode          string          `json:"error_code,omitempty"`
	CancelRequestedAt  *time.Time      `json:"cancel_requested_at,omitempty"`
	StartedAt          *time.Time      `json:"started_at,omitempty"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	ExpiresAt          time.Time       `json:"expires_at"`
	Version            int64           `json:"version"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type JobPage struct {
	Jobs   []Job `json:"jobs"`
	Total  int   `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type EventKind string

const (
	EventJobAccepted       EventKind = "JOB_ACCEPTED"
	EventJobStarted        EventKind = "JOB_STARTED"
	EventCatalogValidated  EventKind = "CATALOG_VALIDATED"
	EventCandidatesStarted EventKind = "CANDIDATES_STARTED"
	EventCandidatesReady   EventKind = "CANDIDATES_READY"
	EventSolverStarted     EventKind = "SOLVER_STARTED"
	EventJobCompleted      EventKind = "JOB_COMPLETED"
	EventJobIncomplete     EventKind = "JOB_INCOMPLETE"
	EventJobFailed         EventKind = "JOB_FAILED"
	EventJobCancelled      EventKind = "JOB_CANCELLED"
)

func (kind EventKind) Terminal() bool {
	return kind == EventJobCompleted || kind == EventJobIncomplete || kind == EventJobFailed || kind == EventJobCancelled
}

type JobEvent struct {
	JobID           Identifier `json:"-"`
	Sequence        int64      `json:"sequence"`
	Kind            EventKind  `json:"kind"`
	ProgressCurrent *int       `json:"progress_current,omitempty"`
	ProgressTotal   *int       `json:"progress_total,omitempty"`
	CandidateCount  *int       `json:"candidate_count,omitempty"`
	ResultCount     *int       `json:"result_count,omitempty"`
	ErrorCode       string     `json:"error_code,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type EventPage struct {
	Events       []JobEvent `json:"events"`
	LastSequence int64      `json:"last_sequence"`
	Terminal     bool       `json:"terminal"`
}

type ResultPage struct {
	Job          Job           `json:"job"`
	Compositions []Composition `json:"compositions"`
	Total        int           `json:"total"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
}

type Capability struct {
	Enabled                   bool          `json:"enabled"`
	SemanticInterpretation    bool          `json:"semantic_interpretation"`
	DirectTypedSpecifications bool          `json:"direct_typed_specifications"`
	MaximumTaskTextRunes      int           `json:"maximum_task_text_runes"`
	MaximumRequirements       int           `json:"maximum_requirements"`
	MaximumRoles              int           `json:"maximum_roles"`
	MaximumConstraints        int           `json:"maximum_constraints"`
	MaximumCandidatesPerRole  int           `json:"maximum_candidates_per_role"`
	MaximumBranches           int           `json:"maximum_branches"`
	MaximumSolutions          int           `json:"maximum_solutions"`
	MaximumDuration           time.Duration `json:"-"`
	MaximumRequests           int           `json:"maximum_requests_per_hour"`
	Retention                 time.Duration `json:"-"`
}

type CreateDraftInput struct {
	ID              Identifier
	OwnerUserID     auth.Identifier
	CatalogVersion  string
	State           DraftState
	Spec            TaskSpec
	SpecFingerprint [32]byte
	ExpiresAt       time.Time
	Now             time.Time
}

type UpdateDraftInput struct {
	ID              Identifier
	OwnerUserID     auth.Identifier
	CatalogVersion  string
	Spec            TaskSpec
	SpecFingerprint [32]byte
	Version         int64
	Now             time.Time
}

type CreateJobInput struct {
	ID                 Identifier
	OwnerUserID        auth.Identifier
	DraftID            Identifier
	RetryOfJobID       *Identifier
	IdempotencyKey     string
	RequestFingerprint [32]byte
	CatalogVersion     string
	ExpiresAt          time.Time
	Now                time.Time
}

type CompleteJobInput struct {
	JobID     Identifier
	State     JobState
	Result    SolveResult
	Now       time.Time
	ErrorCode string
}

type Store interface {
	CurrentUser(context.Context, auth.Identifier) (auth.User, error)
	CreateDraft(context.Context, CreateDraftInput) (Draft, error)
	UpdateDraft(context.Context, UpdateDraftInput) (Draft, error)
	GetDraft(context.Context, Identifier, auth.Identifier) (Draft, error)
	CreateJob(context.Context, CreateJobInput, time.Time, int) (Job, bool, error)
	AttachRiverJob(context.Context, Identifier, auth.Identifier, int64, time.Time) (Job, error)
	ListJobs(context.Context, auth.Identifier, int, int) (JobPage, error)
	GetJob(context.Context, Identifier, auth.Identifier) (Job, error)
	GetJobForWorker(context.Context, Identifier) (Job, error)
	GetDraftForWorker(context.Context, Identifier) (Draft, error)
	ClaimJob(context.Context, Identifier, time.Time) (Job, bool, error)
	RecordProgress(context.Context, Identifier, EventKind, int, int, int, time.Time) (Job, error)
	CompleteJob(context.Context, CompleteJobInput) (Job, error)
	FailJob(context.Context, Identifier, string, JobState, time.Time) (Job, error)
	RequestCancellation(context.Context, Identifier, auth.Identifier, time.Time) (Job, error)
	ListEvents(context.Context, Identifier, auth.Identifier, int64, int) (EventPage, error)
	ListResults(context.Context, Identifier, auth.Identifier, int, int) (ResultPage, error)
	RecoverStaleJobs(context.Context, time.Time, time.Time, int) (int, error)
	DeleteExpired(context.Context, time.Time, int) (int, error)
	SaveAudit(context.Context, AuditEvent) error
}

type Jobs interface {
	EnqueueSolve(context.Context, Identifier) (int64, error)
	Cancel(context.Context, int64) error
}

type QueryGateway interface {
	CatalogV2(context.Context, auth.Session, string) (queryengine.Catalog, error)
	RunV2(context.Context, auth.Session, queryengine.QueryPlan, string) (queryengine.AdvancedResult, error)
}

type AuditEventType string

const (
	AuditDraftCreated AuditEventType = "DRAFT_CREATED"
	AuditDraftReviewed AuditEventType = "DRAFT_REVIEWED"
	AuditJobCreated AuditEventType = "JOB_CREATED"
	AuditJobRead AuditEventType = "JOB_READ"
	AuditJobStarted AuditEventType = "JOB_STARTED"
	AuditJobCancelled AuditEventType = "JOB_CANCELLED"
	AuditJobCompleted AuditEventType = "JOB_COMPLETED"
	AuditJobFailed AuditEventType = "JOB_FAILED"
	AuditJobRecovered AuditEventType = "JOB_RECOVERED"
	AuditResultRead AuditEventType = "RESULT_READ"
)

type AuditEvent struct {
	ID            Identifier
	ActorUserID   *auth.Identifier
	DraftID       *Identifier
	JobID         *Identifier
	EventType     AuditEventType
	Outcome       auth.AuditOutcome
	AffectedCount *int
	ErrorCode     string
	RequestID     string
	CreatedAt     time.Time
}

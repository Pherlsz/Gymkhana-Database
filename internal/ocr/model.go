package ocr

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	SchemaVersion                  = "v1"
	MaximumSuggestions             = 100
	MaximumEvidenceRunes           = 500
	MaximumValueRunes              = 5000
	MaximumSourceBytes       int64 = 20 << 20
	MaximumPages                   = 20
	MaximumPixels            int64 = 40_000_000
	MaximumProviderUsage     int64 = 100_000_000
	MaximumConfidence              = 10_000
	MaximumRegionCoord             = 1_000_000
	MaximumEventPage               = 200
	MaximumJobPage                 = 100
	MaximumAttempts                = 3
	MinimumIdempotencyLength       = 8
	MaximumIdempotencyLength       = 128
)

var ErrInvalidIdentifier = errors.New("invalid OCR identifier")

type Identifier [16]byte

func NewIdentifier() (Identifier, error) {
	var value Identifier
	if _, err := rand.Read(value[:]); err != nil {
		return Identifier{}, err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return value, nil
}

func ParseIdentifier(value string) (Identifier, error) {
	compact := strings.ReplaceAll(strings.TrimSpace(value), "-", "")
	if len(compact) != 32 {
		return Identifier{}, ErrInvalidIdentifier
	}
	decoded, err := hex.DecodeString(compact)
	if err != nil {
		return Identifier{}, ErrInvalidIdentifier
	}
	var identifier Identifier
	copy(identifier[:], decoded)
	return identifier, nil
}

func (identifier Identifier) String() string {
	encoded := hex.EncodeToString(identifier[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func (identifier Identifier) IsZero() bool { return identifier == Identifier{} }

type JobState string

const (
	JobQueued    JobState = "QUEUED"
	JobRunning   JobState = "RUNNING"
	JobCompleted JobState = "COMPLETED"
	JobFailed    JobState = "FAILED"
	JobCancelled JobState = "CANCELLED"
)

func (state JobState) Valid() bool {
	switch state {
	case JobQueued, JobRunning, JobCompleted, JobFailed, JobCancelled:
		return true
	default:
		return false
	}
}

func (state JobState) Terminal() bool {
	return state == JobCompleted || state == JobFailed || state == JobCancelled
}

type Job struct {
	ID                 Identifier
	OwnerUserID        auth.Identifier
	AttachmentID       attachment.Identifier
	RetryOfJobID       *Identifier
	IdempotencyKey     string
	RequestFingerprint [32]byte
	SourceSHA256       [32]byte
	CatalogFingerprint [32]byte
	SchemaVersion      string
	SourceMIME         string
	SourceBytes        int64
	State              JobState
	RiverJobID         int64
	AttemptCount       int
	PageCount          int
	PixelCount         int64
	SuggestionCount    int
	ProviderUsage      int64
	ErrorCode          string
	CancelRequestedAt  *time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type JobPage struct {
	Jobs   []Job
	Total  int
	Limit  int
	Offset int
}

type EventKind string

const (
	EventJobAccepted      EventKind = "JOB_ACCEPTED"
	EventJobStarted       EventKind = "JOB_STARTED"
	EventSourceValidated  EventKind = "SOURCE_VALIDATED"
	EventSuggestionsReady EventKind = "SUGGESTIONS_READY"
	EventJobCompleted     EventKind = "JOB_COMPLETED"
	EventJobFailed        EventKind = "JOB_FAILED"
	EventJobCancelled     EventKind = "JOB_CANCELLED"
)

func (kind EventKind) Terminal() bool {
	return kind == EventJobCompleted || kind == EventJobFailed || kind == EventJobCancelled
}

type JobEvent struct {
	JobID           Identifier
	Sequence        int64
	Kind            EventKind
	PageCount       *int
	SuggestionCount *int
	ErrorCode       string
	CreatedAt       time.Time
}

type EventPage struct {
	Events       []JobEvent
	LastSequence int64
	Terminal     bool
}

type TargetKind string

const (
	TargetProfile     TargetKind = "PROFILE"
	TargetDocument    TargetKind = "DOCUMENT"
	TargetBill        TargetKind = "BILL"
	TargetCustomField TargetKind = "CUSTOM_FIELD"
)

func (kind TargetKind) Valid() bool {
	return kind == TargetProfile || kind == TargetDocument || kind == TargetBill || kind == TargetCustomField
}

type ValueKind string

const (
	ValueText       ValueKind = "TEXT"
	ValueLongText   ValueKind = "LONG_TEXT"
	ValueInteger    ValueKind = "INTEGER"
	ValueDecimal    ValueKind = "DECIMAL"
	ValueBoolean    ValueKind = "BOOLEAN"
	ValueCivilDate  ValueKind = "CIVIL_DATE"
	ValueCivilMonth ValueKind = "CIVIL_MONTH"
	ValueEmail      ValueKind = "EMAIL"
	ValuePhone      ValueKind = "PHONE"
)

func (kind ValueKind) Valid() bool {
	switch kind {
	case ValueText, ValueLongText, ValueInteger, ValueDecimal, ValueBoolean, ValueCivilDate, ValueCivilMonth, ValueEmail, ValuePhone:
		return true
	default:
		return false
	}
}

type TargetReference struct {
	Kind TargetKind
	ID   Identifier
}

func (reference TargetReference) Valid() bool {
	return reference.Kind.Valid() && !reference.ID.IsZero()
}

type FieldSchema struct {
	Key           string
	Label         string
	Kind          ValueKind
	Required      bool
	Target        TargetReference
	TargetVersion int64
}

type Catalog struct {
	Fields      []FieldSchema
	Fingerprint [32]byte
}

type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

func (region Region) Valid() bool {
	return region.X >= 0 && region.Y >= 0 && region.Width > 0 && region.Height > 0 &&
		region.X <= MaximumRegionCoord && region.Y <= MaximumRegionCoord &&
		region.Width <= MaximumRegionCoord && region.Height <= MaximumRegionCoord &&
		region.X+region.Width <= MaximumRegionCoord && region.Y+region.Height <= MaximumRegionCoord
}

type Evidence struct {
	Page       int
	Region     *Region
	Excerpt    string
	Confidence *int
}

type ReviewState string

const (
	ReviewPending  ReviewState = "PENDING"
	ReviewAccepted ReviewState = "ACCEPTED"
	ReviewRejected ReviewState = "REJECTED"
	ReviewApplied  ReviewState = "APPLIED"
	ReviewStale    ReviewState = "STALE"
)

func (state ReviewState) Valid() bool {
	switch state {
	case ReviewPending, ReviewAccepted, ReviewRejected, ReviewApplied, ReviewStale:
		return true
	default:
		return false
	}
}

type Suggestion struct {
	ID               Identifier
	JobID            Identifier
	Ordinal          int
	Target           TargetReference
	TargetVersion    int64
	FieldKey         string
	FieldLabel       string
	Kind             ValueKind
	ProposedValue    string
	Evidence         Evidence
	ReviewState      ReviewState
	ReviewedValue    *string
	ReviewedByUserID *auth.Identifier
	ReviewedAt       *time.Time
	AppliedAt        *time.Time
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type SuggestionView struct {
	Suggestion     Suggestion
	CurrentValue   string
	CurrentVersion int64
	Stale          bool
}

type SuggestionPage struct {
	Suggestions []SuggestionView
	Total       int
	Limit       int
	Offset      int
}

type ReviewAction string

const (
	ReviewAccept ReviewAction = "ACCEPT"
	ReviewReject ReviewAction = "REJECT"
)

type ReviewInput struct {
	Action  ReviewAction
	Value   *string
	Version int64
}

type ApplyReceiptState string

const (
	ApplyRunning   ApplyReceiptState = "RUNNING"
	ApplyCompleted ApplyReceiptState = "COMPLETED"
)

type ApplyReceipt struct {
	ID                 Identifier
	JobID              Identifier
	OwnerUserID        auth.Identifier
	IdempotencyKey     string
	RequestFingerprint [32]byte
	State              ApplyReceiptState
	Results            []ApplyResult
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CompletedAt        *time.Time
}

type ApplyOutcome string

const (
	ApplyApplied ApplyOutcome = "APPLIED"
	ApplyStale   ApplyOutcome = "STALE"
	ApplyFailed  ApplyOutcome = "FAILED"
)

type ApplyResult struct {
	ReceiptID     Identifier
	SuggestionID  Identifier
	Outcome       ApplyOutcome
	TargetVersion *int64
	ErrorCode     string
	CreatedAt     time.Time
}

type Capability struct {
	Enabled              bool
	SupportedMIMEs       []string
	MaximumSourceBytes   int64
	MaximumPages         int
	MaximumPixels        int64
	MaximumSuggestions   int
	MaximumDuration      time.Duration
	MaximumRequests      int
	MaximumProviderUsage int64
}

type AuditEventType string

const (
	AuditJobCreated          AuditEventType = "JOB_CREATED"
	AuditJobRead             AuditEventType = "JOB_READ"
	AuditJobStarted          AuditEventType = "JOB_STARTED"
	AuditJobCancelled        AuditEventType = "JOB_CANCELLED"
	AuditJobCompleted        AuditEventType = "JOB_COMPLETED"
	AuditJobFailed           AuditEventType = "JOB_FAILED"
	AuditJobRecovered        AuditEventType = "JOB_RECOVERED"
	AuditSuggestionsRead     AuditEventType = "SUGGESTIONS_READ"
	AuditSuggestionReviewed  AuditEventType = "SUGGESTION_REVIEWED"
	AuditApplicationStarted  AuditEventType = "APPLICATION_STARTED"
	AuditApplicationComplete AuditEventType = "APPLICATION_COMPLETED"
)

type AuditEvent struct {
	ID            Identifier
	ActorUserID   *auth.Identifier
	JobID         *Identifier
	SuggestionID  *Identifier
	FieldKey      string
	ReviewAction  ReviewAction
	EventType     AuditEventType
	Outcome       auth.AuditOutcome
	AffectedCount *int
	ErrorCode     string
	RequestID     string
	CreatedAt     time.Time
}

package aichat

import (
	"crypto/rand"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/google/uuid"
)

const (
	MaximumThreadTitleRunes = 120
	MaximumMessageRunes     = 20_000
	MaximumThreadsPage      = 100
	MaximumMessagesPage     = 100
	MaximumEventsPage       = 200
	MaximumToolCalls        = 8
	MaximumToolRows         = 100
	MaximumResultPage       = 500
	MaximumToolResultBytes  = 256 * 1024
	MaximumToolFields       = 20
	MinimumIdempotencySize  = 8
	MaximumIdempotencySize  = 128
	DefaultThreadTitle      = "Nova conversa"
)

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
	parsed, err := uuid.Parse(value)
	if err != nil {
		return Identifier{}, err
	}
	return Identifier(parsed), nil
}

func (id Identifier) String() string { return uuid.UUID(id).String() }
func (id Identifier) IsZero() bool   { return id == Identifier{} }

type RunState string

const (
	RunQueued      RunState = "QUEUED"
	RunRunning     RunState = "RUNNING"
	RunToolRunning RunState = "TOOL_RUNNING"
	RunCompleted   RunState = "COMPLETED"
	RunFailed      RunState = "FAILED"
	RunCancelled   RunState = "CANCELLED"
)

func (state RunState) Active() bool {
	return state == RunQueued || state == RunRunning || state == RunToolRunning
}

func (state RunState) Terminal() bool {
	return state == RunCompleted || state == RunFailed || state == RunCancelled
}

type MessageRole string

const (
	MessageUser      MessageRole = "USER"
	MessageAssistant MessageRole = "ASSISTANT"
)

type ToolKind string

const (
	ToolCatalog ToolKind = "CATALOG"
	ToolSearch  ToolKind = "SEARCH"
	ToolQuery   ToolKind = "QUERY"
	ToolResult  ToolKind = "RESULT"
)

func (kind ToolKind) Valid() bool {
	return kind == ToolCatalog || kind == ToolSearch || kind == ToolQuery || kind == ToolResult
}

type ToolStepState string

const (
	ToolStepRunning   ToolStepState = "RUNNING"
	ToolStepCompleted ToolStepState = "COMPLETED"
	ToolStepFailed    ToolStepState = "FAILED"
	ToolStepCancelled ToolStepState = "CANCELLED"
)

type ResultReferenceKind string

const (
	ResultReferenceSearch ResultReferenceKind = "SEARCH"
	ResultReferenceQuery  ResultReferenceKind = "QUERY"
)

func (kind ResultReferenceKind) Valid() bool {
	return kind == ResultReferenceSearch || kind == ResultReferenceQuery
}

type EventKind string

const (
	EventRunAccepted     EventKind = "RUN_ACCEPTED"
	EventRunStarted      EventKind = "RUN_STARTED"
	EventTextDelta       EventKind = "TEXT_DELTA"
	EventToolStarted     EventKind = "TOOL_STARTED"
	EventToolCompleted   EventKind = "TOOL_COMPLETED"
	EventResultReference EventKind = "RESULT_REFERENCE"
	EventRunCompleted    EventKind = "RUN_COMPLETED"
	EventRunFailed       EventKind = "RUN_FAILED"
	EventRunCancelled    EventKind = "RUN_CANCELLED"
)

type Thread struct {
	ID                      Identifier
	OwnerUserID             auth.Identifier
	Title                   string
	ActiveResultReferenceID *Identifier
	RetentionExpiresAt      time.Time
	Version                 int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type ThreadPage struct {
	Threads []Thread
	Total   int
	Limit   int
	Offset  int
}

type Message struct {
	ID                 Identifier
	ThreadID           Identifier
	RunID              Identifier
	Sequence           int64
	Role               MessageRole
	Content            string
	ResultReferenceIDs []Identifier
	CreatedAt          time.Time
}

type MessagePage struct {
	Messages []Message
	Total    int
	Limit    int
	Offset   int
}

type Run struct {
	ID                 Identifier
	ThreadID           Identifier
	OwnerUserID        auth.Identifier
	RetryOfRunID       *Identifier
	State              RunState
	ToolCallCount      int
	InputUsage         int64
	OutputUsage        int64
	ResultBytes        int64
	ErrorCode          string
	CancelRequestedAt  *time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	IdempotencyKey     string
	RequestFingerprint [32]byte
}

type RunCreation struct {
	Run         Run
	UserMessage Message
	Created     bool
}

type ToolStep struct {
	ID                   Identifier
	RunID                Identifier
	Sequence             int
	Kind                 ToolKind
	State                ToolStepState
	ArgumentsFingerprint [32]byte
	ResultReferenceID    *Identifier
	RowCount             int
	ResultBytes          int
	ErrorCode            string
	StartedAt            time.Time
	CompletedAt          *time.Time
}

type ResultReference struct {
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
	CreatedAt          time.Time
}

type RunEvent struct {
	RunID             Identifier
	Sequence          int64
	Kind              EventKind
	TextDelta         string
	ToolStepID        *Identifier
	ResultReferenceID *Identifier
	ErrorCode         string
	CreatedAt         time.Time
}

type EventPage struct {
	Events       []RunEvent
	LastSequence int64
	Terminal     bool
}

type Capability struct {
	Enabled          bool
	MaximumToolCalls int
	MaximumRows      int
	MaximumBytes     int
	MaximumUsage     int64
	MaximumDuration  time.Duration
	MaximumMessage   int
}

func DefaultCapability() Capability {
	return Capability{
		MaximumToolCalls: MaximumToolCalls, MaximumRows: MaximumToolRows,
		MaximumBytes: MaximumToolResultBytes, MaximumUsage: defaultUsageLimit,
		MaximumDuration: defaultRunTimeout, MaximumMessage: MaximumMessageRunes,
	}
}

type AuditEventType string

const (
	AuditThreadCreated  AuditEventType = "THREAD_CREATED"
	AuditThreadRead     AuditEventType = "THREAD_READ"
	AuditThreadRenamed  AuditEventType = "THREAD_RENAMED"
	AuditThreadDeleted  AuditEventType = "THREAD_DELETED"
	AuditMessagesRead   AuditEventType = "MESSAGES_READ"
	AuditContextChanged AuditEventType = "CONTEXT_CHANGED"
	AuditRunCreated     AuditEventType = "RUN_CREATED"
	AuditRunStarted     AuditEventType = "RUN_STARTED"
	AuditRunCancelled   AuditEventType = "RUN_CANCELLED"
	AuditRunCompleted   AuditEventType = "RUN_COMPLETED"
	AuditRunFailed      AuditEventType = "RUN_FAILED"
	AuditRunRecovered   AuditEventType = "RUN_RECOVERED"
	AuditToolExecuted   AuditEventType = "TOOL_EXECUTED"
	AuditResultRead     AuditEventType = "RESULT_READ"
	AuditRetentionClean AuditEventType = "RETENTION_CLEANUP"
)

type AuditEvent struct {
	ID                Identifier
	ActorUserID       *auth.Identifier
	ThreadID          *Identifier
	RunID             *Identifier
	ResultReferenceID *Identifier
	EventType         AuditEventType
	Outcome           auth.AuditOutcome
	ToolKind          *ToolKind
	AffectedCount     *int
	ErrorCode         string
	RequestID         string
	CreatedAt         time.Time
}

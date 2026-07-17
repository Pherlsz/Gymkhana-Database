package googleforms

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/operations"
)

const (
	ScopeBodyReadonly      = "https://www.googleapis.com/auth/forms.body.readonly"
	ScopeResponsesReadonly = "https://www.googleapis.com/auth/forms.responses.readonly"
	MaximumSourcesPerOwner = 25
	MaximumQuestions       = 256
	MaximumResponsesPerRun = 500
	MaximumPagesPerRun     = 10
	OAuthStateTTL          = 10 * time.Minute
	CursorOverlap          = time.Second
)

var RequiredScopes = []string{ScopeBodyReadonly, ScopeResponsesReadonly}

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
		return Identifier{}, ErrInvalidInput
	}
	decoded, err := hex.DecodeString(compact)
	if err != nil {
		return Identifier{}, ErrInvalidInput
	}
	var id Identifier
	copy(id[:], decoded)
	return id, nil
}

func (id Identifier) String() string {
	encoded := hex.EncodeToString(id[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func (id Identifier) IsZero() bool { return id == Identifier{} }

func (id Identifier) OperationIdentifier() operations.Identifier {
	return operations.Identifier(id)
}

type ConnectionState string

const (
	ConnectionActive       ConnectionState = "ACTIVE"
	ConnectionNeedsReauth  ConnectionState = "NEEDS_REAUTH"
	ConnectionDisconnected ConnectionState = "DISCONNECTED"
)

type Connection struct {
	ID                     Identifier
	OwnerUserID            auth.Identifier
	State                  ConnectionState
	RefreshTokenCiphertext []byte
	RefreshTokenNonce      []byte
	TokenKeyVersion        uint16
	GrantedScopes          []string
	ErrorCode              string
	ConnectedAt            *time.Time
	DisconnectedAt         *time.Time
	Version                int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type OAuthState struct {
	StateHash          [sha256.Size]byte
	OwnerUserID        auth.Identifier
	SessionID          auth.Identifier
	VerifierCiphertext []byte
	VerifierNonce      []byte
	TokenKeyVersion    uint16
	ReturnPath         string
	CreatedAt          time.Time
	ExpiresAt          time.Time
	ConsumedAt         *time.Time
}

type SourceState string

const (
	SourceDraft       SourceState = "DRAFT"
	SourceActive      SourceState = "ACTIVE"
	SourcePaused      SourceState = "PAUSED"
	SourceSchemaDrift SourceState = "SCHEMA_DRIFT"
	SourceNeedsReauth SourceState = "NEEDS_REAUTH"
	SourceError       SourceState = "ERROR"
)

type SyncMode string

const (
	SyncManual SyncMode = "MANUAL"
	SyncPoll   SyncMode = "POLL"
)

type Source struct {
	ID                Identifier
	ConnectionID      Identifier
	OwnerUserID       auth.Identifier
	ProviderFormID    string
	Title             string
	Module            operations.Module
	State             SourceState
	SchemaRevision    string
	SchemaFingerprint [sha256.Size]byte
	SyncMode          SyncMode
	PollInterval      time.Duration
	CursorSubmittedAt *time.Time
	ResponsePageToken string
	PageTokenCursor   *time.Time
	LastSyncedAt      *time.Time
	NextSyncAt        *time.Time
	ErrorCode         string
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Questions         []Question
}

type AnswerKind string

const (
	AnswerText        AnswerKind = "TEXT"
	AnswerDate        AnswerKind = "DATE"
	AnswerTime        AnswerKind = "TIME"
	AnswerScale       AnswerKind = "SCALE"
	AnswerUnsupported AnswerKind = "UNSUPPORTED"
)

type Question struct {
	ID                  string
	Position            int
	Title               string
	AnswerKind          AnswerKind
	Required            bool
	Supported           bool
	UnsupportedCode     string
	QuestionFingerprint [sha256.Size]byte
	TargetField         string
}

type TriggerKind string

const (
	TriggerManual    TriggerKind = "MANUAL"
	TriggerScheduled TriggerKind = "SCHEDULED"
)

type SyncState string

const (
	SyncQueued    SyncState = "QUEUED"
	SyncRunning   SyncState = "RUNNING"
	SyncCompleted SyncState = "COMPLETED"
	SyncFailed    SyncState = "FAILED"
	SyncCancelled SyncState = "CANCELLED"
)

type SyncRun struct {
	ID                Identifier
	SourceID          Identifier
	OwnerUserID       auth.Identifier
	ActorUserID       *auth.Identifier
	TriggerKind       TriggerKind
	State             SyncState
	IdempotencyKey    string
	OperationImportID *operations.Identifier
	CursorStartedAt   *time.Time
	CursorCompletedAt *time.Time
	ReceivedCount     int
	StagedCount       int
	DuplicateCount    int
	RiverJobID        int64
	ErrorCode         string
	StartedAt         *time.Time
	CompletedAt       *time.Time
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Form struct {
	ID          string
	Title       string
	Revision    string
	Questions   []Question
	Fingerprint [sha256.Size]byte
}

type Response struct {
	ID          string
	SubmittedAt time.Time
	Answers     map[string][]string
}

type ResponsePage struct {
	Responses     []Response
	NextPageToken string
}

type SourcePage struct {
	Sources []Source
	Total   int
}

type SyncRunPage struct {
	Runs  []SyncRun
	Total int
}

type CreateSourceInput struct {
	FormReference string
	Module        operations.Module
}

type MappingInput struct {
	QuestionID  string
	TargetField string
}

type UpdateSourceInput struct {
	SyncMode     SyncMode
	PollInterval time.Duration
	Enabled      bool
}

type AuditEventType string

const (
	AuditConnectionStarted      AuditEventType = "CONNECTION_STARTED"
	AuditConnectionCompleted    AuditEventType = "CONNECTION_COMPLETED"
	AuditConnectionFailed       AuditEventType = "CONNECTION_FAILED"
	AuditConnectionDisconnected AuditEventType = "CONNECTION_DISCONNECTED"
	AuditSourceCreated          AuditEventType = "SOURCE_CREATED"
	AuditSourceMapped           AuditEventType = "SOURCE_MAPPED"
	AuditSourceStateChanged     AuditEventType = "SOURCE_STATE_CHANGED"
	AuditSyncRequested          AuditEventType = "SYNC_REQUESTED"
	AuditSyncStarted            AuditEventType = "SYNC_STARTED"
	AuditSyncCompleted          AuditEventType = "SYNC_COMPLETED"
	AuditSyncFailed             AuditEventType = "SYNC_FAILED"
	AuditSyncCancelled          AuditEventType = "SYNC_CANCELLED"
)

type AuditEvent struct {
	ID            Identifier
	ActorUserID   *auth.Identifier
	ConnectionID  *Identifier
	SourceID      *Identifier
	SyncRunID     *Identifier
	EventType     AuditEventType
	Outcome       auth.AuditOutcome
	AffectedCount *int
	RequestID     string
	CreatedAt     time.Time
}

func canonicalScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			seen[scope] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for scope := range seen {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result
}

func hasRequiredScopes(scopes []string) bool {
	canonical := canonicalScopes(scopes)
	if len(canonical) != len(RequiredScopes) {
		return false
	}
	want := canonicalScopes(RequiredScopes)
	return compareScopes(canonical, want) == nil
}

func compareScopes(got, want []string) error {
	if len(got) != len(want) {
		return ErrOAuthScopes
	}
	for index := range want {
		if got[index] != want[index] {
			return ErrOAuthScopes
		}
	}
	return nil
}

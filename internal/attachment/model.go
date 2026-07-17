package attachment

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	MaxFileNameLength = 255
	MaxMIMELength     = 127
)

var ErrInvalidIdentifier = errors.New("invalid attachment identifier")

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

func (identifier Identifier) IsZero() bool {
	return identifier == Identifier{}
}

type OwnerKind string

const (
	OwnerDocument    OwnerKind = "DOCUMENT"
	OwnerBill        OwnerKind = "BILL"
	OwnerCustomField OwnerKind = "CUSTOM_FIELD"
)

func (kind OwnerKind) Valid() bool {
	return kind == OwnerDocument || kind == OwnerBill || kind == OwnerCustomField
}

type CustomTargetKind string

const (
	CustomTargetProfile      CustomTargetKind = "PROFILE"
	CustomTargetDocument     CustomTargetKind = "DOCUMENT"
	CustomTargetBill         CustomTargetKind = "BILL"
	CustomTargetCustomEntity CustomTargetKind = "CUSTOM_ENTITY"
)

func (kind CustomTargetKind) Valid() bool {
	return kind == CustomTargetProfile || kind == CustomTargetDocument || kind == CustomTargetBill || kind == CustomTargetCustomEntity
}

type OwnerReference struct {
	Kind              OwnerKind
	ID                Identifier
	CustomTargetKind  CustomTargetKind
	FieldDefinitionID Identifier
}

func (owner OwnerReference) Valid() bool {
	if !owner.Kind.Valid() || owner.ID.IsZero() {
		return false
	}
	if owner.Kind != OwnerCustomField {
		return owner.CustomTargetKind == "" && owner.FieldDefinitionID.IsZero()
	}
	return owner.CustomTargetKind.Valid() && !owner.FieldDefinitionID.IsZero()
}

type LifecycleState string

const (
	LifecycleActive  LifecycleState = "ACTIVE"
	LifecycleTrashed LifecycleState = "TRASHED"
)

func (state LifecycleState) Valid() bool {
	return state == LifecycleActive || state == LifecycleTrashed
}

type UploadIntent struct {
	ID               Identifier
	ActorUserID      auth.Identifier
	Owner            OwnerReference
	OriginalFileName string
	DeclaredMIME     string
	ExpectedSize     int64
	ObjectKey        string
	ExpiresAt        time.Time
	ConsumedAt       *time.Time
	CreatedAt        time.Time
}

type Attachment struct {
	ID               Identifier
	Owner            OwnerReference
	OriginalFileName string
	DeclaredMIME     string
	DetectedMIME     string
	ByteSize         int64
	SHA256           [32]byte
	ObjectKey        string
	LifecycleState   LifecycleState
	DeletedAt        *time.Time
	PurgeAfter       *time.Time
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateUploadIntentInput struct {
	Owner            OwnerReference
	OriginalFileName string
	DeclaredMIME     string
	ExpectedSize     int64
}

type SignedRequest struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

type VerifiedObject struct {
	DetectedMIME string
	ByteSize     int64
	SHA256       [32]byte
}

type AuditEventType string

const (
	AuditUploadIntentCreated  AuditEventType = "ATTACHMENT_UPLOAD_INTENT_CREATED"
	AuditUploadConfirmed      AuditEventType = "ATTACHMENT_UPLOAD_CONFIRMED"
	AuditDownloadRequested    AuditEventType = "ATTACHMENT_DOWNLOAD_REQUESTED"
	AuditTrashed              AuditEventType = "ATTACHMENT_TRASHED"
	AuditRestored             AuditEventType = "ATTACHMENT_RESTORED"
	AuditPurged               AuditEventType = "ATTACHMENT_PURGED"
	AuditExpiredUploadCleaned AuditEventType = "ATTACHMENT_EXPIRED_UPLOAD_CLEANED"
)

type AuditEvent struct {
	ID             Identifier
	ActorUserID    *auth.Identifier
	AttachmentID   *Identifier
	UploadIntentID *Identifier
	Owner          *OwnerReference
	EventType      AuditEventType
	Outcome        auth.AuditOutcome
	RequestID      string
}

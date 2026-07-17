package matching

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	MaximumCandidates      = 2_000
	MaximumCasePageSize    = 100
	MaximumAnalysisRate    = 5
	MaximumActiveAnalyses  = 1
	AnalysisWindow         = time.Hour
	AnalysisTimeout        = 15 * time.Second
	AnalysisRetention      = 30 * 24 * time.Hour
	CleanupBatchSize       = 250
	MergeConfirmation      = "MESCLAR"
	MinimumCandidateScore  = 50
	MediumCandidateScore   = 70
	HighCandidateScore     = 90
	MaximumCanonicalFields = 14
	MaximumCustomFields    = 256
)

var ErrInvalidIdentifier = errors.New("invalid matching identifier")

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

func (identifier Identifier) AuthIdentifier() auth.Identifier { return auth.Identifier(identifier) }

type AnalysisState string

const (
	AnalysisQueued    AnalysisState = "QUEUED"
	AnalysisRunning   AnalysisState = "RUNNING"
	AnalysisCompleted AnalysisState = "COMPLETED"
	AnalysisFailed    AnalysisState = "FAILED"
	AnalysisCancelled AnalysisState = "CANCELLED"
)

func (state AnalysisState) Valid() bool {
	switch state {
	case AnalysisQueued, AnalysisRunning, AnalysisCompleted, AnalysisFailed, AnalysisCancelled:
		return true
	default:
		return false
	}
}

func (state AnalysisState) Terminal() bool {
	return state == AnalysisCompleted || state == AnalysisFailed || state == AnalysisCancelled
}

type Analysis struct {
	ID                Identifier
	ActorUserID       auth.Identifier
	IdempotencyKey    string
	State             AnalysisState
	RiverJobID        int64
	ProfilesScanned   int
	CandidateCount    int
	RefreshedCount    int
	ErrorCode         string
	CancelRequestedAt *time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
	ExpiresAt         time.Time
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type EvidenceKind string

const (
	EvidenceCPFExact       EvidenceKind = "CPF_EXACT"
	EvidenceEmailExact     EvidenceKind = "EMAIL_EXACT"
	EvidenceMobileExact    EvidenceKind = "MOBILE_EXACT"
	EvidenceLandlineExact  EvidenceKind = "LANDLINE_EXACT"
	EvidenceNameExact      EvidenceKind = "NAME_EXACT"
	EvidenceNameSimilar    EvidenceKind = "NAME_SIMILAR"
	EvidencePostalExact    EvidenceKind = "POSTAL_EXACT"
	EvidenceCityExact      EvidenceKind = "CITY_EXACT"
	EvidenceAddressSimilar EvidenceKind = "ADDRESS_SIMILAR"
)

type EvidenceDefinition struct {
	Kind  EvidenceKind
	Label string
}

var evidenceCatalog = []EvidenceDefinition{
	{Kind: EvidenceCPFExact, Label: "CPF igual"},
	{Kind: EvidenceEmailExact, Label: "E-mail igual"},
	{Kind: EvidenceMobileExact, Label: "Celular igual"},
	{Kind: EvidenceLandlineExact, Label: "Telefone igual"},
	{Kind: EvidenceNameExact, Label: "Nome igual"},
	{Kind: EvidenceNameSimilar, Label: "Nome semelhante"},
	{Kind: EvidencePostalExact, Label: "CEP igual"},
	{Kind: EvidenceCityExact, Label: "Cidade igual"},
	{Kind: EvidenceAddressSimilar, Label: "Endereço semelhante"},
}

func EvidenceCatalog() []EvidenceDefinition {
	result := make([]EvidenceDefinition, len(evidenceCatalog))
	copy(result, evidenceCatalog)
	return result
}

func (kind EvidenceKind) Valid() bool {
	for _, definition := range evidenceCatalog {
		if definition.Kind == kind {
			return true
		}
	}
	return false
}

type Evidence struct {
	Kind         EvidenceKind
	Strength     int
	Contribution int
}

type ScoreBand string

const (
	ScoreLow    ScoreBand = "LOW"
	ScoreMedium ScoreBand = "MEDIUM"
	ScoreHigh   ScoreBand = "HIGH"
)

func (band ScoreBand) Valid() bool {
	return band == ScoreLow || band == ScoreMedium || band == ScoreHigh
}

func scoreBand(score int) ScoreBand {
	switch {
	case score >= HighCandidateScore:
		return ScoreHigh
	case score >= MediumCandidateScore:
		return ScoreMedium
	default:
		return ScoreLow
	}
}

type CaseState string

const (
	CasePending      CaseState = "PENDING"
	CaseNotDuplicate CaseState = "NOT_DUPLICATE"
	CaseMerged       CaseState = "MERGED"
	CaseStale        CaseState = "STALE"
)

func (state CaseState) Valid() bool {
	return state == CasePending || state == CaseNotDuplicate || state == CaseMerged || state == CaseStale
}

type ProfileSnapshot struct {
	ID                  Identifier
	FullName            string
	SocialName          string
	CPF                 string
	Email               string
	MobilePhone         string
	LandlinePhone       string
	AddressStreet       string
	AddressNumber       string
	AddressComplement   string
	AddressNeighborhood string
	AddressCity         string
	AddressState        string
	AddressPostalCode   string
	Notes               string
	Version             int64
	UpdatedAt           time.Time
}

type Case struct {
	ID                  Identifier
	LeftProfileID       Identifier
	RightProfileID      Identifier
	LeftProfileVersion  int64
	RightProfileVersion int64
	Score               int
	ScoreBand           ScoreBand
	State               CaseState
	Evidence            []Evidence
	Left                *ProfileSnapshot
	Right               *ProfileSnapshot
	DecidedByUserID     *auth.Identifier
	DecidedAt           *time.Time
	MergedSurvivorID    *Identifier
	MergedSourceID      *Identifier
	MergedAt            *time.Time
	Version             int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Candidate struct {
	LeftProfileID       Identifier
	RightProfileID      Identifier
	LeftProfileVersion  int64
	RightProfileVersion int64
	Score               int
	ScoreBand           ScoreBand
	Evidence            []Evidence
}

type AnalysisStats struct {
	ProfilesScanned int
	CandidateCount  int
	RefreshedCount  int
}

type CaseListOptions struct {
	States []CaseState
	Bands  []ScoreBand
	Limit  int
	Offset int
}

type CasePage struct {
	Cases  []Case
	Total  int
	Limit  int
	Offset int
}

type FieldSource string

const (
	FieldFromSurvivor FieldSource = "SURVIVOR"
	FieldFromSource   FieldSource = "SOURCE"
)

func (source FieldSource) Valid() bool {
	return source == FieldFromSurvivor || source == FieldFromSource
}

type FieldChoice struct {
	FieldKey string
	Source   FieldSource
}

type MergePreviewInput struct {
	CaseID          Identifier
	SurvivorID      Identifier
	SourceID        Identifier
	SurvivorVersion int64
	SourceVersion   int64
	Choices         []FieldChoice
}

type MergeField struct {
	Key            string
	Label          string
	Kind           string
	SurvivorValue  *string
	SourceValue    *string
	Conflict       bool
	ChoiceRequired bool
	SelectedSource FieldSource
}

type DependencyCount struct {
	Kind  string
	Count int
}

const (
	DependencyDocumentOwner    = "DOCUMENT_OWNER"
	DependencyDocumentHolder   = "DOCUMENT_HOLDER"
	DependencyBillOwner        = "BILL_OWNER"
	DependencyBillHolder       = "BILL_HOLDER"
	DependencyCustomEntity     = "CUSTOM_ENTITY_OWNER"
	DependencyCustomValue      = "CUSTOM_PROFILE_VALUE"
	DependencyAttachmentIntent = "ATTACHMENT_INTENT"
	DependencyAttachment       = "ATTACHMENT"
	ConflictDocumentUnique     = "DOCUMENT_UNIQUENESS"
	ConflictCustomEntity       = "CUSTOM_ENTITY_CARDINALITY"
)

type DependencyConflict struct {
	Kind  string
	Count int
}

type MergePreview struct {
	CaseID               Identifier
	Survivor             ProfileSnapshot
	Source               ProfileSnapshot
	Fields               []MergeField
	Dependencies         []DependencyCount
	Conflicts            []DependencyConflict
	UnresolvedFieldCount int
	PreviewFingerprint   [sha256.Size]byte
	Confirmation         string
	GeneratedAt          time.Time
}

type MergeInput struct {
	MergePreviewInput
	PreviewFingerprint [sha256.Size]byte
	IdempotencyKey     string
	Confirmation       string
}

type MergeResult struct {
	CaseID            Identifier
	SurvivorProfileID Identifier
	SourceProfileID   Identifier
	SurvivorVersion   int64
	MovedDependencies []DependencyCount
	MergedAt          time.Time
}

type AuditEventType string

const (
	AuditAnalysisCreated   AuditEventType = "ANALYSIS_CREATED"
	AuditAnalysisStarted   AuditEventType = "ANALYSIS_STARTED"
	AuditAnalysisCompleted AuditEventType = "ANALYSIS_COMPLETED"
	AuditAnalysisCancelled AuditEventType = "ANALYSIS_CANCELLED"
	AuditAnalysisFailed    AuditEventType = "ANALYSIS_FAILED"
	AuditCaseReviewed      AuditEventType = "CASE_REVIEWED"
	AuditCaseDismissed     AuditEventType = "CASE_DISMISSED"
	AuditMergePreviewed    AuditEventType = "MERGE_PREVIEWED"
	AuditMergeCompleted    AuditEventType = "MERGE_COMPLETED"
	AuditMergeDenied       AuditEventType = "MERGE_DENIED"
)

type AuditEvent struct {
	ID            Identifier
	ActorUserID   *auth.Identifier
	AnalysisID    *Identifier
	CaseID        *Identifier
	EventType     AuditEventType
	Outcome       auth.AuditOutcome
	ScoreBand     *ScoreBand
	AffectedCount *int
	ErrorCode     string
	RequestID     string
	CreatedAt     time.Time
}

package operations

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	BulkDeleteConfirmation = "Confirmar"
	MaximumFileSize        = int64(25 << 20)
	MaximumRows            = 100_000
	MaximumExportRows      = 1_048_575 // One header row plus Excel's worksheet row limit.
	MaximumColumns         = 256
	MaximumCells           = 6_000_000
	headerRowSlack         = 8 // Title rows plus header before data, as in TNC/Forms dumps.
	MaximumCellBytes       = 32_768
	MaximumPreviewRows     = 200
	MaximumReportRows      = 500
	MaximumBatchSize       = 100
	MaximumBulkSelection   = 500
	CleanupClaimTTL        = 30 * time.Minute
)

var ErrInvalidIdentifier = errors.New("invalid operation identifier")

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

type Module string

const (
	ModuleProfiles  Module = "PROFILES"
	ModuleDocuments Module = "DOCUMENTS"
	ModuleBills     Module = "BILLS"
)

func (module Module) Valid() bool {
	return module == ModuleProfiles || module == ModuleDocuments || module == ModuleBills
}

func (module Module) Label() string {
	switch module {
	case ModuleProfiles:
		return "Pessoas"
	case ModuleDocuments:
		return "Documentos"
	case ModuleBills:
		return "Contas"
	default:
		return ""
	}
}

func SupportedModules() []Module {
	return []Module{ModuleProfiles, ModuleDocuments, ModuleBills}
}

type SourceKind string

const (
	SourceXLSX        SourceKind = "XLSX"
	SourceGoogleForms SourceKind = "GOOGLE_FORMS"
)

type ImportState string

const (
	ImportUploading         ImportState = "UPLOADING"
	ImportUploaded          ImportState = "UPLOADED"
	ImportParsing           ImportState = "PARSING"
	ImportMapping           ImportState = "MAPPING"
	ImportPreviewReady      ImportState = "PREVIEW_READY"
	ImportDecisionsRequired ImportState = "DECISIONS_REQUIRED"
	ImportReady             ImportState = "READY"
	ImportQueued            ImportState = "QUEUED"
	ImportRunning           ImportState = "RUNNING"
	ImportCompleted         ImportState = "COMPLETED"
	ImportFailed            ImportState = "FAILED"
	ImportCancelled         ImportState = "CANCELLED"
	ImportExpired           ImportState = "EXPIRED"
)

func (state ImportState) Terminal() bool {
	return state == ImportCompleted || state == ImportFailed || state == ImportCancelled || state == ImportExpired
}

type Stage string

const (
	StageUpload  Stage = "UPLOAD"
	StageParse   Stage = "PARSE"
	StageMap     Stage = "MAP"
	StagePreview Stage = "PREVIEW"
	StageExecute Stage = "EXECUTE"
	StageReport  Stage = "REPORT"
	StageCleanup Stage = "CLEANUP"
)

type Import struct {
	ID                   Identifier
	ActorUserID          auth.Identifier
	Module               Module
	SourceKind           SourceKind
	OriginalFilename     string
	DeclaredSize         int64
	ActualSize           int64
	ContentSHA256        [32]byte
	ObjectKey            string
	IdempotencyKey       string
	State                ImportState
	Stage                Stage
	SelectedSheetIndex   *int
	MappingVersion       int64
	UnresolvedCount      int
	ValidationErrorCount int
	InsertedCount        int
	UpdatedCount         int
	LinkedCount          int
	SkippedCount         int
	ErroredCount         int
	ConflictedCount      int
	RiverJobID           int64
	ErrorCode            string
	ExpiresAt            time.Time
	CancelledAt          *time.Time
	CompletedAt          *time.Time
	Version              int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Sheets               []Sheet
	Columns              []Column
	Preview              []Row
}

type Sheet struct {
	Index       int
	Name        string
	RowCount    int
	ColumnCount int
}

type Column struct {
	SheetIndex   int
	SourceColumn int
	SourceHeader string
	TargetField  string
}

type ValueKind string

const (
	ValueEmpty   ValueKind = "EMPTY"
	ValueText    ValueKind = "TEXT"
	ValueNumber  ValueKind = "NUMBER"
	ValueBoolean ValueKind = "BOOLEAN"
	ValueDate    ValueKind = "DATE"
)

type Cell struct {
	SourceColumn   int
	RawValue       string
	ValueKind      ValueKind
	FormulaPresent bool
	ValidationCode string
}

type Action string

const (
	ActionCreate Action = "CREATE"
	ActionUpdate Action = "UPDATE"
	ActionLink   Action = "LINK"
	ActionSkip   Action = "SKIP"
	ActionError  Action = "ERROR"
)

type Row struct {
	SheetIndex           int
	RowNumber            int
	ProposedAction       Action
	Decision             Action
	TargetID             *Identifier
	TargetVersion        int64
	ValidationErrorCount int
	DecisionRequired     bool
	SourceFingerprint    [32]byte
	Cells                []Cell
	Outcome              *Outcome
}

type OutcomeKind string

const (
	OutcomeInserted   OutcomeKind = "INSERTED"
	OutcomeUpdated    OutcomeKind = "UPDATED"
	OutcomeLinked     OutcomeKind = "LINKED"
	OutcomeSkipped    OutcomeKind = "SKIPPED"
	OutcomeErrored    OutcomeKind = "ERRORED"
	OutcomeConflicted OutcomeKind = "CONFLICTED"
)

type Outcome struct {
	Kind        OutcomeKind
	TargetID    *Identifier
	ErrorCode   string
	CommittedAt time.Time
}

type ExportState string

const (
	ExportQueued    ExportState = "QUEUED"
	ExportRunning   ExportState = "RUNNING"
	ExportCompleted ExportState = "COMPLETED"
	ExportFailed    ExportState = "FAILED"
	ExportCancelled ExportState = "CANCELLED"
	ExportExpired   ExportState = "EXPIRED"
)

type Export struct {
	ID             Identifier
	ActorUserID    auth.Identifier
	Module         Module
	IdempotencyKey string
	State          ExportState
	ObjectKey      string
	Filename       string
	RowCount       int
	ByteSize       int64
	ContentSHA256  [32]byte
	RiverJobID     int64
	ErrorCode      string
	ExpiresAt      time.Time
	CompletedAt    *time.Time
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateImportInput struct {
	Module           Module
	OriginalFilename string
	DeclaredSize     int64
	IdempotencyKey   string
}

type MappingInput struct {
	SourceColumn int
	TargetField  string
}

type DecisionInput struct {
	RowNumber int
	Action    Action
	TargetID  *Identifier
	Version   int64
}

type BulkItem struct {
	ID      Identifier
	Version int64
}

type BulkDeleteResult struct {
	Module  Module
	Deleted int
}

type ListOptions struct {
	Limit  int
	Offset int
}

type ImportPage struct {
	Imports []Import
	Total   int64
	Limit   int
	Offset  int
}

type ExportPage struct {
	Exports []Export
	Total   int64
	Limit   int
	Offset  int
}

type UploadGrant struct {
	Import    Import
	UploadURL string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

type DownloadGrant struct {
	URL       string
	Method    string
	ExpiresAt time.Time
}

type Report struct {
	ImportID         Identifier
	State            ImportState
	Inserted         int
	Updated          int
	Linked           int
	Skipped          int
	Errored          int
	Conflicted       int
	Decisions        int
	Unresolved       int
	ValidationErrors int
	Rows             []ReportRow
}

type ReportRow struct {
	SheetIndex int
	RowNumber  int
	Outcome    OutcomeKind
	ErrorCode  string
}

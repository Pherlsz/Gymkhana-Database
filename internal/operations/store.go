package operations

import (
	"context"
	"io"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type ObjectStore interface {
	PresignOperationUpload(context.Context, string, int64, time.Duration) (attachment.SignedRequest, error)
	PresignDownload(context.Context, string, string, string, time.Duration) (attachment.SignedRequest, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
}

type Jobs interface {
	EnqueueParse(context.Context, Identifier) (int64, error)
	EnqueueExecute(context.Context, Identifier) (int64, error)
	EnqueueExport(context.Context, Identifier) (int64, error)
	EnqueueCleanup(context.Context, time.Time) (int64, error)
	Cancel(context.Context, int64) error
}

type Limits struct {
	WindowStart     time.Time
	MaximumRequests int
	MaximumActive   int
}

type Mutation struct {
	Module       Module
	Profile      *profile.Values
	Document     *document.Values
	Bill         *bill.Values
	CustomValues []CustomValueMutation
}

type CustomFieldDefinition struct {
	Field      Field
	Definition customdata.FieldDefinition
}

type CustomValueMutation struct {
	Definition customdata.FieldDefinition
	Value      customdata.ValueInput
}

type MappedRow struct {
	Row    Row
	Values map[string]string
}

type ExportDataset struct {
	Headers []string
	Rows    [][]string
}

type CleanupCandidate struct {
	Kind                   string
	ID                     Identifier
	ActorUserID            auth.Identifier
	Module                 Module
	ObjectKey              string
	RequiresObjectDeletion bool
}

type AuditEventType string

const (
	AuditImportCreated    AuditEventType = "IMPORT_CREATED"
	AuditImportConfirmed  AuditEventType = "IMPORT_CONFIRMED"
	AuditImportParsed     AuditEventType = "IMPORT_PARSED"
	AuditImportMapped     AuditEventType = "IMPORT_MAPPED"
	AuditImportPreviewed  AuditEventType = "IMPORT_PREVIEWED"
	AuditImportDecided    AuditEventType = "IMPORT_DECIDED"
	AuditImportStarted    AuditEventType = "IMPORT_STARTED"
	AuditImportCancelled  AuditEventType = "IMPORT_CANCELLED"
	AuditImportCompleted  AuditEventType = "IMPORT_COMPLETED"
	AuditImportFailed     AuditEventType = "IMPORT_FAILED"
	AuditImportExpired    AuditEventType = "IMPORT_EXPIRED"
	AuditExportCreated    AuditEventType = "EXPORT_CREATED"
	AuditExportCompleted  AuditEventType = "EXPORT_COMPLETED"
	AuditExportDownloaded AuditEventType = "EXPORT_DOWNLOADED"
	AuditExportFailed     AuditEventType = "EXPORT_FAILED"
	AuditExportExpired    AuditEventType = "EXPORT_EXPIRED"
	AuditBulkDelete       AuditEventType = "BULK_DELETE"
)

type AuditEvent struct {
	ID            Identifier
	ActorUserID   *auth.Identifier
	ImportID      *Identifier
	ExportID      *Identifier
	Module        Module
	EventType     AuditEventType
	Outcome       auth.AuditOutcome
	AffectedCount *int
	RequestID     string
	CreatedAt     time.Time
}

type Store interface {
	CreateImport(context.Context, Import, Limits) (Import, error)
	ConfirmImport(context.Context, Identifier, auth.Identifier, int64, [32]byte, int64, time.Time) (Import, error)
	GetImport(context.Context, Identifier, auth.Identifier) (Import, error)
	GetImportForWorker(context.Context, Identifier) (Import, error)
	GetReport(context.Context, Identifier, auth.Identifier) (Report, error)
	ListImports(context.Context, auth.Identifier, ListOptions) (ImportPage, error)
	StageWorkbook(context.Context, Identifier, Workbook, time.Time) error
	ApplySuggestedColumnMapping(context.Context, Identifier, Module, *int) error
	SelectSheet(context.Context, Identifier, auth.Identifier, int64, int, time.Time) (Import, error)
	SaveMapping(context.Context, Identifier, auth.Identifier, int64, []MappingInput, time.Time) (Import, error)
	LoadMappedRows(context.Context, Identifier) ([]MappedRow, error)
	SavePreview(context.Context, Identifier, auth.Identifier, int64, []Row, int, int, time.Time) (Import, error)
	SaveDecisions(context.Context, Identifier, auth.Identifier, int64, []DecisionInput, time.Time) (Import, error)
	QueueImport(context.Context, Identifier, auth.Identifier, int64, int64, time.Time) (Import, error)
	BeginImport(context.Context, Identifier, time.Time) (Import, error)
	ListPendingRows(context.Context, Identifier, int) ([]MappedRow, error)
	ApplyRow(context.Context, Import, MappedRow, Mutation, Identifier, string, time.Time) (Outcome, error)
	ReopenImportForPreview(context.Context, Identifier, time.Time) (Import, error)
	CompleteImport(context.Context, Identifier, time.Time) (Import, error)
	FailImport(context.Context, Identifier, ImportState, string, time.Time) (bool, error)
	CancelImport(context.Context, Identifier, auth.Identifier, int64, time.Time) (Import, error)
	DeleteImport(context.Context, Identifier, auth.Identifier) error
	CreateExport(context.Context, Export, Limits) (Export, error)
	GetExport(context.Context, Identifier, auth.Identifier) (Export, error)
	GetExportForWorker(context.Context, Identifier) (Export, error)
	ListExports(context.Context, auth.Identifier, ListOptions) (ExportPage, error)
	BeginExport(context.Context, Identifier, time.Time) (Export, error)
	ExportDataset(context.Context, Module) (ExportDataset, error)
	CompleteExport(context.Context, Identifier, int, int64, [32]byte, time.Time) (Export, error)
	FailExport(context.Context, Identifier, ExportState, string, time.Time) (bool, error)
	ClaimCleanupCandidates(context.Context, time.Time, int) ([]CleanupCandidate, error)
	MarkObjectDeleted(context.Context, CleanupCandidate, time.Time) error
	ReleaseCleanupCandidate(context.Context, CleanupCandidate) error
	BulkDelete(context.Context, auth.Identifier, Module, []BulkItem, string, time.Time) (int, error)
	CustomFields(context.Context, Module) ([]CustomFieldDefinition, error)
	CanonicalValues(context.Context, Module, Identifier) (map[string]string, error)
	FindProfileIDsByCPF(context.Context, string) ([]Identifier, error)
	DocumentType(context.Context, document.Identifier) (document.TypeDefinition, error)
	BillType(context.Context, bill.Identifier) (bill.TypeDefinition, error)
	GetActor(context.Context, auth.Identifier) (auth.Session, error)
	RecordAuditEvent(context.Context, AuditEvent) error
}

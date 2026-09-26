package queryengine

import (
	"context"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type DynamicEntityDefinition struct {
	ID                 string
	TechnicalKey       string
	Label              string
	ProfileCardinality string
}

type DynamicFieldDefinition struct {
	ID                 string
	TargetKind         string
	DocumentTypeID     string
	BillTypeID         string
	CustomEntityTypeID string
	TechnicalKey       string
	Label              string
	FieldKind          string
	Options            []OptionDefinition
}

type CatalogDefinitions struct {
	Entities []DynamicEntityDefinition
	Fields   []DynamicFieldDefinition
}

type CompiledPlan struct {
	SQL            string
	Arguments      []any
	Columns        []ResultColumn
	Fingerprint    [32]byte
	CatalogVersion string
	RootEntity     string
	EntityKind     string
	MaximumRows    int
	Cost           int
}

type RawResultRow struct {
	EntityKind  string
	EntityID    string
	EntityLabel string
	UpdatedAt   time.Time
	Values      []*string
}

type ExecutionInput struct {
	ID              Identifier
	OwnerUserID     auth.Identifier
	IdempotencyKey  string
	PlanFingerprint [32]byte
	CatalogVersion  string
	RootEntity      string
	MaximumRows     int
	StartedAt       time.Time
	ExpiresAt       time.Time
}

type Store interface {
	CurrentUser(context.Context, auth.Identifier) (auth.User, error)
	CatalogDefinitions(context.Context) (CatalogDefinitions, error)
	CreateExecution(context.Context, ExecutionInput, time.Time, int) (Execution, bool, error)
	ExecuteReadOnly(context.Context, CompiledPlan, time.Duration) ([]RawResultRow, error)
	CountReadOnly(context.Context, string, []any, time.Duration) (int64, error)
	ScanTexts(context.Context, string, []any, int, int, time.Duration) ([][]string, error)
	CompleteExecution(context.Context, Identifier, []ResultColumn, []ResultRow, time.Time) (Execution, error)
	FailExecution(context.Context, Identifier, string, ExecutionState, time.Time) error
	GetExecution(context.Context, Identifier, auth.Identifier) (Execution, error)
	GetResultPage(context.Context, Identifier, auth.Identifier, int, int) (ResultPage, error)
	DeleteExpired(context.Context, time.Time, int) (int, error)
	SaveAudit(context.Context, AuditEvent) error
}

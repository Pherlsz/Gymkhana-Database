package queryengine

import (
	"crypto/rand"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/google/uuid"
)

const (
	PlanVersionV1          = "v1"
	MaximumProjections     = 20
	MaximumFilterNodes     = 40
	MaximumFilterDepth     = 6
	MaximumRelationDepth   = 3
	MaximumPredicateValues = 25
	MaximumSortFields      = 3
	MaximumRows            = 500
	MaximumPageSize        = 100
	MaximumIdempotencySize = 128
	MinimumIdempotencySize = 8
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

type ValueKind string

const (
	ValueText       ValueKind = "text"
	ValueLongText   ValueKind = "long_text"
	ValueIdentifier ValueKind = "identifier"
	ValueInteger    ValueKind = "integer"
	ValueDecimal    ValueKind = "decimal"
	ValueBoolean    ValueKind = "boolean"
	ValueCivilDate  ValueKind = "civil_date"
	ValueCivilMonth ValueKind = "civil_month"
	ValueTimestamp  ValueKind = "timestamp"
	ValueEnum       ValueKind = "enum"
)

func (kind ValueKind) Valid() bool {
	switch kind {
	case ValueText, ValueLongText, ValueIdentifier, ValueInteger, ValueDecimal,
		ValueBoolean, ValueCivilDate, ValueCivilMonth, ValueTimestamp, ValueEnum:
		return true
	default:
		return false
	}
}

type Operator string

const (
	OperatorEqual      Operator = "eq"
	OperatorNotEqual   Operator = "neq"
	OperatorContains   Operator = "contains"
	OperatorStartsWith Operator = "starts_with"
	OperatorGreater    Operator = "gt"
	OperatorGreaterEq  Operator = "gte"
	OperatorLess       Operator = "lt"
	OperatorLessEq     Operator = "lte"
	OperatorBetween    Operator = "between"
	OperatorIn         Operator = "in"
	OperatorIsNull     Operator = "is_null"
	OperatorNotNull    Operator = "not_null"
)

type EntityDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Navigable   bool   `json:"navigable"`
	DefaultSort string `json:"default_sort"`
}

type OptionDefinition struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type FieldDefinition struct {
	Key         string             `json:"key"`
	Entity      string             `json:"entity"`
	Label       string             `json:"label"`
	Kind        ValueKind          `json:"kind"`
	Nullable    bool               `json:"nullable"`
	Projectable bool               `json:"projectable"`
	Filterable  bool               `json:"filterable"`
	Sortable    bool               `json:"sortable"`
	Operators   []Operator         `json:"operators"`
	Options     []OptionDefinition `json:"options,omitempty"`
}

type RelationCardinality string

const (
	CardinalityOne  RelationCardinality = "ONE"
	CardinalityMany RelationCardinality = "MANY"
)

type RelationDefinition struct {
	Key         string              `json:"key"`
	FromEntity  string              `json:"from_entity"`
	ToEntity    string              `json:"to_entity"`
	Label       string              `json:"label"`
	Cardinality RelationCardinality `json:"cardinality"`
}

type OperatorDefinition struct {
	Key           Operator `json:"key"`
	Label         string   `json:"label"`
	MinimumValues int      `json:"minimum_values"`
	MaximumValues int      `json:"maximum_values"`
}

type CatalogLimits struct {
	MaximumProjections     int `json:"maximum_projections"`
	MaximumFilterNodes     int `json:"maximum_filter_nodes"`
	MaximumFilterDepth     int `json:"maximum_filter_depth"`
	MaximumRelationDepth   int `json:"maximum_relation_depth"`
	MaximumPredicateValues int `json:"maximum_predicate_values"`
	MaximumSortFields      int `json:"maximum_sort_fields"`
	MaximumRows            int `json:"maximum_rows"`
	MaximumPageSize        int `json:"maximum_page_size"`
}

type Catalog struct {
	Version   string               `json:"version"`
	Entities  []EntityDefinition   `json:"entities"`
	Fields    []FieldDefinition    `json:"fields"`
	Relations []RelationDefinition `json:"relations"`
	Operators []OperatorDefinition `json:"operators"`
	Limits    CatalogLimits        `json:"limits"`
}

type FilterKind string

const (
	FilterPredicate FilterKind = "predicate"
	FilterGroup     FilterKind = "group"
	FilterRelation  FilterKind = "relation"
	FilterNot       FilterKind = "not"
)

type Conjunction string

const (
	ConjunctionAnd Conjunction = "AND"
	ConjunctionOr  Conjunction = "OR"
)

type FilterNode struct {
	Kind        FilterKind   `json:"kind"`
	Conjunction Conjunction  `json:"conjunction,omitempty"`
	Field       string       `json:"field,omitempty"`
	Operator    Operator     `json:"operator,omitempty"`
	Values      []string     `json:"values,omitempty"`
	Relation    string       `json:"relation,omitempty"`
	Children    []FilterNode `json:"children,omitempty"`
}

type SortDirection string

const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

type Sort struct {
	Field     string        `json:"field"`
	Direction SortDirection `json:"direction"`
}

type QueryPlan struct {
	Version        string      `json:"version"`
	CatalogVersion string      `json:"catalog_version"`
	RootEntity     string      `json:"root_entity"`
	Projections    []string    `json:"projections"`
	Filter         *FilterNode `json:"filter,omitempty"`
	Sort           []Sort      `json:"sort,omitempty"`
	MaximumRows    int         `json:"maximum_rows"`
}

type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

type ValidationError struct {
	Fields []FieldError `json:"fields"`
}

func (err *ValidationError) Error() string { return "query plan validation failed" }

func (err *ValidationError) add(field, code string) {
	err.Fields = append(err.Fields, FieldError{Field: field, Code: code})
}

func (err *ValidationError) empty() bool { return len(err.Fields) == 0 }

type PlanEstimate struct {
	Valid       bool           `json:"valid"`
	Fingerprint string         `json:"fingerprint,omitempty"`
	Cost        int            `json:"cost,omitempty"`
	Columns     []ResultColumn `json:"columns,omitempty"`
}

type ExecutionState string

const (
	ExecutionRunning   ExecutionState = "RUNNING"
	ExecutionCompleted ExecutionState = "COMPLETED"
	ExecutionFailed    ExecutionState = "FAILED"
	ExecutionCancelled ExecutionState = "CANCELLED"
)

type Execution struct {
	ID              Identifier      `json:"id"`
	OwnerUserID     auth.Identifier `json:"-"`
	State           ExecutionState  `json:"state"`
	IdempotencyKey  string          `json:"-"`
	PlanFingerprint [32]byte        `json:"-"`
	CatalogVersion  string          `json:"catalog_version"`
	RootEntity      string          `json:"root_entity"`
	MaximumRows     int             `json:"maximum_rows"`
	RowCount        int             `json:"row_count"`
	ColumnCount     int             `json:"column_count"`
	ErrorCode       string          `json:"error_code,omitempty"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	ExpiresAt       time.Time       `json:"expires_at"`
	Version         int64           `json:"version"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type ResultColumn struct {
	Position int       `json:"position"`
	FieldKey string    `json:"field_key"`
	Label    string    `json:"label"`
	Kind     ValueKind `json:"kind"`
}

type ResultCell struct {
	ColumnPosition int        `json:"column_position"`
	Kind           ValueKind  `json:"kind"`
	IsNull         bool       `json:"is_null"`
	TextValue      *string    `json:"text_value,omitempty"`
	IntegerValue   *int64     `json:"integer_value,omitempty"`
	DecimalValue   *string    `json:"decimal_value,omitempty"`
	BooleanValue   *bool      `json:"boolean_value,omitempty"`
	CivilDateValue *string    `json:"civil_date_value,omitempty"`
	TimestampValue *time.Time `json:"timestamp_value,omitempty"`
}

type ResultRow struct {
	Position    int          `json:"position"`
	EntityKind  string       `json:"entity_kind"`
	EntityID    string       `json:"entity_id"`
	EntityLabel string       `json:"entity_label"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Cells       []ResultCell `json:"cells"`
}

type ResultPage struct {
	Execution Execution      `json:"execution"`
	Columns   []ResultColumn `json:"columns"`
	Rows      []ResultRow    `json:"rows"`
	Total     int            `json:"total"`
	Limit     int            `json:"limit"`
	Offset    int            `json:"offset"`
}

type AuditEventType string

const (
	AuditCatalogRead       AuditEventType = "CATALOG_READ"
	AuditPlanValidated     AuditEventType = "PLAN_VALIDATED"
	AuditExecutionStarted  AuditEventType = "EXECUTION_STARTED"
	AuditExecutionComplete AuditEventType = "EXECUTION_COMPLETED"
	AuditExecutionFailed   AuditEventType = "EXECUTION_FAILED"
	AuditResultRead        AuditEventType = "RESULT_READ"
	AuditResultExpired     AuditEventType = "RESULT_EXPIRED"
)

type AuditEvent struct {
	ID            Identifier
	ActorUserID   *auth.Identifier
	ExecutionID   *Identifier
	EventType     AuditEventType
	Outcome       string
	AffectedCount *int
	RequestID     string
	CreatedAt     time.Time
}

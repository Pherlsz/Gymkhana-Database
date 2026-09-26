package queryengine

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/google/uuid"
)

const (
	PlanVersionV1 = "v1"
	PlanVersionV2 = "v2"

	MaximumProjections     = 20
	MaximumFilterNodes     = 40
	MaximumFilterDepth     = 6
	MaximumRelationDepth   = 3
	MaximumPredicateValues = 25
	MaximumSortFields      = 3
	MaximumRows            = 500
	MaximumPageSize        = 100
	// MaximumSequenceScan is the ceiling for an alphabet-chain read.
	// ponytail: one indexed scan per letter if a filtered type exceeds this.
	MaximumSequenceScan    = 4000
	MaximumIdempotencySize = 128
	MinimumIdempotencySize = 8

	MaximumGroupKeys             = 8
	MaximumAggregates            = 12
	MaximumAggregateFilterNodes  = 24
	MaximumSetInputs             = 6
	MaximumSetDepth              = 4
	MaximumPatternLength         = 160
	MaximumPatternTokens         = 32
	MaximumCombinationDimensions = 8
	MaximumCombinationSize       = 10_000
	// MaximumShownValues caps one cell that lists formed or matched values.
	MaximumShownValues = 256
	MaximumShownRunes  = 500
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
	Source      string             `json:"source,omitempty"`
	Replaces    bool               `json:"replaces,omitempty"`
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

type AggregateFunction string

const (
	AggregateCount   AggregateFunction = "count"
	AggregateSum     AggregateFunction = "sum"
	AggregateAverage AggregateFunction = "average"
	AggregateMinimum AggregateFunction = "minimum"
	AggregateMaximum AggregateFunction = "maximum"
)

type AggregateFunctionDefinition struct {
	Key                AggregateFunction `json:"key"`
	Label              string            `json:"label"`
	InputKinds         []ValueKind       `json:"input_kinds,omitempty"`
	OutputKind         ValueKind         `json:"output_kind"`
	AllowsDistinct     bool              `json:"allows_distinct"`
	AllowsNullInput    bool              `json:"allows_null_input"`
	RequiresInputField bool              `json:"requires_input_field"`
}

type SetOperator string

const (
	SetUnion        SetOperator = "union"
	SetIntersection SetOperator = "intersection"
	SetDifference   SetOperator = "difference"
)

type SetOperatorDefinition struct {
	Key           SetOperator `json:"key"`
	Label         string      `json:"label"`
	MinimumInputs int         `json:"minimum_inputs"`
	MaximumInputs int         `json:"maximum_inputs"`
}

type PatternGrammar string

const (
	PatternLiteralSequence PatternGrammar = "literal_sequence"
	PatternCharacterClass  PatternGrammar = "character_class"
	PatternBinaryDigits    PatternGrammar = "binary_digits"
	PatternDigits          PatternGrammar = "digits"
	PatternLetters         PatternGrammar = "letters"
	PatternAlphaNumeric    PatternGrammar = "alphanumeric"
)

type PatternGrammarDefinition struct {
	Key               PatternGrammar `json:"key"`
	Label             string         `json:"label"`
	MaximumLength     int            `json:"maximum_length"`
	MaximumTokens     int            `json:"maximum_tokens"`
	SupportsAnchoring bool           `json:"supports_anchoring"`
	SupportsCaseFold  bool           `json:"supports_case_fold"`
}

type FieldCapability struct {
	Field              string              `json:"field"`
	Groupable          bool                `json:"groupable"`
	AggregateFunctions []AggregateFunction `json:"aggregate_functions,omitempty"`
	PatternGrammars    []PatternGrammar    `json:"pattern_grammars,omitempty"`
}

type AdvancedCatalogLimits struct {
	MaximumGroupKeys             int `json:"maximum_group_keys"`
	MaximumAggregates            int `json:"maximum_aggregates"`
	MaximumAggregateFilterNodes  int `json:"maximum_aggregate_filter_nodes"`
	MaximumSetInputs             int `json:"maximum_set_inputs"`
	MaximumSetDepth              int `json:"maximum_set_depth"`
	MaximumPatternLength         int `json:"maximum_pattern_length"`
	MaximumPatternTokens         int `json:"maximum_pattern_tokens"`
	MaximumCombinationDimensions int `json:"maximum_combination_dimensions"`
	MaximumCombinationSize       int `json:"maximum_combination_size"`
}

type AdvancedCatalog struct {
	FieldCapabilities  []FieldCapability             `json:"field_capabilities"`
	AggregateFunctions []AggregateFunctionDefinition `json:"aggregate_functions"`
	SetOperators       []SetOperatorDefinition       `json:"set_operators"`
	PatternGrammars    []PatternGrammarDefinition    `json:"pattern_grammars"`
	Limits             AdvancedCatalogLimits         `json:"limits"`
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
	Advanced  *AdvancedCatalog     `json:"advanced,omitempty"`
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
	OtherField  string       `json:"other_field,omitempty"`
	Operator    Operator     `json:"operator,omitempty"`
	Values      []string     `json:"values,omitempty"`
	Relation    string       `json:"relation,omitempty"`
	Children    []FilterNode `json:"children,omitempty"`
}

func (node *FilterNode) UnmarshalJSON(data []byte) error {
	type rawFilterNode struct {
		Kind        FilterKind      `json:"kind"`
		Conjunction Conjunction     `json:"conjunction,omitempty"`
		Field       string          `json:"field,omitempty"`
		OtherField  string          `json:"other_field,omitempty"`
		Operator    Operator        `json:"operator,omitempty"`
		Values      json.RawMessage `json:"values,omitempty"`
		Relation    string          `json:"relation,omitempty"`
		Children    json.RawMessage `json:"children,omitempty"`
	}
	var raw rawFilterNode
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	node.Kind = raw.Kind
	node.Conjunction = raw.Conjunction
	node.Field = raw.Field
	node.OtherField = raw.OtherField
	node.Operator = raw.Operator
	node.Relation = raw.Relation

	if len(raw.Values) > 0 && string(raw.Values) != "null" {
		var stringValues []string
		if err := json.Unmarshal(raw.Values, &stringValues); err == nil {
			node.Values = stringValues
		} else {
			var anyValues []any
			if err := json.Unmarshal(raw.Values, &anyValues); err == nil {
				node.Values = make([]string, 0, len(anyValues))
				for _, v := range anyValues {
					if v != nil {
						node.Values = append(node.Values, fmt.Sprint(v))
					}
				}
			}
		}
	}

	if len(raw.Children) > 0 && string(raw.Children) != "null" {
		var childNodes []FilterNode
		if err := json.Unmarshal(raw.Children, &childNodes); err == nil {
			node.Children = childNodes
		} else {
			var rawStrings []string
			if err := json.Unmarshal(raw.Children, &rawStrings); err == nil && len(node.Values) == 0 {
				node.Values = rawStrings
			} else {
				var rawAnys []any
				if err := json.Unmarshal(raw.Children, &rawAnys); err == nil && len(node.Values) == 0 {
					node.Values = make([]string, 0, len(rawAnys))
					for _, v := range rawAnys {
						if v != nil {
							node.Values = append(node.Values, fmt.Sprint(v))
						}
					}
				} else {
					return err
				}
			}
		}
	}

	if node.Kind == "" {
		switch {
		case node.Field != "" || node.Operator != "":
			node.Kind = FilterPredicate
		case node.Relation != "":
			node.Kind = FilterRelation
		case node.Conjunction != "" || len(node.Children) > 0:
			node.Kind = FilterGroup
		}
	}

	return nil
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

type Aggregate struct {
	Key      string            `json:"key"`
	Function AggregateFunction `json:"function"`
	Field    string            `json:"field,omitempty"`
	Distinct bool              `json:"distinct,omitempty"`
}

type AggregateReference struct {
	Aggregate string   `json:"aggregate"`
	Operator  Operator `json:"operator"`
	Values    []string `json:"values,omitempty"`
}

type AggregateFilterNode struct {
	Conjunction Conjunction           `json:"conjunction,omitempty"`
	Predicate   *AggregateReference   `json:"predicate,omitempty"`
	Children    []AggregateFilterNode `json:"children,omitempty"`
	Negated     bool                  `json:"negated,omitempty"`
}

type PatternPredicate struct {
	Field      string         `json:"field"`
	Grammar    PatternGrammar `json:"grammar"`
	Pattern    string         `json:"pattern"`
	Anchored   bool           `json:"anchored,omitempty"`
	CaseFold   bool           `json:"case_fold,omitempty"`
	AllowEmpty bool           `json:"allow_empty,omitempty"`
}

type SetExpression struct {
	Operator SetOperator     `json:"operator"`
	Inputs   []SetExpression `json:"inputs,omitempty"`
	Plan     *QueryPlan      `json:"plan,omitempty"`
}

type CombinationInput struct {
	Key             string     `json:"key"`
	Plan            *QueryPlan `json:"plan"`
	MinimumSelected int        `json:"minimum_selected"`
	MaximumSelected int        `json:"maximum_selected"`
}

type CombinationSpec struct {
	Inputs              []CombinationInput   `json:"inputs"`
	MaximumCombinations int                  `json:"maximum_combinations"`
	RequireDistinctRows bool                 `json:"require_distinct_rows"`
	Aggregates          []Aggregate          `json:"aggregates,omitempty"`
	Having              *AggregateFilterNode `json:"having,omitempty"`
}

// Derivation is a pure column the model composes from a field or an earlier column.
type Derivation struct {
	As      string `json:"as"`
	Op      string `json:"op"`
	From    string `json:"from"`
	Class   string `json:"class,omitempty"`
	Index   *int   `json:"index,omitempty"`
	Pattern string `json:"pattern,omitempty"`
	Which   string `json:"which,omitempty"`
}

// MatchSpec keeps or rearranges symbols and records every value that fit.
type MatchSpec struct {
	On         string         `json:"on"`
	Grammar    PatternGrammar `json:"grammar,omitempty"`
	Pattern    string         `json:"pattern,omitempty"`
	Order      string         `json:"order,omitempty"`
	Width      int            `json:"width,omitempty"`
	Alphabet   string         `json:"alphabet,omitempty"`
	Anchored   bool           `json:"anchored,omitempty"`
	Equals     []string       `json:"equals,omitempty"`
	Quantifier string         `json:"quantifier,omitempty"`
	Interpret  string         `json:"interpret,omitempty"`
	Show       string         `json:"show,omitempty"`
}

type SequenceAlong struct {
	Field string `json:"field"`
	Step  string `json:"step"`
}

// SequenceSpec is a generic chain: alphabet, step, and an optional second column.
type SequenceSpec struct {
	By        string         `json:"by"`
	Alphabet  string         `json:"alphabet,omitempty"`
	Step      string         `json:"step"`
	Along     *SequenceAlong `json:"along,omitempty"`
	OnePer    string         `json:"one_per,omitempty"`
	Partition string         `json:"partition,omitempty"`
	Pick      string         `json:"pick,omitempty"`
	Tie       string         `json:"tie,omitempty"`
	Minimum   int            `json:"minimum,omitempty"`
	Distinct  string         `json:"distinct,omitempty"`
}

// QueryPlan is the single versioned public query contract. Version v1 uses the
// legacy projection/filter/sort subset. Version v2 enables the advanced fields.
type QueryPlan struct {
	Version        string      `json:"version"`
	CatalogVersion string      `json:"catalog_version"`
	RootEntity     string      `json:"root_entity"`
	Projections    []string    `json:"projections"`
	Filter         *FilterNode `json:"filter,omitempty"`
	Sort           []Sort      `json:"sort,omitempty"`
	MaximumRows    int         `json:"maximum_rows"`

	GroupBy     []string             `json:"group_by,omitempty"`
	Aggregates  []Aggregate          `json:"aggregates,omitempty"`
	Having      *AggregateFilterNode `json:"having,omitempty"`
	Patterns    []PatternPredicate   `json:"patterns,omitempty"`
	Set         *SetExpression       `json:"set,omitempty"`
	Combination *CombinationSpec     `json:"combination,omitempty"`
	Derive      []Derivation         `json:"derive,omitempty"`
	Matches     []MatchSpec          `json:"matches,omitempty"`
	Sequence    *SequenceSpec        `json:"sequence,omitempty"`
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

type TextScanRow struct {
	ID     string
	Label  string
	Values []string
}

type TextScan struct {
	Rows      []TextScanRow
	Truncated bool
}

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
	Position     int                `json:"position"`
	FieldKey     string             `json:"field_key"`
	Label        string             `json:"label"`
	Kind         ValueKind          `json:"kind"`
	AggregateKey string             `json:"aggregate_key,omitempty"`
	Lineage      []ResultLineageRef `json:"lineage,omitempty"`
}

type ResultLineageRef struct {
	Entity   string `json:"entity"`
	Field    string `json:"field,omitempty"`
	Relation string `json:"relation,omitempty"`
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

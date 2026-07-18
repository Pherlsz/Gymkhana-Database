package taskengine

import (
	"errors"

	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

const (
	SpecVersionV1            = "v1"
	MaximumTaskTextRunes     = 20_000
	MaximumRoles             = 12
	MaximumRequirements      = 64
	MaximumConstraints       = 64
	MaximumAmbiguities       = 32
	MaximumRoleCount         = 8
	MaximumSolutions         = 100
	DefaultCandidateRows     = 250
	MaximumCandidateRows     = 500
	MaximumRequirementValues = 25
	MaximumLogicalIdentifier = 64
	MaximumEvidencePerResult = 512
)

var (
	ErrInvalidSpec          = errors.New("invalid task specification")
	ErrUnresolved           = errors.New("task specification has unresolved ambiguities")
	ErrStaleCatalog         = errors.New("task catalog is stale")
	ErrInterpreterDisabled  = errors.New("semantic interpreter is disabled")
	ErrUnsupportedTask      = errors.New("unsupported task")
	ErrInterpreterMalformed = errors.New("semantic interpreter returned malformed output")
	ErrTaskTooLarge         = errors.New("task text is too large")
)

type SpecState string

const (
	SpecProposed SpecState = "PROPOSED"
	SpecReviewed SpecState = "REVIEWED"
)

type FieldBinding struct {
	Field        string   `json:"field"`
	RelationPath []string `json:"relation_path,omitempty"`
}

type PatternBinding struct {
	Grammar  queryengine.PatternGrammar `json:"grammar"`
	Value    string                     `json:"value"`
	Anchored bool                       `json:"anchored,omitempty"`
	CaseFold bool                       `json:"case_fold,omitempty"`
}

type Requirement struct {
	Key      string               `json:"key"`
	Role     string               `json:"role"`
	Binding  FieldBinding         `json:"binding"`
	Operator queryengine.Operator `json:"operator,omitempty"`
	Values   []string             `json:"values,omitempty"`
	Pattern  *PatternBinding      `json:"pattern,omitempty"`
	Optional bool                 `json:"optional,omitempty"`
}

type CandidateRole struct {
	Key          string `json:"key"`
	Entity       string `json:"entity"`
	MinimumCount int    `json:"minimum_count"`
	MaximumCount int    `json:"maximum_count"`
}

type ConstraintKind string

const (
	ConstraintEqual      ConstraintKind = "equal"
	ConstraintNotEqual   ConstraintKind = "not_equal"
	ConstraintMembership ConstraintKind = "membership"
	ConstraintBefore     ConstraintKind = "before"
	ConstraintAfter      ConstraintKind = "after"
	ConstraintDistinct   ConstraintKind = "distinct"
)

type ConstraintOperand struct {
	Role    string                `json:"role"`
	Binding *FieldBinding         `json:"binding,omitempty"`
	Kind    queryengine.ValueKind `json:"kind,omitempty"`
}

type Constraint struct {
	Key    string             `json:"key"`
	Kind   ConstraintKind     `json:"kind"`
	Left   ConstraintOperand  `json:"left"`
	Right  *ConstraintOperand `json:"right,omitempty"`
	Values []string           `json:"values,omitempty"`
}

type CompositionGoal struct {
	MinimumSolutions int `json:"minimum_solutions"`
	MaximumSolutions int `json:"maximum_solutions"`
}

type Ambiguity struct {
	Key        string   `json:"key"`
	Code       string   `json:"code"`
	Candidates []string `json:"candidates,omitempty"`
}

type TaskSpec struct {
	Version        string          `json:"version"`
	CatalogVersion string          `json:"catalog_version"`
	State          SpecState       `json:"state"`
	Roles          []CandidateRole `json:"roles"`
	Requirements   []Requirement   `json:"requirements"`
	Constraints    []Constraint    `json:"constraints,omitempty"`
	Goal           CompositionGoal `json:"goal"`
	Ambiguities    []Ambiguity     `json:"ambiguities,omitempty"`
}

type Proposal struct {
	Spec                TaskSpec `json:"spec"`
	Interpreter         string   `json:"interpreter"`
	RequiresHumanReview bool     `json:"requires_human_review"`
}

type Evidence struct {
	Requirement  string `json:"requirement,omitempty"`
	Constraint   string `json:"constraint,omitempty"`
	Role         string `json:"role"`
	Field        string `json:"field,omitempty"`
	SourceEntity string `json:"source_entity"`
	SourceID     string `json:"source_id"`
	Satisfied    bool   `json:"satisfied"`
	Code         string `json:"code"`
}

type Candidate struct {
	Entity   string            `json:"entity"`
	ID       string            `json:"id"`
	Label    string            `json:"label"`
	Values   map[string]string `json:"values,omitempty"`
	Evidence []Evidence        `json:"evidence"`
}

type CandidateSet struct {
	Role       string      `json:"role"`
	Candidates []Candidate `json:"candidates"`
}

type Composition struct {
	Position int                    `json:"position"`
	Selected map[string][]Candidate `json:"selected"`
	Evidence []Evidence             `json:"evidence"`
}

type SolveState string

const (
	SolveComplete   SolveState = "COMPLETE"
	SolveNoSolution SolveState = "NO_SOLUTION"
	SolveIncomplete SolveState = "INCOMPLETE"
	SolveCancelled  SolveState = "CANCELLED"
)

type SolveLimits struct {
	MaximumBranches          int
	MaximumCompositions      int
	MaximumCandidatesPerRole int
	MaximumEvidence          int
	MaximumBytes             int
}

type SolveResult struct {
	State        SolveState    `json:"state"`
	Compositions []Composition `json:"compositions"`
	Branches     int           `json:"branches"`
	Rejected     int           `json:"rejected"`
	LimitCode    string        `json:"limit_code,omitempty"`
}

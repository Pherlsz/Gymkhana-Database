package search

import (
	"errors"
	"time"
)

const (
	MaxTerms             = 16
	MaxTermLength        = 128
	MaxQueryLength       = 512
	MaxFields            = 50
	MaxPageSize          = 100
	MaxOffset            = 10_000
	MaxResultCardinality = 50_000
	MaxSuggest           = 50
)

var (
	ErrForbidden           = errors.New("search is forbidden")
	ErrInvalidQuery        = errors.New("search query is invalid")
	ErrRateLimited         = errors.New("search rate limit exceeded")
	ErrCostLimit           = errors.New("search cost limit exceeded")
	ErrCardinalityLimit    = errors.New("search result cardinality limit exceeded")
	ErrQueryTimeout        = errors.New("search query timed out")
	ErrUnsafeResult        = errors.New("search result violated the authorized catalog")
	ErrInvalidServiceSetup = errors.New("search service setup is invalid")
)

type Module string

const (
	ModuleProfiles    Module = "profiles"
	ModuleDocuments   Module = "documents"
	ModuleBills       Module = "bills"
	ModuleCustomData  Module = "custom_data"
	ModuleAttachments Module = "attachments"
)

func (module Module) Valid() bool {
	switch module {
	case ModuleProfiles, ModuleDocuments, ModuleBills, ModuleCustomData, ModuleAttachments:
		return true
	default:
		return false
	}
}

type SortField string

const (
	SortRelevance SortField = "relevance"
	SortUpdatedAt SortField = "updated_at"
)

func (field SortField) Valid() bool {
	return field == SortRelevance || field == SortUpdatedAt
}

type SortOrder string

const (
	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

func (order SortOrder) Valid() bool {
	return order == SortAscending || order == SortDescending
}

type ModuleDefinition struct {
	Key   Module
	Label string
}

type FieldDefinition struct {
	Key    string
	Module Module
	// Group is the UI bucket (pessoa/documento/conta). Custom fields keep
	// Module=custom_data for search authorization but Group follows the owner.
	Group Module
	Label string
	Kind  string
}

type CatalogLimits struct {
	MaximumTerms             int
	MaximumTermLength        int
	MaximumFields            int
	MaximumPageSize          int32
	MaximumOffset            int32
	MaximumResultCardinality int64
}

type Catalog struct {
	Modules   []ModuleDefinition
	Fields    []FieldDefinition
	Operators []QueryToken
	Limits    CatalogLimits
}

type Query struct {
	Q       string
	Terms   []string
	Modules []Module
	Fields  []string
	Limit   int32
	Offset  int32
	Sort    SortField
	Order   SortOrder
}

type TermSpec struct {
	Term      string   `json:"term"`
	Pattern   string   `json:"pattern"`
	Patterns  []string `json:"patterns,omitempty"`
	FieldKeys []string `json:"field_keys"`
	Exclude   bool     `json:"exclude"`
	Compare   string   `json:"compare"`
	Value     string   `json:"value"`
	Value2    string   `json:"value2"`
}

type Plan struct {
	Terms            []string
	Includes         []TermSpec
	Excludes         []TermSpec
	Modules          []Module
	Fields           []string
	Limit            int32
	Offset           int32
	Sort             SortField
	Order            SortOrder
	StatementTimeout time.Duration
	CandidateLimit   int64
}

type SuggestQuery struct {
	Field string
	Q     string
	Limit int32
	Grain Module
}

type SuggestHit struct {
	Value string
	Label string
}

type Result struct {
	Module      Module
	EntityKind  string
	EntityID    string
	ProfileID   string
	TargetKind  string
	TargetID    string
	EntityLabel string
	FieldKey    string
	FieldLabel  string
	Preview     string
	Score       int32
	UpdatedAt   time.Time
}

type Page struct {
	Results []Result
	Total   int64
	Limit   int32
	Offset  int32
	Sort    SortField
	Order   SortOrder
}

type FieldError struct {
	Field  string
	Code   string
	Detail string
}

type ValidationError struct {
	Fields []FieldError
}

func (err *ValidationError) Error() string {
	return "search validation failed"
}

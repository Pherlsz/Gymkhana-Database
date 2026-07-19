package queryengine

import (
	"crypto/sha256"
	"fmt"
)

// ExecutionPlan is the internal, application-owned representation emitted by
// the v2 compiler. It never contains user-selected physical identifiers.
type ExecutionPlan struct {
	SQL             string
	Arguments       []any
	Columns         []ResultColumn
	Fingerprint     [32]byte
	CatalogVersion  string
	RootEntity      string
	EntityKind      string
	MaximumRows     int
	Cost            int
	GroupCount      int
	AggregateCount  int
	SetInputCount   int
	PatternCount    int
	CombinationSize int
}

type advancedCompileState struct {
	catalog    resolvedCatalog
	arguments  []any
	validation *ValidationError
}

func compileAdvancedPlan(plan QueryPlan, catalog resolvedCatalog, maximumCost int) (ExecutionPlan, QueryPlan, error) {
	if maximumCost <= 0 {
		maximumCost = defaultMaximumCost
	}
	normalized, validation := normalizeAdvancedPlan(plan, catalog)
	if !validation.empty() {
		return ExecutionPlan{}, QueryPlan{}, validation
	}
	if normalized.CatalogVersion != catalog.Public.Version {
		return ExecutionPlan{}, QueryPlan{}, ErrStaleCatalog
	}
	state := &advancedCompileState{catalog: catalog, validation: &ValidationError{}}
	var query string
	var columns []ResultColumn
	var kind string
	var setInputs int
	var combinationSize int
	var err error
	switch {
	case normalized.Set != nil:
		query, columns, kind, setInputs, err = state.compileSet(*normalized.Set, normalized.MaximumRows, 1, "set")
	case normalized.Combination != nil:
		query, columns, kind, combinationSize, err = state.compileCombination(*normalized.Combination, normalized.MaximumRows)
	default:
		query, columns, kind, err = state.compileSelect(normalized, true)
	}
	if err != nil {
		return ExecutionPlan{}, QueryPlan{}, err
	}
	if !state.validation.empty() {
		return ExecutionPlan{}, QueryPlan{}, state.validation
	}
	cost := estimateAdvancedCost(normalized, setInputs, combinationSize)
	if cost > maximumCost {
		return ExecutionPlan{}, QueryPlan{}, ErrCostLimit
	}
	encoded, err := canonicalAdvancedPlanJSON(normalized)
	if err != nil {
		return ExecutionPlan{}, QueryPlan{}, fmt.Errorf("encode advanced query plan: %w", err)
	}
	fingerprint := sha256.Sum256(encoded)
	return ExecutionPlan{
		SQL: query, Arguments: state.arguments, Columns: columns, Fingerprint: fingerprint,
		CatalogVersion: catalog.Public.Version, RootEntity: normalized.RootEntity,
		EntityKind: kind, MaximumRows: normalized.MaximumRows, Cost: cost,
		GroupCount: len(normalized.GroupBy), AggregateCount: len(normalized.Aggregates),
		SetInputCount: setInputs, PatternCount: len(normalized.Patterns), CombinationSize: combinationSize,
	}, normalized, nil
}

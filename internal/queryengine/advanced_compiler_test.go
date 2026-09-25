package queryengine

import (
	"errors"
	"strings"
	"testing"
)

func TestAdvancedCatalogDerivesCapabilitiesFromAuthorizedFields(t *testing.T) {
	catalog := advancedTestCatalog()
	capabilities := advancedCatalogFor(catalog)
	if len(capabilities.FieldCapabilities) != len(catalog.Fields) {
		t.Fatalf("field capabilities = %d, want %d", len(capabilities.FieldCapabilities), len(catalog.Fields))
	}
	var amount FieldCapability
	for _, value := range capabilities.FieldCapabilities {
		if value.Field == "bill.amount" {
			amount = value
		}
	}
	if !amount.Groupable || !containsAggregate(amount.AggregateFunctions, AggregateSum) {
		t.Fatalf("amount capability = %#v", amount)
	}
	if len(amount.PatternGrammars) != 0 {
		t.Fatalf("numeric field unexpectedly supports patterns: %#v", amount.PatternGrammars)
	}
}

func TestCompileAdvancedAggregateUsesTrustedExpressionsAndBoundValues(t *testing.T) {
	catalog := advancedTestCatalog()
	plan := QueryPlan{
		Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, RootEntity: "bills", MaximumRows: 25,
		GroupBy:    []string{"bill.currency"},
		Aggregates: []Aggregate{{Key: "total", Function: AggregateSum, Field: "bill.amount"}},
		Having:     &AggregateFilterNode{Predicate: &AggregateReference{Aggregate: "total", Operator: OperatorGreaterEq, Values: []string{"100.00"}}},
		Patterns:   []PatternPredicate{{Field: "bill.reference", Grammar: PatternDigits, Pattern: "12?", Anchored: true}},
	}
	compiled, normalized, err := compileAdvancedPlan(plan, catalog, 1_000_000)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if normalized.Version != PlanVersionV2 || len(compiled.Columns) != 2 {
		t.Fatalf("normalized/columns = %#v / %#v", normalized, compiled.Columns)
	}
	if strings.Contains(compiled.SQL, "100.00") || strings.Contains(compiled.SQL, "12?") {
		t.Fatalf("untrusted values leaked into SQL: %s", compiled.SQL)
	}
	if !strings.Contains(compiled.SQL, "GROUP BY") || !strings.Contains(compiled.SQL, "HAVING") || !strings.Contains(compiled.SQL, "sum(") {
		t.Fatalf("advanced SQL missing expected clauses: %s", compiled.SQL)
	}
	if len(compiled.Arguments) != 3 {
		t.Fatalf("arguments = %#v, want pattern, having and limit", compiled.Arguments)
	}
}

func TestCompileAdvancedUnionCanonicalizesCommutativeInputs(t *testing.T) {
	catalog := advancedTestCatalog()
	left := QueryPlan{Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, RootEntity: "bills", Projections: []string{"bill.reference"}, MaximumRows: 10}
	right := QueryPlan{Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, RootEntity: "bills", Projections: []string{"bill.reference"}, MaximumRows: 20}
	first := QueryPlan{Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, MaximumRows: 30, Set: &SetExpression{Operator: SetUnion, Inputs: []SetExpression{{Plan: &left}, {Plan: &right}}}}
	second := QueryPlan{Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, MaximumRows: 30, Set: &SetExpression{Operator: SetUnion, Inputs: []SetExpression{{Plan: &right}, {Plan: &left}}}}
	compiledFirst, _, err := compileAdvancedPlan(first, catalog, 1_000_000)
	if err != nil {
		t.Fatalf("compile first: %v", err)
	}
	compiledSecond, _, err := compileAdvancedPlan(second, catalog, 1_000_000)
	if err != nil {
		t.Fatalf("compile second: %v", err)
	}
	if compiledFirst.Fingerprint != compiledSecond.Fingerprint {
		t.Fatalf("commutative union fingerprints differ")
	}
	if compiledFirst.SetInputCount != 2 || !strings.Contains(compiledFirst.SQL, "UNION") {
		t.Fatalf("set compilation = %#v / %s", compiledFirst, compiledFirst.SQL)
	}
}

func TestCompileAdvancedRejectsPatternInjection(t *testing.T) {
	catalog := advancedTestCatalog()
	plan := QueryPlan{
		Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, RootEntity: "bills", Projections: []string{"bill.reference"}, MaximumRows: 10,
		Patterns: []PatternPredicate{{Field: "bill.reference", Grammar: PatternBinaryDigits, Pattern: "0.*1", Anchored: true}},
	}
	_, _, err := compileAdvancedPlan(plan, catalog, 1_000_000)
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
}

func TestCompileAdvancedCombinationIsBounded(t *testing.T) {
	catalog := advancedTestCatalog()
	candidate := func(rows int) *QueryPlan {
		return &QueryPlan{Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, RootEntity: "bills", Projections: []string{"bill.reference"}, MaximumRows: rows}
	}
	plan := QueryPlan{Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, MaximumRows: 100,
		Combination: &CombinationSpec{MaximumCombinations: 50, RequireDistinctRows: true, Inputs: []CombinationInput{
			{Key: "first", Plan: candidate(5), MinimumSelected: 1, MaximumSelected: 1},
			{Key: "second", Plan: candidate(5), MinimumSelected: 1, MaximumSelected: 1},
		}},
	}
	compiled, _, err := compileAdvancedPlan(plan, catalog, 10_000_000)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if compiled.CombinationSize != 25 || !strings.Contains(compiled.SQL, "CROSS JOIN") || !strings.Contains(compiled.SQL, "<>") {
		t.Fatalf("combination = %#v / %s", compiled, compiled.SQL)
	}
}

func TestCompileCombinationAggregateFollowsTheJoin(t *testing.T) {
	catalog := advancedTestCatalog()
	amount := func(rows int) *QueryPlan {
		return &QueryPlan{Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, RootEntity: "bills", Projections: []string{"bill.amount"}, MaximumRows: rows}
	}
	plan := QueryPlan{Version: PlanVersionV2, CatalogVersion: catalog.Public.Version, MaximumRows: 100,
		Combination: &CombinationSpec{
			MaximumCombinations: 50,
			RequireDistinctRows: true,
			Inputs: []CombinationInput{
				{Key: "water", Plan: amount(2), MinimumSelected: 1, MaximumSelected: 1},
				{Key: "power", Plan: amount(2), MinimumSelected: 1, MaximumSelected: 1},
				{Key: "phone", Plan: amount(2), MinimumSelected: 1, MaximumSelected: 1},
			},
			Aggregates: []Aggregate{{Key: "soma", Function: AggregateSum, Field: "*:bill.amount"}},
			Having:     &AggregateFilterNode{Predicate: &AggregateReference{Aggregate: "soma", Operator: OperatorGreaterEq, Values: []string{"100.00"}}},
		},
	}
	compiled, _, err := compileAdvancedPlan(plan, catalog, 10_000_000)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !strings.Contains(compiled.SQL, "sum(") || !strings.Contains(compiled.SQL, "HAVING") || strings.Contains(compiled.SQL, "100.00") {
		t.Fatalf("aggregate SQL = %s", compiled.SQL)
	}
}

func advancedTestCatalog() resolvedCatalog {
	catalog := resolvedCatalog{
		Public:   Catalog{Version: strings.Repeat("a", 64)},
		Entities: map[string]sqlEntityDefinition{}, Fields: map[string]sqlFieldDefinition{}, Relations: map[string]sqlRelationDefinition{},
	}
	catalog.Entities["bills"] = sqlEntityDefinition{Public: EntityDefinition{Key: "bills", Kind: "bill", DefaultSort: "bill.reference"}, FromTemplate: "bills {root}", IDExpression: "{root}.id::text", LabelExpression: "{root}.reference_value", UpdatedExpression: "{root}.updated_at"}
	catalog.Fields["bill.reference"] = sqlFieldDefinition{Public: FieldDefinition{Key: "bill.reference", Entity: "bills", Label: "Referência", Kind: ValueIdentifier, Projectable: true, Filterable: true, Sortable: true, Operators: []Operator{OperatorEqual}}, Expression: "{root}.reference_value"}
	catalog.Fields["bill.currency"] = sqlFieldDefinition{Public: FieldDefinition{Key: "bill.currency", Entity: "bills", Label: "Moeda", Kind: ValueEnum, Projectable: true, Filterable: true, Sortable: true, Operators: []Operator{OperatorEqual}}, Expression: "{root}.currency"}
	catalog.Fields["bill.amount"] = sqlFieldDefinition{Public: FieldDefinition{Key: "bill.amount", Entity: "bills", Label: "Valor", Kind: ValueDecimal, Projectable: true, Filterable: true, Sortable: true, Operators: []Operator{OperatorGreaterEq}}, Expression: "{root}.amount"}
	return catalog
}

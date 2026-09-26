package queryengine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	testEntityID = "11111111-1111-4111-8111-111111111111"
	testFieldID  = "22222222-2222-4222-8222-222222222222"
)

func TestCatalogIsDeterministicPermissionFilteredAndLogical(t *testing.T) {
	store := newFakeQueryStore()
	store.definitions = CatalogDefinitions{
		Entities: []DynamicEntityDefinition{
			{ID: testEntityID, TechnicalKey: "membership", Label: "Vínculos", ProfileCardinality: "MANY_PER_PROFILE"},
			{ID: "not-a-uuid'; DROP TABLE profiles;--", TechnicalKey: "invalid", Label: "Inválido"},
		},
		Fields: []DynamicFieldDefinition{
			{ID: testFieldID, TargetKind: "PROFILE", TechnicalKey: "status", Label: "Situação", FieldKind: "SINGLE_SELECT",
				Options: []OptionDefinition{{Key: "active", Label: "Ativo"}, {Key: "inactive", Label: "Inativo"}}},
		},
	}
	first, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	second, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("second loadCatalog() error = %v", err)
	}
	if first.Public.Version == "" || first.Public.Version != second.Public.Version || len(first.Public.Version) != 64 {
		t.Fatalf("catalog versions are not deterministic: %q and %q", first.Public.Version, second.Public.Version)
	}
	if _, ok := first.Entities["custom_entity."+testEntityID]; !ok {
		t.Fatal("active custom entity is absent from catalog")
	}
	if len(first.Entities) != 5 {
		t.Fatalf("catalog entity count = %d, want 5", len(first.Entities))
	}
	field, ok := first.Fields["custom."+testFieldID]
	if !ok || field.Public.Entity != "profiles" || field.Public.Kind != ValueEnum || len(field.Public.Options) != 2 {
		t.Fatalf("dynamic catalog field = %#v", field.Public)
	}
	if strings.Contains(field.Expression, "Ativo") || !strings.Contains(field.Expression, "technical_key") {
		t.Fatalf("dynamic select expression must use technical keys: %s", field.Expression)
	}
	for index := 1; index < len(first.Public.Fields); index++ {
		left, right := first.Public.Fields[index-1], first.Public.Fields[index]
		if left.Entity > right.Entity || left.Entity == right.Entity && left.Label > right.Label {
			t.Fatalf("public fields are not stable at %d: %#v then %#v", index, left, right)
		}
	}
	if _, err := loadCatalog(context.Background(), store, auth.Role("UNKNOWN")); !errors.Is(err, ErrForbidden) {
		t.Fatalf("loadCatalog(unknown role) error = %v", err)
	}
	for _, role := range []auth.Role{auth.RoleExternal, auth.RoleAdmin, auth.RoleSuperadmin} {
		permitted, err := loadCatalog(context.Background(), store, role)
		if err != nil || len(permitted.Public.Entities) != 5 || len(permitted.Public.Relations) == 0 {
			t.Fatalf("loadCatalog(%s) = %d entities/%d relations, error=%v", role, len(permitted.Public.Entities), len(permitted.Public.Relations), err)
		}
	}
	encoded, err := json.Marshal(first.Public)
	if err != nil || strings.Contains(string(encoded), "custom_field_values") || strings.Contains(string(encoded), "JOIN ") || strings.Contains(string(encoded), "technical_key") {
		t.Fatalf("serialized catalog leaked physical metadata: %s, error=%v", encoded, err)
	}
	fullName := first.Fields["profile.full_name"].Public
	if containsOperator(fullName.Operators, OperatorIsNull) || containsOperator(fullName.Operators, OperatorNotNull) {
		t.Fatalf("non-null field exposes null operators: %#v", fullName.Operators)
	}
	store.definitions = CatalogDefinitions{}
	changed, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil || changed.Public.Version == first.Public.Version {
		t.Fatalf("dynamic catalog did not refresh: %q / %q, error=%v", first.Public.Version, changed.Public.Version, err)
	}
	if _, exists := changed.Entities["custom_entity."+testEntityID]; exists {
		t.Fatal("removed dynamic definition remained in catalog")
	}
}

func TestCompilerBuildsParameterizedNestedPlanWithoutMutatingInput(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	injected := "  Ana%' OR true --  "
	plan := QueryPlan{
		Version:        PlanVersionV1,
		CatalogVersion: catalog.Public.Version,
		RootEntity:     "profiles",
		Projections:    []string{"profile.full_name", "profile.email"},
		MaximumRows:    25,
		Filter: &FilterNode{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
			{Kind: FilterPredicate, Field: "profile.full_name", Operator: OperatorContains, Values: []string{injected}},
			{Kind: FilterRelation, Relation: "profile.documents", Children: []FilterNode{{Kind: FilterPredicate, Field: "document.date", Operator: OperatorGreaterEq, Values: []string{"2025-01-01"}}}},
		}},
		Sort: []Sort{{Field: "profile.full_name", Direction: SortDescending}},
	}
	compiled, normalized, err := compilePlan(plan, catalog, defaultMaximumCost)
	if err != nil {
		t.Fatalf("compilePlan() error = %v", err)
	}
	if !strings.Contains(compiled.SQL, "EXISTS (SELECT 1 FROM documents q1") || !strings.Contains(compiled.SQL, "$1::text") ||
		!strings.Contains(compiled.SQL, "LIMIT $3::integer") || strings.Contains(compiled.SQL, "OR true") {
		t.Fatalf("compiled SQL is not safely parameterized:\n%s", compiled.SQL)
	}
	if !strings.Contains(compiled.SQL, "q0.full_name COLLATE gymkhana_pt_br DESC NULLS LAST") {
		t.Fatalf("compiled SQL does not use Portuguese name collation:\n%s", compiled.SQL)
	}
	if got := compiled.Arguments[0]; got != "%ana\\%' or true --%" {
		t.Fatalf("literal argument = %#v", got)
	}
	if plan.Filter.Children[0].Values[0] != injected {
		t.Fatalf("compilePlan mutated caller input to %q", plan.Filter.Children[0].Values[0])
	}
	if normalized.Version != PlanVersionV1 || normalized.Filter.Children[0].Values[0] != "Ana%' OR true --" ||
		compiled.EntityKind != "profile" || len(compiled.Columns) != 2 || compiled.Cost <= 0 {
		t.Fatalf("normalized/compiled plan = %#v / %#v", normalized, compiled)
	}
}

func TestMatchCountCoversTheFilterWithoutARowLimit(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	plan := QueryPlan{
		Version: PlanVersionV1, CatalogVersion: catalog.Public.Version, RootEntity: "profiles",
		Projections: []string{"profile.full_name"}, MaximumRows: 25,
		Filter: &FilterNode{Kind: FilterPredicate, Field: "profile.full_name", Operator: OperatorContains, Values: []string{"Pedro%' OR true"}},
	}
	query, arguments, err := compileMatchCount(plan, catalog)
	if err != nil || strings.Contains(query, "LIMIT") || strings.Contains(query, "OR true") || !strings.Contains(query, "count(*)::bigint") {
		t.Fatalf("compileMatchCount() = %q args=%v error=%v", query, arguments, err)
	}
	if len(arguments) != 1 || arguments[0] != "%pedro\\%' or true%" {
		t.Fatalf("count arguments = %#v", arguments)
	}
}

func TestFieldScanKeepsTheFilterBoundAndCapsTheRead(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	plan := QueryPlan{
		Version: PlanVersionV1, CatalogVersion: catalog.Public.Version, RootEntity: "bills",
		Projections: []string{"bill.printed_holder_name", "bill.printed_address"}, MaximumRows: 10,
		Filter: &FilterNode{Kind: FilterPredicate, Field: "bill.medium", Operator: OperatorEqual, Values: []string{"PHYSICAL'; DROP TABLE bills"}},
	}
	query, arguments, err := compileFieldScan(plan, catalog, MaximumSequenceScan)
	if err != nil || !strings.HasPrefix(query, "SELECT ") || strings.Contains(query, ";") || strings.Contains(query, "DROP TABLE") {
		t.Fatalf("compileFieldScan() = %q error=%v", query, err)
	}
	if len(arguments) != 2 || arguments[0] != "PHYSICAL'; DROP TABLE bills" || arguments[1] != MaximumSequenceScan {
		t.Fatalf("scan arguments = %#v", arguments)
	}
	if _, _, err := compileFieldScan(plan, catalog, MaximumSequenceScan+1); !errors.Is(err, ErrInvalidPlan) {
		t.Fatalf("compileFieldScan(over cap) error = %v", err)
	}
}

func TestStampAdvancedPlanFillsNestedCatalogVersion(t *testing.T) {
	version := strings.Repeat("ab", 32)
	plan := QueryPlan{Set: &SetExpression{Operator: SetIntersection, Inputs: []SetExpression{
		{Plan: &QueryPlan{RootEntity: "profiles", Projections: []string{"profile.full_name"}}},
		{Plan: &QueryPlan{RootEntity: "bills", Projections: []string{"bill.printed_holder_name"}}},
	}}}
	stampAdvancedPlan(&plan, version)
	if plan.Version != PlanVersionV2 || plan.CatalogVersion != version || plan.MaximumRows != MaximumPageSize {
		t.Fatalf("outer plan = %#v", plan)
	}
	for _, input := range plan.Set.Inputs {
		if input.Plan == nil || input.Plan.Version != PlanVersionV2 || input.Plan.CatalogVersion != version || input.Plan.MaximumRows != MaximumPageSize {
			t.Fatalf("nested plan = %#v", input.Plan)
		}
	}
}

func TestCompilerRejectsStaleInvalidAndOverCostPlans(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	base := QueryPlan{Version: PlanVersionV1, CatalogVersion: catalog.Public.Version, RootEntity: "bills", Projections: []string{"bill.amount"}, MaximumRows: 10}
	missingVersion := base
	missingVersion.Version = ""
	if _, _, err := compilePlan(missingVersion, catalog, defaultMaximumCost); validationCode(err, "version") != "unsupported" {
		t.Fatalf("unversioned compilePlan() error = %#v", err)
	}
	stale := base
	stale.CatalogVersion = strings.Repeat("0", 64)
	if _, _, err := compilePlan(stale, catalog, defaultMaximumCost); !errors.Is(err, ErrStaleCatalog) {
		t.Fatalf("stale compilePlan() error = %v", err)
	}
	invalid := base
	invalid.Filter = &FilterNode{Kind: FilterPredicate, Field: "bill.amount", Operator: OperatorGreater, Values: []string{"12x"}}
	if _, _, err := compilePlan(invalid, catalog, defaultMaximumCost); validationCode(err, "filter.values.0") != "invalid_type" {
		t.Fatalf("invalid decimal compilePlan() error = %#v", err)
	}
	invalidShape := base
	invalidShape.Filter = &FilterNode{Kind: FilterNot, Conjunction: ConjunctionAnd, Children: []FilterNode{{Kind: FilterPredicate, Field: "bill.amount", Operator: OperatorNotNull}}}
	if _, _, err := compilePlan(invalidShape, catalog, defaultMaximumCost); validationCode(err, "filter") != "invalid_shape" {
		t.Fatalf("invalid not shape compilePlan() error = %#v", err)
	}
	if _, _, err := compilePlan(base, catalog, 1); !errors.Is(err, ErrCostLimit) {
		t.Fatalf("over-cost compilePlan() error = %v", err)
	}
}

func validationCode(err error, field string) string {
	var validation *ValidationError
	if !errors.As(err, &validation) {
		return ""
	}
	for _, value := range validation.Fields {
		if value.Field == field {
			return value.Code
		}
	}
	return ""
}

func FuzzCompilePlanFailsClosed(f *testing.F) {
	f.Add("profile.full_name", "contains", "Ana%' OR true --")
	f.Add("profile.secret", "eq", "hidden")
	f.Add("profile.full_name; DROP TABLE profiles", "eq", "x")
	f.Fuzz(func(t *testing.T, field, operator, value string) {
		store := newFakeQueryStore()
		catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
		if err != nil {
			t.Fatalf("loadCatalog() error = %v", err)
		}
		plan := QueryPlan{
			Version: PlanVersionV1, CatalogVersion: catalog.Public.Version,
			RootEntity: "profiles", Projections: []string{"profile.full_name"}, MaximumRows: 10,
			Filter: &FilterNode{Kind: FilterPredicate, Field: field, Operator: Operator(operator), Values: []string{value}},
		}
		compiled, _, compileErr := compilePlan(plan, catalog, defaultMaximumCost)
		definition, fieldAllowed := catalog.Fields[field]
		operatorAllowed := fieldAllowed && containsOperator(definition.Public.Operators, Operator(operator))
		if (!fieldAllowed || !operatorAllowed) && compileErr == nil {
			t.Fatalf("forged field/operator compiled: %q / %q", field, operator)
		}
		if compileErr == nil && (!strings.HasPrefix(compiled.SQL, "SELECT ") || strings.Contains(compiled.SQL, ";")) {
			t.Fatalf("compiler emitted unsafe SQL: %s", compiled.SQL)
		}
	})
}

func TestCompileFieldCompareAndRelatedProjection(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	compared := QueryPlan{
		Version: PlanVersionV1, CatalogVersion: catalog.Public.Version, RootEntity: "documents",
		Projections: []string{"document.identifier"}, MaximumRows: 10,
		Filter: &FilterNode{Kind: FilterPredicate, Field: "document.current_holder_name", OtherField: "document.owner_name", Operator: OperatorNotEqual},
	}
	compiled, _, err := compilePlan(compared, catalog, defaultMaximumCost)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if !strings.Contains(compiled.SQL, "<>") || !strings.Contains(compiled.SQL, "lower(") || len(compiled.Arguments) != 1 {
		t.Fatalf("compare SQL = %s args=%#v", compiled.SQL, compiled.Arguments)
	}
	joined := QueryPlan{
		Version: PlanVersionV1, CatalogVersion: catalog.Public.Version, RootEntity: "profiles",
		Projections: []string{"profile.full_name", "bill.amount"}, MaximumRows: 10,
		Filter: &FilterNode{Kind: FilterRelation, Relation: "profile.bills", Children: []FilterNode{
			{Kind: FilterPredicate, Field: "bill.type", Operator: OperatorEqual, Values: []string{"water"}},
		}},
	}
	compiled, _, err = compilePlan(joined, catalog, defaultMaximumCost)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if !strings.Contains(compiled.SQL, "JOIN") || !strings.Contains(compiled.SQL, "amount") || strings.Contains(compiled.SQL, "water") {
		t.Fatalf("join SQL = %s", compiled.SQL)
	}
}

func TestCatalogExposesFoldedInitialAndIntegerHouseNumber(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	initial := catalog.Fields["profile.name_initial"].Public
	if initial.Label != "Inicial" || initial.Kind != ValueText || !initial.Filterable ||
		!containsOperator(initial.Operators, OperatorIn) || containsOperator(initial.Operators, OperatorStartsWith) ||
		containsOperator(initial.Operators, OperatorBetween) {
		t.Fatalf("name initial = %#v", initial)
	}
	if !strings.Contains(catalog.Fields["profile.name_initial"].Expression, "translate(") ||
		!strings.Contains(catalog.Fields["profile.name_initial"].Expression, "split_part(") ||
		initial.Source != "profile.full_name" || initial.Replaces {
		t.Fatalf("name initial = %#v expr=%s", initial, catalog.Fields["profile.name_initial"].Expression)
	}
	house := catalog.Fields["profile.address_house_number"].Public
	if house.Label != "Número da casa" || house.Kind != ValueInteger || !house.Filterable ||
		!containsOperator(house.Operators, OperatorBetween) || !containsOperator(house.Operators, OperatorGreaterEq) ||
		house.Source != "profile.address_number" || !house.Replaces {
		t.Fatalf("house number = %#v", house)
	}
	number := catalog.Fields["profile.address_number"].Public
	if containsOperator(number.Operators, OperatorBetween) || containsOperator(number.Operators, OperatorGreaterEq) {
		t.Fatalf("text number unexpectedly comparable: %#v", number)
	}
}

func TestCompilerLetterRangeQueryUsesIntegerHouseNumber(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	plan := QueryPlan{
		Version: PlanVersionV1, CatalogVersion: catalog.Public.Version, RootEntity: "profiles",
		Projections: []string{"profile.full_name", "profile.address_street", "profile.address_number"}, MaximumRows: 100,
		Filter: &FilterNode{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
			{Kind: FilterPredicate, Field: "profile.address_street", Operator: OperatorNotNull},
			{Kind: FilterGroup, Conjunction: ConjunctionOr, Children: []FilterNode{
				{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
					{Kind: FilterPredicate, Field: "profile.name_initial", Operator: OperatorEqual, Values: []string{"O"}},
					{Kind: FilterPredicate, Field: "profile.address_house_number", Operator: OperatorBetween, Values: []string{"298", "363"}},
				}},
				{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
					{Kind: FilterPredicate, Field: "profile.name_initial", Operator: OperatorEqual, Values: []string{"W"}},
					{Kind: FilterPredicate, Field: "profile.address_house_number", Operator: OperatorBetween, Values: []string{"852", "1346"}},
				}},
				{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
					{Kind: FilterPredicate, Field: "profile.name_initial", Operator: OperatorEqual, Values: []string{"Z"}},
					{Kind: FilterPredicate, Field: "profile.address_house_number", Operator: OperatorGreaterEq, Values: []string{"1984"}},
				}},
			}},
		}},
	}
	compiled, _, err := compilePlan(plan, catalog, defaultMaximumCost)
	if err != nil {
		t.Fatalf("compilePlan() error = %v", err)
	}
	if !strings.Contains(compiled.SQL, "BETWEEN") || !strings.Contains(compiled.SQL, "::bigint") ||
		!strings.Contains(compiled.SQL, "substring(q0.address_number") ||
		!strings.Contains(compiled.SQL, "translate(split_part") ||
		!strings.Contains(compiled.SQL, "COLLATE gymkhana_pt_br ASC") {
		t.Fatalf("letter-range SQL = %s", compiled.SQL)
	}
	accented := plan
	accented.Filter = &FilterNode{Kind: FilterPredicate, Field: "profile.name_initial", Operator: OperatorEqual, Values: []string{"Ó"}}
	compiled, normalized, err := compilePlan(accented, catalog, defaultMaximumCost)
	if err != nil {
		t.Fatalf("accented initial: %v", err)
	}
	if normalized.Filter == nil || len(normalized.Filter.Values) != 1 || normalized.Filter.Values[0] != "O" {
		t.Fatalf("folded initial = %#v", normalized.Filter)
	}
	_ = compiled
	unsupported := plan
	unsupported.Filter = &FilterNode{Kind: FilterPredicate, Field: "profile.address_number", Operator: OperatorBetween, Values: []string{"298", "363"}}
	if _, _, err := compilePlan(unsupported, catalog, defaultMaximumCost); validationCode(err, "filter.operator") != "unsupported" {
		t.Fatalf("text number between error = %#v", err)
	}
}

func TestShapeScanFoldsRepeatedInitialLikeExecute(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	plan := QueryPlan{
		Version: PlanVersionV1, CatalogVersion: catalog.Public.Version, RootEntity: "profiles",
		Projections: []string{"profile.full_name", "profile.address_street", "profile.address_house_number"}, MaximumRows: 100,
		Filter: &FilterNode{Kind: FilterGroup, Conjunction: ConjunctionOr, Children: []FilterNode{
			{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
				{Kind: FilterPredicate, Field: "profile.name_initial", Operator: OperatorEqual, Values: []string{"OO"}},
				{Kind: FilterPredicate, Field: "profile.address_house_number", Operator: OperatorBetween, Values: []string{"298", "363"}},
			}},
			{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
				{Kind: FilterPredicate, Field: "profile.name_initial", Operator: OperatorEqual, Values: []string{"WW"}},
				{Kind: FilterPredicate, Field: "profile.address_house_number", Operator: OperatorBetween, Values: []string{"852", "1346"}},
			}},
			{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
				{Kind: FilterPredicate, Field: "profile.name_initial", Operator: OperatorEqual, Values: []string{"ZZ"}},
				{Kind: FilterPredicate, Field: "profile.address_house_number", Operator: OperatorGreaterEq, Values: []string{"1984"}},
			}},
		}},
	}
	_, arguments, _, _, err := compileShapeScan(plan, catalog, 50)
	if err != nil {
		t.Fatalf("compileShapeScan() error = %v", err)
	}
	seen := map[string]bool{}
	for _, argument := range arguments {
		text, ok := argument.(string)
		if !ok {
			continue
		}
		seen[text] = true
	}
	if !seen["O"] || !seen["W"] || !seen["Z"] || seen["OO"] || seen["WW"] || seen["ZZ"] {
		t.Fatalf("shape scan initials = %#v", arguments)
	}
}

func TestFilterNodeUnmarshalJSONFlexibilityAndKindInference(t *testing.T) {
	// 1. Group missing "kind" but with conjunction and children
	jsonGroup := []byte(`{
		"conjunction": "AND",
		"children": [
			{"field": "profile.address_street", "operator": "not_null"},
			{"field": "profile.address_number", "operator": "not_null"}
		]
	}`)
	var groupNode FilterNode
	if err := json.Unmarshal(jsonGroup, &groupNode); err != nil {
		t.Fatalf("Unmarshal group error = %v", err)
	}
	if groupNode.Kind != FilterGroup {
		t.Fatalf("groupNode.Kind = %q, want %q", groupNode.Kind, FilterGroup)
	}
	if len(groupNode.Children) != 2 {
		t.Fatalf("groupNode.Children len = %d, want 2", len(groupNode.Children))
	}
	if groupNode.Children[0].Kind != FilterPredicate || groupNode.Children[1].Kind != FilterPredicate {
		t.Fatalf("child kinds = %q, %q; want predicates", groupNode.Children[0].Kind, groupNode.Children[1].Kind)
	}

	// 2. Predicate with integer numbers in values
	jsonNumbers := []byte(`{
		"field": "profile.address_house_number",
		"operator": "between",
		"values": [1756, 5566]
	}`)
	var numberNode FilterNode
	if err := json.Unmarshal(jsonNumbers, &numberNode); err != nil {
		t.Fatalf("Unmarshal numbers error = %v", err)
	}
	if numberNode.Kind != FilterPredicate {
		t.Fatalf("numberNode.Kind = %q, want %q", numberNode.Kind, FilterPredicate)
	}
	if len(numberNode.Values) != 2 || numberNode.Values[0] != "1756" || numberNode.Values[1] != "5566" {
		t.Fatalf("numberNode.Values = %#v, want ['1756', '5566']", numberNode.Values)
	}

	// 3. Predicate with values passed in "children" array (in operator)
	jsonChildrenAsValues := []byte(`{
		"kind": "predicate",
		"field": "profile.address_city",
		"operator": "in",
		"children": ["Porto Alegre", "Canoas"]
	}`)
	var childrenValuesNode FilterNode
	if err := json.Unmarshal(jsonChildrenAsValues, &childrenValuesNode); err != nil {
		t.Fatalf("Unmarshal children-as-values error = %v", err)
	}
	if childrenValuesNode.Kind != FilterPredicate {
		t.Fatalf("childrenValuesNode.Kind = %q, want %q", childrenValuesNode.Kind, FilterPredicate)
	}
	if len(childrenValuesNode.Values) != 2 || childrenValuesNode.Values[0] != "Porto Alegre" || childrenValuesNode.Values[1] != "Canoas" {
		t.Fatalf("childrenValuesNode.Values = %#v, want ['Porto Alegre', 'Canoas']", childrenValuesNode.Values)
	}
	if len(childrenValuesNode.Children) != 0 {
		t.Fatalf("childrenValuesNode.Children should be empty, got len=%d", len(childrenValuesNode.Children))
	}
}

func TestCompileUserQueryComplexFilterTree(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleExternal)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}

	// This JSON simulates the exact plan produced by Gemini for the user's prompt:
	// - Name initial in B, F, G
	// - Address not null
	// - Not in 24 cities
	// - House numbers in specific ranges per initial (numbers passed as raw integers, kind omitted in nested groups)
	rawPlanJSON := []byte(`{
		"version": "v1",
		"catalog_version": "` + catalog.Public.Version + `",
		"root_entity": "profiles",
		"projections": ["profile.full_name", "profile.address_city", "profile.address_number"],
		"maximum_rows": 100,
		"filter": {
			"conjunction": "AND",
			"children": [
				{
					"field": "profile.address_street",
					"operator": "not_null"
				},
				{
					"kind": "not",
					"children": [
						{
							"field": "profile.address_city",
							"operator": "in",
							"values": ["Charqueadas", "Gravataí", "Novo Hamburgo", "Campo Bom", "Três Coroas", "Taquara", "São Leopoldo", "Muitos Capões", "Vacaria", "Arroio dos Ratos", "Estância Velha", "Ipê", "Pelotas", "Capão da Canoa", "Imbé", "Blumenau", "Parobé", "Palmares do Sul", "canoas", "São Jerônimo", "Porto Alegre", "sapucaia", "roca salles"]
						}
					]
				},
				{
					"conjunction": "OR",
					"children": [
						{
							"conjunction": "AND",
							"children": [
								{"field": "profile.name_initial", "operator": "eq", "values": ["B"]},
								{"field": "profile.address_house_number", "operator": "between", "values": [1756, 5566]}
							]
						},
						{
							"conjunction": "AND",
							"children": [
								{"field": "profile.name_initial", "operator": "eq", "values": ["F"]},
								{"field": "profile.address_house_number", "operator": "between", "values": [1047, 1605]}
							]
						},
						{
							"conjunction": "AND",
							"children": [
								{"field": "profile.name_initial", "operator": "eq", "values": ["G"]},
								{"field": "profile.address_house_number", "operator": "between", "values": [878, 1250]}
							]
						}
					]
				}
			]
		}
	}`)

	var plan QueryPlan
	if err := json.Unmarshal(rawPlanJSON, &plan); err != nil {
		t.Fatalf("json.Unmarshal(rawPlanJSON) error = %v", err)
	}

	compiled, _, err := compilePlan(plan, catalog, defaultMaximumCost)
	if err != nil {
		t.Fatalf("compilePlan() error = %v", err)
	}

	if compiled.SQL == "" {
		t.Fatal("compiled SQL is empty")
	}
	if !strings.Contains(compiled.SQL, "BETWEEN") {
		t.Errorf("compiled SQL missing BETWEEN: %s", compiled.SQL)
	}
	if !strings.Contains(compiled.SQL, "NOT (") {
		t.Errorf("compiled SQL missing NOT (: %s", compiled.SQL)
	}
}

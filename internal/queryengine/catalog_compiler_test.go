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
	first, err := loadCatalog(context.Background(), store, auth.RoleMember)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	second, err := loadCatalog(context.Background(), store, auth.RoleMember)
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
	motherName, ok := first.Fields["profile.mother_name"]
	if !ok || motherName.Public.Entity != "profiles" || motherName.Public.Kind != ValueText || !motherName.Public.Filterable || !motherName.Public.Projectable {
		t.Fatalf("canonical profile detail field = %#v", motherName.Public)
	}
	if !strings.Contains(first.Entities["profiles"].FromTemplate, "LEFT JOIN profile_details") || motherName.Expression != "{root}_details.mother_name" {
		t.Fatalf("profile details are not joined through the canonical profile entity: %#v / %q", first.Entities["profiles"], motherName.Expression)
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
	for _, role := range []auth.Role{auth.RoleMember, auth.RoleAdmin, auth.RoleSuperadmin} {
		permitted, err := loadCatalog(context.Background(), store, role)
		if err != nil || len(permitted.Public.Entities) != 5 || len(permitted.Public.Relations) == 0 {
			t.Fatalf("loadCatalog(%s) = %d entities/%d relations, error=%v", role, len(permitted.Public.Entities), len(permitted.Public.Relations), err)
		}
	}
	encoded, err := json.Marshal(first.Public)
	if err != nil || strings.Contains(string(encoded), "custom_field_values") || strings.Contains(string(encoded), "profile_details") || strings.Contains(string(encoded), "JOIN ") || strings.Contains(string(encoded), "technical_key") {
		t.Fatalf("serialized catalog leaked physical metadata: %s, error=%v", encoded, err)
	}
	fullName := first.Fields["profile.full_name"].Public
	if containsOperator(fullName.Operators, OperatorIsNull) || containsOperator(fullName.Operators, OperatorNotNull) {
		t.Fatalf("non-null field exposes null operators: %#v", fullName.Operators)
	}
	store.definitions = CatalogDefinitions{}
	changed, err := loadCatalog(context.Background(), store, auth.RoleMember)
	if err != nil || changed.Public.Version == first.Public.Version {
		t.Fatalf("dynamic catalog did not refresh: %q / %q, error=%v", first.Public.Version, changed.Public.Version, err)
	}
	if _, exists := changed.Entities["custom_entity."+testEntityID]; exists {
		t.Fatal("removed dynamic definition remained in catalog")
	}
}

func TestCompilerBuildsParameterizedNestedPlanWithoutMutatingInput(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleMember)
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
	if !strings.Contains(compiled.SQL, "LEFT JOIN profile_details q0_details") || !strings.Contains(compiled.SQL, "EXISTS (SELECT 1 FROM documents q1") || !strings.Contains(compiled.SQL, "$1::text") ||
		!strings.Contains(compiled.SQL, "LIMIT $3::integer") || strings.Contains(compiled.SQL, "OR true") {
		t.Fatalf("compiled SQL is not safely parameterized:\n%s", compiled.SQL)
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

func TestCompilerCombinesNameCityAndMotherName(t *testing.T) {
	catalog, err := loadCatalog(context.Background(), newFakeQueryStore(), auth.RoleMember)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	plan := QueryPlan{
		Version:        PlanVersionV1,
		CatalogVersion: catalog.Public.Version,
		RootEntity:     "profiles",
		Projections:    []string{"profile.full_name", "profile.address_city", "profile.mother_name"},
		MaximumRows:    50,
		Filter: &FilterNode{Kind: FilterGroup, Conjunction: ConjunctionAnd, Children: []FilterNode{
			{Kind: FilterPredicate, Field: "profile.full_name", Operator: OperatorContains, Values: []string{"Ana Souza"}},
			{Kind: FilterPredicate, Field: "profile.address_city", Operator: OperatorEqual, Values: []string{"Campinas"}},
			{Kind: FilterPredicate, Field: "profile.mother_name", Operator: OperatorContains, Values: []string{"Maria Oliveira"}},
		}},
	}
	compiled, _, err := compilePlan(plan, catalog, defaultMaximumCost)
	if err != nil {
		t.Fatalf("compilePlan() error = %v", err)
	}
	for _, fragment := range []string{
		"LEFT JOIN profile_details q0_details ON q0_details.profile_id=q0.id",
		"(q0_details.mother_name)::text AS value_2",
		"lower((q0.address_city)::text) = lower($2::text)",
		"lower((q0_details.mother_name)::text) LIKE $3::text",
		"LIMIT $4::integer",
	} {
		if !strings.Contains(compiled.SQL, fragment) {
			t.Fatalf("compiled SQL missing %q:\n%s", fragment, compiled.SQL)
		}
	}
	if len(compiled.Arguments) != 4 || compiled.Arguments[0] != "%ana souza%" || compiled.Arguments[1] != "Campinas" || compiled.Arguments[2] != "%maria oliveira%" || compiled.Arguments[3] != 50 {
		t.Fatalf("compiled arguments = %#v", compiled.Arguments)
	}
}

func TestCompilerRejectsStaleInvalidAndOverCostPlans(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleMember)
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
		catalog, err := loadCatalog(context.Background(), store, auth.RoleMember)
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

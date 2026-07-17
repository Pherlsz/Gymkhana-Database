package queryengine

import (
	"context"
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
	for index := 1; index < len(first.Public.Fields); index++ {
		left, right := first.Public.Fields[index-1], first.Public.Fields[index]
		if left.Entity > right.Entity || left.Entity == right.Entity && left.Label > right.Label {
			t.Fatalf("public fields are not stable at %d: %#v then %#v", index, left, right)
		}
	}
	if _, err := loadCatalog(context.Background(), store, auth.Role("UNKNOWN")); !errors.Is(err, ErrForbidden) {
		t.Fatalf("loadCatalog(unknown role) error = %v", err)
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

func TestCompilerRejectsStaleInvalidAndOverCostPlans(t *testing.T) {
	store := newFakeQueryStore()
	catalog, err := loadCatalog(context.Background(), store, auth.RoleMember)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	base := QueryPlan{CatalogVersion: catalog.Public.Version, RootEntity: "bills", Projections: []string{"bill.amount"}, MaximumRows: 10}
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

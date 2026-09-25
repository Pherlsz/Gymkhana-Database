package queryengine

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

func normalizeAdvancedPlan(plan QueryPlan, catalog resolvedCatalog) (QueryPlan, *ValidationError) {
	validation := &ValidationError{}
	if plan.Version != PlanVersionV2 {
		validation.add("version", "unsupported")
	}
	plan.CatalogVersion = strings.TrimSpace(plan.CatalogVersion)
	if len(plan.CatalogVersion) != 64 {
		validation.add("catalog_version", "invalid")
	}
	plan.RootEntity = strings.TrimSpace(plan.RootEntity)
	root, rootExists := catalog.Entities[plan.RootEntity]
	if !rootExists && plan.Set == nil && plan.Combination == nil {
		validation.add("root_entity", "unsupported")
	}
	if plan.MaximumRows == 0 {
		plan.MaximumRows = 100
	}
	if plan.MaximumRows < 1 || plan.MaximumRows > MaximumRows {
		validation.add("maximum_rows", "out_of_range")
	}
	if plan.Set != nil && plan.Combination != nil {
		validation.add("set", "conflict")
		validation.add("combination", "conflict")
	}
	if plan.Set != nil {
		plan.Set = normalizeSet(plan.Set, catalog, validation, "set", 1)
		return plan, validation
	}
	if plan.Combination != nil {
		plan.Combination = normalizeCombination(plan.Combination, catalog, validation)
		return plan, validation
	}
	if !rootExists {
		return plan, validation
	}
	plan.Filter = normalizeOptionalFilter(plan.Filter, validation)
	plan.Projections = normalizeProjectionList(plan.Projections, root.Public.Key, catalog, validation)
	plan.GroupBy = normalizeGroupList(plan.GroupBy, root.Public.Key, catalog, validation)
	plan.Aggregates = normalizeAggregates(plan.Aggregates, root.Public.Key, catalog, validation)
	plan.Having = normalizeHaving(plan.Having, plan.Aggregates, validation, "having", 1)
	plan.Patterns = normalizePatterns(plan.Patterns, root.Public.Key, catalog, validation)
	plan.Sort = normalizeAdvancedSort(plan.Sort, root.Public.Key, catalog, plan.Aggregates, validation)
	if len(plan.GroupBy) == 0 && len(plan.Aggregates) == 0 && len(plan.Projections) == 0 {
		validation.add("projections", "required")
	}
	if len(plan.Aggregates) > 0 && len(plan.Projections) > 0 {
		validation.add("projections", "aggregate_conflict")
	}
	return plan, validation
}

func normalizeOptionalFilter(filter *FilterNode, validation *ValidationError) *FilterNode {
	if filter == nil {
		return nil
	}
	value := normalizeFilter(*filter, validation, "filter", 1)
	return &value
}

func normalizeProjectionList(values []string, entity string, catalog resolvedCatalog, validation *ValidationError) []string {
	if len(values) > MaximumProjections {
		validation.add("projections", "too_many")
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for index, key := range values {
		key = strings.TrimSpace(key)
		field, ok := catalog.Fields[key]
		if !ok || !field.Public.Projectable {
			validation.add(fmt.Sprintf("projections.%d", index), "unsupported")
			continue
		}
		if field.Public.Entity != entity {
			if _, ok := findRelation(catalog, entity, field.Public.Entity, nil); !ok {
				validation.add(fmt.Sprintf("projections.%d", index), "entity_mismatch")
				continue
			}
		}
		if _, duplicate := seen[key]; duplicate {
			validation.add(fmt.Sprintf("projections.%d", index), "duplicate")
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func normalizeGroupList(values []string, entity string, catalog resolvedCatalog, validation *ValidationError) []string {
	if len(values) > MaximumGroupKeys {
		validation.add("group_by", "too_many")
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for index, key := range values {
		key = strings.TrimSpace(key)
		field, ok := catalog.Fields[key]
		if !ok || !field.Public.Projectable || !field.Public.Sortable {
			validation.add(fmt.Sprintf("group_by.%d", index), "unsupported")
			continue
		}
		if field.Public.Entity != entity {
			validation.add(fmt.Sprintf("group_by.%d", index), "entity_mismatch")
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			validation.add(fmt.Sprintf("group_by.%d", index), "duplicate")
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func normalizeAggregates(values []Aggregate, entity string, catalog resolvedCatalog, validation *ValidationError) []Aggregate {
	if len(values) > MaximumAggregates {
		validation.add("aggregates", "too_many")
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]Aggregate, 0, len(values))
	for index, value := range values {
		value.Key = strings.TrimSpace(value.Key)
		value.Field = strings.TrimSpace(value.Field)
		path := fmt.Sprintf("aggregates.%d", index)
		if !validLogicalSegment(value.Key) {
			validation.add(path+".key", "invalid")
			continue
		}
		if _, duplicate := seen[value.Key]; duplicate {
			validation.add(path+".key", "duplicate")
			continue
		}
		seen[value.Key] = struct{}{}
		if value.Function == AggregateCount && value.Field == "" {
			result = append(result, value)
			continue
		}
		field, ok := catalog.Fields[value.Field]
		if !ok || field.Public.Entity != entity {
			validation.add(path+".field", "unsupported")
			continue
		}
		if !containsAggregate(aggregateFunctionsFor(field.Public.Kind), value.Function) {
			validation.add(path+".function", "unsupported")
			continue
		}
		if value.Distinct && (value.Function == AggregateMinimum || value.Function == AggregateMaximum) {
			validation.add(path+".distinct", "unsupported")
			continue
		}
		result = append(result, value)
	}
	return result
}

func normalizeHaving(node *AggregateFilterNode, aggregates []Aggregate, validation *ValidationError, path string, depth int) *AggregateFilterNode {
	if node == nil {
		return nil
	}
	if depth > MaximumFilterDepth {
		validation.add(path, "too_deep")
	}
	known := make(map[string]Aggregate, len(aggregates))
	for _, aggregate := range aggregates {
		known[aggregate.Key] = aggregate
	}
	var normalize func(AggregateFilterNode, string, int) AggregateFilterNode
	nodes := 0
	normalize = func(value AggregateFilterNode, current string, currentDepth int) AggregateFilterNode {
		nodes++
		if nodes > MaximumAggregateFilterNodes {
			validation.add(current, "too_many")
		}
		if currentDepth > MaximumFilterDepth {
			validation.add(current, "too_deep")
		}
		if value.Predicate != nil {
			value.Predicate.Aggregate = strings.TrimSpace(value.Predicate.Aggregate)
			if _, ok := known[value.Predicate.Aggregate]; !ok {
				validation.add(current+".predicate.aggregate", "unsupported")
			}
			minimum, maximum := operatorArity(value.Predicate.Operator)
			if len(value.Predicate.Values) < minimum || len(value.Predicate.Values) > maximum {
				validation.add(current+".predicate.values", "wrong_arity")
			}
			if len(value.Children) > 0 || value.Conjunction != "" {
				validation.add(current, "invalid_shape")
			}
		}
		if len(value.Children) > 0 {
			if value.Predicate != nil || (value.Conjunction != ConjunctionAnd && value.Conjunction != ConjunctionOr) {
				validation.add(current, "invalid_shape")
			}
			children := make([]AggregateFilterNode, 0, len(value.Children))
			for index, child := range value.Children {
				children = append(children, normalize(child, fmt.Sprintf("%s.children.%d", current, index), currentDepth+1))
			}
			value.Children = children
		}
		return value
	}
	value := normalize(*node, path, depth)
	return &value
}

func normalizePatterns(values []PatternPredicate, entity string, catalog resolvedCatalog, validation *ValidationError) []PatternPredicate {
	if len(values) > MaximumFilterNodes {
		validation.add("patterns", "too_many")
	}
	result := make([]PatternPredicate, 0, len(values))
	for index, value := range values {
		path := fmt.Sprintf("patterns.%d", index)
		value.Field = strings.TrimSpace(value.Field)
		value.Pattern = strings.ToValidUTF8(strings.TrimSpace(value.Pattern), "")
		field, ok := catalog.Fields[value.Field]
		if !ok || field.Public.Entity != entity || !containsPattern(patternGrammarsFor(field.Public.Kind), value.Grammar) {
			validation.add(path+".field", "unsupported")
			continue
		}
		if utf8.RuneCountInString(value.Pattern) == 0 && !value.AllowEmpty {
			validation.add(path+".pattern", "required")
			continue
		}
		if utf8.RuneCountInString(value.Pattern) > MaximumPatternLength || patternTokenCount(value.Pattern) > MaximumPatternTokens {
			validation.add(path+".pattern", "too_many")
			continue
		}
		if !validPatternInput(value.Grammar, value.Pattern) {
			validation.add(path+".pattern", "invalid")
			continue
		}
		result = append(result, value)
	}
	return result
}

func normalizeAdvancedSort(values []Sort, entity string, catalog resolvedCatalog, aggregates []Aggregate, validation *ValidationError) []Sort {
	if len(values) > MaximumSortFields {
		validation.add("sort", "too_many")
	}
	aggregateKeys := make(map[string]struct{}, len(aggregates))
	for _, value := range aggregates {
		aggregateKeys[value.Key] = struct{}{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]Sort, 0, len(values))
	for index, value := range values {
		value.Field = strings.TrimSpace(value.Field)
		field, fieldOK := catalog.Fields[value.Field]
		_, aggregateOK := aggregateKeys[value.Field]
		if (!fieldOK || !field.Public.Sortable || field.Public.Entity != entity) && !aggregateOK {
			validation.add(fmt.Sprintf("sort.%d.field", index), "unsupported")
			continue
		}
		if value.Direction == "" {
			value.Direction = SortAscending
		}
		if value.Direction != SortAscending && value.Direction != SortDescending {
			validation.add(fmt.Sprintf("sort.%d.direction", index), "unsupported")
			continue
		}
		if _, duplicate := seen[value.Field]; duplicate {
			validation.add(fmt.Sprintf("sort.%d.field", index), "duplicate")
			continue
		}
		seen[value.Field] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeSet(value *SetExpression, catalog resolvedCatalog, validation *ValidationError, path string, depth int) *SetExpression {
	if value == nil {
		return nil
	}
	if depth > MaximumSetDepth {
		validation.add(path, "too_deep")
	}
	if value.Plan != nil {
		if len(value.Inputs) > 0 || value.Operator != "" {
			validation.add(path, "invalid_shape")
		}
		normalized, childValidation := normalizeAdvancedPlan(*value.Plan, catalog)
		for _, field := range childValidation.Fields {
			validation.add(path+".plan."+field.Field, field.Code)
		}
		value.Plan = &normalized
		return value
	}
	minimum, maximum := setArity(value.Operator)
	if minimum == 0 {
		validation.add(path+".operator", "unsupported")
	}
	if len(value.Inputs) < minimum || len(value.Inputs) > maximum {
		validation.add(path+".inputs", "wrong_arity")
	}
	inputs := make([]SetExpression, 0, len(value.Inputs))
	for index := range value.Inputs {
		child := normalizeSet(&value.Inputs[index], catalog, validation, fmt.Sprintf("%s.inputs.%d", path, index), depth+1)
		inputs = append(inputs, *child)
	}
	value.Inputs = inputs
	if value.Operator == SetUnion || value.Operator == SetIntersection {
		sort.SliceStable(value.Inputs, func(left, right int) bool {
			return canonicalSetKey(value.Inputs[left]) < canonicalSetKey(value.Inputs[right])
		})
	}
	return value
}

func normalizeCombination(value *CombinationSpec, catalog resolvedCatalog, validation *ValidationError) *CombinationSpec {
	if value == nil {
		return nil
	}
	if len(value.Inputs) < 1 || len(value.Inputs) > MaximumCombinationDimensions {
		validation.add("combination.inputs", "out_of_range")
	}
	if value.MaximumCombinations == 0 {
		value.MaximumCombinations = MaximumCombinationSize
	}
	if value.MaximumCombinations < 1 || value.MaximumCombinations > MaximumCombinationSize {
		validation.add("combination.maximum_combinations", "out_of_range")
	}
	seen := make(map[string]struct{}, len(value.Inputs))
	product := 1
	for index := range value.Inputs {
		input := &value.Inputs[index]
		path := fmt.Sprintf("combination.inputs.%d", index)
		input.Key = strings.TrimSpace(input.Key)
		if !validLogicalSegment(input.Key) {
			validation.add(path+".key", "invalid")
		}
		if _, duplicate := seen[input.Key]; duplicate {
			validation.add(path+".key", "duplicate")
		}
		seen[input.Key] = struct{}{}
		if input.Plan == nil {
			validation.add(path+".plan", "required")
			continue
		}
		if input.MinimumSelected == 0 {
			input.MinimumSelected = 1
		}
		if input.MaximumSelected == 0 {
			input.MaximumSelected = 1
		}
		if input.MinimumSelected != 1 || input.MaximumSelected != 1 {
			validation.add(path, "unsupported_cardinality")
		}
		normalized, childValidation := normalizeAdvancedPlan(*input.Plan, catalog)
		for _, field := range childValidation.Fields {
			validation.add(path+".plan."+field.Field, field.Code)
		}
		input.Plan = &normalized
		product *= maximumInt(1, normalized.MaximumRows)
		if product > value.MaximumCombinations || product > MaximumCombinationSize {
			validation.add("combination", "too_many")
			product = MaximumCombinationSize + 1
		}
	}
	value.Aggregates = normalizeCombinationAggregates(value.Aggregates, value.Inputs, validation)
	value.Having = normalizeHaving(value.Having, value.Aggregates, validation, "combination.having", 1)
	return value
}

func normalizeCombinationAggregates(values []Aggregate, inputs []CombinationInput, validation *ValidationError) []Aggregate {
	result := make([]Aggregate, 0, len(values))
	seen := map[string]struct{}{}
	for index, value := range values {
		value.Key = strings.TrimSpace(value.Key)
		value.Field = strings.TrimSpace(value.Field)
		path := fmt.Sprintf("combination.aggregates.%d", index)
		if !validLogicalSegment(value.Key) {
			validation.add(path+".key", "invalid")
			continue
		}
		if _, duplicate := seen[value.Key]; duplicate {
			validation.add(path+".key", "duplicate")
			continue
		}
		seen[value.Key] = struct{}{}
		if value.Function == AggregateCount && value.Field == "" {
			result = append(result, value)
			continue
		}
		if !combinationFieldExists(inputs, value.Field) {
			validation.add(path+".field", "unsupported")
			continue
		}
		result = append(result, value)
	}
	return result
}

func combinationFieldExists(inputs []CombinationInput, field string) bool {
	name := field
	key := ""
	if strings.HasPrefix(field, "*:") {
		name = strings.TrimPrefix(field, "*:")
	} else if cut, rest, ok := strings.Cut(field, ":"); ok {
		key, name = cut, rest
	}
	for _, input := range inputs {
		if input.Plan == nil || (key != "" && input.Key != key) {
			continue
		}
		for _, projection := range input.Plan.Projections {
			if projection == name {
				return true
			}
		}
	}
	return false
}

package queryengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const defaultMaximumCost = 250_000

var decimalPattern = regexp.MustCompile(`^-?[0-9]{1,28}(\.[0-9]{1,10})?$`)
var civilMonthPattern = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)

type compileState struct {
	catalog      resolvedCatalog
	arguments    []any
	nextRelation int
	nodes        int
	relations    int
	validation   *ValidationError
}

func compilePlan(plan QueryPlan, catalog resolvedCatalog, maximumCost int) (CompiledPlan, QueryPlan, error) {
	if maximumCost <= 0 {
		maximumCost = defaultMaximumCost
	}
	normalized, validation := normalizePlan(plan, catalog)
	if !validation.empty() {
		return CompiledPlan{}, QueryPlan{}, validation
	}
	if normalized.CatalogVersion != catalog.Public.Version {
		return CompiledPlan{}, QueryPlan{}, ErrStaleCatalog
	}
	state := &compileState{catalog: catalog, validation: &ValidationError{}}
	root := catalog.Entities[normalized.RootEntity]
	rootPrefix := "q0"
	columns := make([]ResultColumn, 0, len(normalized.Projections))
	selects := []string{
		expandSQL(root.IDExpression, rootPrefix) + " AS entity_id",
		expandSQL(root.LabelExpression, rootPrefix) + " AS entity_label",
		expandSQL(root.UpdatedExpression, rootPrefix) + " AS entity_updated_at",
	}
	for position, key := range normalized.Projections {
		definition := catalog.Fields[key]
		columns = append(columns, ResultColumn{Position: position, FieldKey: key, Label: definition.Public.Label, Kind: definition.Public.Kind})
		selects = append(selects, fmt.Sprintf("(%s)::text AS value_%d", expandSQL(definition.Expression, rootPrefix), position))
	}
	where := make([]string, 0, 2)
	if root.BaseCondition != "" {
		where = append(where, expandSQL(root.BaseCondition, rootPrefix))
	}
	if normalized.Filter != nil {
		compiled := state.compileFilter(*normalized.Filter, normalized.RootEntity, rootPrefix, 1, 0, "filter")
		if compiled != "" {
			where = append(where, compiled)
		}
	}
	if !state.validation.empty() {
		return CompiledPlan{}, QueryPlan{}, state.validation
	}
	order := make([]string, 0, len(normalized.Sort)+1)
	for _, sortValue := range normalized.Sort {
		definition := catalog.Fields[sortValue.Field]
		direction := "ASC"
		if sortValue.Direction == SortDescending {
			direction = "DESC"
		}
		order = append(order, fmt.Sprintf("%s %s NULLS LAST", expandSQL(definition.Expression, rootPrefix), direction))
	}
	order = append(order, expandSQL(root.IDExpression, rootPrefix)+" ASC")
	limitPosition := state.bind(normalized.MaximumRows)
	query := "SELECT " + strings.Join(selects, ",\n       ") + "\nFROM " + expandSQL(root.FromTemplate, rootPrefix)
	if len(where) > 0 {
		query += "\nWHERE " + strings.Join(where, " AND ")
	}
	query += "\nORDER BY " + strings.Join(order, ", ") + fmt.Sprintf("\nLIMIT $%d::integer", limitPosition)
	cost := estimateCost(normalized, state.nodes, state.relations)
	if cost > maximumCost {
		return CompiledPlan{}, QueryPlan{}, ErrCostLimit
	}
	encoded, _ := json.Marshal(normalized)
	fingerprint := sha256.Sum256(encoded)
	return CompiledPlan{SQL: query, Arguments: state.arguments, Columns: columns, Fingerprint: fingerprint,
		CatalogVersion: catalog.Public.Version, RootEntity: normalized.RootEntity, EntityKind: root.Public.Kind,
		MaximumRows: normalized.MaximumRows, Cost: cost}, normalized, nil
}

func normalizePlan(plan QueryPlan, catalog resolvedCatalog) (QueryPlan, *ValidationError) {
	validation := &ValidationError{}
	if plan.Version == "" {
		plan.Version = PlanVersionV1
	}
	if plan.Version != PlanVersionV1 {
		validation.add("version", "unsupported")
	}
	plan.CatalogVersion = strings.TrimSpace(plan.CatalogVersion)
	if len(plan.CatalogVersion) != 64 {
		validation.add("catalog_version", "invalid")
	}
	plan.RootEntity = strings.TrimSpace(plan.RootEntity)
	root, rootExists := catalog.Entities[plan.RootEntity]
	if !rootExists {
		validation.add("root_entity", "unsupported")
	}
	if plan.MaximumRows == 0 {
		plan.MaximumRows = 100
	}
	if plan.MaximumRows < 1 || plan.MaximumRows > MaximumRows {
		validation.add("maximum_rows", "out_of_range")
	}
	if len(plan.Projections) == 0 {
		validation.add("projections", "required")
	}
	if len(plan.Projections) > MaximumProjections {
		validation.add("projections", "too_many")
	}
	seenProjections := make(map[string]struct{}, len(plan.Projections))
	normalizedProjections := make([]string, 0, len(plan.Projections))
	for index, key := range plan.Projections {
		key = strings.TrimSpace(key)
		definition, ok := catalog.Fields[key]
		if !ok || !definition.Public.Projectable {
			validation.add(fmt.Sprintf("projections.%d", index), "unsupported")
			continue
		}
		if rootExists && definition.Public.Entity != root.Public.Key {
			validation.add(fmt.Sprintf("projections.%d", index), "entity_mismatch")
			continue
		}
		if _, duplicate := seenProjections[key]; duplicate {
			validation.add(fmt.Sprintf("projections.%d", index), "duplicate")
			continue
		}
		seenProjections[key] = struct{}{}
		normalizedProjections = append(normalizedProjections, key)
	}
	plan.Projections = normalizedProjections
	if len(plan.Sort) == 0 && rootExists {
		plan.Sort = []Sort{{Field: root.Public.DefaultSort, Direction: SortAscending}}
	}
	if len(plan.Sort) > MaximumSortFields {
		validation.add("sort", "too_many")
	}
	seenSorts := make(map[string]struct{}, len(plan.Sort))
	normalizedSorts := make([]Sort, 0, len(plan.Sort))
	for index, value := range plan.Sort {
		value.Field = strings.TrimSpace(value.Field)
		definition, ok := catalog.Fields[value.Field]
		if !ok || !definition.Public.Sortable {
			validation.add(fmt.Sprintf("sort.%d.field", index), "unsupported")
			continue
		}
		if rootExists && definition.Public.Entity != root.Public.Key {
			validation.add(fmt.Sprintf("sort.%d.field", index), "entity_mismatch")
			continue
		}
		if value.Direction == "" {
			value.Direction = SortAscending
		}
		if value.Direction != SortAscending && value.Direction != SortDescending {
			validation.add(fmt.Sprintf("sort.%d.direction", index), "unsupported")
			continue
		}
		if _, duplicate := seenSorts[value.Field]; duplicate {
			validation.add(fmt.Sprintf("sort.%d.field", index), "duplicate")
			continue
		}
		seenSorts[value.Field] = struct{}{}
		normalizedSorts = append(normalizedSorts, value)
	}
	plan.Sort = normalizedSorts
	if plan.Filter != nil {
		copyValue := normalizeFilter(*plan.Filter, validation, "filter", 1)
		plan.Filter = &copyValue
	}
	return plan, validation
}

func normalizeFilter(node FilterNode, validation *ValidationError, path string, depth int) FilterNode {
	if depth > MaximumFilterDepth {
		validation.add(path, "too_deep")
	}
	node.Field = strings.TrimSpace(node.Field)
	node.Relation = strings.TrimSpace(node.Relation)
	values := make([]string, 0, len(node.Values))
	for _, value := range node.Values {
		values = append(values, strings.TrimSpace(strings.ToValidUTF8(value, "")))
	}
	node.Values = values
	children := make([]FilterNode, 0, len(node.Children))
	for index, child := range node.Children {
		children = append(children, normalizeFilter(child, validation, fmt.Sprintf("%s.children.%d", path, index), depth+1))
	}
	node.Children = children
	return node
}

func (state *compileState) compileFilter(node FilterNode, entityKey, prefix string, depth, relationDepth int, path string) string {
	state.nodes++
	if state.nodes > MaximumFilterNodes {
		state.validation.add(path, "too_many")
		return ""
	}
	if depth > MaximumFilterDepth {
		state.validation.add(path, "too_deep")
		return ""
	}
	switch node.Kind {
	case FilterPredicate:
		return state.compilePredicate(node, entityKey, prefix, path)
	case FilterGroup:
		if node.Field != "" || node.Relation != "" || node.Operator != "" || len(node.Values) > 0 {
			state.validation.add(path, "invalid_shape")
		}
		if node.Conjunction != ConjunctionAnd && node.Conjunction != ConjunctionOr {
			state.validation.add(path+".conjunction", "unsupported")
			return ""
		}
		if len(node.Children) < 1 || len(node.Children) > MaximumFilterNodes {
			state.validation.add(path+".children", "out_of_range")
			return ""
		}
		parts := make([]string, 0, len(node.Children))
		for index, child := range node.Children {
			compiled := state.compileFilter(child, entityKey, prefix, depth+1, relationDepth, fmt.Sprintf("%s.children.%d", path, index))
			if compiled != "" {
				parts = append(parts, compiled)
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return "(" + strings.Join(parts, " "+string(node.Conjunction)+" ") + ")"
	case FilterNot:
		if len(node.Children) != 1 || node.Field != "" || node.Relation != "" || node.Operator != "" || node.Conjunction != "" || len(node.Values) > 0 {
			state.validation.add(path, "invalid_shape")
			return ""
		}
		compiled := state.compileFilter(node.Children[0], entityKey, prefix, depth+1, relationDepth, path+".children.0")
		if compiled == "" {
			return ""
		}
		return "NOT (" + compiled + ")"
	case FilterRelation:
		if relationDepth >= MaximumRelationDepth {
			state.validation.add(path, "relation_too_deep")
			return ""
		}
		relation, ok := state.catalog.Relations[node.Relation]
		if !ok || relation.Public.FromEntity != entityKey {
			state.validation.add(path+".relation", "unsupported")
			return ""
		}
		if len(node.Children) != 1 || node.Field != "" || node.Operator != "" || node.Conjunction != "" || len(node.Values) > 0 {
			state.validation.add(path, "invalid_shape")
			return ""
		}
		state.relations++
		state.nextRelation++
		targetPrefix := fmt.Sprintf("q%d", state.nextRelation)
		targetEntity := state.catalog.Entities[relation.TargetEntityKey]
		child := state.compileFilter(node.Children[0], relation.TargetEntityKey, targetPrefix, depth+1, relationDepth+1, path+".children.0")
		if child == "" {
			return ""
		}
		conditions := []string{expandRelationSQL(relation.JoinCondition, prefix, targetPrefix), child}
		if targetEntity.BaseCondition != "" {
			conditions = append(conditions, expandSQL(targetEntity.BaseCondition, targetPrefix))
		}
		return "EXISTS (SELECT 1 FROM " + expandSQL(targetEntity.FromTemplate, targetPrefix) + " WHERE " + strings.Join(conditions, " AND ") + ")"
	default:
		state.validation.add(path+".kind", "unsupported")
		return ""
	}
}

func (state *compileState) compilePredicate(node FilterNode, entityKey, prefix, path string) string {
	if len(node.Children) > 0 || node.Relation != "" || node.Conjunction != "" {
		state.validation.add(path, "invalid_shape")
		return ""
	}
	definition, ok := state.catalog.Fields[node.Field]
	if !ok || !definition.Public.Filterable || definition.Public.Entity != entityKey {
		state.validation.add(path+".field", "unsupported")
		return ""
	}
	if !containsOperator(definition.Public.Operators, node.Operator) {
		state.validation.add(path+".operator", "unsupported")
		return ""
	}
	minimum, maximum := operatorArity(node.Operator)
	if len(node.Values) < minimum || len(node.Values) > maximum {
		state.validation.add(path+".values", "wrong_arity")
		return ""
	}
	values := make([]string, 0, len(node.Values))
	for index, value := range node.Values {
		if !validPredicateValue(value, definition.Public.Kind) {
			state.validation.add(fmt.Sprintf("%s.values.%d", path, index), "invalid_type")
			continue
		}
		values = append(values, value)
	}
	if len(values) != len(node.Values) {
		return ""
	}
	expression := expandSQL(definition.Expression, prefix)
	switch node.Operator {
	case OperatorIsNull:
		return "(" + expression + ") IS NULL"
	case OperatorNotNull:
		return "(" + expression + ") IS NOT NULL"
	case OperatorContains:
		position := state.bind(literalPattern(values[0], true))
		return fmt.Sprintf("lower((%s)::text) LIKE $%d::text ESCAPE '\\'", expression, position)
	case OperatorStartsWith:
		position := state.bind(literalPattern(values[0], false))
		return fmt.Sprintf("lower((%s)::text) LIKE $%d::text ESCAPE '\\'", expression, position)
	case OperatorIn:
		parts := make([]string, 0, len(values))
		for _, value := range values {
			parts = append(parts, state.placeholder(value, definition.Public.Kind))
		}
		if isTextKind(definition.Public.Kind) {
			return fmt.Sprintf("lower((%s)::text) IN (%s)", expression, strings.Join(lowerPlaceholders(parts), ","))
		}
		return fmt.Sprintf("(%s) IN (%s)", expression, strings.Join(parts, ","))
	case OperatorBetween:
		left := state.placeholder(values[0], definition.Public.Kind)
		right := state.placeholder(values[1], definition.Public.Kind)
		return fmt.Sprintf("(%s) BETWEEN %s AND %s", expression, left, right)
	default:
		operator := map[Operator]string{OperatorEqual: "=", OperatorNotEqual: "<>", OperatorGreater: ">", OperatorGreaterEq: ">=", OperatorLess: "<", OperatorLessEq: "<="}[node.Operator]
		placeholder := state.placeholder(values[0], definition.Public.Kind)
		if isTextKind(definition.Public.Kind) {
			return fmt.Sprintf("lower((%s)::text) %s lower(%s::text)", expression, operator, placeholder)
		}
		return fmt.Sprintf("(%s) %s %s", expression, operator, placeholder)
	}
}

func (state *compileState) placeholder(value string, kind ValueKind) string {
	position := state.bind(value)
	typeName := map[ValueKind]string{ValueInteger: "bigint", ValueDecimal: "numeric", ValueBoolean: "boolean",
		ValueCivilDate: "date", ValueTimestamp: "timestamptz"}[kind]
	if typeName == "" {
		typeName = "text"
	}
	return fmt.Sprintf("$%d::%s", position, typeName)
}

func (state *compileState) bind(value any) int {
	state.arguments = append(state.arguments, value)
	return len(state.arguments)
}

func expandSQL(value, prefix string) string {
	return strings.NewReplacer(
		"{root}", prefix,
		"{type}", prefix+"_type",
		"{owner}", prefix+"_owner",
		"{current}", prefix+"_current",
		"{holder}", prefix+"_holder",
	).Replace(value)
}

func expandRelationSQL(value, from, to string) string {
	replacer := strings.NewReplacer(
		"{from.root}", from, "{from.type}", from+"_type", "{from.owner}", from+"_owner",
		"{from.current}", from+"_current", "{from.holder}", from+"_holder",
		"{to.root}", to, "{to.type}", to+"_type", "{to.owner}", to+"_owner",
		"{to.current}", to+"_current", "{to.holder}", to+"_holder",
	)
	return replacer.Replace(value)
}

func containsOperator(values []Operator, expected Operator) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func operatorArity(operator Operator) (int, int) {
	switch operator {
	case OperatorIsNull, OperatorNotNull:
		return 0, 0
	case OperatorBetween:
		return 2, 2
	case OperatorIn:
		return 1, MaximumPredicateValues
	default:
		return 1, 1
	}
}

func validPredicateValue(value string, kind ValueKind) bool {
	if value == "" || utf8.RuneCountInString(value) > 500 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	switch kind {
	case ValueInteger:
		_, err := strconv.ParseInt(value, 10, 64)
		return err == nil
	case ValueDecimal:
		return decimalPattern.MatchString(value)
	case ValueBoolean:
		return value == "true" || value == "false"
	case ValueCivilDate:
		_, err := time.Parse("2006-01-02", value)
		return err == nil
	case ValueCivilMonth:
		return civilMonthPattern.MatchString(value)
	case ValueTimestamp:
		_, err := time.Parse(time.RFC3339, value)
		return err == nil
	default:
		return true
	}
}

func literalPattern(value string, contains bool) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
	if contains {
		return "%" + value + "%"
	}
	return value + "%"
}

func isTextKind(kind ValueKind) bool {
	switch kind {
	case ValueText, ValueLongText, ValueIdentifier, ValueCivilMonth, ValueEnum:
		return true
	default:
		return false
	}
}

func lowerPlaceholders(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, "lower("+value+")")
	}
	return result
}

func estimateCost(plan QueryPlan, nodes, relations int) int {
	base := maximumInt(1, len(plan.Projections)) * maximumInt(1, plan.MaximumRows)
	return base * maximumInt(1, nodes+1) * (relations*4 + 1)
}

func maximumInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func fingerprintString(value [32]byte) string { return hex.EncodeToString(value[:]) }

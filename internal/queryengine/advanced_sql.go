package queryengine

import (
	"fmt"
	"strings"
)

func (state *advancedCompileState) compileSelect(plan QueryPlan, includeLimit bool) (string, []ResultColumn, string, error) {
	root := state.catalog.Entities[plan.RootEntity]
	prefix := "v2q0"
	baseState := &compileState{catalog: state.catalog, arguments: state.arguments, validation: state.validation}
	where := make([]string, 0, 1+len(plan.Patterns))
	if root.BaseCondition != "" {
		where = append(where, expandSQL(root.BaseCondition, prefix))
	}
	if plan.Filter != nil {
		compiled := baseState.compileFilter(*plan.Filter, plan.RootEntity, prefix, 1, 0, "filter")
		if compiled != "" {
			where = append(where, compiled)
		}
	}
	state.arguments = baseState.arguments
	joins, aliasByEntity := baseState.projectionJoins(plan.RootEntity, prefix, append(append([]string{}, plan.Projections...), plan.GroupBy...), plan.Filter)
	state.arguments = baseState.arguments
	for index, pattern := range plan.Patterns {
		compiled, err := state.compilePattern(pattern, prefix)
		if err != nil {
			state.validation.add(fmt.Sprintf("patterns.%d", index), "invalid")
			continue
		}
		where = append(where, compiled)
	}
	if !state.validation.empty() {
		return "", nil, "", state.validation
	}
	selects := make([]string, 0, 3+len(plan.Projections)+len(plan.GroupBy)+len(plan.Aggregates))
	columns := make([]ResultColumn, 0, len(plan.Projections)+len(plan.GroupBy)+len(plan.Aggregates))
	groupExpressions := make([]string, 0, len(plan.GroupBy))
	groupLabels := make([]string, 0, len(plan.GroupBy))
	for position, key := range plan.GroupBy {
		field := state.catalog.Fields[key]
		expression := expandSQL(field.Expression, fieldPrefix(aliasByEntity, plan.RootEntity, prefix, field.Public.Entity))
		groupExpressions = append(groupExpressions, expression)
		groupLabels = append(groupLabels, fmt.Sprintf("coalesce((%s)::text, '<null>')", expression))
		selects = append(selects, fmt.Sprintf("(%s)::text AS value_%d", expression, position))
		columns = append(columns, ResultColumn{Position: position, FieldKey: key, Label: field.Public.Label, Kind: field.Public.Kind, Lineage: []ResultLineageRef{{Entity: plan.RootEntity, Field: key}}})
	}
	for position, key := range plan.Projections {
		field := state.catalog.Fields[key]
		expression := expandSQL(field.Expression, fieldPrefix(aliasByEntity, plan.RootEntity, prefix, field.Public.Entity))
		selects = append(selects, fmt.Sprintf("(%s)::text AS value_%d", expression, position))
		columns = append(columns, ResultColumn{Position: position, FieldKey: key, Label: field.Public.Label, Kind: field.Public.Kind, Lineage: []ResultLineageRef{{Entity: plan.RootEntity, Field: key}}})
	}
	aggregateExpressions := make(map[string]string, len(plan.Aggregates))
	for _, aggregate := range plan.Aggregates {
		expression, kind := state.aggregateExpression(aggregate, prefix)
		aggregateExpressions[aggregate.Key] = expression
		position := len(columns)
		selects = append(selects, fmt.Sprintf("(%s)::text AS value_%d", expression, position))
		lineage := []ResultLineageRef{{Entity: plan.RootEntity}}
		if aggregate.Field != "" {
			lineage[0].Field = aggregate.Field
		}
		columns = append(columns, ResultColumn{Position: position, FieldKey: aggregate.Field, AggregateKey: aggregate.Key, Label: aggregate.Key, Kind: kind, Lineage: lineage})
	}
	entityID := expandSQL(root.IDExpression, prefix)
	entityLabel := expandSQL(root.LabelExpression, prefix)
	entityUpdated := expandSQL(root.UpdatedExpression, prefix)
	if len(plan.GroupBy) > 0 || len(plan.Aggregates) > 0 {
		if len(groupLabels) == 0 {
			entityID = "'aggregate'"
			entityLabel = "'Aggregate result'"
		} else {
			entityID = "concat_ws(E'\\x1f', " + strings.Join(groupLabels, ", ") + ")"
			entityLabel = entityID
		}
		entityUpdated = "max(" + entityUpdated + ")"
	}
	selects = append([]string{entityID + " AS entity_id", entityLabel + " AS entity_label", entityUpdated + " AS entity_updated_at"}, selects...)
	query := "SELECT " + strings.Join(selects, ",\n       ") + "\nFROM " + expandSQL(root.FromTemplate, prefix)
	if len(joins) > 0 {
		query += "\n" + strings.Join(joins, "\n")
	}
	if len(where) > 0 {
		query += "\nWHERE " + strings.Join(where, " AND ")
	}
	if len(groupExpressions) > 0 {
		query += "\nGROUP BY " + strings.Join(groupExpressions, ", ")
	}
	if plan.Having != nil {
		compiled := state.compileHaving(*plan.Having, aggregateExpressions)
		if compiled != "" {
			query += "\nHAVING " + compiled
		}
	}
	order := state.compileAdvancedOrder(plan, prefix, aggregateExpressions)
	if len(order) == 0 {
		if len(groupExpressions) > 0 {
			for _, expression := range groupExpressions {
				order = append(order, expression+" ASC NULLS LAST")
			}
		} else {
			order = append(order, entityID+" ASC")
		}
	}
	query += "\nORDER BY " + strings.Join(order, ", ")
	if includeLimit {
		position := state.bind(plan.MaximumRows)
		query += fmt.Sprintf("\nLIMIT $%d::integer", position)
	}
	return query, columns, root.Public.Kind, nil
}

func (state *advancedCompileState) aggregateExpression(value Aggregate, prefix string) (string, ValueKind) {
	distinct := ""
	if value.Distinct {
		distinct = "DISTINCT "
	}
	if value.Function == AggregateCount && value.Field == "" {
		return "count(*)", ValueInteger
	}
	field := state.catalog.Fields[value.Field]
	expression := expandSQL(field.Expression, prefix)
	function := map[AggregateFunction]string{
		AggregateCount: "count", AggregateSum: "sum", AggregateAverage: "avg",
		AggregateMinimum: "min", AggregateMaximum: "max",
	}[value.Function]
	return fmt.Sprintf("%s(%s%s)", function, distinct, expression), aggregateOutputKind(value.Function, field.Public.Kind)
}

func (state *advancedCompileState) compilePattern(value PatternPredicate, prefix string) (string, error) {
	field := state.catalog.Fields[value.Field]
	expression := expandSQL(field.Expression, prefix)
	pattern, err := compilePatternExpression(value)
	if err != nil {
		return "", err
	}
	position := state.bind(pattern)
	operator := "~"
	if value.CaseFold {
		operator = "~*"
	}
	return fmt.Sprintf("coalesce((%s)::text, '') %s $%d::text", expression, operator, position), nil
}

func (state *advancedCompileState) compileHaving(node AggregateFilterNode, expressions map[string]string) string {
	if node.Predicate != nil {
		expression := expressions[node.Predicate.Aggregate]
		minimum, _ := operatorArity(node.Predicate.Operator)
		if minimum == 0 {
			if node.Predicate.Operator == OperatorIsNull {
				return "(" + expression + ") IS NULL"
			}
			return "(" + expression + ") IS NOT NULL"
		}
		values := make([]string, 0, len(node.Predicate.Values))
		for _, value := range node.Predicate.Values {
			position := state.bind(value)
			values = append(values, fmt.Sprintf("$%d::numeric", position))
		}
		var result string
		switch node.Predicate.Operator {
		case OperatorBetween:
			result = fmt.Sprintf("(%s) BETWEEN %s AND %s", expression, values[0], values[1])
		case OperatorIn:
			result = fmt.Sprintf("(%s) IN (%s)", expression, strings.Join(values, ","))
		default:
			operator := map[Operator]string{OperatorEqual: "=", OperatorNotEqual: "<>", OperatorGreater: ">", OperatorGreaterEq: ">=", OperatorLess: "<", OperatorLessEq: "<="}[node.Predicate.Operator]
			result = fmt.Sprintf("(%s) %s %s", expression, operator, values[0])
		}
		if node.Negated {
			return "NOT (" + result + ")"
		}
		return result
	}
	parts := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		compiled := state.compileHaving(child, expressions)
		if compiled != "" {
			parts = append(parts, compiled)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	result := "(" + strings.Join(parts, " "+string(node.Conjunction)+" ") + ")"
	if node.Negated {
		return "NOT " + result
	}
	return result
}

func (state *advancedCompileState) compileAdvancedOrder(plan QueryPlan, prefix string, aggregates map[string]string) []string {
	order := make([]string, 0, len(plan.Sort))
	for _, value := range plan.Sort {
		direction := "ASC"
		if value.Direction == SortDescending {
			direction = "DESC"
		}
		if expression := aggregates[value.Field]; expression != "" {
			order = append(order, expression+" "+direction+" NULLS LAST")
			continue
		}
		field := state.catalog.Fields[value.Field]
		order = append(order, orderExpression(expandSQL(field.Expression, prefix), field.Public.Kind, direction))
	}
	return order
}

func (state *advancedCompileState) compileSet(value SetExpression, maximumRows, depth int, path string) (string, []ResultColumn, string, int, error) {
	if depth > MaximumSetDepth {
		return "", nil, "", 0, ErrCostLimit
	}
	if value.Plan != nil {
		query, columns, kind, err := state.compileSelect(*value.Plan, false)
		return "(" + query + ")", columns, kind, 1, err
	}
	parts := make([]string, 0, len(value.Inputs))
	var columns []ResultColumn
	kind := ""
	inputs := 0
	for index, child := range value.Inputs {
		query, childColumns, childKind, childInputs, err := state.compileSet(child, maximumRows, depth+1, fmt.Sprintf("%s.inputs.%d", path, index))
		if err != nil {
			return "", nil, "", 0, err
		}
		if columns == nil {
			columns, kind = childColumns, childKind
		} else if !compatibleColumns(columns, childColumns) || kind != childKind {
			state.validation.add(path+".inputs", "incompatible_schema")
		}
		parts = append(parts, query)
		inputs += childInputs
	}
	operator := map[SetOperator]string{SetUnion: "UNION", SetIntersection: "INTERSECT", SetDifference: "EXCEPT"}[value.Operator]
	query := "(" + strings.Join(parts, "\n"+operator+"\n") + ")"
	if depth == 1 {
		position := state.bind(maximumRows)
		query = "SELECT * FROM " + query + " AS set_result ORDER BY entity_id ASC LIMIT " + fmt.Sprintf("$%d::integer", position)
	}
	return query, columns, kind, inputs, nil
}

func (state *advancedCompileState) compileCombination(value CombinationSpec, maximumRows int) (string, []ResultColumn, string, int, error) {
	ctes := make([]string, 0, len(value.Inputs))
	joins := make([]string, 0, len(value.Inputs))
	idParts := make([]string, 0, len(value.Inputs))
	labelParts := make([]string, 0, len(value.Inputs))
	updatedParts := make([]string, 0, len(value.Inputs))
	selects := make([]string, 0, len(value.Inputs)*2)
	columns := make([]ResultColumn, 0, len(value.Inputs)*2)
	product := 1
	for index, input := range value.Inputs {
		query, _, _, err := state.compileSelect(*input.Plan, false)
		if err != nil {
			return "", nil, "", 0, err
		}
		alias := fmt.Sprintf("c%d", index)
		limitPosition := state.bind(input.Plan.MaximumRows)
		ctes = append(ctes, fmt.Sprintf("%s AS (SELECT * FROM (%s) AS source_%d LIMIT $%d::integer)", alias, query, index, limitPosition))
		joins = append(joins, alias)
		idParts = append(idParts, alias+".entity_id")
		labelParts = append(labelParts, alias+".entity_label")
		updatedParts = append(updatedParts, alias+".entity_updated_at")
		position := len(columns)
		selects = append(selects, fmt.Sprintf("%s.entity_id::text AS value_%d", alias, position))
		columns = append(columns, ResultColumn{Position: position, FieldKey: input.Key + ".entity_id", Label: input.Key + " ID", Kind: ValueIdentifier, Lineage: []ResultLineageRef{{Entity: input.Plan.RootEntity}}})
		position = len(columns)
		selects = append(selects, fmt.Sprintf("%s.entity_label::text AS value_%d", alias, position))
		columns = append(columns, ResultColumn{Position: position, FieldKey: input.Key + ".entity_label", Label: input.Key, Kind: ValueText, Lineage: []ResultLineageRef{{Entity: input.Plan.RootEntity}}})
		product *= maximumInt(1, input.Plan.MaximumRows)
	}
	where := ""
	if value.RequireDistinctRows && len(value.Inputs) > 1 {
		checks := make([]string, 0, len(value.Inputs)*(len(value.Inputs)-1)/2)
		for left := 0; left < len(value.Inputs); left++ {
			for right := left + 1; right < len(value.Inputs); right++ {
				checks = append(checks, fmt.Sprintf("c%d.entity_id <> c%d.entity_id", left, right))
			}
		}
		where = "\nWHERE " + strings.Join(checks, " AND ")
	}
	limit := maximumRows
	if value.MaximumCombinations < limit {
		limit = value.MaximumCombinations
	}
	limitPosition := state.bind(limit)
	from := strings.Join(joins, " CROSS JOIN ") + where
	if len(value.Aggregates) > 0 {
		expressions := map[string]string{}
		selects = selects[:0]
		columns = nil
		for _, aggregate := range value.Aggregates {
			expression, kind := combinationAggregate(value, aggregate)
			expressions[aggregate.Key] = expression
			position := len(columns)
			selects = append(selects, fmt.Sprintf("(%s)::text AS value_%d", expression, position))
			columns = append(columns, ResultColumn{Position: position, FieldKey: aggregate.Field, AggregateKey: aggregate.Key, Label: aggregate.Key, Kind: kind})
		}
		having := ""
		if value.Having != nil {
			compiled := state.compileHaving(*value.Having, expressions)
			if compiled != "" {
				having = "\nHAVING " + compiled
			}
		}
		query := "WITH " + strings.Join(ctes, ",\n") + "\nSELECT 'aggregate' AS entity_id,\n       'Combinação' AS entity_label,\n       now() AS entity_updated_at,\n       " + strings.Join(selects, ",\n       ") + "\nFROM " + from + having + "\nLIMIT " + fmt.Sprintf("$%d::integer", limitPosition)
		return query, columns, "aggregate", product, nil
	}
	query := "WITH " + strings.Join(ctes, ",\n") + "\nSELECT concat_ws(E'\\x1f', " + strings.Join(idParts, ", ") + ") AS entity_id,\n       concat_ws(' + ', " + strings.Join(labelParts, ", ") + ") AS entity_label,\n       greatest(" + strings.Join(updatedParts, ", ") + ") AS entity_updated_at,\n       " + strings.Join(selects, ",\n       ") + "\nFROM " + from + "\nORDER BY entity_id ASC\nLIMIT " + fmt.Sprintf("$%d::integer", limitPosition)
	return query, columns, "combination", product, nil
}

func combinationAggregate(spec CombinationSpec, aggregate Aggregate) (string, ValueKind) {
	if aggregate.Function == AggregateCount && aggregate.Field == "" {
		return "count(*)", ValueInteger
	}
	field := aggregate.Field
	key := ""
	if strings.HasPrefix(field, "*:") {
		field = strings.TrimPrefix(field, "*:")
	} else if cut, name, ok := strings.Cut(field, ":"); ok {
		key, field = cut, name
	}
	parts := []string{}
	for index, input := range spec.Inputs {
		if input.Plan == nil || (key != "" && input.Key != key) {
			continue
		}
		for position, projection := range input.Plan.Projections {
			if projection == field {
				parts = append(parts, fmt.Sprintf("(c%d.value_%d)::numeric", index, position))
			}
		}
	}
	body := "0"
	if len(parts) > 0 {
		body = strings.Join(parts, " + ")
	}
	function := map[AggregateFunction]string{AggregateSum: "sum", AggregateAverage: "avg", AggregateMinimum: "min", AggregateMaximum: "max", AggregateCount: "count"}[aggregate.Function]
	if function == "" {
		function = "sum"
	}
	return function + "(" + body + ")", ValueDecimal
}

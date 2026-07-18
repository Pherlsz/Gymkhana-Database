package queryengine

import "sort"

func advancedCatalogFor(catalog resolvedCatalog) AdvancedCatalog {
	capabilities := make([]FieldCapability, 0, len(catalog.Fields))
	for _, field := range catalog.Fields {
		capability := FieldCapability{
			Field:              field.Public.Key,
			Groupable:          field.Public.Projectable && field.Public.Sortable,
			AggregateFunctions: aggregateFunctionsFor(field.Public.Kind),
			PatternGrammars:    patternGrammarsFor(field.Public.Kind),
		}
		capabilities = append(capabilities, capability)
	}
	sort.Slice(capabilities, func(left, right int) bool {
		return capabilities[left].Field < capabilities[right].Field
	})
	return AdvancedCatalog{
		FieldCapabilities: capabilities,
		AggregateFunctions: []AggregateFunctionDefinition{
			{Key: AggregateCount, Label: "Contagem", OutputKind: ValueInteger, AllowsDistinct: true, AllowsNullInput: true},
			{Key: AggregateSum, Label: "Soma", InputKinds: []ValueKind{ValueInteger, ValueDecimal}, OutputKind: ValueDecimal, AllowsDistinct: true, RequiresInputField: true},
			{Key: AggregateAverage, Label: "Média", InputKinds: []ValueKind{ValueInteger, ValueDecimal}, OutputKind: ValueDecimal, AllowsDistinct: true, RequiresInputField: true},
			{Key: AggregateMinimum, Label: "Mínimo", InputKinds: []ValueKind{ValueInteger, ValueDecimal, ValueCivilDate, ValueCivilMonth, ValueTimestamp, ValueText, ValueIdentifier}, OutputKind: ValueText, RequiresInputField: true},
			{Key: AggregateMaximum, Label: "Máximo", InputKinds: []ValueKind{ValueInteger, ValueDecimal, ValueCivilDate, ValueCivilMonth, ValueTimestamp, ValueText, ValueIdentifier}, OutputKind: ValueText, RequiresInputField: true},
		},
		SetOperators: []SetOperatorDefinition{
			{Key: SetUnion, Label: "União", MinimumInputs: 2, MaximumInputs: MaximumSetInputs},
			{Key: SetIntersection, Label: "Interseção", MinimumInputs: 2, MaximumInputs: MaximumSetInputs},
			{Key: SetDifference, Label: "Diferença", MinimumInputs: 2, MaximumInputs: 2},
		},
		PatternGrammars: []PatternGrammarDefinition{
			{Key: PatternLiteralSequence, Label: "Sequência literal", MaximumLength: MaximumPatternLength, MaximumTokens: MaximumPatternTokens, SupportsAnchoring: true, SupportsCaseFold: true},
			{Key: PatternCharacterClass, Label: "Conjunto de caracteres", MaximumLength: MaximumPatternLength, MaximumTokens: MaximumPatternTokens, SupportsAnchoring: true, SupportsCaseFold: true},
			{Key: PatternBinaryDigits, Label: "Dígitos binários", MaximumLength: MaximumPatternLength, MaximumTokens: MaximumPatternTokens, SupportsAnchoring: true},
			{Key: PatternDigits, Label: "Dígitos", MaximumLength: MaximumPatternLength, MaximumTokens: MaximumPatternTokens, SupportsAnchoring: true},
			{Key: PatternLetters, Label: "Letras", MaximumLength: MaximumPatternLength, MaximumTokens: MaximumPatternTokens, SupportsAnchoring: true, SupportsCaseFold: true},
			{Key: PatternAlphaNumeric, Label: "Letras e números", MaximumLength: MaximumPatternLength, MaximumTokens: MaximumPatternTokens, SupportsAnchoring: true, SupportsCaseFold: true},
		},
		Limits: AdvancedCatalogLimits{
			MaximumGroupKeys:             MaximumGroupKeys,
			MaximumAggregates:            MaximumAggregates,
			MaximumAggregateFilterNodes:  MaximumAggregateFilterNodes,
			MaximumSetInputs:             MaximumSetInputs,
			MaximumSetDepth:              MaximumSetDepth,
			MaximumPatternLength:         MaximumPatternLength,
			MaximumPatternTokens:         MaximumPatternTokens,
			MaximumCombinationDimensions: MaximumCombinationDimensions,
			MaximumCombinationSize:       MaximumCombinationSize,
		},
	}
}

func aggregateFunctionsFor(kind ValueKind) []AggregateFunction {
	values := []AggregateFunction{AggregateCount}
	switch kind {
	case ValueInteger, ValueDecimal:
		values = append(values, AggregateSum, AggregateAverage, AggregateMinimum, AggregateMaximum)
	case ValueCivilDate, ValueCivilMonth, ValueTimestamp, ValueText, ValueLongText, ValueIdentifier, ValueEnum:
		values = append(values, AggregateMinimum, AggregateMaximum)
	}
	return values
}

func patternGrammarsFor(kind ValueKind) []PatternGrammar {
	switch kind {
	case ValueText, ValueLongText, ValueIdentifier, ValueEnum, ValueCivilMonth:
		return []PatternGrammar{PatternLiteralSequence, PatternCharacterClass, PatternBinaryDigits, PatternDigits, PatternLetters, PatternAlphaNumeric}
	default:
		return nil
	}
}

func aggregateOutputKind(function AggregateFunction, input ValueKind) ValueKind {
	switch function {
	case AggregateCount:
		return ValueInteger
	case AggregateSum, AggregateAverage:
		return ValueDecimal
	case AggregateMinimum, AggregateMaximum:
		return input
	default:
		return ""
	}
}

package queryengine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func (state *advancedCompileState) bind(value any) int {
	state.arguments = append(state.arguments, value)
	return len(state.arguments)
}

func compilePatternExpression(value PatternPredicate) (string, error) {
	var body string
	switch value.Grammar {
	case PatternLiteralSequence:
		body = regexp.QuoteMeta(value.Pattern)
	case PatternBinaryDigits:
		body = tokenPattern(value.Pattern, "01", "[01]")
	case PatternDigits:
		body = tokenPattern(value.Pattern, "0123456789", "[0-9]")
	case PatternLetters:
		body = tokenPattern(value.Pattern, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", "[[:alpha:]]")
	case PatternAlphaNumeric:
		body = tokenPattern(value.Pattern, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", "[[:alnum:]]")
	case PatternCharacterClass:
		if value.Pattern == "" {
			return "", fmt.Errorf("empty character class")
		}
		var builder strings.Builder
		builder.WriteString("[")
		for _, character := range value.Pattern {
			if strings.ContainsRune(`\\^-]`, character) {
				builder.WriteRune('\\')
			}
			builder.WriteRune(character)
		}
		builder.WriteString("]+")
		body = builder.String()
	default:
		return "", fmt.Errorf("unsupported pattern grammar")
	}
	if value.Anchored {
		return "^(?:" + body + ")$", nil
	}
	return body, nil
}

func tokenPattern(value, literals, wildcard string) string {
	var builder strings.Builder
	for _, character := range value {
		if character == '?' {
			builder.WriteString(wildcard)
			continue
		}
		if strings.ContainsRune(literals, character) {
			builder.WriteString(regexp.QuoteMeta(string(character)))
		}
	}
	return builder.String()
}

func validPatternInput(grammar PatternGrammar, value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
		switch grammar {
		case PatternBinaryDigits:
			if character != '0' && character != '1' && character != '?' {
				return false
			}
		case PatternDigits:
			if (character < '0' || character > '9') && character != '?' {
				return false
			}
		case PatternLetters:
			if !unicode.IsLetter(character) && character != '?' {
				return false
			}
		case PatternAlphaNumeric:
			if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '?' {
				return false
			}
		}
	}
	return true
}

func patternTokenCount(value string) int { return utf8.RuneCountInString(value) }

func compatibleColumns(left, right []ResultColumn) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Kind != right[index].Kind || left[index].FieldKey != right[index].FieldKey || left[index].AggregateKey != right[index].AggregateKey {
			return false
		}
	}
	return true
}

func containsAggregate(values []AggregateFunction, expected AggregateFunction) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsPattern(values []PatternGrammar, expected PatternGrammar) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func setArity(operator SetOperator) (int, int) {
	switch operator {
	case SetUnion, SetIntersection:
		return 2, MaximumSetInputs
	case SetDifference:
		return 2, 2
	default:
		return 0, 0
	}
}

func canonicalSetKey(value SetExpression) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func canonicalAdvancedPlanJSON(plan QueryPlan) ([]byte, error) {
	return json.Marshal(plan)
}

func estimateAdvancedCost(plan QueryPlan, setInputs, combinationSize int) int {
	base := maximumInt(1, plan.MaximumRows)
	shape := maximumInt(1, len(plan.Projections)+len(plan.GroupBy)*2+len(plan.Aggregates)*4+len(plan.Patterns)*3)
	if setInputs > 0 {
		shape *= setInputs * 3
	}
	if combinationSize > 0 {
		shape *= maximumInt(1, combinationSize)
	}
	return base * shape
}

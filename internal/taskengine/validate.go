package taskengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

var (
	logicalIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	decimalValuePattern      = regexp.MustCompile(`^-?[0-9]{1,28}(\.[0-9]{1,10})?$`)
	civilMonthValuePattern   = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
)

type specIndexes struct {
	entities     map[string]queryengine.EntityDefinition
	fields       map[string]queryengine.FieldDefinition
	relations    map[string]queryengine.RelationDefinition
	capabilities map[string]queryengine.FieldCapability
	roles        map[string]CandidateRole
}

func NormalizeAndValidate(spec TaskSpec, catalog queryengine.Catalog, requireReviewed bool) (TaskSpec, string, error) {
	validation := &specValidation{}
	if spec.Version != SpecVersionV1 {
		validation.add("version", "unsupported")
	}
	spec.CatalogVersion = strings.TrimSpace(spec.CatalogVersion)
	if spec.CatalogVersion == "" || spec.CatalogVersion != catalog.Version {
		return TaskSpec{}, "", ErrStaleCatalog
	}
	if spec.State != SpecProposed && spec.State != SpecReviewed {
		validation.add("state", "unsupported")
	}
	if requireReviewed && spec.State != SpecReviewed {
		validation.add("state", "review_required")
	}
	if len(spec.Roles) < 1 || len(spec.Roles) > MaximumRoles {
		validation.add("roles", "out_of_range")
	}
	if len(spec.Requirements) > MaximumRequirements {
		validation.add("requirements", "too_many")
	}
	if len(spec.Constraints) > MaximumConstraints {
		validation.add("constraints", "too_many")
	}
	if len(spec.Ambiguities) > MaximumAmbiguities {
		validation.add("ambiguities", "too_many")
	}
	if requireReviewed && len(spec.Ambiguities) > 0 {
		return TaskSpec{}, "", ErrUnresolved
	}
	indexes := buildIndexes(catalog)
	spec.Roles = normalizeRoles(spec.Roles, indexes, validation)
	indexes.roles = make(map[string]CandidateRole, len(spec.Roles))
	for _, role := range spec.Roles {
		indexes.roles[role.Key] = role
	}
	spec.Requirements = normalizeRequirements(spec.Requirements, indexes, validation)
	spec.Constraints = normalizeConstraints(spec.Constraints, indexes, validation)
	spec.Ambiguities = normalizeAmbiguities(spec.Ambiguities, validation)
	normalizeGoal(&spec.Goal, validation)
	if len(validation.Fields) > 0 {
		return TaskSpec{}, "", validation
	}
	sort.Slice(spec.Roles, func(left, right int) bool { return spec.Roles[left].Key < spec.Roles[right].Key })
	sort.Slice(spec.Requirements, func(left, right int) bool { return spec.Requirements[left].Key < spec.Requirements[right].Key })
	sort.Slice(spec.Constraints, func(left, right int) bool { return spec.Constraints[left].Key < spec.Constraints[right].Key })
	sort.Slice(spec.Ambiguities, func(left, right int) bool { return spec.Ambiguities[left].Key < spec.Ambiguities[right].Key })
	encoded, err := json.Marshal(spec)
	if err != nil {
		return TaskSpec{}, "", fmt.Errorf("encode task specification: %w", err)
	}
	fingerprint := sha256.Sum256(encoded)
	return spec, hex.EncodeToString(fingerprint[:]), nil
}

type specFieldError struct {
	Field string
	Code  string
}

type specValidation struct {
	Fields []specFieldError
}

func (value *specValidation) Error() string { return ErrInvalidSpec.Error() }
func (value *specValidation) Unwrap() error { return ErrInvalidSpec }
func (value *specValidation) add(field, code string) {
	value.Fields = append(value.Fields, specFieldError{Field: field, Code: code})
}

func buildIndexes(catalog queryengine.Catalog) specIndexes {
	indexes := specIndexes{
		entities:     make(map[string]queryengine.EntityDefinition, len(catalog.Entities)),
		fields:       make(map[string]queryengine.FieldDefinition, len(catalog.Fields)),
		relations:    make(map[string]queryengine.RelationDefinition, len(catalog.Relations)),
		capabilities: make(map[string]queryengine.FieldCapability),
	}
	for _, value := range catalog.Entities {
		indexes.entities[value.Key] = value
	}
	for _, value := range catalog.Fields {
		indexes.fields[value.Key] = value
	}
	for _, value := range catalog.Relations {
		indexes.relations[value.Key] = value
	}
	if catalog.Advanced != nil {
		for _, value := range catalog.Advanced.FieldCapabilities {
			indexes.capabilities[value.Field] = value
		}
	}
	return indexes
}

func normalizeRoles(values []CandidateRole, indexes specIndexes, validation *specValidation) []CandidateRole {
	seen := make(map[string]struct{}, len(values))
	result := make([]CandidateRole, 0, len(values))
	for index, value := range values {
		path := fmt.Sprintf("roles.%d", index)
		value.Key = strings.TrimSpace(value.Key)
		value.Entity = strings.TrimSpace(value.Entity)
		if !logicalIdentifierPattern.MatchString(value.Key) {
			validation.add(path+".key", "invalid")
			continue
		}
		if _, duplicate := seen[value.Key]; duplicate {
			validation.add(path+".key", "duplicate")
			continue
		}
		seen[value.Key] = struct{}{}
		if _, ok := indexes.entities[value.Entity]; !ok {
			validation.add(path+".entity", "unsupported")
			continue
		}
		if value.MinimumCount == 0 {
			value.MinimumCount = 1
		}
		if value.MaximumCount == 0 {
			value.MaximumCount = value.MinimumCount
		}
		if value.MinimumCount < 1 || value.MaximumCount < value.MinimumCount || value.MaximumCount > MaximumRoleCount {
			validation.add(path, "count_out_of_range")
			continue
		}
		result = append(result, value)
	}
	return result
}

func normalizeRequirements(values []Requirement, indexes specIndexes, validation *specValidation) []Requirement {
	seen := make(map[string]struct{}, len(values))
	result := make([]Requirement, 0, len(values))
	for index, value := range values {
		path := fmt.Sprintf("requirements.%d", index)
		value.Key = strings.TrimSpace(value.Key)
		value.Role = strings.TrimSpace(value.Role)
		if !logicalIdentifierPattern.MatchString(value.Key) {
			validation.add(path+".key", "invalid")
			continue
		}
		if _, duplicate := seen[value.Key]; duplicate {
			validation.add(path+".key", "duplicate")
			continue
		}
		seen[value.Key] = struct{}{}
		role, ok := indexes.roles[value.Role]
		if !ok {
			validation.add(path+".role", "unsupported")
			continue
		}
		field, ok := resolveBinding(role.Entity, &value.Binding, indexes, validation, path+".binding")
		if !ok {
			continue
		}
		if value.Pattern != nil {
			if value.Operator != "" || len(value.Values) > 0 {
				validation.add(path, "invalid_shape")
				continue
			}
			if len(value.Binding.RelationPath) > 0 {
				validation.add(path+".pattern", "relation_pattern_unsupported")
				continue
			}
			value.Pattern.Value = normalizeText(value.Pattern.Value)
			capability := indexes.capabilities[field.Key]
			if !containsPattern(capability.PatternGrammars, value.Pattern.Grammar) {
				validation.add(path+".pattern.grammar", "unsupported")
				continue
			}
			if value.Pattern.Value == "" || utf8.RuneCountInString(value.Pattern.Value) > queryengine.MaximumPatternLength || !validPatternValue(value.Pattern.Grammar, value.Pattern.Value) {
				validation.add(path+".pattern.value", "invalid")
				continue
			}
		} else {
			if !containsOperator(field.Operators, value.Operator) {
				validation.add(path+".operator", "unsupported")
				continue
			}
			minimum, maximum := operatorArity(value.Operator)
			if len(value.Values) < minimum || len(value.Values) > maximum || len(value.Values) > MaximumRequirementValues {
				validation.add(path+".values", "wrong_arity")
				continue
			}
			normalized := make([]string, 0, len(value.Values))
			valid := true
			for valueIndex, raw := range value.Values {
				raw = normalizeText(raw)
				if !validTypedValue(raw, field.Kind) {
					validation.add(fmt.Sprintf("%s.values.%d", path, valueIndex), "invalid_type")
					valid = false
				}
				normalized = append(normalized, raw)
			}
			if !valid {
				continue
			}
			value.Values = normalized
		}
		result = append(result, value)
	}
	return result
}

func normalizeConstraints(values []Constraint, indexes specIndexes, validation *specValidation) []Constraint {
	seen := make(map[string]struct{}, len(values))
	result := make([]Constraint, 0, len(values))
	for index, value := range values {
		path := fmt.Sprintf("constraints.%d", index)
		value.Key = strings.TrimSpace(value.Key)
		if !logicalIdentifierPattern.MatchString(value.Key) {
			validation.add(path+".key", "invalid")
			continue
		}
		if _, duplicate := seen[value.Key]; duplicate {
			validation.add(path+".key", "duplicate")
			continue
		}
		seen[value.Key] = struct{}{}
		leftKind, leftOK := normalizeOperand(&value.Left, indexes, validation, path+".left")
		if !leftOK {
			continue
		}
		switch value.Kind {
		case ConstraintMembership:
			if value.Right != nil || len(value.Values) < 1 || len(value.Values) > MaximumRequirementValues {
				validation.add(path, "invalid_shape")
				continue
			}
			for valueIndex := range value.Values {
				value.Values[valueIndex] = normalizeText(value.Values[valueIndex])
				if !validTypedValue(value.Values[valueIndex], leftKind) {
					validation.add(fmt.Sprintf("%s.values.%d", path, valueIndex), "invalid_type")
				}
			}
		case ConstraintDistinct:
			if value.Right == nil || len(value.Values) > 0 {
				validation.add(path, "invalid_shape")
				continue
			}
			if _, ok := indexes.roles[value.Right.Role]; !ok {
				validation.add(path+".right.role", "unsupported")
				continue
			}
		case ConstraintEqual, ConstraintNotEqual, ConstraintBefore, ConstraintAfter:
			if value.Right == nil || len(value.Values) > 0 {
				validation.add(path, "invalid_shape")
				continue
			}
			rightKind, rightOK := normalizeOperand(value.Right, indexes, validation, path+".right")
			if !rightOK || !comparableKinds(leftKind, rightKind, value.Kind) {
				validation.add(path, "incompatible_types")
				continue
			}
		default:
			validation.add(path+".kind", "unsupported")
			continue
		}
		result = append(result, value)
	}
	return result
}

func normalizeOperand(value *ConstraintOperand, indexes specIndexes, validation *specValidation, path string) (queryengine.ValueKind, bool) {
	value.Role = strings.TrimSpace(value.Role)
	role, ok := indexes.roles[value.Role]
	if !ok {
		validation.add(path+".role", "unsupported")
		return "", false
	}
	if value.Binding == nil {
		value.Kind = queryengine.ValueIdentifier
		return value.Kind, true
	}
	field, ok := resolveBinding(role.Entity, value.Binding, indexes, validation, path+".binding")
	if !ok {
		return "", false
	}
	value.Kind = field.Kind
	return field.Kind, true
}

func resolveBinding(entity string, binding *FieldBinding, indexes specIndexes, validation *specValidation, path string) (queryengine.FieldDefinition, bool) {
	binding.Field = strings.TrimSpace(binding.Field)
	current := entity
	for index := range binding.RelationPath {
		binding.RelationPath[index] = strings.TrimSpace(binding.RelationPath[index])
		relation, ok := indexes.relations[binding.RelationPath[index]]
		if !ok || relation.FromEntity != current {
			validation.add(fmt.Sprintf("%s.relation_path.%d", path, index), "unsupported")
			return queryengine.FieldDefinition{}, false
		}
		current = relation.ToEntity
	}
	field, ok := indexes.fields[binding.Field]
	if !ok || field.Entity != current || !field.Filterable {
		validation.add(path+".field", "unsupported")
		return queryengine.FieldDefinition{}, false
	}
	return field, true
}

func normalizeAmbiguities(values []Ambiguity, validation *specValidation) []Ambiguity {
	seen := make(map[string]struct{}, len(values))
	result := make([]Ambiguity, 0, len(values))
	for index, value := range values {
		path := fmt.Sprintf("ambiguities.%d", index)
		value.Key = strings.TrimSpace(value.Key)
		value.Code = strings.TrimSpace(value.Code)
		if !logicalIdentifierPattern.MatchString(value.Key) || !logicalIdentifierPattern.MatchString(value.Code) {
			validation.add(path, "invalid")
			continue
		}
		if _, duplicate := seen[value.Key]; duplicate {
			validation.add(path+".key", "duplicate")
			continue
		}
		seen[value.Key] = struct{}{}
		for candidateIndex := range value.Candidates {
			value.Candidates[candidateIndex] = strings.TrimSpace(value.Candidates[candidateIndex])
		}
		sort.Strings(value.Candidates)
		result = append(result, value)
	}
	return result
}

func normalizeGoal(value *CompositionGoal, validation *specValidation) {
	if value.MinimumSolutions == 0 {
		value.MinimumSolutions = 1
	}
	if value.MaximumSolutions == 0 {
		value.MaximumSolutions = 10
	}
	if value.MinimumSolutions < 1 || value.MaximumSolutions < value.MinimumSolutions || value.MaximumSolutions > MaximumSolutions {
		validation.add("goal", "out_of_range")
	}
}

func normalizeText(value string) string {
	value = strings.ToValidUTF8(value, "")
	return strings.TrimSpace(value)
}

func containsOperator(values []queryengine.Operator, expected queryengine.Operator) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsPattern(values []queryengine.PatternGrammar, expected queryengine.PatternGrammar) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func operatorArity(operator queryengine.Operator) (int, int) {
	switch operator {
	case queryengine.OperatorIsNull, queryengine.OperatorNotNull:
		return 0, 0
	case queryengine.OperatorBetween:
		return 2, 2
	case queryengine.OperatorIn:
		return 1, MaximumRequirementValues
	default:
		return 1, 1
	}
}

func validTypedValue(value string, kind queryengine.ValueKind) bool {
	if value == "" || utf8.RuneCountInString(value) > 500 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	switch kind {
	case queryengine.ValueInteger:
		_, err := strconv.ParseInt(value, 10, 64)
		return err == nil
	case queryengine.ValueDecimal:
		return decimalValuePattern.MatchString(value)
	case queryengine.ValueBoolean:
		return value == "true" || value == "false"
	case queryengine.ValueCivilDate:
		_, err := time.Parse("2006-01-02", value)
		return err == nil
	case queryengine.ValueCivilMonth:
		return civilMonthValuePattern.MatchString(value)
	case queryengine.ValueTimestamp:
		_, err := time.Parse(time.RFC3339, value)
		return err == nil
	default:
		return true
	}
}

func validPatternValue(grammar queryengine.PatternGrammar, value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
		switch grammar {
		case queryengine.PatternBinaryDigits:
			if character != '0' && character != '1' && character != '?' {
				return false
			}
		case queryengine.PatternDigits:
			if (character < '0' || character > '9') && character != '?' {
				return false
			}
		case queryengine.PatternLetters:
			if !unicode.IsLetter(character) && character != '?' {
				return false
			}
		case queryengine.PatternAlphaNumeric:
			if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '?' {
				return false
			}
		}
	}
	return true
}

func comparableKinds(left, right queryengine.ValueKind, constraint ConstraintKind) bool {
	if left == right {
		return true
	}
	if constraint == ConstraintBefore || constraint == ConstraintAfter {
		numeric := func(value queryengine.ValueKind) bool {
			return value == queryengine.ValueInteger || value == queryengine.ValueDecimal
		}
		return numeric(left) && numeric(right)
	}
	return false
}

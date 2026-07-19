package taskengine

import (
	"fmt"
	"sort"

	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

type CandidatePlan struct {
	Role              string                `json:"role"`
	Plan              queryengine.QueryPlan `json:"plan"`
	RequirementKeys   []string              `json:"requirement_keys"`
	ProjectedBindings []FieldBinding        `json:"projected_bindings"`
}

func BuildCandidatePlans(spec TaskSpec, catalog queryengine.Catalog) ([]CandidatePlan, error) {
	normalized, _, err := NormalizeAndValidate(spec, catalog, true)
	if err != nil {
		return nil, err
	}
	plans := make([]CandidatePlan, 0, len(normalized.Roles))
	for _, role := range normalized.Roles {
		filters := make([]queryengine.FilterNode, 0)
		patterns := make([]queryengine.PatternPredicate, 0)
		projections := make(map[string]FieldBinding)
		requirementKeys := make([]string, 0)
		for _, requirement := range normalized.Requirements {
			if requirement.Role != role.Key {
				continue
			}
			requirementKeys = append(requirementKeys, requirement.Key)
			if len(requirement.Binding.RelationPath) == 0 {
				projections[requirement.Binding.Field] = requirement.Binding
			}
			if requirement.Optional {
				continue
			}
			if requirement.Pattern != nil {
				patterns = append(patterns, queryengine.PatternPredicate{
					Field: requirement.Binding.Field, Grammar: requirement.Pattern.Grammar,
					Pattern: requirement.Pattern.Value, Anchored: requirement.Pattern.Anchored,
					CaseFold: requirement.Pattern.CaseFold,
				})
				continue
			}
			filters = append(filters, requirementFilter(requirement))
		}
		for _, constraint := range normalized.Constraints {
			for _, operand := range constraintOperands(constraint) {
				if operand.Role == role.Key && operand.Binding != nil && len(operand.Binding.RelationPath) == 0 {
					projections[operand.Binding.Field] = *operand.Binding
				}
			}
		}
		projectionKeys := make([]string, 0, len(projections))
		projectedBindings := make([]FieldBinding, 0, len(projections))
		for field, binding := range projections {
			projectionKeys = append(projectionKeys, field)
			projectedBindings = append(projectedBindings, binding)
		}
		sort.Strings(projectionKeys)
		sort.Slice(projectedBindings, func(left, right int) bool { return projectedBindings[left].Field < projectedBindings[right].Field })
		sort.Strings(requirementKeys)
		var filter *queryengine.FilterNode
		if len(filters) == 1 {
			filter = &filters[0]
		} else if len(filters) > 1 {
			value := queryengine.FilterNode{Kind: queryengine.FilterGroup, Conjunction: queryengine.ConjunctionAnd, Children: filters}
			filter = &value
		}
		plans = append(plans, CandidatePlan{
			Role: role.Key,
			Plan: queryengine.QueryPlan{
				Version: queryengine.PlanVersionV2, CatalogVersion: catalog.Version,
				RootEntity: role.Entity, Projections: projectionKeys, Filter: filter,
				Patterns: patterns, MaximumRows: DefaultCandidateRows,
			},
			RequirementKeys: requirementKeys, ProjectedBindings: projectedBindings,
		})
	}
	return plans, nil
}

func requirementFilter(value Requirement) queryengine.FilterNode {
	node := queryengine.FilterNode{
		Kind: queryengine.FilterPredicate, Field: value.Binding.Field,
		Operator: value.Operator, Values: append([]string(nil), value.Values...),
	}
	for index := len(value.Binding.RelationPath) - 1; index >= 0; index-- {
		node = queryengine.FilterNode{Kind: queryengine.FilterRelation, Relation: value.Binding.RelationPath[index], Children: []queryengine.FilterNode{node}}
	}
	return node
}

func constraintOperands(value Constraint) []ConstraintOperand {
	result := []ConstraintOperand{value.Left}
	if value.Right != nil {
		result = append(result, *value.Right)
	}
	return result
}

func ValidateCandidatePlans(plans []CandidatePlan, expectedRoles int) error {
	if len(plans) != expectedRoles {
		return fmt.Errorf("candidate plans: %w", ErrInvalidSpec)
	}
	return nil
}

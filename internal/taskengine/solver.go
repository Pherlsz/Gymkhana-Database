package taskengine

import (
	"context"
	"encoding/json"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

func Solve(ctx context.Context, spec TaskSpec, sets []CandidateSet, limits SolveLimits) (SolveResult, error) {
	limits = normalizeSolveLimits(limits, spec)
	if spec.State != SpecReviewed || len(spec.Ambiguities) > 0 || len(spec.Roles) == 0 {
		return SolveResult{}, ErrInvalidSpec
	}
	setsByRole := make(map[string][]Candidate, len(sets))
	for _, set := range sets {
		setsByRole[set.Role] = append(setsByRole[set.Role], set.Candidates...)
	}
	roles := append([]CandidateRole(nil), spec.Roles...)
	sort.Slice(roles, func(left, right int) bool { return roles[left].Key < roles[right].Key })
	candidates := make(map[string][]Candidate, len(roles))
	rejected := 0
	candidateLimited := false
	for _, role := range roles {
		if len(setsByRole[role.Key]) > limits.MaximumCandidatesPerRole {
			candidateLimited = true
		}
		prepared, rejectedForRole, err := prepareCandidates(role, spec.Requirements, setsByRole[role.Key], limits)
		if err != nil {
			return SolveResult{}, err
		}
		rejected += rejectedForRole
		if len(prepared) < role.MinimumCount {
			return SolveResult{State: SolveNoSolution, Rejected: rejected}, nil
		}
		candidates[role.Key] = prepared
	}

	result := SolveResult{State: SolveComplete, Compositions: make([]Composition, 0), Rejected: rejected}
	selected := make(map[string][]Candidate, len(roles))
	stop := false
	var visitRole func(int)
	visitRole = func(roleIndex int) {
		if stop {
			return
		}
		if err := ctx.Err(); err != nil {
			result.State, result.LimitCode, stop = SolveCancelled, "cancelled", true
			return
		}
		if result.Branches >= limits.MaximumBranches {
			result.State, result.LimitCode, stop = SolveIncomplete, "branch_limit", true
			return
		}
		if roleIndex == len(roles) {
			satisfied, evidence := evaluateConstraints(spec.Constraints, selected)
			if !satisfied {
				result.Rejected++
				return
			}
			composition := Composition{Position: len(result.Compositions), Selected: cloneSelection(selected), Evidence: collectEvidence(selected, evidence)}
			if len(composition.Evidence) > limits.MaximumEvidence {
				result.State, result.LimitCode, stop = SolveIncomplete, "evidence_limit", true
				return
			}
			encoded, _ := json.Marshal(composition)
			if len(encoded) > limits.MaximumBytes {
				result.State, result.LimitCode, stop = SolveIncomplete, "byte_limit", true
				return
			}
			result.Compositions = append(result.Compositions, composition)
			if len(result.Compositions) >= limits.MaximumCompositions || len(result.Compositions) >= spec.Goal.MaximumSolutions {
				stop = true
			}
			return
		}
		role := roles[roleIndex]
		available := candidates[role.Key]
		maximum := role.MaximumCount
		if maximum > len(available) {
			maximum = len(available)
		}
		for count := role.MinimumCount; count <= maximum && !stop; count++ {
			chooseCandidateSubset(ctx, available, count, limits.MaximumBranches, &result.Branches, func(choice []Candidate) bool {
				selected[role.Key] = choice
				if constraintsReadyAndSatisfied(spec.Constraints, selected) {
					visitRole(roleIndex + 1)
				} else {
					result.Rejected++
				}
				delete(selected, role.Key)
				return !stop
			})
			if result.Branches >= limits.MaximumBranches && !stop {
				result.State, result.LimitCode, stop = SolveIncomplete, "branch_limit", true
			}
		}
	}
	visitRole(0)
	if ctx.Err() != nil && result.State == SolveComplete {
		result.State, result.LimitCode = SolveCancelled, "cancelled"
	}
	if result.State == SolveComplete && candidateLimited {
		result.State, result.LimitCode = SolveIncomplete, "candidate_limit"
	}
	if result.State == SolveComplete && len(result.Compositions) == 0 {
		result.State = SolveNoSolution
	}
	return result, nil
}

func normalizeSolveLimits(value SolveLimits, spec TaskSpec) SolveLimits {
	if value.MaximumBranches <= 0 {
		value.MaximumBranches = 100_000
	}
	if value.MaximumCompositions <= 0 || value.MaximumCompositions > MaximumSolutions {
		value.MaximumCompositions = spec.Goal.MaximumSolutions
	}
	if value.MaximumCandidatesPerRole <= 0 || value.MaximumCandidatesPerRole > MaximumCandidateRows {
		value.MaximumCandidatesPerRole = MaximumCandidateRows
	}
	if value.MaximumEvidence <= 0 {
		value.MaximumEvidence = MaximumEvidencePerResult
	}
	if value.MaximumBytes <= 0 {
		value.MaximumBytes = 5 << 20
	}
	return value
}

func prepareCandidates(role CandidateRole, requirements []Requirement, values []Candidate, limits SolveLimits) ([]Candidate, int, error) {
	values = append([]Candidate(nil), values...)
	sort.Slice(values, func(left, right int) bool {
		if values[left].Entity != values[right].Entity {
			return values[left].Entity < values[right].Entity
		}
		return values[left].ID < values[right].ID
	})
	if len(values) > limits.MaximumCandidatesPerRole {
		values = values[:limits.MaximumCandidatesPerRole]
	}
	required := make([]string, 0)
	for _, requirement := range requirements {
		if requirement.Role == role.Key && !requirement.Optional {
			required = append(required, requirement.Key)
		}
	}
	sort.Strings(required)
	seen := make(map[string]struct{}, len(values))
	result := make([]Candidate, 0, len(values))
	rejected := 0
	for _, candidate := range values {
		if candidate.Entity != role.Entity || !safeCandidateValue(candidate.ID, 200) || !safeCandidateValue(candidate.Label, 500) {
			rejected++
			continue
		}
		key := candidate.Entity + "\x00" + candidate.ID
		if _, duplicate := seen[key]; duplicate {
			rejected++
			continue
		}
		seen[key] = struct{}{}
		if !hasRequirementEvidence(candidate, role.Key, required) {
			rejected++
			continue
		}
		for field, raw := range candidate.Values {
			if !safeCandidateValue(field, 200) || !safeCandidateValue(raw, 1000) {
				return nil, rejected, ErrInvalidSpec
			}
		}
		sort.Slice(candidate.Evidence, func(left, right int) bool {
			if candidate.Evidence[left].Requirement != candidate.Evidence[right].Requirement {
				return candidate.Evidence[left].Requirement < candidate.Evidence[right].Requirement
			}
			return candidate.Evidence[left].SourceID < candidate.Evidence[right].SourceID
		})
		result = append(result, candidate)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Entity != result[right].Entity {
			return result[left].Entity < result[right].Entity
		}
		return result[left].ID < result[right].ID
	})
	return result, rejected, nil
}

func hasRequirementEvidence(candidate Candidate, role string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	found := make(map[string]bool, len(required))
	for _, evidence := range candidate.Evidence {
		if evidence.Role == role && evidence.Satisfied && evidence.Requirement != "" && evidence.SourceEntity != "" && evidence.SourceID != "" {
			found[evidence.Requirement] = true
		}
	}
	for _, key := range required {
		if !found[key] {
			return false
		}
	}
	return true
}

func chooseCandidateSubset(ctx context.Context, values []Candidate, count, maximumBranches int, branches *int, visit func([]Candidate) bool) bool {
	choice := make([]Candidate, 0, count)
	var choose func(int) bool
	choose = func(start int) bool {
		if ctx.Err() != nil || *branches >= maximumBranches {
			return false
		}
		if len(choice) == count {
			copyValue := append([]Candidate(nil), choice...)
			return visit(copyValue)
		}
		remaining := count - len(choice)
		for index := start; index <= len(values)-remaining; index++ {
			*branches++
			choice = append(choice, values[index])
			if !choose(index + 1) {
				choice = choice[:len(choice)-1]
				return false
			}
			choice = choice[:len(choice)-1]
		}
		return true
	}
	return choose(0)
}

func constraintsReadyAndSatisfied(constraints []Constraint, selected map[string][]Candidate) bool {
	for _, constraint := range constraints {
		if _, ok := selected[constraint.Left.Role]; !ok {
			continue
		}
		if constraint.Right != nil {
			if _, ok := selected[constraint.Right.Role]; !ok {
				continue
			}
		}
		satisfied, _ := evaluateConstraint(constraint, selected)
		if !satisfied {
			return false
		}
	}
	return true
}

func evaluateConstraints(constraints []Constraint, selected map[string][]Candidate) (bool, []Evidence) {
	evidence := make([]Evidence, 0, len(constraints))
	for _, constraint := range constraints {
		satisfied, item := evaluateConstraint(constraint, selected)
		if !satisfied {
			return false, nil
		}
		evidence = append(evidence, item)
	}
	return true, evidence
}

func evaluateConstraint(constraint Constraint, selected map[string][]Candidate) (bool, Evidence) {
	left := operandValues(constraint.Left, selected)
	evidence := Evidence{Constraint: constraint.Key, Role: constraint.Left.Role, Satisfied: false, Code: "constraint_failed"}
	if len(left) > 0 {
		evidence.SourceEntity, evidence.SourceID = left[0].candidate.Entity, left[0].candidate.ID
		if constraint.Left.Binding != nil {
			evidence.Field = constraint.Left.Binding.Field
		}
	}
	var satisfied bool
	switch constraint.Kind {
	case ConstraintMembership:
		allowed := make(map[string]struct{}, len(constraint.Values))
		for _, value := range constraint.Values {
			allowed[value] = struct{}{}
		}
		satisfied = len(left) > 0
		for _, value := range left {
			if _, ok := allowed[value.value]; !ok {
				satisfied = false
				break
			}
		}
	case ConstraintDistinct:
		right := operandValues(*constraint.Right, selected)
		satisfied = distinctCandidateValues(left, right)
	case ConstraintEqual:
		right := operandValues(*constraint.Right, selected)
		satisfied = equalValueSets(left, right)
	case ConstraintNotEqual:
		right := operandValues(*constraint.Right, selected)
		satisfied = disjointValueSets(left, right)
	case ConstraintBefore:
		right := operandValues(*constraint.Right, selected)
		satisfied = orderedValueSets(left, right, constraint.Left.Kind, -1)
	case ConstraintAfter:
		right := operandValues(*constraint.Right, selected)
		satisfied = orderedValueSets(left, right, constraint.Left.Kind, 1)
	}
	evidence.Satisfied = satisfied
	if satisfied {
		evidence.Code = "constraint_satisfied"
	}
	return satisfied, evidence
}

type operandValue struct {
	value     string
	candidate Candidate
}

func operandValues(operand ConstraintOperand, selected map[string][]Candidate) []operandValue {
	candidates := selected[operand.Role]
	result := make([]operandValue, 0, len(candidates))
	for _, candidate := range candidates {
		value := candidate.ID
		if operand.Binding != nil {
			value = candidate.Values[operand.Binding.Field]
		}
		result = append(result, operandValue{value: value, candidate: candidate})
	}
	return result
}

func distinctCandidateValues(left, right []operandValue) bool {
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		seen[value.candidate.Entity+"\x00"+value.candidate.ID] = struct{}{}
	}
	for _, value := range right {
		if _, duplicate := seen[value.candidate.Entity+"\x00"+value.candidate.ID]; duplicate {
			return false
		}
	}
	return len(left) > 0 && len(right) > 0
}

func equalValueSets(left, right []operandValue) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	leftSet, rightSet := make(map[string]struct{}, len(left)), make(map[string]struct{}, len(right))
	for _, value := range left {
		leftSet[value.value] = struct{}{}
	}
	for _, value := range right {
		rightSet[value.value] = struct{}{}
	}
	if len(leftSet) != len(rightSet) {
		return false
	}
	for value := range leftSet {
		if _, ok := rightSet[value]; !ok {
			return false
		}
	}
	return true
}

func disjointValueSets(left, right []operandValue) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, value := range left {
		seen[value.value] = struct{}{}
	}
	for _, value := range right {
		if _, duplicate := seen[value.value]; duplicate {
			return false
		}
	}
	return true
}

func orderedValueSets(left, right []operandValue, kind queryengine.ValueKind, direction int) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	for _, leftValue := range left {
		for _, rightValue := range right {
			comparison, ok := compareTyped(leftValue.value, rightValue.value, kind)
			if !ok || (direction < 0 && comparison >= 0) || (direction > 0 && comparison <= 0) {
				return false
			}
		}
	}
	return true
}

func compareTyped(left, right string, kind queryengine.ValueKind) (int, bool) {
	switch kind {
	case queryengine.ValueInteger:
		leftValue, leftErr := strconv.ParseInt(left, 10, 64)
		rightValue, rightErr := strconv.ParseInt(right, 10, 64)
		if leftErr != nil || rightErr != nil {
			return 0, false
		}
		if leftValue < rightValue {
			return -1, true
		}
		if leftValue > rightValue {
			return 1, true
		}
		return 0, true
	case queryengine.ValueDecimal:
		leftValue, leftOK := new(big.Rat).SetString(left)
		rightValue, rightOK := new(big.Rat).SetString(right)
		if !leftOK || !rightOK {
			return 0, false
		}
		return leftValue.Cmp(rightValue), true
	case queryengine.ValueTimestamp:
		leftValue, leftErr := time.Parse(time.RFC3339, left)
		rightValue, rightErr := time.Parse(time.RFC3339, right)
		if leftErr != nil || rightErr != nil {
			return 0, false
		}
		if leftValue.Before(rightValue) {
			return -1, true
		}
		if leftValue.After(rightValue) {
			return 1, true
		}
		return 0, true
	default:
		return strings.Compare(left, right), true
	}
}

func collectEvidence(selected map[string][]Candidate, constraintEvidence []Evidence) []Evidence {
	roles := make([]string, 0, len(selected))
	for role := range selected {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	result := make([]Evidence, 0)
	for _, role := range roles {
		for _, candidate := range selected[role] {
			result = append(result, candidate.Evidence...)
		}
	}
	result = append(result, constraintEvidence...)
	return result
}

func cloneSelection(selected map[string][]Candidate) map[string][]Candidate {
	result := make(map[string][]Candidate, len(selected))
	for role, candidates := range selected {
		result[role] = append([]Candidate(nil), candidates...)
	}
	return result
}

func safeCandidateValue(value string, maximum int) bool {
	if value == "" || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

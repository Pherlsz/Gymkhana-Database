package taskengine

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

type InterpreterInput struct {
	TaskText string
	Catalog  queryengine.Catalog
}

type Interpreter interface {
	Interpret(context.Context, InterpreterInput) (Proposal, error)
}

type DisabledInterpreter struct{}

func (DisabledInterpreter) Interpret(context.Context, InterpreterInput) (Proposal, error) {
	return Proposal{}, ErrInterpreterDisabled
}

// FixtureInterpreter is the deterministic acceptance adapter. It performs no
// network access and recognizes only explicitly registered normalized fixtures.
type FixtureInterpreter struct {
	Fixtures map[string]TaskSpec
}

func (value FixtureInterpreter) Interpret(ctx context.Context, input InterpreterInput) (Proposal, error) {
	if err := ctx.Err(); err != nil {
		return Proposal{}, err
	}
	key, err := normalizeTaskText(input.TaskText)
	if err != nil {
		return Proposal{}, err
	}
	spec, ok := value.Fixtures[key]
	if !ok {
		return Proposal{}, ErrUnsupportedTask
	}
	spec.Version = SpecVersionV1
	spec.CatalogVersion = input.Catalog.Version
	spec.State = SpecProposed
	normalized, _, err := NormalizeAndValidate(spec, input.Catalog, false)
	if err != nil {
		return Proposal{}, fmt.Errorf("%w: %v", ErrInterpreterMalformed, err)
	}
	return Proposal{Spec: normalized, Interpreter: "fixture-v1", RequiresHumanReview: true}, nil
}

func NewFixtureInterpreter(fixtures map[string]TaskSpec) (FixtureInterpreter, error) {
	normalized := make(map[string]TaskSpec, len(fixtures))
	for task, spec := range fixtures {
		key, err := normalizeTaskText(task)
		if err != nil {
			return FixtureInterpreter{}, err
		}
		if _, duplicate := normalized[key]; duplicate {
			return FixtureInterpreter{}, ErrInterpreterMalformed
		}
		normalized[key] = spec
	}
	return FixtureInterpreter{Fixtures: normalized}, nil
}

func normalizeTaskText(value string) (string, error) {
	value = strings.ToValidUTF8(value, "")
	if utf8.RuneCountInString(value) == 0 || utf8.RuneCountInString(value) > MaximumTaskTextRunes {
		return "", ErrTaskTooLarge
	}
	for _, character := range value {
		if unicode.IsControl(character) && !unicode.IsSpace(character) {
			return "", ErrTaskTooLarge
		}
	}
	return strings.ToLower(strings.Join(strings.Fields(value), " ")), nil
}

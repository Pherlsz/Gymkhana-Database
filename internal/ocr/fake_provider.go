package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	coreocr "github.com/Pherlsz/Gymkhana-Core/ocr"
)

type FakeExtractionStep struct {
	Output ExtractionOutput
	Err    error
}

type FakeExtractor struct {
	mu       sync.Mutex
	steps    []FakeExtractionStep
	requests []coreocr.ExtractionRequest
}

func NewFakeExtractor(steps ...FakeExtractionStep) *FakeExtractor {
	return &FakeExtractor{steps: append([]FakeExtractionStep(nil), steps...)}
}

func NewDeterministicFakeExtractor() *FakeExtractor {
	return &FakeExtractor{}
}

func (extractor *FakeExtractor) Extract(ctx context.Context, input ExtractionInput) (ExtractionOutput, error) {
	if err := ctx.Err(); err != nil {
		return ExtractionOutput{}, err
	}
	extractor.mu.Lock()
	defer extractor.mu.Unlock()
	extractor.requests = append(extractor.requests, input.Request)
	if len(extractor.steps) > 0 {
		step := extractor.steps[0]
		extractor.steps = extractor.steps[1:]
		return step.Output, step.Err
	}
	result, err := deterministicCoreResult(input)
	if err != nil {
		return ExtractionOutput{}, err
	}
	return ExtractionOutput{Result: result, Usage: 1}, nil
}

func (extractor *FakeExtractor) Requests() []coreocr.ExtractionRequest {
	extractor.mu.Lock()
	defer extractor.mu.Unlock()
	return append([]coreocr.ExtractionRequest(nil), extractor.requests...)
}

func deterministicCoreResult(input ExtractionInput) (coreocr.ExtractionResult, error) {
	if len(input.Request.Sources) == 0 {
		return coreocr.ExtractionResult{}, errors.New("fake OCR extractor requires a Core source")
	}
	if len(input.Fields) == 0 {
		return coreocr.ExtractionResult{
			Mode:           coreocr.ModeSchemaGuided,
			StructuredData: json.RawMessage(`{}`),
			Validation:     coreocr.ValidationValid,
			Review:         coreocr.ReviewUnreviewed,
		}, nil
	}
	structured := make(map[string]any, len(input.Fields))
	for _, field := range input.Fields {
		value, err := fakeJSONValue(field.Kind)
		if err != nil {
			return coreocr.ExtractionResult{}, err
		}
		structured[field.Key] = value
	}
	encoded, err := json.Marshal(structured)
	if err != nil {
		return coreocr.ExtractionResult{}, err
	}
	field := input.Fields[0]
	value, err := fakeJSONValue(field.Kind)
	if err != nil {
		return coreocr.ExtractionResult{}, err
	}
	rawValue, err := json.Marshal(value)
	if err != nil {
		return coreocr.ExtractionResult{}, err
	}
	confidence := coreocr.Confidence(9000)
	source := input.Request.Sources[0]
	evidence := coreocr.EvidenceRef{SourceID: source.ID}
	if source.Modality == coreocr.SourceDocument {
		evidence.Page = 1
	}
	return coreocr.ExtractionResult{
		Mode: coreocr.ModeSchemaGuided,
		Observations: []coreocr.Observation{{
			ID:         "obs-1",
			Evidence:   []coreocr.EvidenceRef{evidence},
			RawText:    "Evidência sintética",
			Confidence: &confidence,
		}},
		Candidates: []coreocr.FieldCandidate{{
			Path:           fieldPointer(field.Key),
			State:          coreocr.ValuePresent,
			Value:          rawValue,
			ObservationIDs: []string{"obs-1"},
			Basis:          coreocr.BasisObserved,
			Confidence:     &confidence,
			Validation:     coreocr.ValidationValid,
			Review:         coreocr.ReviewUnreviewed,
		}},
		StructuredData: encoded,
		Validation:     coreocr.ValidationValid,
		Review:         coreocr.ReviewUnreviewed,
	}, nil
}

func fakeJSONValue(kind ValueKind) (any, error) {
	value := fakeValue(kind)
	if value == "" {
		return nil, errors.New("fake OCR extractor has no value for field kind")
	}
	switch kind {
	case ValueInteger:
		return 1, nil
	case ValueBoolean:
		return true, nil
	default:
		return value, nil
	}
}

func fakeValue(kind ValueKind) string {
	switch kind {
	case ValueText, ValueLongText:
		return "Valor extraído"
	case ValueInteger:
		return "1"
	case ValueDecimal:
		return "100.00"
	case ValueBoolean:
		return "true"
	case ValueCivilDate:
		return "2026-07-18"
	case ValueCivilMonth:
		return "2026-07"
	case ValueEmail:
		return "ocr@example.com"
	case ValuePhone:
		return "+5511999999999"
	default:
		return ""
	}
}

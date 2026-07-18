package ocr

import (
	"context"
	"errors"
	"sync"
)

type FakeExtractionStep struct {
	Response ExtractionResponse
	Err      error
}

type FakeExtractor struct {
	mu       sync.Mutex
	steps    []FakeExtractionStep
	requests []ExtractionRequest
}

func NewFakeExtractor(steps ...FakeExtractionStep) *FakeExtractor {
	return &FakeExtractor{steps: append([]FakeExtractionStep(nil), steps...)}
}

func NewDeterministicFakeExtractor() *FakeExtractor {
	return &FakeExtractor{}
}

func (extractor *FakeExtractor) Extract(ctx context.Context, request ExtractionRequest) (ExtractionResponse, error) {
	if err := ctx.Err(); err != nil {
		return ExtractionResponse{}, err
	}
	extractor.mu.Lock()
	defer extractor.mu.Unlock()
	request.Source = nil
	extractor.requests = append(extractor.requests, request)
	if len(extractor.steps) > 0 {
		step := extractor.steps[0]
		extractor.steps = extractor.steps[1:]
		return step.Response, step.Err
	}
	if len(request.Fields) == 0 {
		return ExtractionResponse{}, nil
	}
	field := request.Fields[0]
	value := fakeValue(field.Kind)
	if value == "" {
		return ExtractionResponse{}, errors.New("fake OCR extractor has no value for field kind")
	}
	confidence := 900
	return ExtractionResponse{
		Suggestions: []ProviderSuggestion{{
			FieldKey: field.Key, Value: value,
			Evidence: Evidence{Page: 1, Excerpt: "Evidência sintética", Confidence: &confidence},
		}},
		Usage: 1,
	}, nil
}

func (extractor *FakeExtractor) Requests() []ExtractionRequest {
	extractor.mu.Lock()
	defer extractor.mu.Unlock()
	return append([]ExtractionRequest(nil), extractor.requests...)
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

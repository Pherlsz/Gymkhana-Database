package modelprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Core/assistant"
	coreocr "github.com/Pherlsz/Gymkhana-Core/ocr"
	"github.com/Pherlsz/Gymkhana-Database/internal/ocr"
)

const ocrInstruction = "Extraia somente os campos visíveis neste documento. Omita o que não aparecer. Não invente valores. Responda só JSON."

// OCRExtractor implements ocr.Extractor over Gemini using the shared Administração key.
type OCRExtractor struct {
	adapter      *GoogleAdapter
	keys         *KeyService
	provider     string
	defaultModel string
}

func NewOCRExtractor(adapter *GoogleAdapter, keys *KeyService, defaultModel string) (*OCRExtractor, error) {
	if adapter == nil || keys == nil || defaultModel == "" {
		return nil, ErrInvalidInput
	}
	return &OCRExtractor{adapter: adapter, keys: keys, provider: ProviderGoogle, defaultModel: defaultModel}, nil
}

func NewGoogleOCRExtractor(client *http.Client, keys *KeyService, defaultModel string) (*OCRExtractor, error) {
	adapter, err := NewGoogleAdapter(client, KeyResolver(keys, ProviderGoogle), "")
	if err != nil {
		return nil, err
	}
	return NewOCRExtractor(adapter, keys, defaultModel)
}

func (extractor *OCRExtractor) Extract(ctx context.Context, input ocr.ExtractionInput) (ocr.ExtractionOutput, error) {
	if err := ctx.Err(); err != nil {
		return ocr.ExtractionOutput{}, err
	}
	if input.Source == nil || len(input.Request.Sources) == 0 {
		return ocr.ExtractionOutput{}, ocr.ErrMalformedProvider
	}
	media, err := io.ReadAll(io.LimitReader(input.Source, ocr.MaximumSourceBytes+1))
	if err != nil {
		return ocr.ExtractionOutput{}, err
	}
	if len(media) == 0 || int64(len(media)) > ocr.MaximumSourceBytes {
		return ocr.ExtractionOutput{}, ocr.ErrUnsafeSource
	}
	_, model, err := extractor.keys.Resolve(ctx, extractor.provider)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return ocr.ExtractionOutput{}, ocr.ErrUnavailable
		}
		return ocr.ExtractionOutput{}, ocr.ErrUnavailable
	}
	if model == "" {
		model = extractor.defaultModel
	}
	text, usage, err := extractor.adapter.generateStructured(
		ctx, SharedCredential(extractor.provider), model, ocrInstruction,
		input.Request.Sources[0].MediaType, media, openExtractionSchema(input.Request.TargetSchema),
	)
	if err != nil {
		return ocr.ExtractionOutput{}, mapOCRError(extractor.adapter, err)
	}
	result, err := extractionResultFromJSON(input, structuredJSON(text))
	if err != nil {
		return ocr.ExtractionOutput{}, err
	}
	return ocr.ExtractionOutput{Result: result, Usage: usage}, nil
}

func mapOCRError(adapter *GoogleAdapter, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, ErrNotConfigured) {
		return ocr.ErrUnavailable
	}
	switch adapter.ClassifyFailure(err) {
	case assistant.FailureTimeout:
		return ocr.ErrTimeout
	case assistant.FailureInvalid:
		return ocr.ErrMalformedProvider
	default:
		return ocr.ErrUnavailable
	}
}

func openExtractionSchema(raw json.RawMessage) json.RawMessage {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return raw
	}
	delete(schema, "required")
	encoded, err := json.Marshal(schema)
	if err != nil {
		return raw
	}
	return encoded
}

func structuredJSON(text string) []byte {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	return []byte(strings.TrimSpace(text))
}

func extractionResultFromJSON(input ocr.ExtractionInput, raw []byte) (coreocr.ExtractionResult, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil {
		return coreocr.ExtractionResult{}, ocr.ErrMalformedProvider
	}
	source := input.Request.Sources[0]
	structured := make(map[string]any, len(input.Fields))
	observations := make([]coreocr.Observation, 0, len(input.Fields))
	candidates := make([]coreocr.FieldCandidate, 0, len(input.Fields))
	confidence := coreocr.Confidence(8000)
	for index, field := range input.Fields {
		value, ok := object[field.Key]
		if !ok || value == nil {
			continue
		}
		switch value.(type) {
		case string, json.Number, bool:
		default:
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return coreocr.ExtractionResult{}, ocr.ErrMalformedProvider
		}
		observationID := "obs-" + strconv.Itoa(index+1)
		excerpt := observationExcerpt(value)
		evidence := coreocr.EvidenceRef{SourceID: source.ID}
		if source.Modality == coreocr.SourceDocument {
			evidence.Page = 1
		}
		observations = append(observations, coreocr.Observation{
			ID: observationID, Evidence: []coreocr.EvidenceRef{evidence},
			RawText: excerpt, Confidence: &confidence,
		})
		candidates = append(candidates, coreocr.FieldCandidate{
			Path:           jsonPointer(field.Key),
			State:          coreocr.ValuePresent,
			Value:          encoded,
			ObservationIDs: []string{observationID},
			Basis:          coreocr.BasisObserved,
			Confidence:     &confidence,
			Validation:     coreocr.ValidationValid,
			Review:         coreocr.ReviewUnreviewed,
		})
		structured[field.Key] = value
	}
	encoded, err := json.Marshal(structured)
	if err != nil {
		return coreocr.ExtractionResult{}, ocr.ErrMalformedProvider
	}
	result := coreocr.ExtractionResult{
		Mode: coreocr.ModeSchemaGuided, Observations: observations, Candidates: candidates,
		StructuredData: encoded, Validation: coreocr.ValidationValid, Review: coreocr.ReviewUnreviewed,
	}
	if err := coreocr.ValidateExtractionExchange(input.Request, result); err != nil {
		result.Validation = coreocr.ValidationInvalid
		if err := coreocr.ValidateExtractionExchange(input.Request, result); err != nil {
			return coreocr.ExtractionResult{}, ocr.ErrMalformedProvider
		}
	}
	return result, nil
}

func observationExcerpt(value any) string {
	if text, ok := value.(string); ok {
		return clipRunes(strings.TrimSpace(text), ocr.MaximumEvidenceRunes)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return clipRunes(string(encoded), ocr.MaximumEvidenceRunes)
}

func jsonPointer(key string) string {
	return "/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(key)
}

func clipRunes(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

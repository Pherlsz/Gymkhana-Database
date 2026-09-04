package ocr

import (
	"bytes"
	"encoding/json"
	"strings"

	coreocr "github.com/Pherlsz/Gymkhana-Core/ocr"
)

func suggestionsFromCore(request coreocr.ExtractionRequest, output ExtractionOutput, catalog Catalog, pageCount int) ([]CompleteSuggestionInput, error) {
	if output.Usage < 0 || output.Usage > MaximumProviderUsage {
		return nil, ErrMalformedProvider
	}
	if err := coreocr.ValidateExtractionExchange(request, output.Result); err != nil {
		return nil, ErrMalformedProvider
	}
	if len(output.Result.Candidates) > MaximumSuggestions {
		return nil, ErrMalformedProvider
	}
	mapped := make([]providerCandidate, 0, len(output.Result.Candidates))
	for _, candidate := range output.Result.Candidates {
		if candidate.State != coreocr.ValuePresent {
			continue
		}
		item, err := providerCandidateFromCore(candidate, output.Result.Observations, pageCount)
		if err != nil {
			return nil, err
		}
		mapped = append(mapped, item)
	}
	return normalizeProviderSuggestions(mapped, catalog, pageCount)
}

func providerCandidateFromCore(candidate coreocr.FieldCandidate, observations []coreocr.Observation, pageCount int) (providerCandidate, error) {
	key, ok := fieldKeyFromPointer(candidate.Path)
	if !ok || key == "" {
		return providerCandidate{}, ErrMalformedProvider
	}
	value, err := portableValueText(candidate.Value)
	if err != nil {
		return providerCandidate{}, err
	}
	evidence, err := evidenceFromCore(candidate, observations, pageCount)
	if err != nil {
		return providerCandidate{}, err
	}
	return providerCandidate{FieldKey: key, Value: value, Evidence: evidence}, nil
}

func evidenceFromCore(candidate coreocr.FieldCandidate, observations []coreocr.Observation, pageCount int) (Evidence, error) {
	if len(candidate.ObservationIDs) == 0 {
		return Evidence{}, ErrMalformedProvider
	}
	index := make(map[string]coreocr.Observation, len(observations))
	for _, observation := range observations {
		index[observation.ID] = observation
	}
	observation, ok := index[candidate.ObservationIDs[0]]
	if !ok || len(observation.Evidence) == 0 {
		return Evidence{}, ErrMalformedProvider
	}
	ref := observation.Evidence[0]
	page := ref.Page
	if page == 0 {
		if pageCount != 1 {
			return Evidence{}, ErrMalformedProvider
		}
		page = 1
	}
	evidence := Evidence{Page: page, Excerpt: strings.TrimSpace(observation.RawText)}
	if ref.Region != nil {
		evidence.Region = &Region{X: ref.Region.X, Y: ref.Region.Y, Width: ref.Region.Width, Height: ref.Region.Height}
	}
	confidence := candidate.Confidence
	if confidence == nil {
		confidence = observation.Confidence
	}
	if confidence != nil {
		value := int(*confidence)
		evidence.Confidence = &value
	}
	return evidence, nil
}

func portableValueText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", ErrMalformedProvider
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", ErrMalformedProvider
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case json.Number:
		return typed.String(), nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	default:
		return "", ErrMalformedProvider
	}
}

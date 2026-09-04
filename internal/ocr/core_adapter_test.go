package ocr

import (
	"context"
	"encoding/json"
	"testing"

	coreocr "github.com/Pherlsz/Gymkhana-Core/ocr"
	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
)

func TestCoreExtractionRequestIsSchemaGuidedAndClosed(t *testing.T) {
	field := FieldSchema{
		Key: "bill.printed_holder_name", Label: "Titular impresso", Kind: ValueText,
		Target: TargetReference{Kind: TargetBill, ID: Identifier{3}}, TargetVersion: 1,
	}
	request, err := coreExtractionRequest(attachment.Identifier{2}, "image/png", Catalog{Fields: []FieldSchema{field}})
	if err != nil {
		t.Fatalf("coreExtractionRequest() error = %v", err)
	}
	if err := coreocr.ValidateExtractionRequest(request); err != nil {
		t.Fatalf("ValidateExtractionRequest() error = %v", err)
	}
	if request.Mode != coreocr.ModeSchemaGuided || request.Sources[0].Modality != coreocr.SourceImage {
		t.Fatalf("request = %#v", request)
	}
	var schema map[string]any
	if err := json.Unmarshal(request.TargetSchema, &schema); err != nil {
		t.Fatalf("TargetSchema JSON = %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if _, ok := properties[field.Key]; !ok {
		t.Fatalf("target schema properties = %#v, want dotted field key", properties)
	}
	if fieldPointer(field.Key) != "/bill.printed_holder_name" {
		t.Fatalf("fieldPointer() = %q", fieldPointer(field.Key))
	}
}

func TestFakeExtractorReturnsValidCoreExchange(t *testing.T) {
	field := FieldSchema{
		Key: "bill.printed_holder_name", Label: "Titular impresso", Kind: ValueText,
		Target: TargetReference{Kind: TargetBill, ID: Identifier{3}}, TargetVersion: 1,
	}
	request, err := coreExtractionRequest(attachment.Identifier{2}, "image/png", Catalog{Fields: []FieldSchema{field}})
	if err != nil {
		t.Fatalf("coreExtractionRequest() error = %v", err)
	}
	output, err := NewDeterministicFakeExtractor().Extract(context.Background(), ExtractionInput{
		Request: request,
		Fields:  []FieldSchema{field},
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if err := coreocr.ValidateExtractionExchange(request, output.Result); err != nil {
		t.Fatalf("ValidateExtractionExchange() error = %v", err)
	}
	suggestions, err := suggestionsFromCore(request, output, Catalog{Fields: []FieldSchema{field}}, 1)
	if err != nil {
		t.Fatalf("suggestionsFromCore() error = %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].ProposedValue != "Valor extraído" ||
		suggestions[0].Evidence.Page != 1 || suggestions[0].Evidence.Excerpt != "Evidência sintética" ||
		suggestions[0].Evidence.Confidence == nil || *suggestions[0].Evidence.Confidence != 9000 {
		t.Fatalf("suggestions = %#v", suggestions)
	}
}

func TestSuggestionsFromCoreMapsMillionthRegionAndRejectsMalformedExchange(t *testing.T) {
	field := FieldSchema{
		Key: "bill.amount", Label: "Valor", Kind: ValueDecimal,
		Target: TargetReference{Kind: TargetBill, ID: Identifier{1}}, TargetVersion: 4,
	}
	request, err := coreExtractionRequest(attachment.Identifier{2}, "application/pdf", Catalog{Fields: []FieldSchema{field}})
	if err != nil {
		t.Fatalf("coreExtractionRequest() error = %v", err)
	}
	confidence := coreocr.Confidence(8750)
	region := coreocr.NormalizedRect{X: 100000, Y: 200000, Width: 400000, Height: 150000}
	output := ExtractionOutput{
		Usage: 12,
		Result: coreocr.ExtractionResult{
			Mode: coreocr.ModeSchemaGuided,
			Observations: []coreocr.Observation{{
				ID: "obs-1",
				Evidence: []coreocr.EvidenceRef{{
					SourceID: request.Sources[0].ID, Page: 1, Region: &region,
				}},
				RawText:    "  100.00  ",
				Confidence: &confidence,
			}},
			Candidates: []coreocr.FieldCandidate{{
				Path:           "/bill.amount",
				State:          coreocr.ValuePresent,
				Value:          json.RawMessage(`"100.00"`),
				ObservationIDs: []string{"obs-1"},
				Basis:          coreocr.BasisObserved,
				Confidence:     &confidence,
				Validation:     coreocr.ValidationValid,
				Review:         coreocr.ReviewUnreviewed,
			}},
			StructuredData: json.RawMessage(`{"bill.amount":"100.00"}`),
			Validation:     coreocr.ValidationValid,
			Review:         coreocr.ReviewUnreviewed,
		},
	}
	suggestions, err := suggestionsFromCore(request, output, Catalog{Fields: []FieldSchema{field}}, 1)
	if err != nil {
		t.Fatalf("suggestionsFromCore() error = %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].ProposedValue != "100" ||
		suggestions[0].Evidence.Region == nil || *suggestions[0].Evidence.Region != (Region{X: 100000, Y: 200000, Width: 400000, Height: 150000}) ||
		suggestions[0].Evidence.Confidence == nil || *suggestions[0].Evidence.Confidence != 8750 {
		t.Fatalf("suggestions = %#v", suggestions)
	}

	malformed := output
	malformed.Result.Mode = coreocr.ModeDiscovery
	malformed.Result.StructuredData = nil
	malformed.Result.Validation = coreocr.ValidationNotValidated
	if _, err := suggestionsFromCore(request, malformed, Catalog{Fields: []FieldSchema{field}}, 1); err != ErrMalformedProvider {
		t.Fatalf("malformed exchange error = %v, want ErrMalformedProvider", err)
	}
	if _, err := suggestionsFromCore(request, ExtractionOutput{Usage: MaximumProviderUsage + 1}, Catalog{Fields: []FieldSchema{field}}, 1); err != ErrMalformedProvider {
		t.Fatalf("usage overflow error = %v, want ErrMalformedProvider", err)
	}
}

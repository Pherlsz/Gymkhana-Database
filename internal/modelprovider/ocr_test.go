package modelprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	coreocr "github.com/Pherlsz/Gymkhana-Core/ocr"
	"github.com/Pherlsz/Gymkhana-Database/internal/ocr"
)

func TestOCRExtractorUsesSharedKeyAndClosedSchema(t *testing.T) {
	var captured map[string]json.RawMessage
	server := geminiTestServer(t, func(payload map[string]json.RawMessage) string {
		captured = payload
		return `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"{\"bill.printed_holder_name\":\"Maria Silva\"}"}]}}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":4}}`
	})
	t.Cleanup(server.Close)

	keys, _ := newKeyFixture(t)
	if _, err := keys.Set(context.Background(), adminSession(), ProviderGoogle, "test-secret-0123456789ab", "gemini-test"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	adapter, err := NewGoogleAdapter(server.Client(), KeyResolver(keys, ProviderGoogle), server.URL)
	if err != nil {
		t.Fatalf("NewGoogleAdapter() error = %v", err)
	}
	extractor, err := NewOCRExtractor(adapter, keys, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("NewOCRExtractor() error = %v", err)
	}

	field := ocr.FieldSchema{Key: "bill.printed_holder_name", Label: "Titular impresso", Kind: ocr.ValueText}
	request := coreocr.ExtractionRequest{
		Mode: coreocr.ModeSchemaGuided,
		Sources: []coreocr.SourceRef{{
			ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Modality: coreocr.SourceImage, MediaType: "image/png",
		}},
		TargetSchema:  json.RawMessage(`{"type":"object","properties":{"bill.printed_holder_name":{"type":"string"}},"required":["bill.printed_holder_name"],"additionalProperties":false}`),
		MaxCandidates: ocr.MaximumSuggestions,
	}
	output, err := extractor.Extract(context.Background(), ocr.ExtractionInput{
		Request: request,
		Fields:  []ocr.FieldSchema{field},
		Source:  bytes.NewReader([]byte("fake-png-bytes")),
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if output.Usage != 16 {
		t.Fatalf("usage = %d, want 16", output.Usage)
	}
	if err := coreocr.ValidateExtractionExchange(request, output.Result); err != nil {
		t.Fatalf("ValidateExtractionExchange() error = %v", err)
	}
	if len(output.Result.Candidates) != 1 || string(output.Result.Candidates[0].Value) != `"Maria Silva"` {
		t.Fatalf("candidates = %#v", output.Result.Candidates)
	}
	if output.Result.Observations[0].Evidence[0].Page != 0 {
		t.Fatalf("image evidence must omit page: %#v", output.Result.Observations[0].Evidence)
	}

	var config map[string]any
	if err := json.Unmarshal(captured["generationConfig"], &config); err != nil {
		t.Fatalf("generationConfig = %s", captured["generationConfig"])
	}
	schema, _ := json.Marshal(config["responseJsonSchema"])
	if strings.Contains(string(schema), `"required"`) {
		t.Fatalf("Gemini schema still requires every field: %s", schema)
	}
	if !bytes.Contains(captured["contents"], []byte(`"inlineData"`)) || bytes.Contains(captured["contents"], []byte("fake-png-bytes")) {
		t.Fatalf("source must be sent as inlineData, not raw bytes: %s", captured["contents"])
	}
}

func TestOCRExtractorFailsClosedWithoutSharedKey(t *testing.T) {
	keys, _ := newKeyFixture(t)
	adapter, err := NewGoogleAdapter(http.DefaultClient, KeyResolver(keys, ProviderGoogle), "http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("NewGoogleAdapter() error = %v", err)
	}
	extractor, err := NewOCRExtractor(adapter, keys, "gemini-2.5-flash")
	if err != nil {
		t.Fatalf("NewOCRExtractor() error = %v", err)
	}
	_, err = extractor.Extract(context.Background(), ocr.ExtractionInput{
		Request: coreocr.ExtractionRequest{Sources: []coreocr.SourceRef{{ID: "src", Modality: coreocr.SourceImage, MediaType: "image/png"}}},
		Source:  bytes.NewReader([]byte("png")),
	})
	if !errors.Is(err, ocr.ErrUnavailable) {
		t.Fatalf("Extract() error = %v, want ErrUnavailable", err)
	}
}

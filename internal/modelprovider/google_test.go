package modelprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Pherlsz/Gymkhana-Core/assistant"
	"github.com/Pherlsz/Gymkhana-Core/assistant/adaptertest"
)

func staticResolver(secret string) SecretResolver {
	return func(_ context.Context, credential *assistant.CredentialRef) (string, error) {
		if credential == nil {
			return "", ErrNotConfigured
		}
		return secret, nil
	}
}

func geminiTestServer(t *testing.T, onGenerate func(map[string]json.RawMessage) string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.Header.Get("x-goog-api-key"), "test-secret") {
			http.Error(writer, `{"error":{"code":401,"status":"UNAUTHENTICATED"}}`, http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1beta/models":
			_, _ = writer.Write([]byte(`{"models":[{"name":"models/gemini-test","displayName":"Gemini Test","inputTokenLimit":32768,"outputTokenLimit":8192,"supportedGenerationMethods":["generateContent"]},{"name":"models/embedding-001","supportedGenerationMethods":["embedContent"]}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/v1beta/models/gemini-test:generateContent":
			var payload map[string]json.RawMessage
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				http.Error(writer, "invalid test request", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(onGenerate(payload)))
		default:
			http.NotFound(writer, request)
		}
	}))
}

func TestGoogleAdapterConformance(t *testing.T) {
	server := geminiTestServer(t, func(payload map[string]json.RawMessage) string {
		if len(payload["tools"]) > 0 {
			return `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"functionCall":{"name":"lookup_weather","args":{"city":"Porto Alegre"}}}]}}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":3}}`
		}
		if len(payload["generationConfig"]) > 0 {
			return `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"{\"ok\":true}"}]}}],"usageMetadata":{"promptTokenCount":6,"candidatesTokenCount":4}}`
		}
		return `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"OK"}]}}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1}}`
	})
	defer server.Close()

	adapter, err := NewGoogleAdapter(server.Client(), staticResolver("test-secret"), server.URL)
	if err != nil {
		t.Fatalf("NewGoogleAdapter() error = %v", err)
	}
	toolSchema := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`)
	adaptertest.Run(t, adaptertest.Config{
		Adapter:    adapter,
		Credential: SharedCredential(ProviderGoogle),
		Model:      assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-test"},
		Probes: []adaptertest.Probe{{
			Name:                 "tool-calling",
			RequiredCapabilities: []assistant.Capability{assistant.CapabilityToolCalling},
			Messages:             []assistant.Message{textMessage(assistant.RoleUser, "What is the weather?")},
			Tools:                []assistant.ToolDefinition{{Name: "lookup_weather", Description: "Look up weather", InputSchema: toolSchema}},
			Check: func(response assistant.GenerationResponse) error {
				if response.FinishReason != assistant.FinishToolCalls || len(response.Message.Content) != 1 || response.Message.Content[0].ToolCall == nil {
					return errors.New("expected one normalized tool call")
				}
				return nil
			},
		}, {
			Name:                 "structured-output",
			RequiredCapabilities: []assistant.Capability{assistant.CapabilityStructuredOutput},
			Messages:             []assistant.Message{textMessage(assistant.RoleUser, "Return whether the check passed.")},
			ResponseSchema:       json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`),
			Check: func(response assistant.GenerationResponse) error {
				if response.FinishReason != assistant.FinishStop || response.Message.Content[0].Text != `{"ok":true}` {
					return errors.New("expected normalized structured JSON text")
				}
				return nil
			},
		}},
		FailureProbes: []adaptertest.FailureProbe{
			{Name: "auth", Err: &ProviderHTTPError{Status: http.StatusForbidden}, Want: assistant.FailureAuth},
			{Name: "rate-limit", Err: &ProviderHTTPError{Status: http.StatusTooManyRequests, Code: "RESOURCE_EXHAUSTED"}, Want: assistant.FailureRateLimit},
			{Name: "quota", Err: &ProviderHTTPError{Status: http.StatusTooManyRequests, Message: "You exceeded your current quota"}, Want: assistant.FailureQuota},
			{Name: "prepay-depleted", Err: &ProviderHTTPError{Status: http.StatusPaymentRequired, Code: "RESOURCE_EXHAUSTED"}, Want: assistant.FailureQuota},
			{Name: "timeout", Err: &ProviderHTTPError{Status: http.StatusGatewayTimeout}, Want: assistant.FailureTimeout},
			{Name: "invalid", Err: &ProviderHTTPError{Status: http.StatusBadRequest, Code: "INVALID_ARGUMENT"}, Want: assistant.FailureInvalid},
			{Name: "network", Err: &url.Error{Op: "Post", URL: "http://provider.invalid", Err: errors.New("connection refused")}, Want: assistant.FailureNetwork},
		},
	})
}

func TestGooglePayloadPairsToolCallsWithFunctionResponses(t *testing.T) {
	signatures := newSignatureCache(2)
	signatures.put("call_1", "sig-1")
	payload, err := googlePayload(assistant.GenerationRequest{
		Model: assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-test"},
		Messages: []assistant.Message{
			textMessage(assistant.RoleSystem, "policy"),
			textMessage(assistant.RoleUser, "quantas pessoas?"),
			{Role: assistant.RoleAssistant, Content: []assistant.ContentPart{{Type: assistant.PartToolCall, ToolCall: &assistant.ToolCall{ID: "call_1", Name: "search", Arguments: json.RawMessage(`{"q":"campinas"}`)}}}},
			{Role: assistant.RoleTool, Content: []assistant.ContentPart{{Type: assistant.PartToolResult, ToolResult: &assistant.ToolResult{CallID: "call_1", Content: []assistant.ContentPart{{Type: assistant.PartText, Text: `{"total":12}`}}}}}},
		},
		Tools: []assistant.ToolDefinition{{Name: "search", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"q":{"type":"string","maxLength":10}}}`)}},
	}, signatures.get)
	if err != nil {
		t.Fatalf("googlePayload() error = %v", err)
	}
	encoded, _ := json.Marshal(payload)
	var decoded struct {
		SystemInstruction map[string]any   `json:"systemInstruction"`
		Contents          []map[string]any `json:"contents"`
		Tools             []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded.SystemInstruction == nil || len(decoded.Contents) != 3 {
		t.Fatalf("payload = %s", encoded)
	}
	if decoded.Contents[1]["role"] != "model" || decoded.Contents[2]["role"] != "user" {
		t.Fatalf("tool call/response roles = %s", encoded)
	}
	callPart := decoded.Contents[1]["parts"].([]any)[0].(map[string]any)
	if callPart[thoughtSignatureKeyName] != "sig-1" {
		t.Fatalf("functionCall thoughtSignature = %v", callPart[thoughtSignatureKeyName])
	}
	response := decoded.Contents[2]["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
	if response["name"] != "search" {
		t.Fatalf("functionResponse name = %v", response["name"])
	}
	declaration := decoded.Tools[0]["functionDeclarations"].([]any)[0].(map[string]any)
	if _, ok := declaration["parametersJsonSchema"]; !ok {
		t.Fatalf("tool declaration = %v", declaration)
	}

	if _, err := googlePayload(assistant.GenerationRequest{Messages: []assistant.Message{
		{Role: assistant.RoleTool, Content: []assistant.ContentPart{{Type: assistant.PartToolResult, ToolResult: &assistant.ToolResult{CallID: "orphan", Content: []assistant.ContentPart{{Type: assistant.PartText, Text: "{}"}}}}}},
	}}, signatures.get); err == nil {
		t.Fatal("googlePayload() accepted a tool result without its call")
	}

	unknown, err := googlePayload(assistant.GenerationRequest{Messages: []assistant.Message{
		{Role: assistant.RoleAssistant, Content: []assistant.ContentPart{{Type: assistant.PartToolCall, ToolCall: &assistant.ToolCall{ID: "call_9", Name: "search", Arguments: json.RawMessage(`{}`)}}}},
	}}, signatures.get)
	if err != nil {
		t.Fatalf("googlePayload(unknown call) error = %v", err)
	}
	encoded, _ = json.Marshal(unknown)
	if !strings.Contains(string(encoded), skipThoughtSignature) {
		t.Fatalf("unknown call should carry the bypass signature: %s", encoded)
	}

	signatures.put("call_2", "sig-2")
	signatures.put("call_3", "sig-3")
	if signatures.get("call_1") != "" || signatures.get("call_3") != "sig-3" {
		t.Fatal("signature cache did not evict FIFO")
	}
}

func TestGoogleAdapterReplaysThoughtSignatureFromLeadingPart(t *testing.T) {
	var replayed string
	calls := 0
	server := geminiTestServer(t, func(payload map[string]json.RawMessage) string {
		calls++
		if calls == 1 {
			return `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"thought":true,"text":"hidden","thoughtSignature":"sig-lead"},{"functionCall":{"name":"catalog","args":{}}}]}}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":3}}`
		}
		replayed = string(payload["contents"])
		return `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"ok"}]}}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":1}}`
	})
	defer server.Close()

	adapter, err := NewGoogleAdapter(server.Client(), staticResolver("test-secret"), server.URL)
	if err != nil {
		t.Fatalf("NewGoogleAdapter() error = %v", err)
	}
	ctx := context.Background()
	credential := SharedCredential(ProviderGoogle)
	tools := []assistant.ToolDefinition{{Name: "catalog", Description: "List fields", InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`)}}
	first, err := adapter.Generate(ctx, assistant.GenerationRequest{
		Model:      assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-test"},
		Messages:   []assistant.Message{textMessage(assistant.RoleUser, "quantas pessoas?")},
		Tools:      tools,
		Credential: credential,
	})
	if err != nil {
		t.Fatalf("Generate() first error = %v", err)
	}
	if len(first.Message.Content) != 1 || first.Message.Content[0].ToolCall == nil || first.Message.Content[0].ToolCall.Name != "catalog" {
		t.Fatalf("first response should be the tool call without thought text: %#v", first.Message.Content)
	}
	call := first.Message.Content[0].ToolCall
	if _, err := adapter.Generate(ctx, assistant.GenerationRequest{
		Model: assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-test"},
		Messages: []assistant.Message{
			textMessage(assistant.RoleUser, "quantas pessoas?"),
			{Role: assistant.RoleAssistant, Content: []assistant.ContentPart{{Type: assistant.PartToolCall, ToolCall: call}}},
			{Role: assistant.RoleTool, Content: []assistant.ContentPart{{Type: assistant.PartToolResult, ToolResult: &assistant.ToolResult{CallID: call.ID, Content: []assistant.ContentPart{{Type: assistant.PartText, Text: `{"ok":true}`}}}}}},
		},
		Tools:      tools,
		Credential: credential,
	}); err != nil {
		t.Fatalf("Generate() replay error = %v", err)
	}
	if !strings.Contains(replayed, `"thoughtSignature":"sig-lead"`) {
		t.Fatalf("replayed contents missing leading thought signature: %s", replayed)
	}
}

func TestGooglePayloadSetsGemini3ThinkingLevelMedium(t *testing.T) {
	if got := thinkingLevelFor("gemini-3.5-flash-lite"); got != "MEDIUM" {
		t.Fatalf("thinkingLevelFor(gemini-3.5-flash-lite) = %q", got)
	}
	if got := thinkingLevelFor("gemini-2.5-flash"); got != "" {
		t.Fatalf("thinkingLevelFor(gemini-2.5-flash) = %q", got)
	}
	payload, err := googlePayload(assistant.GenerationRequest{
		Model:    assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-3.5-flash-lite"},
		Messages: []assistant.Message{textMessage(assistant.RoleUser, "quantas pessoas?")},
	}, newSignatureCache(1).get)
	if err != nil {
		t.Fatalf("googlePayload() error = %v", err)
	}
	encoded, _ := json.Marshal(payload)
	var decoded struct {
		GenerationConfig struct {
			ThinkingConfig struct {
				ThinkingLevel string `json:"thinkingLevel"`
			} `json:"thinkingConfig"`
			ThinkingBudget *int `json:"thinkingBudget"`
		} `json:"generationConfig"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded.GenerationConfig.ThinkingConfig.ThinkingLevel != "MEDIUM" || decoded.GenerationConfig.ThinkingBudget != nil {
		t.Fatalf("generationConfig = %s", encoded)
	}
	legacy, err := googlePayload(assistant.GenerationRequest{
		Model:    assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-2.5-flash"},
		Messages: []assistant.Message{textMessage(assistant.RoleUser, "quantas pessoas?")},
	}, newSignatureCache(1).get)
	if err != nil {
		t.Fatalf("googlePayload(2.5) error = %v", err)
	}
	encoded, _ = json.Marshal(legacy)
	if strings.Contains(string(encoded), "thinkingConfig") || strings.Contains(string(encoded), "thinkingBudget") {
		t.Fatalf("gemini-2.5 payload should omit thinking: %s", encoded)
	}
}

func TestGoogleAdapterRetriesTransientGenerateFailures(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-goog-api-key") != "test-secret" {
			http.Error(writer, `{"error":{"code":401,"status":"UNAUTHENTICATED"}}`, http.StatusUnauthorized)
			return
		}
		if request.Method != http.MethodPost || !strings.HasSuffix(request.URL.Path, ":generateContent") {
			http.NotFound(writer, request)
			return
		}
		attempts++
		writer.Header().Set("Content-Type", "application/json")
		if attempts < 3 {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte(`{"error":{"status":"INTERNAL","message":"blip"}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":"ok"}]}}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1}}`))
	}))
	defer server.Close()

	adapter, err := NewGoogleAdapter(server.Client(), staticResolver("test-secret"), server.URL)
	if err != nil {
		t.Fatalf("NewGoogleAdapter() error = %v", err)
	}
	response, err := adapter.Generate(context.Background(), assistant.GenerationRequest{
		Model:      assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-test"},
		Messages:   []assistant.Message{textMessage(assistant.RoleUser, "oi")},
		Credential: SharedCredential(ProviderGoogle),
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if attempts != 3 || response.FinishReason != assistant.FinishStop || len(response.Message.Content) != 1 || response.Message.Content[0].Text != "ok" {
		t.Fatalf("attempts=%d response=%#v", attempts, response)
	}

	always := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":{"status":"UNAVAILABLE"}}`))
	}))
	defer always.Close()
	failing, err := NewGoogleAdapter(always.Client(), staticResolver("test-secret"), always.URL)
	if err != nil {
		t.Fatalf("NewGoogleAdapter(always) error = %v", err)
	}
	attempts = 0
	_, err = failing.Generate(context.Background(), assistant.GenerationRequest{
		Model:      assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-test"},
		Messages:   []assistant.Message{textMessage(assistant.RoleUser, "oi")},
		Credential: SharedCredential(ProviderGoogle),
	})
	var httpErr *ProviderHTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusServiceUnavailable || attempts != maximumGenerateAttempts {
		t.Fatalf("exhausted retry error = %v attempts = %d", err, attempts)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":{"status":"INVALID_ARGUMENT"}}`))
	}))
	defer bad.Close()
	adapter, err = NewGoogleAdapter(bad.Client(), staticResolver("test-secret"), bad.URL)
	if err != nil {
		t.Fatalf("NewGoogleAdapter(bad) error = %v", err)
	}
	attempts = 0
	_, err = adapter.Generate(context.Background(), assistant.GenerationRequest{
		Model:      assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-test"},
		Messages:   []assistant.Message{textMessage(assistant.RoleUser, "oi")},
		Credential: SharedCredential(ProviderGoogle),
	})
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest || attempts != 1 {
		t.Fatalf("INVALID_ARGUMENT should not retry: err=%v attempts=%d", err, attempts)
	}
}

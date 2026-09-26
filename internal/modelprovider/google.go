package modelprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Pherlsz/Gymkhana-Core/assistant"
)

const (
	defaultGoogleBaseURL    = "https://generativelanguage.googleapis.com"
	maximumProviderBody     = 4 << 20
	googleSecretHeaderName  = "x-goog-api-key"
	maximumGenerateAttempts = 3
	generateRetryDelay      = 400 * time.Millisecond
)

// SecretResolver returns the plaintext provider secret for an opaque credential
// reference. The adapter never stores the secret.
type SecretResolver func(context.Context, *assistant.CredentialRef) (string, error)

// GoogleAdapter implements assistant.ProviderAdapter over the Gemini REST API.
// It lives in the product repository on purpose: Gymkhana-Core keeps provider
// HTTP payloads out of its module (docs/ADAPTER-CONFORMANCE.md).
type GoogleAdapter struct {
	baseURL    string
	client     *http.Client
	resolve    SecretResolver
	signatures *signatureCache
}

func NewGoogleAdapter(client *http.Client, resolve SecretResolver, baseURL string) (*GoogleAdapter, error) {
	if client == nil || resolve == nil {
		return nil, ErrInvalidInput
	}
	if baseURL == "" {
		baseURL = defaultGoogleBaseURL
	}
	return &GoogleAdapter{baseURL: strings.TrimRight(baseURL, "/"), client: client, resolve: resolve, signatures: newSignatureCache(maximumSignatures)}, nil
}

// Gemini 3 models sign each functionCall with a thoughtSignature that must be
// echoed when the call is replayed in history, otherwise the API answers 400.
// Core's ToolCall has no slot for it, so the adapter remembers signatures by
// call id. Missing entries fall back to Google's documented bypass token.
//
// ponytail: in-process cache; a second API instance would not see it and would
// degrade to the bypass token. Persist the signature next to the tool step if
// the API ever runs more than one replica.
const (
	maximumSignatures       = 4096
	skipThoughtSignature    = "skip_thought_signature_validator"
	thoughtSignatureKeyName = "thoughtSignature"
)

type signatureCache struct {
	mu     sync.Mutex
	limit  int
	order  []string
	values map[string]string
}

func newSignatureCache(limit int) *signatureCache {
	return &signatureCache{limit: limit, values: make(map[string]string, limit)}
}

func (cache *signatureCache) put(callID, signature string) {
	if callID == "" || signature == "" {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if _, exists := cache.values[callID]; !exists {
		cache.order = append(cache.order, callID)
		for len(cache.order) > cache.limit {
			delete(cache.values, cache.order[0])
			cache.order = cache.order[1:]
		}
	}
	cache.values[callID] = signature
}

func (cache *signatureCache) get(callID string) string {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.values[callID]
}

func (adapter *GoogleAdapter) Descriptor() assistant.ProviderDescriptor {
	return assistant.ProviderDescriptor{
		ID: assistant.ProviderGoogle, DisplayName: "Google Gemini",
		CredentialModes: []assistant.CredentialMode{assistant.CredentialManaged},
	}
}

func (adapter *GoogleAdapter) ListModels(ctx context.Context, credential *assistant.CredentialRef) ([]assistant.ModelDescriptor, error) {
	secret, err := adapter.resolve(ctx, credential)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			InputTokenLimit            int64    `json:"inputTokenLimit"`
			OutputTokenLimit           int64    `json:"outputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := adapter.doJSON(ctx, http.MethodGet, "/v1beta/models?pageSize=200", secret, nil, &payload); err != nil {
		return nil, err
	}
	models := make([]assistant.ModelDescriptor, 0, len(payload.Models))
	for _, item := range payload.Models {
		if !containsString(item.SupportedGenerationMethods, "generateContent") {
			continue
		}
		models = append(models, assistant.ModelDescriptor{
			Ref:         assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: assistant.ModelID(strings.TrimPrefix(item.Name, "models/"))},
			DisplayName: item.DisplayName,
			Access:      assistant.AccessUnknown,
			Roles:       []assistant.ModelRole{assistant.ModelRoleGeneration},
			// ponytail: Gemini does not publish per-model capability flags; every
			// generateContent model is assumed to support text, tools and JSON output.
			Capabilities:    []assistant.Capability{assistant.CapabilityText, assistant.CapabilityToolCalling, assistant.CapabilityStructuredOutput},
			ContextWindow:   item.InputTokenLimit,
			MaxOutputTokens: item.OutputTokenLimit,
		})
	}
	return models, nil
}

func (adapter *GoogleAdapter) Generate(ctx context.Context, request assistant.GenerationRequest) (assistant.GenerationResponse, error) {
	secret, err := adapter.resolve(ctx, request.Credential)
	if err != nil {
		return assistant.GenerationResponse{}, err
	}
	payload, err := googlePayload(request, adapter.signatures.get)
	if err != nil {
		return assistant.GenerationResponse{}, err
	}
	response, err := adapter.postGenerate(ctx, string(request.Model.Model), secret, payload)
	if err != nil {
		return assistant.GenerationResponse{}, err
	}
	usage := assistant.Usage{
		InputTokens: response.Usage.PromptTokens, OutputTokens: response.Usage.CandidateTokens,
		CachedInputTokens: response.Usage.CachedTokens, ReasoningTokens: response.Usage.ReasoningTokens,
	}
	if len(response.Candidates) == 0 {
		if response.PromptFeedback != nil && response.PromptFeedback.BlockReason != "" {
			return assistant.GenerationResponse{Model: request.Model, Message: assistant.Message{Role: assistant.RoleAssistant}, FinishReason: assistant.FinishContentFilter, Usage: usage}, nil
		}
		return assistant.GenerationResponse{}, errors.New("gemini returned no candidates")
	}
	candidate := response.Candidates[0]
	parts := make([]assistant.ContentPart, 0, len(candidate.Content.Parts))
	toolCalls := 0
	pendingSignature := ""
	assignedFirstCallSignature := false
	for index, part := range candidate.Content.Parts {
		if pendingSignature == "" && part.ThoughtSignature != "" {
			pendingSignature = part.ThoughtSignature
		}
		switch {
		case part.FunctionCall != nil:
			arguments := part.FunctionCall.Args
			if len(arguments) == 0 {
				arguments = json.RawMessage(`{}`)
			}
			id := part.FunctionCall.ID
			if id == "" {
				id = "gemini_call_" + strconv.Itoa(index)
			}
			signature := part.ThoughtSignature
			if signature == "" && !assignedFirstCallSignature {
				signature = pendingSignature
			}
			assignedFirstCallSignature = true
			adapter.signatures.put(id, signature)
			parts = append(parts, assistant.ContentPart{Type: assistant.PartToolCall, ToolCall: &assistant.ToolCall{ID: id, Name: part.FunctionCall.Name, Arguments: arguments}})
			toolCalls++
		case part.Thought:
			continue
		case strings.TrimSpace(part.Text) != "":
			parts = append(parts, assistant.ContentPart{Type: assistant.PartText, Text: part.Text})
		}
	}
	finish := assistant.FinishOther
	switch {
	case toolCalls > 0:
		finish = assistant.FinishToolCalls
	case candidate.FinishReason == "STOP":
		finish = assistant.FinishStop
	case candidate.FinishReason == "MAX_TOKENS":
		finish = assistant.FinishLength
	case candidate.FinishReason == "SAFETY", candidate.FinishReason == "RECITATION",
		candidate.FinishReason == "BLOCKLIST", candidate.FinishReason == "PROHIBITED_CONTENT":
		finish = assistant.FinishContentFilter
	}
	return assistant.GenerationResponse{
		Model:        request.Model,
		Message:      assistant.Message{Role: assistant.RoleAssistant, Content: parts},
		FinishReason: finish,
		Usage:        usage,
	}, nil
}

type geminiContentResponse struct {
	Candidates []struct {
		FinishReason string `json:"finishReason"`
		Content      struct {
			Parts []struct {
				Text             string `json:"text"`
				Thought          bool   `json:"thought"`
				ThoughtSignature string `json:"thoughtSignature"`
				FunctionCall     *struct {
					ID   string          `json:"id"`
					Name string          `json:"name"`
					Args json.RawMessage `json:"args"`
				} `json:"functionCall"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
	Usage struct {
		PromptTokens    int64 `json:"promptTokenCount"`
		CandidateTokens int64 `json:"candidatesTokenCount"`
		CachedTokens    int64 `json:"cachedContentTokenCount"`
		ReasoningTokens int64 `json:"thoughtsTokenCount"`
	} `json:"usageMetadata"`
}

func (adapter *GoogleAdapter) postGenerate(ctx context.Context, model, secret string, payload any) (geminiContentResponse, error) {
	var response geminiContentResponse
	endpoint := "/v1beta/models/" + url.PathEscape(model) + ":generateContent"
	if err := adapter.doJSON(ctx, http.MethodPost, endpoint, secret, payload, &response); err != nil {
		return geminiContentResponse{}, err
	}
	return response, nil
}

func (adapter *GoogleAdapter) generateStructured(ctx context.Context, credential *assistant.CredentialRef, model, instruction, mediaType string, media []byte, schema json.RawMessage) (string, int64, error) {
	secret, err := adapter.resolve(ctx, credential)
	if err != nil {
		return "", 0, err
	}
	if model == "" || mediaType == "" || len(media) == 0 || len(schema) == 0 {
		return "", 0, ErrInvalidInput
	}
	var schemaValue any
	if err := json.Unmarshal(schema, &schemaValue); err != nil {
		return "", 0, ErrInvalidInput
	}
	payload := map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": instruction}}},
		"contents": []any{map[string]any{"role": "user", "parts": []any{
			map[string]any{"inlineData": map[string]any{"mimeType": mediaType, "data": media}},
			map[string]any{"text": "Extraia os campos visíveis."},
		}}},
		"generationConfig": map[string]any{"responseMimeType": "application/json", "responseJsonSchema": schemaValue},
	}
	response, err := adapter.postGenerate(ctx, model, secret, payload)
	if err != nil {
		return "", 0, err
	}
	usage := response.Usage.PromptTokens + response.Usage.CandidateTokens
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", usage, errors.New("gemini returned no candidates")
	}
	text := strings.TrimSpace(response.Candidates[0].Content.Parts[0].Text)
	if text == "" {
		return "", usage, errors.New("gemini returned empty structured output")
	}
	if usage < 1 {
		usage = 1
	}
	return text, usage, nil
}

func (adapter *GoogleAdapter) ClassifyFailure(err error) assistant.FailureClass {
	var httpErr *ProviderHTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return assistant.FailureAuth
		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
			return assistant.FailureTimeout
		case http.StatusPaymentRequired:
			// Gemini returns 402 RESOURCE_EXHAUSTED when AI Studio prepay credits are depleted.
			return assistant.FailureQuota
		case http.StatusTooManyRequests:
			if strings.Contains(strings.ToLower(httpErr.Message), "quota") {
				return assistant.FailureQuota
			}
			return assistant.FailureRateLimit
		case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity:
			return assistant.FailureInvalid
		}
		if httpErr.Status >= 500 {
			return assistant.FailureUnavailable
		}
		return assistant.FailureUnknown
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return assistant.FailureTimeout
		}
		return assistant.FailureNetwork
	}
	return assistant.FailureUnknown
}

// ProviderHTTPError carries only the status and the provider's short status
// code; the body is never logged or returned to clients.
type ProviderHTTPError struct {
	Status  int
	Code    string
	Message string
}

func (e *ProviderHTTPError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("gemini request failed with status %d", e.Status)
	}
	return fmt.Sprintf("gemini request failed with status %d (%s)", e.Status, e.Code)
}

func googlePayload(request assistant.GenerationRequest, signatureFor func(string) string) (map[string]any, error) {
	contents := make([]any, 0, len(request.Messages))
	var systemParts []any
	// Gemini addresses a functionResponse by function name, not call id.
	callNames := make(map[string]string)
	for _, message := range request.Messages {
		parts := make([]any, 0, len(message.Content))
		for _, part := range message.Content {
			switch part.Type {
			case assistant.PartText:
				parts = append(parts, map[string]any{"text": part.Text})
			case assistant.PartToolCall:
				arguments := part.ToolCall.Arguments
				if len(arguments) == 0 {
					arguments = json.RawMessage(`{}`)
				}
				callNames[part.ToolCall.ID] = part.ToolCall.Name
				signature := signatureFor(part.ToolCall.ID)
				if signature == "" {
					signature = skipThoughtSignature
				}
				parts = append(parts, map[string]any{
					"functionCall":          map[string]any{"name": part.ToolCall.Name, "args": arguments},
					thoughtSignatureKeyName: signature,
				})
			case assistant.PartToolResult:
				name, ok := callNames[part.ToolResult.CallID]
				if !ok {
					return nil, fmt.Errorf("tool result %q has no preceding tool call", part.ToolResult.CallID)
				}
				parts = append(parts, functionResponsePart(name, *part.ToolResult))
			default:
				return nil, fmt.Errorf("gemini adapter does not support %s parts", part.Type)
			}
		}
		if len(parts) == 0 {
			continue
		}
		switch message.Role {
		case assistant.RoleSystem, assistant.RoleDeveloper:
			systemParts = append(systemParts, parts...)
		case assistant.RoleAssistant:
			contents = append(contents, map[string]any{"role": "model", "parts": parts})
		default:
			contents = append(contents, map[string]any{"role": "user", "parts": parts})
		}
	}
	payload := map[string]any{"contents": contents}
	if len(systemParts) > 0 {
		payload["systemInstruction"] = map[string]any{"parts": systemParts}
	}
	if len(request.Tools) > 0 {
		declarations := make([]any, 0, len(request.Tools))
		for _, tool := range request.Tools {
			// parametersJsonSchema accepts standard JSON Schema (additionalProperties,
			// format, maxLength), unlike the legacy OpenAPI-subset `parameters` field.
			declarations = append(declarations, map[string]any{"name": tool.Name, "description": tool.Description, "parametersJsonSchema": tool.InputSchema})
		}
		payload["tools"] = []any{map[string]any{"functionDeclarations": declarations}}
	}
	generation := map[string]any{}
	if len(request.ResponseSchema) > 0 {
		var schema any
		if err := json.Unmarshal(request.ResponseSchema, &schema); err != nil {
			return nil, err
		}
		generation["responseMimeType"] = "application/json"
		generation["responseJsonSchema"] = schema
	}
	if level := thinkingLevelFor(string(request.Model.Model)); level != "" {
		generation["thinkingConfig"] = map[string]any{"thinkingLevel": level}
	}
	if len(generation) > 0 {
		payload["generationConfig"] = generation
	}
	return payload, nil
}

func thinkingLevelFor(model string) string {
	if strings.Contains(strings.ToLower(model), "gemini-3") {
		return "MEDIUM"
	}
	return ""
}

func functionResponsePart(name string, result assistant.ToolResult) map[string]any {
	var text strings.Builder
	for _, part := range result.Content {
		if part.Type == assistant.PartText {
			text.WriteString(part.Text)
		}
	}
	var content any = text.String()
	if raw := json.RawMessage(text.String()); json.Valid(raw) {
		content = raw
	}
	response := map[string]any{"content": content}
	if result.IsError {
		response["error"] = true
	}
	return map[string]any{"functionResponse": map[string]any{"name": name, "response": response}}
}

func (adapter *GoogleAdapter) doJSON(ctx context.Context, method, path, secret string, requestBody any, responseBody any) error {
	var encoded []byte
	if requestBody != nil {
		var err error
		encoded, err = json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode gemini request: %w", err)
		}
	}
	var last error
	for attempt := 0; attempt < maximumGenerateAttempts; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(attempt) * generateRetryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				if last != nil {
					return last
				}
				return ctx.Err()
			case <-timer.C:
			}
		}
		err := adapter.doJSONOnce(ctx, method, path, secret, encoded, requestBody != nil, responseBody)
		if err == nil {
			return nil
		}
		last = err
		if !retryableProviderError(err) {
			return err
		}
	}
	return last
}

func (adapter *GoogleAdapter) doJSONOnce(ctx context.Context, method, path, secret string, encoded []byte, hasBody bool, responseBody any) error {
	var body io.Reader
	if hasBody {
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, adapter.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create gemini request: %w", err)
	}
	if hasBody {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set(googleSecretHeaderName, secret)
	request.Header.Set("Accept", "application/json")
	response, err := adapter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maximumProviderBody)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var payload struct {
			Error struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(limited).Decode(&payload)
		return &ProviderHTTPError{Status: response.StatusCode, Code: payload.Error.Status, Message: payload.Error.Message}
	}
	if err := json.NewDecoder(limited).Decode(responseBody); err != nil {
		return fmt.Errorf("decode gemini response: %w", err)
	}
	return nil
}

func retryableProviderError(err error) bool {
	var httpErr *ProviderHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Status == http.StatusInternalServerError ||
			httpErr.Status == http.StatusBadGateway ||
			httpErr.Status == http.StatusServiceUnavailable
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return !urlErr.Timeout()
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

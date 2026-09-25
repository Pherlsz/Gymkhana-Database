package modelprovider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Core/assistant"

	"github.com/Pherlsz/Gymkhana-Database/internal/aichat"
)

// maximumDeltaRunes mirrors the AI Chat normalized text delta limit.
const maximumDeltaRunes = 1000

// ChatClient adapts the AI Chat ModelClient port to a Core ProviderAdapter that
// reads the shared Administração key.
type ChatClient struct {
	adapter      assistant.ProviderAdapter
	keys         *KeyService
	provider     string
	defaultModel string
	logger       *slog.Logger
}

func NewChatClient(adapter assistant.ProviderAdapter, keys *KeyService, provider, defaultModel string, logger *slog.Logger) (*ChatClient, error) {
	if adapter == nil || keys == nil || !SupportedProvider(provider) || defaultModel == "" {
		return nil, ErrInvalidInput
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ChatClient{adapter: adapter, keys: keys, provider: provider, defaultModel: defaultModel, logger: logger}, nil
}

// SharedCredential is the opaque reference handed to adapters. It carries no
// secret; KeyResolver turns it back into the plaintext through KeyService.
func SharedCredential(provider string) *assistant.CredentialRef {
	return &assistant.CredentialRef{
		ID: "administracao-" + provider, Provider: assistant.ProviderID(provider),
		Mode: assistant.CredentialManaged, Reference: "db:ai_model_keys/" + provider,
	}
}

// KeyResolver resolves only the shared credential of one provider.
func KeyResolver(keys *KeyService, provider string) SecretResolver {
	expected := SharedCredential(provider).Reference
	return func(ctx context.Context, credential *assistant.CredentialRef) (string, error) {
		if keys == nil || credential == nil || credential.Reference != expected {
			return "", ErrNotConfigured
		}
		secret, _, err := keys.Resolve(ctx, provider)
		return secret, err
	}
}

func (client *ChatClient) Generate(ctx context.Context, request aichat.ModelRequest, emit func(string) error) (aichat.ModelResponse, error) {
	_, model, err := client.keys.Resolve(ctx, client.provider)
	if err != nil {
		if !errors.Is(err, ErrNotConfigured) {
			client.logger.Warn("model key could not be resolved", "provider", client.provider, "error_type", fmt.Sprintf("%T", err))
		}
		return aichat.ModelResponse{}, aichat.ErrProviderUnavailable
	}
	if model == "" {
		model = client.defaultModel
	}
	generation := assistant.GenerationRequest{
		Model:      assistant.ModelRef{Provider: assistant.ProviderID(client.provider), Model: assistant.ModelID(model)},
		Messages:   coreMessages(request),
		Tools:      coreTools(request.Tools),
		Credential: SharedCredential(client.provider),
	}
	// ponytail: non-streaming generateContent; text arrives at once and is
	// re-chunked into deltas. Switch to streamGenerateContent when latency matters.
	response, err := client.adapter.Generate(ctx, generation)
	if err != nil {
		return aichat.ModelResponse{}, client.mapError(err)
	}
	result := aichat.ModelResponse{Usage: aichat.ModelUsage{InputUnits: response.Usage.InputTokens, OutputUnits: response.Usage.OutputTokens}}
	for _, part := range response.Message.Content {
		switch part.Type {
		case assistant.PartText:
			if err := emitChunks(part.Text, emit); err != nil {
				return aichat.ModelResponse{}, err
			}
		case assistant.PartToolCall:
			if result.ToolCall == nil {
				result.ToolCall = &aichat.ToolCall{ID: part.ToolCall.ID, Name: part.ToolCall.Name, Arguments: append([]byte(nil), part.ToolCall.Arguments...)}
			}
		}
	}
	return result, nil
}

func (client *ChatClient) mapError(err error) error {
	class := assistant.FailureUnknown
	if classifier, ok := client.adapter.(assistant.FailureClassifier); ok {
		class = classifier.ClassifyFailure(err)
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) || class == assistant.FailureTimeout {
		return aichat.ErrProviderTimeout
	}
	switch class {
	case assistant.FailureRateLimit:
		return aichat.ErrRateLimited
	case assistant.FailureQuota:
		return aichat.ErrQuotaExceeded
	}
	// Only the HTTP status and Google's short status token are logged; the
	// message body may echo request content and stays redacted.
	var httpErr *ProviderHTTPError
	if errors.As(err, &httpErr) {
		client.logger.Warn("model provider request failed", "provider", client.provider, "failure_class", string(class), "status", httpErr.Status, "provider_code", httpErr.Code)
	} else {
		client.logger.Warn("model provider request failed", "provider", client.provider, "failure_class", string(class), "error_type", fmt.Sprintf("%T", err))
	}
	return aichat.ErrProviderUnavailable
}

func coreMessages(request aichat.ModelRequest) []assistant.Message {
	messages := make([]assistant.Message, 0, len(request.Messages)+2*len(request.ToolResults)+1)
	policy := request.Policy
	if request.ActiveResultReferenceID != "" {
		policy += "\nReferência de resultado ativa: " + request.ActiveResultReferenceID + ". Use a tool result para reabri-la quando a pergunta continuar sobre esse conjunto."
	}
	messages = append(messages, textMessage(assistant.RoleSystem, policy))
	for _, message := range request.Messages {
		role := assistant.RoleUser
		if message.Role == aichat.MessageAssistant {
			role = assistant.RoleAssistant
		}
		messages = append(messages, textMessage(role, message.Content))
	}
	for _, result := range request.ToolResults {
		arguments := result.Arguments
		if len(arguments) == 0 {
			arguments = []byte(`{}`)
		}
		messages = append(messages,
			assistant.Message{Role: assistant.RoleAssistant, Content: []assistant.ContentPart{{
				Type: assistant.PartToolCall, ToolCall: &assistant.ToolCall{ID: result.CallID, Name: result.ToolName, Arguments: arguments},
			}}},
			assistant.Message{Role: assistant.RoleTool, Content: []assistant.ContentPart{{
				Type: assistant.PartToolResult, ToolResult: &assistant.ToolResult{CallID: result.CallID, Content: []assistant.ContentPart{{Type: assistant.PartText, Text: string(result.Data)}}},
			}}},
		)
	}
	return messages
}

func coreTools(tools []aichat.ToolSchema) []assistant.ToolDefinition {
	result := make([]assistant.ToolDefinition, len(tools))
	for index, tool := range tools {
		result[index] = assistant.ToolDefinition{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema}
	}
	return result
}

func textMessage(role assistant.Role, text string) assistant.Message {
	return assistant.Message{Role: role, Content: []assistant.ContentPart{{Type: assistant.PartText, Text: text}}}
}

func emitChunks(text string, emit func(string) error) error {
	for len(text) > 0 {
		end := len(text)
		if utf8.RuneCountInString(text) > maximumDeltaRunes {
			end = 0
			for count := 0; count < maximumDeltaRunes; count++ {
				_, size := utf8.DecodeRuneInString(text[end:])
				end += size
			}
		}
		if err := emit(text[:end]); err != nil {
			return err
		}
		text = text[end:]
	}
	return nil
}

package modelprovider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Pherlsz/Gymkhana-Core/assistant"

	"github.com/Pherlsz/Gymkhana-Database/internal/aichat"
)

type scriptedAdapter struct {
	request  assistant.GenerationRequest
	response assistant.GenerationResponse
	err      error
	class    assistant.FailureClass
}

func (adapter *scriptedAdapter) Descriptor() assistant.ProviderDescriptor {
	return assistant.ProviderDescriptor{ID: assistant.ProviderGoogle, CredentialModes: []assistant.CredentialMode{assistant.CredentialManaged}}
}

func (adapter *scriptedAdapter) ListModels(context.Context, *assistant.CredentialRef) ([]assistant.ModelDescriptor, error) {
	return nil, nil
}

func (adapter *scriptedAdapter) Generate(_ context.Context, request assistant.GenerationRequest) (assistant.GenerationResponse, error) {
	adapter.request = request
	return adapter.response, adapter.err
}

func (adapter *scriptedAdapter) ClassifyFailure(error) assistant.FailureClass {
	if adapter.class != "" {
		return adapter.class
	}
	return assistant.FailureTimeout
}

func TestChatClientBridgesRequestAndResponse(t *testing.T) {
	keys, _ := newKeyFixture(t)
	ctx := context.Background()
	if _, err := keys.Set(ctx, adminSession(), ProviderGoogle, "AIzaSy-test-secret-0123456789abcdef", "gemini-2.5-flash"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	long := strings.Repeat("á", 1500)
	adapter := &scriptedAdapter{response: assistant.GenerationResponse{
		Model:   assistant.ModelRef{Provider: assistant.ProviderGoogle, Model: "gemini-2.5-flash"},
		Message: assistant.Message{Role: assistant.RoleAssistant, Content: []assistant.ContentPart{{Type: assistant.PartText, Text: long}, {Type: assistant.PartToolCall, ToolCall: &assistant.ToolCall{ID: "call_2", Name: "query", Arguments: json.RawMessage(`{"plan":{}}`)}}}},
		Usage:   assistant.Usage{InputTokens: 10, OutputTokens: 4},
	}}
	client, err := NewChatClient(adapter, keys, ProviderGoogle, "gemini-default", nil)
	if err != nil {
		t.Fatalf("NewChatClient() error = %v", err)
	}

	var deltas []string
	response, err := client.Generate(ctx, aichat.ModelRequest{
		Policy:                  "policy",
		Messages:                []aichat.ModelMessage{{Role: aichat.MessageUser, Content: "quantas?"}, {Role: aichat.MessageAssistant, Content: "vou consultar"}},
		Tools:                   []aichat.ToolSchema{{Name: "search", Description: "d", InputSchema: json.RawMessage(`{"type":"object"}`)}},
		ToolResults:             []aichat.ModelToolResult{{CallID: "call_1", ToolName: "search", Arguments: json.RawMessage(`{"q":"x"}`), Data: json.RawMessage(`{"total":1}`)}},
		ActiveResultReferenceID: "ref-1",
	}, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.ToolCall == nil || response.ToolCall.Name != "query" || response.Usage.InputUnits != 10 || response.Usage.OutputUnits != 4 {
		t.Fatalf("response = %#v", response)
	}
	if len(deltas) != 2 || strings.Join(deltas, "") != long || len([]rune(deltas[0])) != 1000 {
		t.Fatalf("deltas = %d chunks, first %d runes", len(deltas), len([]rune(deltas[0])))
	}

	request := adapter.request
	if request.Model.Model != "gemini-2.5-flash" || request.Credential == nil || request.Credential.Reference != SharedCredential(ProviderGoogle).Reference {
		t.Fatalf("request model/credential = %#v %#v", request.Model, request.Credential)
	}
	if len(request.Messages) != 5 || request.Messages[0].Role != assistant.RoleSystem || !strings.Contains(request.Messages[0].Content[0].Text, "ref-1") {
		t.Fatalf("messages = %#v", request.Messages)
	}
	if request.Messages[3].Content[0].ToolCall == nil || request.Messages[3].Content[0].ToolCall.ID != "call_1" ||
		request.Messages[4].Content[0].ToolResult == nil || request.Messages[4].Content[0].ToolResult.CallID != "call_1" {
		t.Fatalf("tool pairing = %#v %#v", request.Messages[3], request.Messages[4])
	}
	if len(request.Tools) != 1 || request.Tools[0].Name != "search" {
		t.Fatalf("tools = %#v", request.Tools)
	}
}

func TestChatClientFailsClosedWithoutKeyAndMapsTimeouts(t *testing.T) {
	keys, _ := newKeyFixture(t)
	adapter := &scriptedAdapter{err: errors.New("boom")}
	client, err := NewChatClient(adapter, keys, ProviderGoogle, "gemini-default", nil)
	if err != nil {
		t.Fatalf("NewChatClient() error = %v", err)
	}
	emit := func(string) error { return nil }
	if _, err := client.Generate(context.Background(), aichat.ModelRequest{}, emit); !errors.Is(err, aichat.ErrProviderUnavailable) {
		t.Fatalf("Generate() without key error = %v, want ErrProviderUnavailable", err)
	}
	if adapter.request.Model.Model != "" {
		t.Fatal("adapter was called without a configured key")
	}

	if _, err := keys.Set(context.Background(), adminSession(), ProviderGoogle, "AIzaSy-test-secret-0123456789abcdef", "gemini-2.5-flash"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, err := client.Generate(context.Background(), aichat.ModelRequest{}, emit); !errors.Is(err, aichat.ErrProviderTimeout) {
		t.Fatalf("Generate() with timeout class error = %v, want ErrProviderTimeout", err)
	}

	adapter.class = assistant.FailureRateLimit
	if _, err := client.Generate(context.Background(), aichat.ModelRequest{}, emit); !errors.Is(err, aichat.ErrRateLimited) {
		t.Fatalf("Generate() with rate-limit class error = %v, want ErrRateLimited", err)
	}
	adapter.class = assistant.FailureQuota
	if _, err := client.Generate(context.Background(), aichat.ModelRequest{}, emit); !errors.Is(err, aichat.ErrQuotaExceeded) {
		t.Fatalf("Generate() with quota class error = %v, want ErrQuotaExceeded", err)
	}
}

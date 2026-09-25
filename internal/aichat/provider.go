package aichat

import (
	_ "embed"
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrProviderUnavailable = errors.New("model provider is unavailable")
	ErrProviderTimeout     = errors.New("model provider timed out")
)

//go:embed policy.txt
var ReadOnlySystemPolicy string

// toolNameToKind is the single source of truth for tool name → kind mapping.
// Both toolKindFromName (orchestrator) and Execute (ToolGateway) derive from this.
var toolNameToKind = map[string]ToolKind{
	"catalog":   ToolCatalog,
	"search":    ToolSearch,
	// query, sequencia, tarefa all stored as QUERY.
	// ponytail: sequencia and tarefa are stored as QUERY. A dedicated kind needs a migration of tool_kind.
	"query":     ToolQuery,
	"sequencia": ToolQuery,
	"tarefa":    ToolQuery,
	"result":    ToolResult,
}

type ModelMessage struct {
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	Untrusted bool        `json:"untrusted"`
}

type ModelToolResult struct {
	CallID      string          `json:"call_id"`
	ToolName    string          `json:"tool_name"`
	Arguments   json.RawMessage `json:"arguments,omitempty"`
	Data        json.RawMessage `json:"data"`
	ReferenceID string          `json:"reference_id,omitempty"`
	Untrusted   bool            `json:"untrusted"`
}

type ModelRequest struct {
	Policy                  string            `json:"policy"`
	Messages                []ModelMessage    `json:"messages"`
	Tools                   []ToolSchema      `json:"tools"`
	ToolResults             []ModelToolResult `json:"tool_results,omitempty"`
	ActiveResultReferenceID string            `json:"active_result_reference_id,omitempty"`
}

type ModelUsage struct {
	InputUnits  int64
	OutputUnits int64
}

type ModelResponse struct {
	ToolCall *ToolCall
	Usage    ModelUsage
}

type ModelClient interface {
	Generate(context.Context, ModelRequest, func(string) error) (ModelResponse, error)
}

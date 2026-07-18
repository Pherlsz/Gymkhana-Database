package aichat

import (
	"context"
	"errors"
	"sync"
)

type FakeModelStep struct {
	Deltas   []string
	ToolCall *ToolCall
	Usage    ModelUsage
	Err      error
}

type FakeProvider struct {
	mutex    sync.Mutex
	steps    []FakeModelStep
	repeat   *FakeModelStep
	requests []ModelRequest
}

func NewFakeProvider(steps ...FakeModelStep) *FakeProvider {
	return &FakeProvider{steps: append([]FakeModelStep(nil), steps...)}
}

func NewRepeatingFakeProvider(step FakeModelStep) *FakeProvider {
	clone := cloneFakeModelStep(step)
	return &FakeProvider{repeat: &clone}
}

func (provider *FakeProvider) Generate(ctx context.Context, request ModelRequest, emit func(string) error) (ModelResponse, error) {
	provider.mutex.Lock()
	if len(provider.steps) == 0 && provider.repeat == nil {
		provider.mutex.Unlock()
		return ModelResponse{}, ErrProviderUnavailable
	}
	var step FakeModelStep
	if len(provider.steps) > 0 {
		step = provider.steps[0]
		provider.steps = provider.steps[1:]
	} else {
		step = cloneFakeModelStep(*provider.repeat)
	}
	provider.requests = append(provider.requests, cloneModelRequest(request))
	provider.mutex.Unlock()
	if step.Err != nil {
		return ModelResponse{}, step.Err
	}
	for _, delta := range step.Deltas {
		if err := ctx.Err(); err != nil {
			return ModelResponse{}, err
		}
		if err := emit(delta); err != nil {
			return ModelResponse{}, err
		}
	}
	return ModelResponse{ToolCall: cloneToolCall(step.ToolCall), Usage: step.Usage}, nil
}

func cloneFakeModelStep(value FakeModelStep) FakeModelStep {
	value.Deltas = append([]string(nil), value.Deltas...)
	value.ToolCall = cloneToolCall(value.ToolCall)
	return value
}

func (provider *FakeProvider) Requests() []ModelRequest {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	result := make([]ModelRequest, len(provider.requests))
	for index, request := range provider.requests {
		result[index] = cloneModelRequest(request)
	}
	return result
}

func (provider *FakeProvider) Remaining() int {
	provider.mutex.Lock()
	defer provider.mutex.Unlock()
	return len(provider.steps)
}

func cloneModelRequest(value ModelRequest) ModelRequest {
	result := value
	result.Messages = append([]ModelMessage(nil), value.Messages...)
	result.Tools = append([]ToolSchema(nil), value.Tools...)
	result.ToolResults = append([]ModelToolResult(nil), value.ToolResults...)
	for index := range result.ToolResults {
		result.ToolResults[index].Data = append([]byte(nil), result.ToolResults[index].Data...)
	}
	return result
}

func cloneToolCall(value *ToolCall) *ToolCall {
	if value == nil {
		return nil
	}
	result := *value
	result.Arguments = append([]byte(nil), value.Arguments...)
	return &result
}

func FakeProviderError(kind string) error {
	switch kind {
	case "unavailable":
		return ErrProviderUnavailable
	case "timeout":
		return ErrProviderTimeout
	default:
		return errors.New("redacted fake provider failure")
	}
}

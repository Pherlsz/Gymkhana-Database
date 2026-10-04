package aichat

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	maximumContextMessages = 30
	maximumContextRunes    = 60_000
	maximumProviderDelta   = 1000
	maximumProviderUsage   = 100_000_000
	finalizationTimeout    = 5 * time.Second
	textDeltaBatchWindow   = 100 * time.Millisecond
	minimumContextMessages = 3 // always include this many most-recent messages
)

// errTurnPanicked is returned to the coordinator when a provider or tool panics.
// The persisted run stores only the public internal_error code.
var errTurnPanicked = errors.New("AI Chat turn panicked")

// panickedTurnFailer marks a runner that can persist a failure after a panic
// escapes RunTurn. Orchestrator implements it; test doubles do not have to.
type panickedTurnFailer interface {
	failPanickedRun(context.Context, auth.Session, Identifier, string)
}

type TurnRunner interface {
	RunTurn(context.Context, auth.Session, Identifier, string) error
}

type Orchestrator struct {
	service  *Service
	tools    *ToolGateway
	provider ModelClient
}

func NewOrchestrator(service *Service, tools *ToolGateway, provider ModelClient) (*Orchestrator, error) {
	if service == nil || tools == nil || provider == nil {
		return nil, ErrInvalidSetup
	}
	return &Orchestrator{service: service, tools: tools, provider: provider}, nil
}

func (orchestrator *Orchestrator) RunTurn(ctx context.Context, actor auth.Session, runID Identifier, requestID string) (resultErr error) {
	if runID.IsZero() {
		return ErrInvalidInput
	}
	user, err := orchestrator.service.authorize(ctx, actor)
	if err != nil {
		return err
	}
	authorizedAt := orchestrator.service.now().UTC()
	revalidate := func(force bool) error {
		now := orchestrator.service.now().UTC()
		if !force && !now.After(authorizedAt.Add(authorizationCacheTTL)) {
			return nil
		}
		fresh, authErr := orchestrator.service.authorizeFresh(ctx, actor)
		if authErr != nil {
			return authErr
		}
		user = fresh
		authorizedAt = now
		return nil
	}
	run, err := orchestrator.service.store.GetRun(ctx, runID, user.ID)
	if err != nil {
		return err
	}
	thread, err := orchestrator.service.store.GetThread(ctx, run.ThreadID, user.ID)
	if err != nil {
		return err
	}
	run, err = orchestrator.service.startRun(ctx, user, runID, requestID)
	if err != nil {
		return err
	}
	var openBatch *textDeltaBatch
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("AI Chat turn panicked", "run_id", runID.String(), "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
			if resultErr == nil {
				resultErr = fmt.Errorf("%w: %v", errTurnPanicked, recovered)
			}
		}
		if resultErr != nil && openBatch != nil {
			_ = openBatch.flush()
		}
		if resultErr != nil {
			finalCtx, finalCancel := detachedFinalizationContext(ctx)
			defer finalCancel()
			_, _ = orchestrator.service.failRun(finalCtx, user, runID, publicErrorCode(resultErr), requestID)
		}
	}()
	runContext, cancel := context.WithTimeout(ctx, orchestrator.service.runTimeout)
	defer cancel()
	messages, err := orchestrator.contextMessages(runContext, actor, thread.ID)
	if err != nil {
		return err
	}
	policy := ReadOnlySystemPolicy
	if note, err := orchestrator.tools.CompactCatalogNote(runContext, actor, requestID); err == nil && note != "" {
		policy += "\n" + note
	}
	request := ModelRequest{Policy: policy, Messages: messages, Tools: orchestrator.tools.Schemas()}
	if thread.ActiveResultReferenceID != nil {
		request.ActiveResultReferenceID = thread.ActiveResultReferenceID.String()
	}
	var roundAssistant strings.Builder
	assistantRunes := 0
	var inputUsage, outputUsage int64
	capability := orchestrator.service.Capability(runContext)
	maximumCalls := capability.MaximumToolCalls
	if maximumCalls <= 0 {
		maximumCalls = MaximumToolCalls
	}
	for round := 0; round <= maximumCalls; round++ {
		if err := revalidate(false); err != nil {
			return err
		}
		current, err := orchestrator.service.runOwned(runContext, user, runID)
		if err != nil {
			return err
		}
		if current.CancelRequestedAt != nil || current.State == RunCancelled {
			if err := revalidate(true); err != nil {
				return err
			}
			return ErrCancelled
		}
		batch := &textDeltaBatch{
			now: orchestrator.service.now,
			write: func(chunk string) error {
				// WithoutCancel so a cancel that already closed the run context
				// still persists the chunk buffered before that cancel.
				_, writeErr := orchestrator.service.appendText(context.WithoutCancel(runContext), user, runID, chunk)
				return writeErr
			},
		}
		openBatch = batch
		response, err := orchestrator.provider.Generate(runContext, request, func(delta string) error {
			deltaRunes := utf8.RuneCountInString(delta)
			if !validProviderDelta(delta) || assistantRunes+deltaRunes > MaximumMessageRunes {
				_ = batch.flush()
				return ErrMalformedProvider
			}
			if runContext.Err() != nil {
				_ = batch.flush()
				return ErrCancelled
			}
			if revalidateErr := revalidate(false); revalidateErr != nil {
				_ = batch.flush()
				return revalidateErr
			}
			current, currentErr := orchestrator.service.runOwned(runContext, user, runID)
			if currentErr != nil {
				_ = batch.flush()
				return currentErr
			}
			if current.CancelRequestedAt != nil || current.State == RunCancelled {
				if flushErr := batch.flush(); flushErr != nil {
					return flushErr
				}
				if revalidateErr := revalidate(true); revalidateErr != nil {
					return revalidateErr
				}
				return ErrCancelled
			}
			if addErr := batch.add(delta); addErr != nil {
				return addErr
			}
			roundAssistant.WriteString(delta)
			assistantRunes += deltaRunes
			return nil
		})
		if flushErr := batch.flush(); flushErr != nil && err == nil {
			err = flushErr
		}
		if err != nil {
			return normalizeProviderError(runContext, err)
		}
		if response.Usage.InputUnits < 0 || response.Usage.OutputUnits < 0 ||
			response.Usage.InputUnits > maximumProviderUsage ||
			response.Usage.OutputUnits > maximumProviderUsage {
			return ErrMalformedProvider
		}
		inputUsage += response.Usage.InputUnits
		outputUsage += response.Usage.OutputUnits
		if err := revalidate(false); err != nil {
			return err
		}
		if _, exceeded, err := orchestrator.service.addUsage(runContext, user, runID, response.Usage); err != nil {
			return err
		} else if exceeded {
			return ErrQuotaExceeded
		}
		if response.ToolCall == nil {
			content := strings.TrimSpace(roundAssistant.String())
			if content == "" && len(request.ToolResults) > 0 {
				content = "Resultados disponíveis para consulta."
			}
			if !validText(content, MaximumMessageRunes, true) {
				return ErrMalformedProvider
			}
			_, _, err := orchestrator.service.completeRun(runContext, user, runID, content, inputUsage, outputUsage, requestID)
			return err
		}
		if round == maximumCalls {
			return ErrQuotaExceeded
		}
		call := *response.ToolCall
		if !validProviderCallID(call.ID) {
			return ErrMalformedProvider
		}
		if _, ok := toolKindFromName(call.Name); !ok {
			return ErrMalformedProvider
		}
		fingerprint, err := toolArgumentsFingerprint(call.Arguments)
		if err != nil {
			return ErrMalformedProvider
		}
		step, err := orchestrator.service.beginTool(runContext, user, runID, call.Name, fingerprint)
		if err != nil {
			return err
		}
		failStep := func(stepErr error) {
			slog.Warn("AI Chat tool step failed", "run_id", runID.String(), "step", step.Sequence, "tool_name", call.Name, "error", stepErr)
			finalCtx, finalCancel := detachedFinalizationContext(runContext)
			defer finalCancel()
			_, _ = orchestrator.service.failTool(finalCtx, user, step.ID, runID, publicErrorCode(stepErr), requestID)
		}
		currentThread, err := orchestrator.service.threadOwned(runContext, user, thread.ID, requestID)
		if err != nil {
			failStep(err)
			return err
		}
		thread = currentThread
		output, err := orchestrator.tools.Execute(runContext, actor, currentThread.ActiveResultReferenceID, runID, step.Sequence, call, currentThread.RetentionExpiresAt, requestID)
		if err != nil {
			if payload, ok := correctableToolPayload(err); ok {
				if _, stepErr := orchestrator.service.failTool(runContext, user, step.ID, runID, "invalid_input", requestID); stepErr != nil {
					return stepErr
				}
				request.ToolResults = append(request.ToolResults, ModelToolResult{
					CallID: call.ID, ToolName: call.Name, Arguments: append([]byte(nil), call.Arguments...), Data: payload, Untrusted: true,
				})
				if roundAssistant.Len() > 0 {
					request.Messages = append(request.Messages, ModelMessage{Role: MessageAssistant, Content: roundAssistant.String(), Untrusted: true})
					roundAssistant.Reset()
				}
				continue
			}
			if errors.Is(err, ErrInvalidInput) {
				err = ErrMalformedProvider
			}
			failStep(err)
			return err
		}
		_, reference, err := orchestrator.service.completeTool(runContext, user, step, thread.ID, output, requestID)
		if err != nil {
			failStep(err)
			return err
		}
		toolResult := ModelToolResult{CallID: call.ID, ToolName: call.Name, Arguments: append([]byte(nil), call.Arguments...), Data: append([]byte(nil), output.Payload...), Untrusted: true}
		if reference != nil {
			toolResult.ReferenceID = reference.ID.String()
			request.ActiveResultReferenceID = reference.ID.String()
		}
		request.ToolResults = append(request.ToolResults, toolResult)
		if roundAssistant.Len() > 0 {
			request.Messages = append(request.Messages, ModelMessage{Role: MessageAssistant, Content: roundAssistant.String(), Untrusted: true})
			roundAssistant.Reset()
		}
	}
	return ErrQuotaExceeded
}

func (orchestrator *Orchestrator) failPanickedRun(ctx context.Context, actor auth.Session, runID Identifier, requestID string) {
	finalCtx, finalCancel := detachedFinalizationContext(ctx)
	defer finalCancel()
	user, err := orchestrator.service.authorize(finalCtx, actor)
	if err != nil {
		return
	}
	_, _ = orchestrator.service.failRun(finalCtx, user, runID, "internal_error", requestID)
}

// textDeltaBatch groups token deltas for about 100ms, or until a provider chunk
// would exceed the per-event rune cap, before one store write.
type textDeltaBatch struct {
	now    func() time.Time
	write  func(string) error
	buf    strings.Builder
	runes  int
	opened time.Time
}

func (batch *textDeltaBatch) add(delta string) error {
	count := utf8.RuneCountInString(delta)
	if batch.buf.Len() > 0 && (batch.expired() || batch.runes+count > maximumProviderDelta) {
		if err := batch.flush(); err != nil {
			return err
		}
	}
	if batch.buf.Len() == 0 {
		batch.opened = batch.now()
	}
	batch.buf.WriteString(delta)
	batch.runes += count
	if batch.runes >= maximumProviderDelta {
		return batch.flush()
	}
	return nil
}

func (batch *textDeltaBatch) expired() bool {
	if batch == nil || batch.opened.IsZero() || batch.now == nil {
		return false
	}
	return !batch.now().Before(batch.opened.Add(textDeltaBatchWindow))
}

func (batch *textDeltaBatch) flush() error {
	if batch == nil || batch.buf.Len() == 0 {
		return nil
	}
	text := batch.buf.String()
	batch.buf.Reset()
	batch.runes = 0
	batch.opened = time.Time{}
	return batch.write(text)
}

func detachedFinalizationContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), finalizationTimeout)
}

func (orchestrator *Orchestrator) contextMessages(ctx context.Context, actor auth.Session, threadID Identifier) ([]ModelMessage, error) {
	page, err := orchestrator.service.Messages(ctx, actor, threadID, MaximumMessagesPage, 0, "chat-context")
	if err != nil {
		return nil, err
	}
	if page.Total > MaximumMessagesPage {
		page, err = orchestrator.service.Messages(ctx, actor, threadID, MaximumMessagesPage, page.Total-MaximumMessagesPage, "chat-context")
		if err != nil {
			return nil, err
		}
	}
	selected := make([]Message, 0, maximumContextMessages)
	runes := 0
	for index := len(page.Messages) - 1; index >= 0 && len(selected) < maximumContextMessages; index-- {
		msg := page.Messages[index]
		count := utf8.RuneCountInString(msg.Content)
		// Always include the most recent minimumContextMessages messages, even if
		// they are long. Only apply the rune budget to older messages.
		recent := index >= len(page.Messages)-minimumContextMessages
		if !recent && runes+count > maximumContextRunes && len(selected) > 0 {
			break
		}
		selected = append(selected, msg)
		runes += count
	}
	result := make([]ModelMessage, len(selected))
	for index := range selected {
		message := selected[len(selected)-1-index]
		result[index] = ModelMessage{Role: message.Role, Content: message.Content, Untrusted: true}
	}
	return result, nil
}

func (service *Service) startRun(ctx context.Context, user auth.User, id Identifier, requestID string) (Run, error) {
	run, err := service.store.StartRun(ctx, id, user.ID, service.now().UTC())
	service.audit(ctx, &user.ID, nil, &id, nil, AuditRunStarted, auditOutcome(err), nil, nil, err, requestID)
	return run, err
}

func (service *Service) appendText(ctx context.Context, user auth.User, id Identifier, delta string) (RunEvent, error) {
	if !validProviderDelta(delta) {
		return RunEvent{}, ErrMalformedProvider
	}
	return service.store.AppendTextDelta(ctx, id, user.ID, delta, service.now().UTC())
}

func (service *Service) addUsage(ctx context.Context, user auth.User, id Identifier, usage ModelUsage) (Run, bool, error) {
	return service.store.AddRunUsage(ctx, id, user.ID, usage.InputUnits, usage.OutputUnits, service.usageLimit, service.now().UTC())
}

func (service *Service) beginTool(ctx context.Context, user auth.User, runID Identifier, name string, fingerprint [sha256.Size]byte) (ToolStep, error) {
	kind, ok := toolKindFromName(name)
	if !ok || fingerprint == ([sha256.Size]byte{}) {
		return ToolStep{}, ErrInvalidInput
	}
	id, err := NewIdentifier()
	if err != nil {
		return ToolStep{}, fmt.Errorf("generate AI Chat tool step identifier: %w", err)
	}
	return service.store.BeginTool(ctx, BeginToolInput{ID: id, RunID: runID, OwnerUserID: user.ID, Kind: kind, ArgumentsFingerprint: fingerprint, Now: service.now().UTC()})
}

func (service *Service) completeTool(ctx context.Context, user auth.User, step ToolStep, threadID Identifier, output ToolOutput, requestID string) (ToolStep, *ResultReference, error) {
	if !output.Kind.Valid() || output.Kind != step.Kind || output.RowCount < 0 || output.RowCount > MaximumToolRows ||
		output.FieldCount < 0 || output.FieldCount > MaximumToolFields || output.ByteCount != len(output.Payload) ||
		output.ByteCount < 0 || output.ByteCount > MaximumToolResultBytes || !json.Valid(output.Payload) {
		return ToolStep{}, nil, ErrUnsafeResult
	}
	if output.Reference != nil && !validResultReferenceDraft(*output.Reference, output, service.now().UTC()) {
		return ToolStep{}, nil, ErrUnsafeResult
	}
	input := CompleteToolInput{StepID: step.ID, RunID: step.RunID, OwnerUserID: user.ID,
		RowCount: output.RowCount, ResultBytes: output.ByteCount, Now: service.now().UTC()}
	if output.Reference != nil {
		id, err := NewIdentifier()
		if err != nil {
			return ToolStep{}, nil, fmt.Errorf("generate AI Chat result reference identifier: %w", err)
		}
		draft := output.Reference
		input.ResultReference = &CreateResultReferenceInput{ID: id, ThreadID: threadID, RunID: step.RunID, OwnerUserID: user.ID,
			Kind: draft.Kind, QueryExecutionID: draft.QueryExecutionID, LogicalRequest: draft.LogicalRequest,
			ContextFingerprint: draft.ContextFingerprint, Label: draft.Label, RowCount: draft.RowCount,
			ColumnCount: draft.ColumnCount, ExpiresAt: draft.ExpiresAt, Now: input.Now}
	}
	completed, reference, err := service.store.CompleteTool(ctx, input)
	affected := output.RowCount
	kind := output.Kind
	service.audit(ctx, &user.ID, nil, &step.RunID, nil, AuditToolExecuted, auditOutcome(err), &kind, &affected, err, requestID)
	return completed, reference, err
}

func validResultReferenceDraft(reference ResultReferenceDraft, output ToolOutput, now time.Time) bool {
	if !reference.Kind.Valid() || !validText(reference.Label, 160, false) || reference.RowCount != output.RowCount ||
		reference.RowCount < 0 || reference.RowCount > MaximumToolRows || reference.ColumnCount != output.FieldCount || reference.ColumnCount < 0 ||
		reference.ColumnCount > MaximumToolFields || len(reference.LogicalRequest) == 0 ||
		len(reference.LogicalRequest) > 32*1024 || !json.Valid(reference.LogicalRequest) ||
		reference.ContextFingerprint == ([sha256.Size]byte{}) || !reference.ExpiresAt.After(now) {
		return false
	}
	if reference.Kind == ResultReferenceQuery && reference.QueryExecutionID == nil {
		return referencePlanNeedsShape(reference.LogicalRequest)
	}
	return (reference.Kind == ResultReferenceQuery) == (reference.QueryExecutionID != nil)
}

func (service *Service) failTool(ctx context.Context, user auth.User, stepID, runID Identifier, code, requestID string) (ToolStep, error) {
	if code == "" || len(code) > 80 {
		code = "tool_failed"
	}
	step, err := service.store.FailTool(ctx, stepID, runID, user.ID, code, service.now().UTC())
	var kind *ToolKind
	if step.Kind.Valid() {
		value := step.Kind
		kind = &value
	}
	auditErr := err
	if auditErr == nil {
		auditErr = errorFromPublicCode(step.ErrorCode)
	}
	service.audit(ctx, &user.ID, nil, &runID, nil, AuditToolExecuted, auditOutcome(auditErr), kind, nil, auditErr, requestID)
	return step, err
}

func (service *Service) completeRun(ctx context.Context, user auth.User, runID Identifier, content string, inputUsage, outputUsage int64, requestID string) (Run, Message, error) {
	if !validText(content, MaximumMessageRunes, true) || inputUsage < 0 || outputUsage < 0 {
		return Run{}, Message{}, ErrMalformedProvider
	}
	messageID, err := NewIdentifier()
	if err != nil {
		return Run{}, Message{}, fmt.Errorf("generate AI Chat assistant message identifier: %w", err)
	}
	run, message, err := service.store.CompleteRun(ctx, CompleteRunInput{MessageID: messageID, RunID: runID,
		OwnerUserID: user.ID, Content: content, InputUsage: inputUsage, OutputUsage: outputUsage, Now: service.now().UTC()})
	service.audit(ctx, &user.ID, nil, &runID, nil, AuditRunCompleted, auditOutcome(err), nil, nil, err, requestID)
	return run, message, err
}

func (service *Service) failRun(ctx context.Context, user auth.User, runID Identifier, code, requestID string) (Run, error) {
	if code == "" || len(code) > 80 {
		code = "internal_error"
	}
	run, err := service.store.FailRun(ctx, runID, user.ID, code, service.now().UTC())
	event := AuditRunFailed
	if run.State == RunCancelled || code == "cancelled" {
		event = AuditRunCancelled
	}
	auditErr := err
	if auditErr == nil && event == AuditRunFailed {
		auditErr = errorFromPublicCode(code)
	}
	service.audit(ctx, &user.ID, nil, &runID, nil, event, auditOutcome(auditErr), nil, nil, auditErr, requestID)
	return run, err
}

func toolArgumentsFingerprint(raw json.RawMessage) ([sha256.Size]byte, error) {
	if len(raw) == 0 || len(raw) > 32*1024 || !json.Valid(raw) {
		return [sha256.Size]byte{}, ErrMalformedProvider
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return [sha256.Size]byte{}, ErrMalformedProvider
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(canonical), nil
}

func toolKindFromName(value string) (ToolKind, bool) {
	kind, ok := toolNameToKind[value]
	return kind, ok
}

func validProviderDelta(value string) bool {
	return validText(value, maximumProviderDelta, true)
}

func normalizeProviderError(ctx context.Context, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrCancelled):
		return ErrCancelled
	case errors.Is(err, context.Canceled), errors.Is(ctx.Err(), context.Canceled):
		return ErrCancelled
	case errors.Is(err, context.DeadlineExceeded), errors.Is(ctx.Err(), context.DeadlineExceeded), errors.Is(err, ErrProviderTimeout):
		return ErrTimeout
	case errors.Is(err, ErrRateLimited):
		return ErrRateLimited
	case errors.Is(err, ErrQuotaExceeded):
		return ErrQuotaExceeded
	case errors.Is(err, ErrMalformedProvider), errors.Is(err, ErrInvalidInput):
		return ErrMalformedProvider
	case errors.Is(err, ErrForbidden), errors.Is(err, ErrNotFound), errors.Is(err, ErrStaleContext):
		return err
	case errors.Is(err, ErrProviderUnavailable):
		return ErrUnavailable
	default:
		return ErrUnavailable
	}
}

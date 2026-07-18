package aichat

import (
	"context"
	"crypto/subtle"
	"sort"
	"sync"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type memoryStore struct {
	mutex       sync.Mutex
	users       map[auth.Identifier]auth.User
	threads     map[Identifier]Thread
	messages    map[Identifier][]Message
	runs        map[Identifier]Run
	steps       map[Identifier]ToolStep
	references  map[Identifier]ResultReference
	events      map[Identifier][]RunEvent
	idempotency map[string]Identifier
	requestRate map[auth.Identifier]int
	usage       map[auth.Identifier]int64
	audits      []AuditEvent
	currentErr  error
}

func newMemoryStore(users ...auth.User) *memoryStore {
	store := &memoryStore{
		users: make(map[auth.Identifier]auth.User), threads: make(map[Identifier]Thread), messages: make(map[Identifier][]Message),
		runs: make(map[Identifier]Run), steps: make(map[Identifier]ToolStep), references: make(map[Identifier]ResultReference),
		events: make(map[Identifier][]RunEvent), idempotency: make(map[string]Identifier), requestRate: make(map[auth.Identifier]int),
		usage: make(map[auth.Identifier]int64),
	}
	for _, user := range users {
		store.users[user.ID] = user
	}
	return store
}

func (store *memoryStore) CurrentUser(_ context.Context, id auth.Identifier) (auth.User, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.currentErr != nil {
		return auth.User{}, store.currentErr
	}
	user, ok := store.users[id]
	if !ok {
		return auth.User{}, ErrForbidden
	}
	return user, nil
}

func (store *memoryStore) CreateThread(_ context.Context, input CreateThreadInput) (Thread, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	thread := Thread{ID: input.ID, OwnerUserID: input.OwnerUserID, Title: input.Title, RetentionExpiresAt: input.RetentionExpiresAt,
		Version: 1, CreatedAt: input.Now, UpdatedAt: input.Now}
	store.threads[thread.ID] = thread
	return thread, nil
}

func (store *memoryStore) ListThreads(_ context.Context, owner auth.Identifier, now time.Time, limit, offset int) (ThreadPage, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	values := make([]Thread, 0)
	for _, thread := range store.threads {
		if thread.OwnerUserID == owner && thread.RetentionExpiresAt.After(now) {
			values = append(values, thread)
		}
	}
	sort.Slice(values, func(left, right int) bool {
		if values[left].UpdatedAt.Equal(values[right].UpdatedAt) {
			return values[left].ID.String() > values[right].ID.String()
		}
		return values[left].UpdatedAt.After(values[right].UpdatedAt)
	})
	return ThreadPage{Threads: pageSlice(values, limit, offset), Total: len(values), Limit: limit, Offset: offset}, nil
}

func (store *memoryStore) GetThread(_ context.Context, id Identifier, owner auth.Identifier) (Thread, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return store.ownedThread(id, owner)
}

func (store *memoryStore) RenameThread(_ context.Context, id Identifier, owner auth.Identifier, title string, version int64, now time.Time) (Thread, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	thread, err := store.ownedThread(id, owner)
	if err != nil {
		return Thread{}, err
	}
	if thread.Version != version {
		return Thread{}, ErrConflict
	}
	thread.Title, thread.Version, thread.UpdatedAt = title, thread.Version+1, now
	store.threads[id] = thread
	return thread, nil
}

func (store *memoryStore) DeleteThread(_ context.Context, id Identifier, owner auth.Identifier) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if _, err := store.ownedThread(id, owner); err != nil {
		return err
	}
	for _, run := range store.runs {
		if run.ThreadID == id && run.State.Active() {
			return ErrConflict
		}
	}
	store.deleteThread(id)
	return nil
}

func (store *memoryStore) ListMessages(_ context.Context, threadID Identifier, owner auth.Identifier, limit, offset int) (MessagePage, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if _, err := store.ownedThread(threadID, owner); err != nil {
		return MessagePage{}, err
	}
	values := append([]Message(nil), store.messages[threadID]...)
	sort.Slice(values, func(left, right int) bool { return values[left].Sequence < values[right].Sequence })
	for index := range values {
		if values[index].Role != MessageAssistant {
			values[index].ResultReferenceIDs = []Identifier{}
			continue
		}
		references := make([]ResultReference, 0)
		for _, reference := range store.references {
			if reference.RunID == values[index].RunID {
				references = append(references, reference)
			}
		}
		sort.Slice(references, func(left, right int) bool {
			if references[left].CreatedAt.Equal(references[right].CreatedAt) {
				return references[left].ID.String() < references[right].ID.String()
			}
			return references[left].CreatedAt.Before(references[right].CreatedAt)
		})
		values[index].ResultReferenceIDs = make([]Identifier, 0, len(references))
		for _, reference := range references {
			values[index].ResultReferenceIDs = append(values[index].ResultReferenceIDs, reference.ID)
		}
	}
	return MessagePage{Messages: pageSlice(values, limit, offset), Total: len(values), Limit: limit, Offset: offset}, nil
}

func (store *memoryStore) CreateRun(_ context.Context, input CreateRunInput, _ time.Time, maximumRequests int, maximumUsage int64) (RunCreation, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	key := input.OwnerUserID.String() + ":" + input.IdempotencyKey
	if existingID, ok := store.idempotency[key]; ok {
		existing := store.runs[existingID]
		if subtle.ConstantTimeCompare(existing.RequestFingerprint[:], input.RequestFingerprint[:]) != 1 {
			return RunCreation{}, ErrConflict
		}
		return RunCreation{Run: existing, UserMessage: store.messageForRun(existingID, MessageUser), Created: false}, nil
	}
	thread, err := store.ownedThread(input.ThreadID, input.OwnerUserID)
	if err != nil {
		return RunCreation{}, err
	}
	if !sameOptionalIdentifier(thread.ActiveResultReferenceID, input.ActiveResultReferenceID) {
		return RunCreation{}, ErrStaleContext
	}
	for _, run := range store.runs {
		if run.ThreadID == input.ThreadID && run.State.Active() {
			return RunCreation{}, ErrConflict
		}
	}
	if input.RetryOfRunID != nil {
		retry, ok := store.runs[*input.RetryOfRunID]
		if !ok {
			return RunCreation{}, ErrNotFound
		}
		if retry.ThreadID != input.ThreadID || retry.OwnerUserID != input.OwnerUserID || !retry.State.Terminal() {
			return RunCreation{}, ErrInvalidState
		}
	}
	if store.usage[input.OwnerUserID] >= maximumUsage {
		return RunCreation{}, ErrQuotaExceeded
	}
	if store.requestRate[input.OwnerUserID] >= maximumRequests {
		return RunCreation{}, ErrRateLimited
	}
	store.requestRate[input.OwnerUserID]++
	run := Run{ID: input.ID, ThreadID: input.ThreadID, OwnerUserID: input.OwnerUserID, RetryOfRunID: cloneIdentifier(input.RetryOfRunID),
		State: RunQueued, IdempotencyKey: input.IdempotencyKey, RequestFingerprint: input.RequestFingerprint, Version: 1,
		CreatedAt: input.Now, UpdatedAt: input.Now}
	message := Message{ID: input.MessageID, ThreadID: input.ThreadID, RunID: input.ID, Sequence: store.nextMessageSequence(input.ThreadID),
		Role: MessageUser, Content: input.Content, CreatedAt: input.Now}
	store.runs[run.ID], store.idempotency[key] = run, run.ID
	store.messages[input.ThreadID] = append(store.messages[input.ThreadID], message)
	store.appendEvent(RunEvent{RunID: run.ID, Kind: EventRunAccepted, CreatedAt: input.Now})
	return RunCreation{Run: run, UserMessage: message, Created: true}, nil
}

func (store *memoryStore) GetRun(_ context.Context, id Identifier, owner auth.Identifier) (Run, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return store.ownedRun(id, owner)
}

func (store *memoryStore) RequestCancellation(_ context.Context, id Identifier, owner auth.Identifier, now time.Time) (Run, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(id, owner)
	if err != nil {
		return Run{}, err
	}
	if run.State.Terminal() {
		return run, nil
	}
	if run.CancelRequestedAt == nil {
		requested := now
		run.CancelRequestedAt = &requested
		run.Version++
		run.UpdatedAt = now
	}
	if run.State == RunQueued {
		run.State, run.ErrorCode = RunCancelled, "cancelled"
		completed := now
		run.CompletedAt = &completed
		store.appendEvent(RunEvent{RunID: id, Kind: EventRunCancelled, ErrorCode: "cancelled", CreatedAt: now})
	}
	store.runs[id] = run
	return run, nil
}

func (store *memoryStore) StartRun(_ context.Context, id Identifier, owner auth.Identifier, now time.Time) (Run, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(id, owner)
	if err != nil {
		return Run{}, err
	}
	if run.State == RunCancelled || run.CancelRequestedAt != nil {
		return Run{}, ErrCancelled
	}
	if run.State != RunQueued {
		return Run{}, ErrInvalidState
	}
	run.State, run.StartedAt, run.UpdatedAt, run.Version = RunRunning, timePointer(now), now, run.Version+1
	store.runs[id] = run
	store.appendEvent(RunEvent{RunID: id, Kind: EventRunStarted, CreatedAt: now})
	return run, nil
}

func (store *memoryStore) AppendTextDelta(_ context.Context, id Identifier, owner auth.Identifier, delta string, now time.Time) (RunEvent, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(id, owner)
	if err != nil {
		return RunEvent{}, err
	}
	if run.CancelRequestedAt != nil {
		return RunEvent{}, ErrCancelled
	}
	if run.State != RunRunning {
		return RunEvent{}, ErrInvalidState
	}
	return store.appendEvent(RunEvent{RunID: id, Kind: EventTextDelta, TextDelta: delta, CreatedAt: now}), nil
}

func (store *memoryStore) AddRunUsage(_ context.Context, id Identifier, owner auth.Identifier, input, output, maximum int64, now time.Time) (Run, bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(id, owner)
	if err != nil {
		return Run{}, false, err
	}
	if !run.State.Active() {
		return Run{}, false, ErrInvalidState
	}
	run.InputUsage, run.OutputUsage = run.InputUsage+input, run.OutputUsage+output
	run.Version, run.UpdatedAt = run.Version+1, now
	store.runs[id] = run
	store.usage[owner] += input + output
	return run, store.usage[owner] > maximum, nil
}

func (store *memoryStore) BeginTool(_ context.Context, input BeginToolInput) (ToolStep, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(input.RunID, input.OwnerUserID)
	if err != nil {
		return ToolStep{}, err
	}
	if run.CancelRequestedAt != nil {
		return ToolStep{}, ErrCancelled
	}
	if run.State != RunRunning {
		return ToolStep{}, ErrInvalidState
	}
	if run.ToolCallCount >= MaximumToolCalls {
		return ToolStep{}, ErrQuotaExceeded
	}
	step := ToolStep{ID: input.ID, RunID: input.RunID, Sequence: run.ToolCallCount + 1, Kind: input.Kind, State: ToolStepRunning,
		ArgumentsFingerprint: input.ArgumentsFingerprint, StartedAt: input.Now}
	run.State, run.ToolCallCount, run.Version, run.UpdatedAt = RunToolRunning, step.Sequence, run.Version+1, input.Now
	store.steps[step.ID], store.runs[run.ID] = step, run
	stepID := step.ID
	store.appendEvent(RunEvent{RunID: run.ID, Kind: EventToolStarted, ToolStepID: &stepID, CreatedAt: input.Now})
	return step, nil
}

func (store *memoryStore) CompleteTool(_ context.Context, input CompleteToolInput) (ToolStep, *ResultReference, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(input.RunID, input.OwnerUserID)
	if err != nil {
		return ToolStep{}, nil, err
	}
	step, ok := store.steps[input.StepID]
	if !ok || step.RunID != run.ID || step.State != ToolStepRunning || run.State != RunToolRunning {
		return ToolStep{}, nil, ErrInvalidState
	}
	if run.CancelRequestedAt != nil {
		return ToolStep{}, nil, ErrCancelled
	}
	if input.ResultBytes < 0 || run.ResultBytes+int64(input.ResultBytes) > MaximumToolResultBytes {
		return ToolStep{}, nil, ErrQuotaExceeded
	}
	step.State, step.RowCount, step.ResultBytes, step.CompletedAt = ToolStepCompleted, input.RowCount, input.ResultBytes, timePointer(input.Now)
	var reference *ResultReference
	if input.ResultReference != nil {
		draft := input.ResultReference
		value := ResultReference{ID: draft.ID, ThreadID: draft.ThreadID, RunID: draft.RunID, OwnerUserID: draft.OwnerUserID,
			Kind: draft.Kind, QueryExecutionID: cloneIdentifier(draft.QueryExecutionID), LogicalRequest: append([]byte(nil), draft.LogicalRequest...),
			ContextFingerprint: draft.ContextFingerprint, Label: draft.Label, RowCount: draft.RowCount, ColumnCount: draft.ColumnCount,
			ExpiresAt: draft.ExpiresAt, CreatedAt: draft.Now}
		store.references[value.ID], reference = value, &value
		step.ResultReferenceID = &value.ID
		thread := store.threads[run.ThreadID]
		thread.ActiveResultReferenceID, thread.Version, thread.UpdatedAt = &value.ID, thread.Version+1, input.Now
		store.threads[thread.ID] = thread
	}
	run.State, run.ResultBytes, run.Version, run.UpdatedAt = RunRunning, run.ResultBytes+int64(input.ResultBytes), run.Version+1, input.Now
	store.steps[step.ID], store.runs[run.ID] = step, run
	stepID := step.ID
	store.appendEvent(RunEvent{RunID: run.ID, Kind: EventToolCompleted, ToolStepID: &stepID, CreatedAt: input.Now})
	if reference != nil {
		referenceID := reference.ID
		store.appendEvent(RunEvent{RunID: run.ID, Kind: EventResultReference, ResultReferenceID: &referenceID, CreatedAt: input.Now})
	}
	return step, reference, nil
}

func (store *memoryStore) FailTool(_ context.Context, stepID, runID Identifier, owner auth.Identifier, code string, now time.Time) (ToolStep, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(runID, owner)
	if err != nil {
		return ToolStep{}, err
	}
	step, ok := store.steps[stepID]
	if !ok || step.RunID != runID || step.State != ToolStepRunning || run.State != RunToolRunning {
		return ToolStep{}, ErrInvalidState
	}
	step.State = ToolStepFailed
	if run.CancelRequestedAt != nil {
		step.State, code = ToolStepCancelled, "cancelled"
	}
	step.ErrorCode, step.CompletedAt = code, timePointer(now)
	run.State, run.Version, run.UpdatedAt = RunRunning, run.Version+1, now
	store.steps[stepID], store.runs[runID] = step, run
	return step, nil
}

func (store *memoryStore) CompleteRun(_ context.Context, input CompleteRunInput) (Run, Message, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(input.RunID, input.OwnerUserID)
	if err != nil {
		return Run{}, Message{}, err
	}
	if run.CancelRequestedAt != nil {
		run.State, run.ErrorCode, run.CompletedAt = RunCancelled, "cancelled", timePointer(input.Now)
		store.runs[run.ID] = run
		store.appendEvent(RunEvent{RunID: run.ID, Kind: EventRunCancelled, ErrorCode: "cancelled", CreatedAt: input.Now})
		return Run{}, Message{}, ErrCancelled
	}
	if run.State != RunRunning {
		return Run{}, Message{}, ErrInvalidState
	}
	message := Message{ID: input.MessageID, ThreadID: run.ThreadID, RunID: run.ID, Sequence: store.nextMessageSequence(run.ThreadID),
		Role: MessageAssistant, Content: input.Content, CreatedAt: input.Now}
	store.messages[run.ThreadID] = append(store.messages[run.ThreadID], message)
	run.State, run.InputUsage, run.OutputUsage = RunCompleted, input.InputUsage, input.OutputUsage
	run.CompletedAt, run.UpdatedAt, run.Version = timePointer(input.Now), input.Now, run.Version+1
	store.runs[run.ID] = run
	store.appendEvent(RunEvent{RunID: run.ID, Kind: EventRunCompleted, CreatedAt: input.Now})
	return run, message, nil
}

func (store *memoryStore) FailRun(_ context.Context, id Identifier, owner auth.Identifier, code string, now time.Time) (Run, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(id, owner)
	if err != nil {
		return Run{}, err
	}
	if run.State.Terminal() {
		return run, nil
	}
	kind := EventRunFailed
	if run.CancelRequestedAt != nil {
		run.State, run.ErrorCode, kind = RunCancelled, "cancelled", EventRunCancelled
	} else {
		run.State, run.ErrorCode = RunFailed, code
	}
	run.CompletedAt, run.UpdatedAt, run.Version = timePointer(now), now, run.Version+1
	store.runs[id] = run
	store.appendEvent(RunEvent{RunID: id, Kind: kind, ErrorCode: run.ErrorCode, CreatedAt: now})
	return run, nil
}

func (store *memoryStore) FailStaleRuns(_ context.Context, before, now time.Time, limit int) (int, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	identifiers := make([]Identifier, 0)
	for id, run := range store.runs {
		if run.State.Active() && !run.CreatedAt.After(before) {
			identifiers = append(identifiers, id)
		}
	}
	sort.Slice(identifiers, func(left, right int) bool { return identifiers[left].String() < identifiers[right].String() })
	if len(identifiers) > limit {
		identifiers = identifiers[:limit]
	}
	for _, id := range identifiers {
		run := store.runs[id]
		kind := EventRunFailed
		if run.CancelRequestedAt == nil {
			run.State, run.ErrorCode = RunFailed, "timeout"
		} else {
			run.State, run.ErrorCode, kind = RunCancelled, "cancelled", EventRunCancelled
		}
		run.CompletedAt, run.UpdatedAt, run.Version = timePointer(now), now, run.Version+1
		store.runs[id] = run
		for stepID, step := range store.steps {
			if step.RunID != id || step.State != ToolStepRunning {
				continue
			}
			step.CompletedAt, step.ErrorCode = timePointer(now), run.ErrorCode
			if run.State == RunCancelled {
				step.State = ToolStepCancelled
			} else {
				step.State = ToolStepFailed
			}
			store.steps[stepID] = step
		}
		store.appendEvent(RunEvent{RunID: id, Kind: kind, ErrorCode: run.ErrorCode, CreatedAt: now})
	}
	return len(identifiers), nil
}

func (store *memoryStore) ListRunEvents(_ context.Context, runID Identifier, owner auth.Identifier, after int64, limit int) (EventPage, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	run, err := store.ownedRun(runID, owner)
	if err != nil {
		return EventPage{}, err
	}
	values := make([]RunEvent, 0, limit)
	hasMore := false
	for _, event := range store.events[runID] {
		if event.Sequence <= after {
			continue
		}
		if len(values) == limit {
			hasMore = true
			break
		}
		values = append(values, event)
	}
	last := after
	if len(values) > 0 {
		last = values[len(values)-1].Sequence
	}
	return EventPage{Events: values, LastSequence: last, Terminal: run.State.Terminal() && !hasMore}, nil
}

func (store *memoryStore) GetResultReference(_ context.Context, id Identifier, owner auth.Identifier) (ResultReference, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	reference, ok := store.references[id]
	if !ok || reference.OwnerUserID != owner {
		return ResultReference{}, ErrNotFound
	}
	return reference, nil
}

func (store *memoryStore) SetActiveResultReference(_ context.Context, threadID Identifier, owner auth.Identifier, referenceID *Identifier, now time.Time) (Thread, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	thread, err := store.ownedThread(threadID, owner)
	if err != nil {
		return Thread{}, err
	}
	if referenceID != nil {
		reference, ok := store.references[*referenceID]
		if !ok || reference.OwnerUserID != owner || reference.ThreadID != threadID || !reference.ExpiresAt.After(now) {
			return Thread{}, ErrStaleContext
		}
	}
	thread.ActiveResultReferenceID, thread.Version, thread.UpdatedAt = cloneIdentifier(referenceID), thread.Version+1, now
	store.threads[threadID] = thread
	return thread, nil
}

func (store *memoryStore) CleanupExpired(_ context.Context, now time.Time, limit int) (int, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	identifiers := make([]Identifier, 0)
	for id, thread := range store.threads {
		if thread.RetentionExpiresAt.After(now) {
			continue
		}
		active := false
		for _, run := range store.runs {
			active = active || (run.ThreadID == id && run.State.Active())
		}
		if !active {
			identifiers = append(identifiers, id)
		}
	}
	sort.Slice(identifiers, func(left, right int) bool { return identifiers[left].String() < identifiers[right].String() })
	if len(identifiers) > limit {
		identifiers = identifiers[:limit]
	}
	for _, id := range identifiers {
		store.deleteThread(id)
	}
	return len(identifiers), nil
}

func (store *memoryStore) SaveAudit(_ context.Context, event AuditEvent) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.audits = append(store.audits, event)
	return nil
}

func (store *memoryStore) ownedThread(id Identifier, owner auth.Identifier) (Thread, error) {
	thread, ok := store.threads[id]
	if !ok || thread.OwnerUserID != owner {
		return Thread{}, ErrNotFound
	}
	return thread, nil
}

func (store *memoryStore) ownedRun(id Identifier, owner auth.Identifier) (Run, error) {
	run, ok := store.runs[id]
	if !ok || run.OwnerUserID != owner {
		return Run{}, ErrNotFound
	}
	return run, nil
}

func (store *memoryStore) messageForRun(id Identifier, role MessageRole) Message {
	for _, message := range store.messages[store.runs[id].ThreadID] {
		if message.RunID == id && message.Role == role {
			return message
		}
	}
	return Message{}
}

func (store *memoryStore) nextMessageSequence(threadID Identifier) int64 {
	var maximum int64
	for _, message := range store.messages[threadID] {
		if message.Sequence > maximum {
			maximum = message.Sequence
		}
	}
	return maximum + 1
}

func (store *memoryStore) appendEvent(event RunEvent) RunEvent {
	event.Sequence = int64(len(store.events[event.RunID]) + 1)
	store.events[event.RunID] = append(store.events[event.RunID], event)
	return event
}

func (store *memoryStore) deleteThread(id Identifier) {
	delete(store.threads, id)
	delete(store.messages, id)
	for runID, run := range store.runs {
		if run.ThreadID != id {
			continue
		}
		delete(store.runs, runID)
		delete(store.events, runID)
		for stepID, step := range store.steps {
			if step.RunID == runID {
				delete(store.steps, stepID)
			}
		}
	}
	for referenceID, reference := range store.references {
		if reference.ThreadID == id {
			delete(store.references, referenceID)
		}
	}
}

func pageSlice[T any](values []T, limit, offset int) []T {
	if offset >= len(values) {
		return []T{}
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	return append([]T(nil), values[offset:end]...)
}

func cloneIdentifier(value *Identifier) *Identifier {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func timePointer(value time.Time) *time.Time { return &value }

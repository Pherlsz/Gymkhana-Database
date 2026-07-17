package googleforms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func (service *Service) Sync(ctx context.Context, id Identifier) error {
	if service == nil || !service.enabled {
		return ErrDisabled
	}
	now := service.now().UTC()
	run, source, connection, actor, err := service.store.BeginSync(ctx, id, now)
	if err != nil {
		if terminalSyncStartError(err) {
			changed, markErr := service.store.FailSync(ctx, id, syncErrorCode(err), now)
			if markErr != nil {
				return errors.Join(err, markErr)
			}
			if changed {
				service.audit(ctx, nil, nil, nil, &id, AuditSyncFailed, auth.AuditOutcomeFailure, nil, workerRequestID("sync", id))
			}
		}
		return err
	}
	service.audit(ctx, run.ActorUserID, nil, nil, &run.ID, AuditSyncStarted, auth.AuditOutcomeSuccess, nil, workerRequestID("sync", run.ID))
	refreshToken, err := service.openRefreshToken(connection)
	if err != nil {
		return service.failSync(ctx, run, err)
	}
	form, err := retryProvider(ctx, func() (Form, error) {
		return service.provider.GetForm(ctx, refreshToken, source.ProviderFormID)
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return service.failSync(ctx, run, service.handleProviderConnectionError(ctx, source.OwnerUserID, err))
	}
	refreshed, drifted, err := service.store.RefreshSourceSchema(ctx, source.ID, source.OwnerUserID, form, service.now().UTC())
	if err != nil {
		return service.failSync(ctx, run, err)
	}
	if drifted {
		return service.failSync(ctx, run, ErrSchemaDrift)
	}
	source = refreshed
	after := run.CursorStartedAt
	pageToken := source.ResponsePageToken
	seenTokens := make(map[string]struct{})
	if pageToken != "" {
		seenTokens[pageToken] = struct{}{}
	}
	responses := make([]Response, 0, service.responsePageSize)
	for pageNumber := 0; pageNumber < MaximumPagesPerRun && len(responses) < MaximumResponsesPerRun; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		pageSize := min(service.responsePageSize, MaximumResponsesPerRun-len(responses))
		page, pageErr := retryProvider(ctx, func() (ResponsePage, error) {
			return service.provider.ListResponses(ctx, refreshToken, source.ProviderFormID, after, pageToken, pageSize)
		})
		if pageErr != nil {
			if errors.Is(pageErr, context.Canceled) || errors.Is(pageErr, context.DeadlineExceeded) {
				return pageErr
			}
			return service.failSync(ctx, run, service.handleProviderConnectionError(ctx, source.OwnerUserID, pageErr))
		}
		if len(page.Responses) > pageSize {
			return service.failSync(ctx, run, fmt.Errorf("provider exceeded requested page size: %w", ErrProvider))
		}
		responses = append(responses, page.Responses...)
		nextPageToken := page.NextPageToken
		if nextPageToken == "" {
			pageToken = ""
			break
		}
		if _, cycle := seenTokens[nextPageToken]; cycle {
			return service.failSync(ctx, run, fmt.Errorf("provider pagination cycle: %w", ErrProvider))
		}
		seenTokens[nextPageToken] = struct{}{}
		pageToken = nextPageToken
	}
	if len(responses) > MaximumResponsesPerRun*MaximumPagesPerRun {
		return service.failSync(ctx, run, ErrProvider)
	}
	sort.Slice(responses, func(left, right int) bool {
		if responses[left].SubmittedAt.Equal(responses[right].SubmittedAt) {
			return responses[left].ID < responses[right].ID
		}
		return responses[left].SubmittedAt.Before(responses[right].SubmittedAt)
	})
	staged, err := service.store.StageResponses(ctx, run, source, responses, service.now().UTC(), service.now().UTC().Add(service.importRetention))
	if err != nil {
		if errors.Is(err, ErrCancelled) || errors.Is(err, context.Canceled) {
			return err
		}
		return service.failSync(ctx, run, err)
	}
	if staged.Import != nil {
		if _, err := service.operations.Preview(ctx, actor, staged.Import.ID, staged.Import.Version, workerRequestID("preview", run.ID)); err != nil {
			return service.failSync(ctx, run, fmt.Errorf("preview staged responses: %w", err))
		}
	}
	completed, err := service.store.CompleteSync(ctx, run.ID, staged.Cursor, pageToken, staged.ReceivedCount, staged.StagedCount, staged.DuplicateCount, service.now().UTC())
	if err != nil {
		return err
	}
	affected := completed.StagedCount
	service.audit(ctx, completed.ActorUserID, nil, nil, &completed.ID, AuditSyncCompleted, auth.AuditOutcomeSuccess, &affected, workerRequestID("sync", run.ID))
	return nil
}

func (service *Service) FailSync(ctx context.Context, id Identifier, cause error) error {
	if cause == nil {
		cause = ErrProvider
	}
	changed, err := service.store.FailSync(ctx, id, syncErrorCode(cause), service.now().UTC())
	if err != nil {
		return errors.Join(cause, err)
	}
	if changed {
		service.audit(ctx, nil, nil, nil, &id, AuditSyncFailed, auth.AuditOutcomeFailure, nil, workerRequestID("sync", id))
	}
	return cause
}

func (service *Service) failSync(ctx context.Context, run SyncRun, cause error) error {
	changed, err := service.store.FailSync(ctx, run.ID, syncErrorCode(cause), service.now().UTC())
	if err != nil {
		return errors.Join(cause, err)
	}
	if changed {
		service.audit(ctx, run.ActorUserID, nil, nil, &run.ID, AuditSyncFailed, auth.AuditOutcomeFailure, nil, workerRequestID("sync", run.ID))
	}
	return cause
}

func (service *Service) EnqueueDueSources(ctx context.Context) (int, error) {
	if service == nil || !service.enabled {
		return 0, ErrDisabled
	}
	now := service.now().UTC()
	sources, err := service.store.DueSources(ctx, now, service.syncBatchSize)
	if err != nil {
		return 0, err
	}
	queued := 0
	for _, source := range sources {
		id, idErr := NewIdentifier()
		if idErr != nil {
			return queued, fmt.Errorf("generate scheduled sync identifier: %w", idErr)
		}
		window := now
		if source.NextSyncAt != nil {
			window = source.NextSyncAt.UTC()
		}
		idempotencyKey := "poll:" + source.ID.String() + ":" + window.Format("20060102T150405Z")
		run, createErr := service.store.CreateSyncRun(ctx, SyncRun{
			ID: id, SourceID: source.ID, OwnerUserID: source.OwnerUserID,
			TriggerKind: TriggerScheduled, State: SyncQueued, IdempotencyKey: idempotencyKey,
			CursorStartedAt: syncCursor(source), CreatedAt: now, UpdatedAt: now,
		})
		if createErr != nil {
			if errors.Is(createErr, ErrConflict) || errors.Is(createErr, ErrRateLimited) {
				continue
			}
			return queued, createErr
		}
		if run.ID != id || run.RiverJobID > 0 {
			continue
		}
		jobID, enqueueErr := service.jobs.EnqueueSync(ctx, run.ID)
		if enqueueErr != nil {
			_, failErr := service.store.FailSync(ctx, run.ID, "enqueue_failed", service.now().UTC())
			return queued, errors.Join(enqueueErr, failErr)
		}
		if setErr := service.store.SetSyncJob(ctx, run.ID, jobID, now); setErr != nil {
			cancelErr := service.jobs.Cancel(ctx, jobID)
			_, failErr := service.store.FailSync(ctx, run.ID, "enqueue_failed", service.now().UTC())
			return queued, errors.Join(setErr, cancelErr, failErr)
		}
		service.audit(ctx, nil, nil, nil, &run.ID, AuditSyncRequested, auth.AuditOutcomeSuccess, nil, workerRequestID("due", run.ID))
		queued++
	}
	return queued, nil
}

func (service *Service) ScheduleDuePoll(ctx context.Context) error {
	if service == nil || !service.enabled {
		return nil
	}
	window := service.now().UTC().Truncate(5 * time.Minute)
	_, err := service.jobs.EnqueueDue(ctx, window)
	return err
}

func syncErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrNeedsReauth):
		return "needs_reauth"
	case errors.Is(err, ErrSchemaDrift):
		return "schema_drift"
	case errors.Is(err, ErrResponseChanged):
		return "response_changed"
	case errors.Is(err, ErrUnsupportedForm):
		return "unsupported_form"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrProviderRetryable):
		return "provider_retry_exhausted"
	case errors.Is(err, ErrProvider):
		return "provider_failed"
	case errors.Is(err, ErrCancelled), errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, ErrInvalidInput):
		return "invalid_response"
	default:
		return "sync_failed"
	}
}

func terminalSyncStartError(err error) bool {
	return errors.Is(err, ErrCancelled) || errors.Is(err, ErrForbidden) ||
		errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidInput) ||
		errors.Is(err, ErrInvalidState) || errors.Is(err, ErrNeedsReauth) ||
		errors.Is(err, ErrSchemaDrift) || errors.Is(err, ErrUnsupportedForm)
}

func workerRequestID(kind string, id Identifier) string {
	return "worker:" + kind + ":" + id.String()
}

func retryProvider[T any](ctx context.Context, call func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; attempt < 4; attempt++ {
		value, err := call()
		if err == nil {
			return value, nil
		}
		if !errors.Is(err, ErrProviderRetryable) || attempt == 3 {
			return zero, err
		}
		delay := time.Duration(1<<attempt) * 250 * time.Millisecond
		var providerError *ProviderError
		if errors.As(err, &providerError) && providerError.RetryAfter > delay {
			delay = providerError.RetryAfter
		}
		delay = jitteredProviderDelay(delay)
		if providerError != nil && delay < providerError.RetryAfter {
			delay = providerError.RetryAfter
		}
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, ErrProvider
}

func jitteredProviderDelay(value time.Duration) time.Duration {
	if value <= 0 {
		return 0
	}
	var sample [1]byte
	if _, err := rand.Read(sample[:]); err != nil {
		return value
	}
	// Bounded +/-25% jitter prevents concurrent sources from retrying in lockstep.
	offset := int64(sample[0]) - 128
	return value + time.Duration(int64(value)*offset/512)
}

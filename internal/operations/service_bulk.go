package operations

import (
	"context"
	"errors"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func (service *Service) BulkDelete(ctx context.Context, actor auth.Session, module Module, items []BulkItem, confirmation, requestID string) (BulkDeleteResult, error) {
	catalog, ok := moduleCatalog(actor.User.Role, module)
	if !actor.User.Active || !ok || !catalog.CanBulkDelete {
		service.audit(ctx, &actor.User.ID, nil, nil, module, AuditBulkDelete, auth.AuditOutcomeDenied, nil, requestID)
		return BulkDeleteResult{}, ErrForbidden
	}
	if confirmation != BulkDeleteConfirmation {
		return BulkDeleteResult{}, ErrInvalidConfirmation
	}
	if len(items) == 0 || len(items) > MaximumBulkSelection {
		return BulkDeleteResult{}, ErrInvalidInput
	}
	deleted, err := service.store.BulkDelete(ctx, actor.User.ID, module, items, requestID, service.now().UTC())
	if err != nil {
		service.audit(ctx, &actor.User.ID, nil, nil, module, AuditBulkDelete, auth.AuditOutcomeFailure, nil, requestID)
		return BulkDeleteResult{}, err
	}
	return BulkDeleteResult{Module: module, Deleted: deleted}, nil
}

func (service *Service) Cleanup(ctx context.Context) (int, error) {
	now := service.now().UTC()
	candidates, err := service.store.ClaimCleanupCandidates(ctx, now, service.cleanupBatch)
	if err != nil {
		return 0, err
	}
	cleaned := 0
	for _, candidate := range candidates {
		if candidate.RequiresObjectDeletion {
			if err := service.objects.Delete(ctx, candidate.ObjectKey); err != nil {
				releaseErr := service.store.ReleaseCleanupCandidate(ctx, candidate)
				return cleaned, errors.Join(err, releaseErr)
			}
		}
		if err := service.store.MarkObjectDeleted(ctx, candidate, now); err != nil {
			return cleaned, err
		}
		if candidate.Kind == "IMPORT" {
			service.audit(ctx, &candidate.ActorUserID, &candidate.ID, nil, candidate.Module, AuditImportExpired, auth.AuditOutcomeSuccess, nil, workerRequestID("cleanup", candidate.ID))
		} else {
			service.audit(ctx, &candidate.ActorUserID, nil, &candidate.ID, candidate.Module, AuditExportExpired, auth.AuditOutcomeSuccess, nil, workerRequestID("cleanup", candidate.ID))
		}
		cleaned++
	}
	return cleaned, nil
}

func (service *Service) ScheduleCleanup(ctx context.Context) error {
	window := service.now().UTC().Truncate(15 * time.Minute)
	_, err := service.jobs.EnqueueCleanup(ctx, window)
	return err
}

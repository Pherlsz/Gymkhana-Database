package operations

import (
	"context"
	"errors"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

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
		service.audit(ctx, &candidate.ActorUserID, &candidate.ID, nil, candidate.Module, AuditImportExpired, auth.AuditOutcomeSuccess, nil, workerRequestID("cleanup", candidate.ID))
		cleaned++
	}
	return cleaned, nil
}

func (service *Service) ScheduleCleanup(ctx context.Context) error {
	window := service.now().UTC().Truncate(15 * time.Minute)
	_, err := service.jobs.EnqueueCleanup(ctx, window)
	return err
}

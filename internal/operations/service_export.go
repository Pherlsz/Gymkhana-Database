package operations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func (service *Service) CreateExport(ctx context.Context, actor auth.Session, module Module, idempotencyKey, requestID string) (Export, error) {
	catalog, ok := moduleCatalog(actor.User.Role, module)
	if !actor.User.Active || !ok || !catalog.CanExport {
		service.audit(ctx, &actor.User.ID, nil, nil, module, AuditExportCreated, auth.AuditOutcomeDenied, nil, requestID)
		return Export{}, ErrForbidden
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !validIdempotencyKey(idempotencyKey) {
		return Export{}, ErrInvalidInput
	}
	id, err := NewIdentifier()
	if err != nil {
		return Export{}, err
	}
	now := service.now().UTC()
	jobID, err := service.jobs.EnqueueExport(ctx, id)
	if err != nil {
		return Export{}, err
	}
	value := Export{
		ID: id, ActorUserID: actor.User.ID, Module: module, IdempotencyKey: idempotencyKey,
		State: ExportQueued, ObjectKey: exportObjectKey(id, now), Filename: exportFilename(module, now),
		RiverJobID: jobID, ExpiresAt: now.Add(service.exportRetention), CreatedAt: now, UpdatedAt: now,
	}
	created, err := service.store.CreateExport(ctx, value, service.limits(now))
	if err != nil {
		_ = service.jobs.Cancel(ctx, jobID)
		return Export{}, err
	}
	if created.ID != id {
		_ = service.jobs.Cancel(ctx, jobID)
		if created.Module != module || created.State == ExportFailed || created.State == ExportCancelled || created.State == ExportExpired || !created.ExpiresAt.After(now) {
			service.audit(ctx, &actor.User.ID, nil, &created.ID, created.Module, AuditExportCreated, auth.AuditOutcomeFailure, nil, requestID)
			return Export{}, ErrConflict
		}
	}
	service.audit(ctx, &actor.User.ID, nil, &created.ID, module, AuditExportCreated, auth.AuditOutcomeSuccess, nil, requestID)
	return created, nil
}

func (service *Service) GenerateExport(ctx context.Context, id Identifier) error {
	value, err := service.store.BeginExport(ctx, id, service.now().UTC())
	if err != nil {
		return err
	}
	if value.State == ExportCompleted {
		return nil
	}
	actor, err := service.store.GetActor(ctx, value.ActorUserID)
	catalog, ok := moduleCatalog(actor.User.Role, value.Module)
	if err != nil || !actor.User.Active || !ok || !catalog.CanExport {
		return service.failExport(ctx, value, ExportRunning, "actor_forbidden", workerRequestID("export", id), ErrForbidden)
	}
	dataset, err := service.store.ExportDataset(ctx, value.Module)
	if err != nil {
		return service.failExport(ctx, value, ExportRunning, "query_failed", workerRequestID("export", id), err)
	}
	data, err := WriteWorkbook(value.Module.Label(), dataset.Headers, dataset.Rows)
	if err != nil {
		return service.failExport(ctx, value, ExportRunning, "generation_failed", workerRequestID("export", id), err)
	}
	if err := service.objects.Put(ctx, value.ObjectKey, bytes.NewReader(data), int64(len(data)), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"); err != nil {
		return service.failExport(ctx, value, ExportRunning, "storage_unavailable", workerRequestID("export", id), err)
	}
	digest := sha256.Sum256(data)
	completed, err := service.store.CompleteExport(ctx, id, len(dataset.Rows), int64(len(data)), digest, service.now().UTC())
	if err != nil {
		return err
	}
	service.audit(ctx, &value.ActorUserID, nil, &id, value.Module, AuditExportCompleted, auth.AuditOutcomeSuccess, &completed.RowCount, workerRequestID("export", id))
	return nil
}

func (service *Service) GetExport(ctx context.Context, actor auth.Session, id Identifier) (Export, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return Export{}, ErrForbidden
	}
	return service.store.GetExport(ctx, id, actor.User.ID)
}

func (service *Service) ListExports(ctx context.Context, actor auth.Session, options ListOptions) (ExportPage, error) {
	if !actor.User.Active || !actor.User.Role.CanUseOperations() {
		return ExportPage{}, ErrForbidden
	}
	return service.store.ListExports(ctx, actor.User.ID, options)
}

func (service *Service) DownloadExport(ctx context.Context, actor auth.Session, id Identifier, requestID string) (DownloadGrant, error) {
	value, err := service.store.GetExport(ctx, id, actor.User.ID)
	if err != nil {
		return DownloadGrant{}, err
	}
	catalog, ok := moduleCatalog(actor.User.Role, value.Module)
	if !actor.User.Active || !ok || !catalog.CanExport {
		return DownloadGrant{}, ErrForbidden
	}
	if value.State != ExportCompleted || !value.ExpiresAt.After(service.now().UTC()) {
		return DownloadGrant{}, ErrInvalidState
	}
	signed, err := service.objects.PresignDownload(ctx, value.ObjectKey, value.Filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", service.downloadTTL)
	if err != nil {
		return DownloadGrant{}, err
	}
	service.audit(ctx, &actor.User.ID, nil, &id, value.Module, AuditExportDownloaded, auth.AuditOutcomeSuccess, nil, requestID)
	return DownloadGrant{URL: signed.URL, Method: signed.Method, ExpiresAt: signed.ExpiresAt}, nil
}

func (service *Service) failExport(ctx context.Context, value Export, expected ExportState, code, requestID string, cause error) error {
	changed, failErr := service.store.FailExport(ctx, value.ID, expected, code, service.now().UTC())
	if changed {
		service.audit(ctx, &value.ActorUserID, nil, &value.ID, value.Module, AuditExportFailed, auth.AuditOutcomeFailure, nil, requestID)
	}
	return errors.Join(cause, failErr)
}

func exportObjectKey(id Identifier, now time.Time) string {
	return fmt.Sprintf("operations/exports/%04d/%02d/%s.xlsx", now.Year(), now.Month(), id.String())
}

func exportFilename(module Module, now time.Time) string {
	return strings.ToLower(string(module)) + "-" + now.Format("20060102-150405") + ".xlsx"
}

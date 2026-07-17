package customdata

import (
	"context"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func (service *Service) GetValues(ctx context.Context, actor auth.Session, target TargetReference) (ValueSet, error) {
	if !canRead(actor) {
		return ValueSet{}, ErrForbidden
	}
	if !target.Valid() {
		return ValueSet{}, ErrInvalidTarget
	}
	return service.store.GetValues(ctx, target)
}

func (service *Service) ReplaceValues(ctx context.Context, actor auth.Session, target TargetReference, version int64, values []ValueInput, requestID string) (ValueSet, error) {
	if !target.Valid() {
		return ValueSet{}, ErrInvalidTarget
	}
	if !canWrite(actor) {
		service.recordAudit(ctx, actor, AuditResourceFieldValues, nil, &target, AuditEventValuesReplaced, auth.AuditOutcomeDenied, requestID)
		return ValueSet{}, ErrForbidden
	}
	updated, err := service.store.ReplaceValues(ctx, target, version, values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldValues, nil, &target, AuditEventValuesReplaced, mutationOutcome(err), requestID)
		return ValueSet{}, err
	}
	service.recordAudit(ctx, actor, AuditResourceFieldValues, nil, &target, AuditEventValuesReplaced, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) ListEntities(ctx context.Context, actor auth.Session, options EntityListOptions) (EntityPage, error) {
	if !canRead(actor) {
		return EntityPage{}, ErrForbidden
	}
	normalized, err := normalizeEntityListOptions(options)
	if err != nil {
		return EntityPage{}, err
	}
	total, err := service.store.CountEntities(ctx, normalized)
	if err != nil {
		return EntityPage{}, err
	}
	items, err := service.store.ListEntities(ctx, normalized)
	if err != nil {
		return EntityPage{}, err
	}
	return EntityPage{Entities: items, Total: total, TypeID: normalized.TypeID, OwnerProfileID: normalized.OwnerProfileID, Limit: normalized.Limit, Offset: normalized.Offset}, nil
}

func (service *Service) GetEntity(ctx context.Context, actor auth.Session, id Identifier) (Entity, error) {
	if !canRead(actor) {
		return Entity{}, ErrForbidden
	}
	if id.IsZero() {
		return Entity{}, ErrNotFound
	}
	return service.store.GetEntity(ctx, id)
}

func (service *Service) CreateEntity(ctx context.Context, actor auth.Session, typeID Identifier, ownerProfileID *Identifier, values []ValueInput, requestID string) (Entity, error) {
	id, err := NewIdentifier()
	if err != nil {
		return Entity{}, fmt.Errorf("generate custom entity identifier: %w", err)
	}
	if !canWrite(actor) {
		service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, nil, AuditEventEntityCreated, auth.AuditOutcomeDenied, requestID)
		return Entity{}, ErrForbidden
	}
	if typeID.IsZero() || (ownerProfileID != nil && ownerProfileID.IsZero()) {
		service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, nil, AuditEventEntityCreated, auth.AuditOutcomeDenied, requestID)
		return Entity{}, ErrInvalidTarget
	}
	created, err := service.store.CreateEntity(ctx, id, typeID, ownerProfileID, values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, nil, AuditEventEntityCreated, mutationOutcome(err), requestID)
		return Entity{}, err
	}
	target := TargetReference{Kind: ValueTargetCustomEntity, ID: created.ID}
	service.recordAudit(ctx, actor, AuditResourceCustomEntity, &created.ID, &target, AuditEventEntityCreated, auth.AuditOutcomeSuccess, requestID)
	return created, nil
}

func (service *Service) UpdateEntity(ctx context.Context, actor auth.Session, id Identifier, version int64, values []ValueInput, requestID string) (Entity, error) {
	target := TargetReference{Kind: ValueTargetCustomEntity, ID: id}
	if !canWrite(actor) {
		service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, &target, AuditEventEntityUpdated, auth.AuditOutcomeDenied, requestID)
		return Entity{}, ErrForbidden
	}
	if id.IsZero() {
		return Entity{}, ErrNotFound
	}
	updated, err := service.store.UpdateEntity(ctx, id, version, values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, &target, AuditEventEntityUpdated, mutationOutcome(err), requestID)
		return Entity{}, err
	}
	service.recordAudit(ctx, actor, AuditResourceCustomEntity, &updated.ID, &target, AuditEventEntityUpdated, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) DeleteEntity(ctx context.Context, actor auth.Session, id Identifier, version int64, confirmation, requestID string) error {
	target := TargetReference{Kind: ValueTargetCustomEntity, ID: id}
	if !canDeleteEntity(actor) {
		service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, &target, AuditEventEntityDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrForbidden
	}
	if confirmation != DeleteConfirmation {
		service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, &target, AuditEventEntityDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrInvalidConfirmation
	}
	if err := service.store.DeleteEntity(ctx, id, version); err != nil {
		service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, &target, AuditEventEntityDeleted, mutationOutcome(err), requestID)
		return err
	}
	service.recordAudit(ctx, actor, AuditResourceCustomEntity, &id, &target, AuditEventEntityDeleted, auth.AuditOutcomeSuccess, requestID)
	return nil
}

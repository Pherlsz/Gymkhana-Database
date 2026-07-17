package customdata

import (
	"context"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func (service *Service) ListEntityTypes(ctx context.Context, actor auth.Session, options EntityTypeListOptions) (EntityTypePage, error) {
	if !canRead(actor) {
		return EntityTypePage{}, ErrForbidden
	}
	normalized, err := normalizeEntityTypeListOptions(options)
	if err != nil {
		return EntityTypePage{}, err
	}
	total, err := service.store.CountEntityTypes(ctx, normalized.Filters)
	if err != nil {
		return EntityTypePage{}, err
	}
	items, err := service.store.ListEntityTypes(ctx, normalized)
	if err != nil {
		return EntityTypePage{}, err
	}
	return EntityTypePage{Types: items, Total: total, Limit: normalized.Limit, Offset: normalized.Offset, SortField: normalized.SortField, SortOrder: normalized.SortOrder, Filters: normalized.Filters}, nil
}

func (service *Service) GetEntityType(ctx context.Context, actor auth.Session, id Identifier) (EntityType, error) {
	if !canRead(actor) {
		return EntityType{}, ErrForbidden
	}
	if id.IsZero() {
		return EntityType{}, ErrNotFound
	}
	return service.store.GetEntityType(ctx, id)
}

func (service *Service) CreateEntityType(ctx context.Context, actor auth.Session, values EntityTypeValues, requestID string) (EntityType, error) {
	id, err := NewIdentifier()
	if err != nil {
		return EntityType{}, fmt.Errorf("generate custom entity type identifier: %w", err)
	}
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeCreated, auth.AuditOutcomeDenied, requestID)
		return EntityType{}, ErrForbidden
	}
	normalized, err := NormalizeEntityType(values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeCreated, mutationOutcome(err), requestID)
		return EntityType{}, err
	}
	created, err := service.store.CreateEntityType(ctx, id, normalized)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeCreated, mutationOutcome(err), requestID)
		return EntityType{}, err
	}
	service.recordAudit(ctx, actor, AuditResourceEntityType, &created.ID, nil, AuditEventEntityTypeCreated, auth.AuditOutcomeSuccess, requestID)
	return created, nil
}

func (service *Service) UpdateEntityType(ctx context.Context, actor auth.Session, id Identifier, version int64, values EntityTypeValues, requestID string) (EntityType, error) {
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeUpdated, auth.AuditOutcomeDenied, requestID)
		return EntityType{}, ErrForbidden
	}
	normalized, err := NormalizeEntityType(values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeUpdated, mutationOutcome(err), requestID)
		return EntityType{}, err
	}
	updated, err := service.store.UpdateEntityType(ctx, id, version, normalized)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeUpdated, mutationOutcome(err), requestID)
		return EntityType{}, err
	}
	service.recordAudit(ctx, actor, AuditResourceEntityType, &updated.ID, nil, AuditEventEntityTypeUpdated, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) DeleteEntityType(ctx context.Context, actor auth.Session, id Identifier, version int64, confirmation, requestID string) error {
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrForbidden
	}
	if confirmation != DeleteConfirmation {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrInvalidConfirmation
	}
	if err := service.store.DeleteEntityType(ctx, id, version); err != nil {
		service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeDeleted, mutationOutcome(err), requestID)
		return err
	}
	service.recordAudit(ctx, actor, AuditResourceEntityType, &id, nil, AuditEventEntityTypeDeleted, auth.AuditOutcomeSuccess, requestID)
	return nil
}

package customdata

import (
	"context"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func (service *Service) ListFieldDefinitions(ctx context.Context, actor auth.Session, options FieldDefinitionListOptions) (FieldDefinitionPage, error) {
	if !canRead(actor) {
		return FieldDefinitionPage{}, ErrForbidden
	}
	normalized, err := normalizeFieldDefinitionListOptions(options)
	if err != nil {
		return FieldDefinitionPage{}, err
	}
	total, err := service.store.CountFieldDefinitions(ctx, normalized.Filters)
	if err != nil {
		return FieldDefinitionPage{}, err
	}
	items, err := service.store.ListFieldDefinitions(ctx, normalized)
	if err != nil {
		return FieldDefinitionPage{}, err
	}
	return FieldDefinitionPage{Definitions: items, Total: total, Limit: normalized.Limit, Offset: normalized.Offset, SortField: normalized.SortField, SortOrder: normalized.SortOrder, Filters: normalized.Filters}, nil
}

func (service *Service) GetFieldDefinition(ctx context.Context, actor auth.Session, id Identifier) (FieldDefinition, error) {
	if !canRead(actor) {
		return FieldDefinition{}, ErrForbidden
	}
	if id.IsZero() {
		return FieldDefinition{}, ErrNotFound
	}
	return service.store.GetFieldDefinition(ctx, id)
}

func (service *Service) CreateFieldDefinition(ctx context.Context, actor auth.Session, values FieldDefinitionValues, requestID string) (FieldDefinition, error) {
	id, err := NewIdentifier()
	if err != nil {
		return FieldDefinition{}, fmt.Errorf("generate custom field identifier: %w", err)
	}
	target := definitionTarget(values)
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, target, AuditEventFieldCreated, auth.AuditOutcomeDenied, requestID)
		return FieldDefinition{}, ErrForbidden
	}
	normalized, err := normalizeManageableFieldDefinition(values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, target, AuditEventFieldCreated, mutationOutcome(err), requestID)
		return FieldDefinition{}, err
	}
	created, err := service.store.CreateFieldDefinition(ctx, id, normalized)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, target, AuditEventFieldCreated, mutationOutcome(err), requestID)
		return FieldDefinition{}, err
	}
	service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &created.ID, definitionTarget(created.Values), AuditEventFieldCreated, auth.AuditOutcomeSuccess, requestID)
	return created, nil
}

func (service *Service) UpdateFieldDefinition(ctx context.Context, actor auth.Session, id Identifier, version int64, values FieldDefinitionValues, requestID string) (FieldDefinition, error) {
	target := definitionTarget(values)
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, target, AuditEventFieldUpdated, auth.AuditOutcomeDenied, requestID)
		return FieldDefinition{}, ErrForbidden
	}
	normalized, err := normalizeManageableFieldDefinition(values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, target, AuditEventFieldUpdated, mutationOutcome(err), requestID)
		return FieldDefinition{}, err
	}
	updated, err := service.store.UpdateFieldDefinition(ctx, id, version, normalized)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, target, AuditEventFieldUpdated, mutationOutcome(err), requestID)
		return FieldDefinition{}, err
	}
	service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &updated.ID, definitionTarget(updated.Values), AuditEventFieldUpdated, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) DeleteFieldDefinition(ctx context.Context, actor auth.Session, id Identifier, version int64, confirmation, requestID string) error {
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, nil, AuditEventFieldDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrForbidden
	}
	if confirmation != DeleteConfirmation {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, nil, AuditEventFieldDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrInvalidConfirmation
	}
	if err := service.store.DeleteFieldDefinition(ctx, id, version); err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, nil, AuditEventFieldDeleted, mutationOutcome(err), requestID)
		return err
	}
	service.recordAudit(ctx, actor, AuditResourceFieldDefinition, &id, nil, AuditEventFieldDeleted, auth.AuditOutcomeSuccess, requestID)
	return nil
}

func normalizeManageableFieldDefinition(values FieldDefinitionValues) (FieldDefinitionValues, error) {
	normalized, err := NormalizeFieldDefinition(values)
	if err != nil {
		return FieldDefinitionValues{}, err
	}
	if normalized.Kind == FieldAttachment && normalized.Required {
		validation := &ValidationError{}
		validation.add("required", "unsupported")
		return FieldDefinitionValues{}, validation
	}
	return normalized, nil
}

package customdata

import (
	"context"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

func (service *Service) ListOptions(ctx context.Context, actor auth.Session, fieldID Identifier) ([]Option, error) {
	if !canRead(actor) {
		return nil, ErrForbidden
	}
	if fieldID.IsZero() {
		return nil, ErrNotFound
	}
	return service.store.ListOptions(ctx, fieldID)
}

func (service *Service) CreateOption(ctx context.Context, actor auth.Session, fieldID Identifier, values OptionValues, requestID string) (Option, error) {
	id, err := NewIdentifier()
	if err != nil {
		return Option{}, fmt.Errorf("generate custom field option identifier: %w", err)
	}
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &id, nil, AuditEventOptionCreated, auth.AuditOutcomeDenied, requestID)
		return Option{}, ErrForbidden
	}
	normalized, err := NormalizeOption(values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &id, nil, AuditEventOptionCreated, mutationOutcome(err), requestID)
		return Option{}, err
	}
	created, err := service.store.CreateOption(ctx, id, fieldID, normalized)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &id, nil, AuditEventOptionCreated, mutationOutcome(err), requestID)
		return Option{}, err
	}
	service.recordAudit(ctx, actor, AuditResourceFieldOption, &created.ID, nil, AuditEventOptionCreated, auth.AuditOutcomeSuccess, requestID)
	return created, nil
}

func (service *Service) UpdateOption(ctx context.Context, actor auth.Session, fieldID, optionID Identifier, version int64, values OptionValues, requestID string) (Option, error) {
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &optionID, nil, AuditEventOptionUpdated, auth.AuditOutcomeDenied, requestID)
		return Option{}, ErrForbidden
	}
	normalized, err := NormalizeOption(values)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &optionID, nil, AuditEventOptionUpdated, mutationOutcome(err), requestID)
		return Option{}, err
	}
	updated, err := service.store.UpdateOption(ctx, optionID, fieldID, version, normalized)
	if err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &optionID, nil, AuditEventOptionUpdated, mutationOutcome(err), requestID)
		return Option{}, err
	}
	service.recordAudit(ctx, actor, AuditResourceFieldOption, &updated.ID, nil, AuditEventOptionUpdated, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) DeleteOption(ctx context.Context, actor auth.Session, fieldID, optionID Identifier, version int64, confirmation, requestID string) error {
	if !canManageDefinitions(actor) {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &optionID, nil, AuditEventOptionDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrForbidden
	}
	if confirmation != DeleteConfirmation {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &optionID, nil, AuditEventOptionDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrInvalidConfirmation
	}
	if err := service.store.DeleteOption(ctx, optionID, fieldID, version); err != nil {
		service.recordAudit(ctx, actor, AuditResourceFieldOption, &optionID, nil, AuditEventOptionDeleted, mutationOutcome(err), requestID)
		return err
	}
	service.recordAudit(ctx, actor, AuditResourceFieldOption, &optionID, nil, AuditEventOptionDeleted, auth.AuditOutcomeSuccess, requestID)
	return nil
}

func definitionTarget(values FieldDefinitionValues) *TargetReference {
	kind, ok := valueTargetKindForDefinition(values.TargetKind)
	if !ok || values.TargetKind == TargetProfile {
		return nil
	}
	target := TargetReference{Kind: kind, ID: values.TargetID}
	if !target.Valid() {
		return nil
	}
	return &target
}

func valueTargetKindForDefinition(kind TargetKind) (ValueTargetKind, bool) {
	switch kind {
	case TargetProfile:
		return ValueTargetProfile, true
	case TargetDocumentType:
		return ValueTargetDocument, true
	case TargetBillType:
		return ValueTargetBill, true
	case TargetCustomEntityType:
		return ValueTargetCustomEntity, true
	default:
		return "", false
	}
}

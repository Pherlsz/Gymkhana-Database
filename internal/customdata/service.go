package customdata

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type Store interface {
	CountEntityTypes(context.Context, EntityTypeFilters) (int64, error)
	ListEntityTypes(context.Context, EntityTypeListOptions) ([]EntityType, error)
	GetEntityType(context.Context, Identifier) (EntityType, error)
	CreateEntityType(context.Context, Identifier, EntityTypeValues) (EntityType, error)
	UpdateEntityType(context.Context, Identifier, int64, EntityTypeValues) (EntityType, error)
	DeleteEntityType(context.Context, Identifier, int64) error

	CountFieldDefinitions(context.Context, FieldDefinitionFilters) (int64, error)
	ListFieldDefinitions(context.Context, FieldDefinitionListOptions) ([]FieldDefinition, error)
	GetFieldDefinition(context.Context, Identifier) (FieldDefinition, error)
	CreateFieldDefinition(context.Context, Identifier, FieldDefinitionValues) (FieldDefinition, error)
	UpdateFieldDefinition(context.Context, Identifier, int64, FieldDefinitionValues) (FieldDefinition, error)
	DeleteFieldDefinition(context.Context, Identifier, int64) error

	ListOptions(context.Context, Identifier) ([]Option, error)
	CreateOption(context.Context, Identifier, Identifier, OptionValues) (Option, error)
	UpdateOption(context.Context, Identifier, Identifier, int64, OptionValues) (Option, error)
	DeleteOption(context.Context, Identifier, Identifier, int64) error

	GetValues(context.Context, TargetReference) (ValueSet, error)
	ReplaceValues(context.Context, TargetReference, int64, []ValueInput) (ValueSet, error)

	CountEntities(context.Context, EntityListOptions) (int64, error)
	ListEntities(context.Context, EntityListOptions) ([]Entity, error)
	GetEntity(context.Context, Identifier) (Entity, error)
	CreateEntity(context.Context, Identifier, Identifier, *Identifier, []ValueInput) (Entity, error)
	UpdateEntity(context.Context, Identifier, int64, []ValueInput) (Entity, error)
	DeleteEntity(context.Context, Identifier, int64) error
}

type AuditStore interface {
	RecordAuditEvent(context.Context, AuditEvent) error
}

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	OnAuditFailure AuditFailureHandler
}

type Service struct {
	store          Store
	audit          AuditStore
	onAuditFailure AuditFailureHandler
}

func NewService(store Store, options ServiceOptions) (*Service, error) {
	if store == nil {
		return nil, ErrInvalidServiceSetup
	}
	audit, ok := store.(AuditStore)
	if !ok {
		return nil, fmt.Errorf("%w: audit store is unavailable", ErrInvalidServiceSetup)
	}
	return &Service{store: store, audit: audit, onAuditFailure: options.OnAuditFailure}, nil
}

func canRead(actor auth.Session) bool {
	return actor.User.Active && actor.User.Role.Valid()
}

func canWrite(actor auth.Session) bool {
	return actor.User.Active && actor.User.Role.Valid()
}

func canManageDefinitions(actor auth.Session) bool {
	return actor.User.Active && (actor.User.Role == auth.RoleAdmin || actor.User.Role == auth.RoleSuperadmin)
}

func canDeleteEntity(actor auth.Session) bool {
	return canManageDefinitions(actor)
}

func mutationOutcome(err error) auth.AuditOutcome {
	var validation *ValidationError
	if errors.As(err, &validation) ||
		errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) ||
		errors.Is(err, ErrTechnicalKeyConflict) || errors.Is(err, ErrTechnicalKeyImmutable) ||
		errors.Is(err, ErrDefinitionInUse) || errors.Is(err, ErrDefinitionChangeUnsafe) ||
		errors.Is(err, ErrOptionInUse) || errors.Is(err, ErrEntityTypeInUse) ||
		errors.Is(err, ErrEntityTypeInactive) || errors.Is(err, ErrDefinitionInactive) ||
		errors.Is(err, ErrReferenceNotFound) || errors.Is(err, ErrCardinalityConflict) ||
		errors.Is(err, ErrInvalidConfirmation) || errors.Is(err, ErrInvalidTarget) ||
		errors.Is(err, ErrInvalidListOptions) {
		return auth.AuditOutcomeDenied
	}
	return auth.AuditOutcomeFailure
}

func (service *Service) recordAudit(
	ctx context.Context,
	actor auth.Session,
	resourceKind AuditResourceKind,
	resourceID *Identifier,
	target *TargetReference,
	eventType AuditEventType,
	outcome auth.AuditOutcome,
	requestID string,
) {
	event := AuditEvent{
		ActorUserID:  actor.User.ID,
		ResourceKind: resourceKind,
		ResourceID:   resourceID,
		Target:       target,
		EventType:    eventType,
		Outcome:      outcome,
		RequestID:    normalizeRequestID(requestID),
	}
	id, err := NewIdentifier()
	if err != nil {
		service.reportAuditFailure(ctx, event, fmt.Errorf("generate custom data audit event identifier: %w", err))
		return
	}
	event.ID = id
	if err := service.audit.RecordAuditEvent(ctx, event); err != nil {
		service.reportAuditFailure(ctx, event, fmt.Errorf("record custom data audit event: %w", err))
	}
}

func (service *Service) reportAuditFailure(ctx context.Context, event AuditEvent, err error) {
	if service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

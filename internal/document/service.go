package document

import (
	"context"
	"errors"
	"fmt"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

const DeleteConfirmation = "Confirmar"

var (
	ErrForbidden              = errors.New("document operation is forbidden")
	ErrInvalidConfirmation    = errors.New("document delete confirmation is invalid")
	ErrInvalidListOptions     = errors.New("document list options are invalid")
	ErrInvalidTypeListOptions = errors.New("document type list options are invalid")
	ErrInvalidServiceSetup    = errors.New("document service setup is invalid")
)

type TypePage struct {
	Types     []TypeDefinition
	Total     int64
	Limit     int32
	Offset    int32
	SortField TypeSortField
	SortOrder SortOrder
	Filters   TypeFilters
}

type Page struct {
	Documents []Document
	Total     int64
	Limit     int32
	Offset    int32
	SortField SortField
	SortOrder SortOrder
	Filters   Filters
}

type AuditEventType string

const (
	AuditEventTypeCreated AuditEventType = "DOCUMENT_TYPE_CREATED"
	AuditEventTypeUpdated AuditEventType = "DOCUMENT_TYPE_UPDATED"
	AuditEventTypeDeleted AuditEventType = "DOCUMENT_TYPE_DELETED"
	AuditEventCreated     AuditEventType = "DOCUMENT_CREATED"
	AuditEventUpdated     AuditEventType = "DOCUMENT_UPDATED"
	AuditEventDuplicated  AuditEventType = "DOCUMENT_DUPLICATED"
	AuditEventDeleted     AuditEventType = "DOCUMENT_DELETED"
	AuditEventUseAssigned AuditEventType = "DOCUMENT_USE_ASSIGNED"
	AuditEventUseReturned AuditEventType = "DOCUMENT_USE_RETURNED"
)

type AuditEvent struct {
	ID               Identifier
	ActorUserID      auth.Identifier
	DocumentID       *Identifier
	SourceDocumentID *Identifier
	DocumentTypeID   *Identifier
	HolderProfileID  *profile.Identifier
	EventType        AuditEventType
	Outcome          auth.AuditOutcome
	RequestID        string
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

func (service *Service) ListTypes(ctx context.Context, actor auth.Session, options TypeListOptions) (TypePage, error) {
	if !actor.User.Active || !actor.User.Role.CanReadDocuments() {
		return TypePage{}, ErrForbidden
	}
	normalized, err := normalizeTypeListOptions(options)
	if err != nil {
		return TypePage{}, err
	}
	total, err := service.store.CountTypes(ctx, normalized.Filters)
	if err != nil {
		return TypePage{}, err
	}
	values, err := service.store.ListTypes(ctx, normalized)
	if err != nil {
		return TypePage{}, err
	}
	return TypePage{Types: values, Total: total, Limit: normalized.Limit, Offset: normalized.Offset, SortField: normalized.SortField, SortOrder: normalized.SortOrder, Filters: normalized.Filters}, nil
}

func (service *Service) GetType(ctx context.Context, actor auth.Session, id Identifier) (TypeDefinition, error) {
	if !actor.User.Active || !actor.User.Role.CanReadDocuments() {
		return TypeDefinition{}, ErrForbidden
	}
	return service.store.GetType(ctx, id)
}

func (service *Service) CreateType(ctx context.Context, actor auth.Session, values TypeValues, requestID string) (TypeDefinition, error) {
	id, err := NewIdentifier()
	if err != nil {
		return TypeDefinition{}, fmt.Errorf("generate document type identifier: %w", err)
	}
	if !actor.User.Active || !actor.User.Role.CanManageDocumentTypes() {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeCreated, auth.AuditOutcomeDenied, requestID)
		return TypeDefinition{}, ErrForbidden
	}
	created, err := service.store.CreateType(ctx, id, values)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeCreated, mutationOutcome(err), requestID)
		return TypeDefinition{}, err
	}
	service.recordAudit(ctx, actor.User.ID, nil, nil, &created.ID, nil, AuditEventTypeCreated, auth.AuditOutcomeSuccess, requestID)
	return created, nil
}

func (service *Service) UpdateType(ctx context.Context, actor auth.Session, id Identifier, version int64, values TypeValues, requestID string) (TypeDefinition, error) {
	if !actor.User.Active || !actor.User.Role.CanManageDocumentTypes() {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeUpdated, auth.AuditOutcomeDenied, requestID)
		return TypeDefinition{}, ErrForbidden
	}
	updated, err := service.store.UpdateType(ctx, id, version, values)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeUpdated, mutationOutcome(err), requestID)
		return TypeDefinition{}, err
	}
	service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeUpdated, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) DeleteType(ctx context.Context, actor auth.Session, id Identifier, version int64, confirmation, requestID string) error {
	if !actor.User.Active || !actor.User.Role.CanManageDocumentTypes() {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrForbidden
	}
	if confirmation != DeleteConfirmation {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrInvalidConfirmation
	}
	if err := service.store.DeleteType(ctx, id, version); err != nil {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeDeleted, mutationOutcome(err), requestID)
		return err
	}
	service.recordAudit(ctx, actor.User.ID, nil, nil, &id, nil, AuditEventTypeDeleted, auth.AuditOutcomeSuccess, requestID)
	return nil
}

func (service *Service) List(ctx context.Context, actor auth.Session, options ListOptions) (Page, error) {
	if !actor.User.Active || !actor.User.Role.CanReadDocuments() {
		return Page{}, ErrForbidden
	}
	normalized, err := normalizeListOptions(options)
	if err != nil {
		return Page{}, err
	}
	total, err := service.store.Count(ctx, normalized.Filters)
	if err != nil {
		return Page{}, err
	}
	values, err := service.store.List(ctx, normalized)
	if err != nil {
		return Page{}, err
	}
	return Page{Documents: values, Total: total, Limit: normalized.Limit, Offset: normalized.Offset, SortField: normalized.SortField, SortOrder: normalized.SortOrder, Filters: normalized.Filters}, nil
}

func (service *Service) Get(ctx context.Context, actor auth.Session, id Identifier) (Document, error) {
	if !actor.User.Active || !actor.User.Role.CanReadDocuments() {
		return Document{}, ErrForbidden
	}
	return service.store.Get(ctx, id)
}

func (service *Service) Create(ctx context.Context, actor auth.Session, values Values, requestID string) (Document, error) {
	id, err := NewIdentifier()
	if err != nil {
		return Document{}, fmt.Errorf("generate document identifier: %w", err)
	}
	if !actor.User.Active || !actor.User.Role.CanWriteDocuments() {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &values.TypeID, nil, AuditEventCreated, auth.AuditOutcomeDenied, requestID)
		return Document{}, ErrForbidden
	}
	created, err := service.store.Create(ctx, id, values)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &values.TypeID, nil, AuditEventCreated, mutationOutcome(err), requestID)
		return Document{}, err
	}
	service.recordAudit(ctx, actor.User.ID, &created.ID, nil, &created.Type.ID, nil, AuditEventCreated, auth.AuditOutcomeSuccess, requestID)
	return created, nil
}

func (service *Service) Update(ctx context.Context, actor auth.Session, id Identifier, version int64, values Values, requestID string) (Document, error) {
	if !actor.User.Active || !actor.User.Role.CanWriteDocuments() {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &values.TypeID, nil, AuditEventUpdated, auth.AuditOutcomeDenied, requestID)
		return Document{}, ErrForbidden
	}
	updated, err := service.store.Update(ctx, id, version, values)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &values.TypeID, nil, AuditEventUpdated, mutationOutcome(err), requestID)
		return Document{}, err
	}
	service.recordAudit(ctx, actor.User.ID, &id, nil, &updated.Type.ID, nil, AuditEventUpdated, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) Duplicate(ctx context.Context, actor auth.Session, sourceID Identifier, requestID string) (Document, error) {
	newID, err := NewIdentifier()
	if err != nil {
		return Document{}, fmt.Errorf("generate duplicate document identifier: %w", err)
	}
	if !actor.User.Active || !actor.User.Role.CanWriteDocuments() {
		service.recordAudit(ctx, actor.User.ID, &newID, &sourceID, nil, nil, AuditEventDuplicated, auth.AuditOutcomeDenied, requestID)
		return Document{}, ErrForbidden
	}
	duplicated, err := service.store.Duplicate(ctx, newID, sourceID)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, &newID, &sourceID, nil, nil, AuditEventDuplicated, mutationOutcome(err), requestID)
		return Document{}, err
	}
	service.recordAudit(ctx, actor.User.ID, &duplicated.ID, &sourceID, &duplicated.Type.ID, nil, AuditEventDuplicated, auth.AuditOutcomeSuccess, requestID)
	return duplicated, nil
}

func (service *Service) Delete(ctx context.Context, actor auth.Session, id Identifier, version int64, confirmation, requestID string) error {
	if !actor.User.Active || !actor.User.Role.CanDeleteDocuments() {
		service.recordAudit(ctx, actor.User.ID, &id, nil, nil, nil, AuditEventDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrForbidden
	}
	if confirmation != DeleteConfirmation {
		service.recordAudit(ctx, actor.User.ID, &id, nil, nil, nil, AuditEventDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrInvalidConfirmation
	}
	if err := service.store.Delete(ctx, id, version); err != nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, nil, nil, AuditEventDeleted, mutationOutcome(err), requestID)
		return err
	}
	service.recordAudit(ctx, actor.User.ID, &id, nil, nil, nil, AuditEventDeleted, auth.AuditOutcomeSuccess, requestID)
	return nil
}

func (service *Service) AssignCurrentUse(ctx context.Context, actor auth.Session, id Identifier, holderProfileID profile.Identifier, requestID string) (CurrentUse, error) {
	if !actor.User.Active || !actor.User.Role.CanManageDocumentCurrentUse() {
		service.recordAudit(ctx, actor.User.ID, &id, nil, nil, &holderProfileID, AuditEventUseAssigned, auth.AuditOutcomeDenied, requestID)
		return CurrentUse{}, ErrForbidden
	}
	documentValue, err := service.store.Get(ctx, id)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, nil, &holderProfileID, AuditEventUseAssigned, mutationOutcome(err), requestID)
		return CurrentUse{}, err
	}
	if !documentValue.Values.Medium.SupportsCurrentUse() {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &documentValue.Type.ID, &holderProfileID, AuditEventUseAssigned, auth.AuditOutcomeDenied, requestID)
		return CurrentUse{}, ErrCurrentUseUnsupported
	}
	currentUse, err := service.store.AssignCurrentUse(ctx, id, holderProfileID)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &documentValue.Type.ID, &holderProfileID, AuditEventUseAssigned, mutationOutcome(err), requestID)
		return CurrentUse{}, err
	}
	service.recordAudit(ctx, actor.User.ID, &id, nil, &documentValue.Type.ID, &holderProfileID, AuditEventUseAssigned, auth.AuditOutcomeSuccess, requestID)
	return currentUse, nil
}

func (service *Service) ReturnCurrentUse(ctx context.Context, actor auth.Session, id Identifier, requestID string) error {
	if !actor.User.Active || !actor.User.Role.CanManageDocumentCurrentUse() {
		service.recordAudit(ctx, actor.User.ID, &id, nil, nil, nil, AuditEventUseReturned, auth.AuditOutcomeDenied, requestID)
		return ErrForbidden
	}
	documentValue, err := service.store.Get(ctx, id)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, nil, nil, AuditEventUseReturned, mutationOutcome(err), requestID)
		return err
	}
	currentUse, err := service.store.GetCurrentUse(ctx, id)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &documentValue.Type.ID, nil, AuditEventUseReturned, mutationOutcome(err), requestID)
		return err
	}
	if currentUse == nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &documentValue.Type.ID, nil, AuditEventUseReturned, auth.AuditOutcomeDenied, requestID)
		return ErrCurrentUseNotFound
	}
	if err := service.store.ReturnCurrentUse(ctx, id); err != nil {
		service.recordAudit(ctx, actor.User.ID, &id, nil, &documentValue.Type.ID, &currentUse.HolderProfileID, AuditEventUseReturned, mutationOutcome(err), requestID)
		return err
	}
	service.recordAudit(ctx, actor.User.ID, &id, nil, &documentValue.Type.ID, &currentUse.HolderProfileID, AuditEventUseReturned, auth.AuditOutcomeSuccess, requestID)
	return nil
}

func (service *Service) UpsertPresence(ctx context.Context, actor auth.Session, owner profile.Identifier, typeID Identifier, claim Claim, identifier, requestID string) (Presence, error) {
	if !actor.User.Active || !actor.User.Role.CanWriteDocuments() {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &typeID, nil, AuditEventUpdated, auth.AuditOutcomeDenied, requestID)
		return Presence{}, ErrForbidden
	}
	presence, err := service.store.UpsertPresence(ctx, owner, typeID, claim, identifier)
	if err != nil {
		service.recordAudit(ctx, actor.User.ID, nil, nil, &typeID, nil, AuditEventUpdated, mutationOutcome(err), requestID)
		return Presence{}, err
	}
	service.recordAudit(ctx, actor.User.ID, nil, nil, &presence.TypeID, nil, AuditEventUpdated, auth.AuditOutcomeSuccess, requestID)
	return presence, nil
}

func normalizeTypeListOptions(options TypeListOptions) (TypeListOptions, error) {
	if options.Limit == 0 {
		options.Limit = 100
	}
	if options.Limit < 1 || options.Limit > 1000 || options.Offset < 0 {
		return TypeListOptions{}, ErrInvalidTypeListOptions
	}
	if options.SortField == "" {
		options.SortField = TypeSortLabel
	}
	if options.SortOrder == "" {
		options.SortOrder = SortAscending
	}
	if !options.SortField.Valid() || !options.SortOrder.Valid() {
		return TypeListOptions{}, ErrInvalidTypeListOptions
	}
	options.Filters.Label = normalize.SearchText(options.Filters.Label)
	return options, nil
}

func normalizeListOptions(options ListOptions) (ListOptions, error) {
	if options.Limit == 0 {
		options.Limit = 100
	}
	if options.Limit < 1 || options.Limit > 1000 || options.Offset < 0 {
		return ListOptions{}, ErrInvalidListOptions
	}
	if options.SortField == "" {
		options.SortField = SortIdentifier
	}
	if options.SortOrder == "" {
		options.SortOrder = SortAscending
	}
	if !options.SortField.Valid() || !options.SortOrder.Valid() {
		return ListOptions{}, ErrInvalidListOptions
	}
	if options.Filters.OwnerProfileID != nil && *options.Filters.OwnerProfileID == (profile.Identifier{}) {
		return ListOptions{}, ErrInvalidListOptions
	}
	if options.Filters.TypeID != nil && options.Filters.TypeID.IsZero() {
		return ListOptions{}, ErrInvalidListOptions
	}
	if options.Filters.HolderProfileID != nil && *options.Filters.HolderProfileID == (profile.Identifier{}) {
		return ListOptions{}, ErrInvalidListOptions
	}
	if options.Filters.Medium != "" && !options.Filters.Medium.Valid() {
		return ListOptions{}, ErrInvalidListOptions
	}
	if options.Filters.Status != "" && !options.Filters.Status.Valid() {
		return ListOptions{}, ErrInvalidListOptions
	}
	options.Filters.Identifier = normalize.SearchText(options.Filters.Identifier)
	return options, nil
}

func (field SortField) Valid() bool {
	switch field {
	case SortIdentifier, SortTypeLabel, SortDocumentDate, SortCreatedAt, SortUpdatedAt:
		return true
	default:
		return false
	}
}

func (field TypeSortField) Valid() bool {
	switch field {
	case TypeSortLabel, TypeSortTechnicalKey, TypeSortCreatedAt, TypeSortUpdatedAt:
		return true
	default:
		return false
	}
}

func (order SortOrder) Valid() bool {
	return order == SortAscending || order == SortDescending
}

func (status Status) Valid() bool {
	return status == StatusAvailable || status == StatusInUse
}

func mutationOutcome(err error) auth.AuditOutcome {
	var validation *ValidationError
	if errors.As(err, &validation) ||
		errors.Is(err, ErrNotFound) || errors.Is(err, ErrTypeNotFound) ||
		errors.Is(err, ErrConflict) || errors.Is(err, ErrTypeConflict) ||
		errors.Is(err, ErrTypeInactive) || errors.Is(err, ErrTypeInUse) ||
		errors.Is(err, ErrTechnicalKeyImmutable) || errors.Is(err, ErrTechnicalKeyConflict) ||
		errors.Is(err, ErrUniquenessConflict) || errors.Is(err, ErrReferenceNotFound) ||
		errors.Is(err, ErrCurrentUseExists) || errors.Is(err, ErrCurrentUseNotFound) ||
		errors.Is(err, ErrCurrentUseUnsupported) || errors.Is(err, ErrDuplicateNotSupported) {
		return auth.AuditOutcomeDenied
	}
	return auth.AuditOutcomeFailure
}

func (service *Service) recordAudit(
	ctx context.Context,
	actorUserID auth.Identifier,
	documentID, sourceDocumentID, documentTypeID *Identifier,
	holderProfileID *profile.Identifier,
	eventType AuditEventType,
	outcome auth.AuditOutcome,
	requestID string,
) {
	event := AuditEvent{ActorUserID: actorUserID, DocumentID: documentID, SourceDocumentID: sourceDocumentID, DocumentTypeID: documentTypeID, HolderProfileID: holderProfileID, EventType: eventType, Outcome: outcome, RequestID: requestID}
	id, err := NewIdentifier()
	if err != nil {
		service.reportAuditFailure(ctx, event, fmt.Errorf("generate document audit event id: %w", err))
		return
	}
	event.ID = id
	if err := service.audit.RecordAuditEvent(ctx, event); err != nil {
		service.reportAuditFailure(ctx, event, fmt.Errorf("record document audit event: %w", err))
	}
}

func (service *Service) reportAuditFailure(ctx context.Context, event AuditEvent, err error) {
	if service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

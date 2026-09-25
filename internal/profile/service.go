package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const DeleteConfirmation = "Confirmar"

var (
	ErrForbidden           = errors.New("profile operation is forbidden")
	ErrInvalidConfirmation = errors.New("profile delete confirmation is invalid")
	ErrInvalidListOptions  = errors.New("profile list options are invalid")
	ErrInvalidServiceSetup = errors.New("profile service setup is invalid")
)

type SortField string

type SortOrder string

const (
	SortFullName     SortField = "full_name"
	SortCPF          SortField = "cpf"
	SortEmail        SortField = "email"
	SortCity         SortField = "address_city"
	SortStreet       SortField = "address_street"
	SortNeighborhood SortField = "address_neighborhood"
	SortMobilePhone  SortField = "mobile_phone"
	SortBirthDate    SortField = "birth_date"
	SortCreatedAt    SortField = "created_at"
	SortUpdatedAt    SortField = "updated_at"

	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

type Filters struct {
	FullName    string
	CPF         string
	Email       string
	City        string
	State       string
	RestrictIDs bool
	IDFilter    []Identifier
}

type ListOptions struct {
	Limit     int32
	Offset    int32
	SortField SortField
	SortOrder SortOrder
	Filters   Filters
}

type Page struct {
	Profiles  []Profile
	Total     int64
	Limit     int32
	Offset    int32
	SortField SortField
	SortOrder SortOrder
	Filters   Filters
}

type AuditEventType string

const (
	AuditEventCreated    AuditEventType = "PROFILE_CREATED"
	AuditEventUpdated    AuditEventType = "PROFILE_UPDATED"
	AuditEventDuplicated AuditEventType = "PROFILE_DUPLICATED"
	AuditEventDeleted    AuditEventType = "PROFILE_DELETED"
)

type AuditEvent struct {
	ID              Identifier
	ActorUserID     auth.Identifier
	ProfileID       Identifier
	SourceProfileID *Identifier
	EventType       AuditEventType
	Outcome         auth.AuditOutcome
	RequestID       string
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

func (service *Service) List(ctx context.Context, actor auth.Session, options ListOptions) (Page, error) {
	if !actor.User.Active || !actor.User.Role.CanReadProfiles() {
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
	profiles, err := service.store.List(ctx, normalized)
	if err != nil {
		return Page{}, err
	}
	return Page{
		Profiles:  profiles,
		Total:     total,
		Limit:     normalized.Limit,
		Offset:    normalized.Offset,
		SortField: normalized.SortField,
		SortOrder: normalized.SortOrder,
		Filters:   normalized.Filters,
	}, nil
}

func (service *Service) Get(ctx context.Context, actor auth.Session, id Identifier) (Profile, error) {
	if !actor.User.Active || !actor.User.Role.CanReadProfiles() {
		return Profile{}, ErrForbidden
	}
	return service.store.Get(ctx, id)
}

// DistinctCities returns the distinct non-empty cities across the profile set
// that matches the given filters, so the grid's filter-by-values menu can list
// every city instead of only the ones on the loaded page.
func (service *Service) DistinctCities(ctx context.Context, actor auth.Session, filters Filters, limit int32) ([]string, error) {
	if !actor.User.Active || !actor.User.Role.CanReadProfiles() {
		return nil, ErrForbidden
	}
	normalized, err := normalizeListOptions(ListOptions{Limit: 1, Filters: filters})
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}
	return service.store.DistinctCities(ctx, normalized.Filters, limit)
}

func (service *Service) Create(ctx context.Context, actor auth.Session, values Values, requestID string) (Profile, error) {
	id, err := NewIdentifier()
	if err != nil {
		return Profile{}, fmt.Errorf("generate profile identifier: %w", err)
	}
	if !actor.User.Active || !actor.User.Role.CanWriteProfiles() {
		service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventCreated, auth.AuditOutcomeDenied, requestID)
		return Profile{}, ErrForbidden
	}
	created, err := service.store.Create(ctx, id, values)
	if err != nil {
		outcome := auth.AuditOutcomeFailure
		var validation *ValidationError
		if errors.As(err, &validation) {
			outcome = auth.AuditOutcomeDenied
		}
		service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventCreated, outcome, requestID)
		return Profile{}, err
	}
	service.recordAudit(ctx, actor.User.ID, created.ID, nil, AuditEventCreated, auth.AuditOutcomeSuccess, requestID)
	return created, nil
}

func (service *Service) Update(ctx context.Context, actor auth.Session, id Identifier, version int64, values Values, requestID string) (Profile, error) {
	if !actor.User.Active || !actor.User.Role.CanWriteProfiles() {
		service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventUpdated, auth.AuditOutcomeDenied, requestID)
		return Profile{}, ErrForbidden
	}
	updated, err := service.store.Update(ctx, id, version, values)
	if err != nil {
		outcome := auth.AuditOutcomeFailure
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
			outcome = auth.AuditOutcomeDenied
		}
		var validation *ValidationError
		if errors.As(err, &validation) {
			outcome = auth.AuditOutcomeDenied
		}
		service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventUpdated, outcome, requestID)
		return Profile{}, err
	}
	service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventUpdated, auth.AuditOutcomeSuccess, requestID)
	return updated, nil
}

func (service *Service) Duplicate(ctx context.Context, actor auth.Session, sourceID Identifier, requestID string) (Profile, error) {
	newID, err := NewIdentifier()
	if err != nil {
		return Profile{}, fmt.Errorf("generate duplicate profile identifier: %w", err)
	}
	if !actor.User.Active || !actor.User.Role.CanWriteProfiles() {
		service.recordAudit(ctx, actor.User.ID, newID, &sourceID, AuditEventDuplicated, auth.AuditOutcomeDenied, requestID)
		return Profile{}, ErrForbidden
	}
	duplicated, err := service.store.Duplicate(ctx, newID, sourceID)
	if err != nil {
		outcome := auth.AuditOutcomeFailure
		if errors.Is(err, ErrNotFound) {
			outcome = auth.AuditOutcomeDenied
		}
		service.recordAudit(ctx, actor.User.ID, newID, &sourceID, AuditEventDuplicated, outcome, requestID)
		return Profile{}, err
	}
	service.recordAudit(ctx, actor.User.ID, duplicated.ID, &sourceID, AuditEventDuplicated, auth.AuditOutcomeSuccess, requestID)
	return duplicated, nil
}

func (service *Service) Delete(ctx context.Context, actor auth.Session, id Identifier, version int64, confirmation, requestID string) error {
	if !actor.User.Active || !actor.User.Role.CanDeleteProfiles() {
		service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrForbidden
	}
	if confirmation != DeleteConfirmation {
		service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventDeleted, auth.AuditOutcomeDenied, requestID)
		return ErrInvalidConfirmation
	}
	if err := service.store.Delete(ctx, id, version); err != nil {
		outcome := auth.AuditOutcomeFailure
		if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
			outcome = auth.AuditOutcomeDenied
		}
		service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventDeleted, outcome, requestID)
		return err
	}
	service.recordAudit(ctx, actor.User.ID, id, nil, AuditEventDeleted, auth.AuditOutcomeSuccess, requestID)
	return nil
}

func normalizeFilterPattern(raw string, fn func(string) string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "^") {
		return "^" + fn(raw[1:])
	}
	if strings.HasPrefix(raw, "=") {
		return "=" + fn(raw[1:])
	}
	return fn(raw)
}

func normalizeListOptions(options ListOptions) (ListOptions, error) {
	if options.Limit == 0 {
		options.Limit = 100
	}
	if options.Limit < 1 || options.Limit > 1000 || options.Offset < 0 {
		return ListOptions{}, ErrInvalidListOptions
	}
	if options.SortField == "" {
		options.SortField = SortFullName
	}
	if options.SortOrder == "" {
		options.SortOrder = SortAscending
	}
	if !options.SortField.Valid() || !options.SortOrder.Valid() {
		return ListOptions{}, ErrInvalidListOptions
	}
	options.Filters.FullName = normalizeFilterPattern(options.Filters.FullName, normalize.SearchText)
	options.Filters.CPF = normalizeFilterPattern(options.Filters.CPF, normalize.Digits)
	options.Filters.Email = normalizeFilterPattern(options.Filters.Email, func(s string) string { return strings.ToLower(strings.TrimSpace(s)) })
	options.Filters.City = normalizeFilterPattern(options.Filters.City, normalize.SearchText)
	options.Filters.State = strings.ToUpper(strings.TrimSpace(options.Filters.State))
	if options.Filters.State != "" && len(options.Filters.State) != 2 {
		return ListOptions{}, ErrInvalidListOptions
	}
	return options, nil
}

func (field SortField) Valid() bool {
	switch field {
	case SortFullName, SortCPF, SortEmail, SortCity, SortStreet, SortNeighborhood,
		SortMobilePhone, SortBirthDate, SortCreatedAt, SortUpdatedAt:
		return true
	default:
		return false
	}
}

func (order SortOrder) Valid() bool {
	return order == SortAscending || order == SortDescending
}

func (service *Service) recordAudit(
	ctx context.Context,
	actorUserID auth.Identifier,
	profileID Identifier,
	sourceProfileID *Identifier,
	eventType AuditEventType,
	outcome auth.AuditOutcome,
	requestID string,
) {
	event := AuditEvent{
		ActorUserID:     actorUserID,
		ProfileID:       profileID,
		SourceProfileID: sourceProfileID,
		EventType:       eventType,
		Outcome:         outcome,
		RequestID:       requestID,
	}
	id, err := NewIdentifier()
	if err != nil {
		service.reportAuditFailure(ctx, event, fmt.Errorf("generate profile audit event id: %w", err))
		return
	}
	event.ID = id
	if err := service.audit.RecordAuditEvent(ctx, event); err != nil {
		service.reportAuditFailure(ctx, event, fmt.Errorf("record profile audit event: %w", err))
	}
}

func (service *Service) reportAuditFailure(ctx context.Context, event AuditEvent, err error) {
	if service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

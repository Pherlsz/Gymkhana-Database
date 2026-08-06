package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	ErrCapabilityNotFound     = errors.New("capability not granted")
	ErrCapabilityConflict     = errors.New("capability conflict")
	ErrCapabilityAlreadyGranted = errors.New("capability already granted")
)

type Capability string

const (
	CapProfiles     Capability = "PROFILES"
	CapDataTables   Capability = "DATA_TABLES"
	CapSearch       Capability = "SEARCH"
	CapOCR          Capability = "OCR"
	CapOperations   Capability = "OPERATIONS"
	CapMatching     Capability = "MATCHING"
	CapGoogleForms  Capability = "GOOGLE_FORMS"
	CapAttachments  Capability = "ATTACHMENTS"
	CapChat         Capability = "CHAT"
	CapQuery        Capability = "QUERY"
	CapTasks        Capability = "TASKS"
	CapCustomData   Capability = "CUSTOM_DATA"
)

var allCapabilities = []Capability{
	CapProfiles, CapDataTables, CapSearch, CapOCR, CapOperations,
	CapMatching, CapGoogleForms, CapAttachments, CapChat, CapQuery,
	CapTasks, CapCustomData,
}

func (c Capability) Valid() bool {
	return slices.Contains(allCapabilities, c)
}

func ParseCapability(value string) (Capability, error) {
	cap := Capability(strings.ToUpper(strings.TrimSpace(value)))
	if !cap.Valid() {
		return "", ErrCapabilityConflict
	}
	return cap, nil
}

// HasCapability returns true when the role implicitly grants the capability
// (ADMIN and SUPERADMIN bypass all capability checks).
func (role Role) HasCapability(Capability) bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

type CapabilityStore interface {
	GrantCapability(ctx context.Context, userID Identifier, capability Capability) error
	RevokeCapability(ctx context.Context, userID Identifier, capability Capability) error
	ListCapabilities(ctx context.Context, userID Identifier) ([]Capability, error)
}

type CapabilityGrant struct {
	UserID     Identifier
	Capability Capability
}

func (service *Service) GrantCapability(ctx context.Context, actor Session, params CapabilityGrant, requestID string) error {
	if !actor.User.Role.CanManageUsers() || !actor.User.Active {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityGranted, AuditOutcomeDenied, requestID, actor.User.Email)
		return ErrForbidden
	}
	if !params.Capability.Valid() {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityGranted, AuditOutcomeDenied, requestID, actor.User.Email)
		return ErrCapabilityConflict
	}
	if actor.User.ID == params.UserID {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityGranted, AuditOutcomeDenied, requestID, actor.User.Email)
		return ErrSelfAccessChange
	}

	store, ok := service.store.(interface {
		CapabilityStore
		UserAdministrationStore
	})
	if !ok {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityGranted, AuditOutcomeFailure, requestID, actor.User.Email)
		return fmt.Errorf("%w: capability store is unavailable", ErrInvalidServiceSetup)
	}

	target, err := store.FindUserByID(ctx, params.UserID)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityGranted, AuditOutcomeFailure, requestID, actor.User.Email)
		return err
	}
	if target.User.Role != RoleExternal {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityGranted, AuditOutcomeDenied, requestID, target.User.Email)
		return fmt.Errorf("%w: can only grant capabilities to EXTERNAL users", ErrCapabilityConflict)
	}

	if err := store.GrantCapability(ctx, params.UserID, params.Capability); err != nil {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityGranted, AuditOutcomeFailure, requestID, target.User.Email)
		return err
	}
	service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityGranted, AuditOutcomeSuccess, requestID, target.User.Email)
	return nil
}

func (service *Service) RevokeCapability(ctx context.Context, actor Session, params CapabilityGrant, requestID string) error {
	if !actor.User.Role.CanManageUsers() || !actor.User.Active {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityRevoked, AuditOutcomeDenied, requestID, actor.User.Email)
		return ErrForbidden
	}
	if !params.Capability.Valid() {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityRevoked, AuditOutcomeDenied, requestID, actor.User.Email)
		return ErrCapabilityConflict
	}
	if actor.User.ID == params.UserID {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityRevoked, AuditOutcomeDenied, requestID, actor.User.Email)
		return ErrSelfAccessChange
	}

	store, ok := service.store.(CapabilityStore)
	if !ok {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityRevoked, AuditOutcomeFailure, requestID, actor.User.Email)
		return fmt.Errorf("%w: capability store is unavailable", ErrInvalidServiceSetup)
	}

	if err := store.RevokeCapability(ctx, params.UserID, params.Capability); err != nil {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityRevoked, AuditOutcomeFailure, requestID, actor.User.Email)
		return err
	}
	service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventCapabilityRevoked, AuditOutcomeSuccess, requestID, actor.User.Email)
	return nil
}

func (service *Service) ListCapabilities(ctx context.Context, actor Session, userID Identifier, requestID string) ([]Capability, error) {
	if !actor.User.Role.CanManageUsers() || !actor.User.Active {
		return nil, ErrForbidden
	}
	store, ok := service.store.(CapabilityStore)
	if !ok {
		return nil, fmt.Errorf("%w: capability store is unavailable", ErrInvalidServiceSetup)
	}
	return store.ListCapabilities(ctx, userID)
}

package auth

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrForbidden           = errors.New("forbidden")
	ErrUserAccessConflict  = errors.New("user access conflict")
	ErrProtectedSuperadmin = errors.New("superadmin access is protected")
	ErrSelfAccessChange    = errors.New("users cannot change their own access")
	ErrInvalidUserAccess   = errors.New("invalid user access")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrUserInUse           = errors.New("user in use")
)

type ManagedUser struct {
	User    User
	Version int64
}

type UpdateUserAccessParams struct {
	UserID      Identifier
	Role        Role
	Active      bool
	Version     int64
	DisplayName string
	Email       string
}

type ProvisionUserParams struct {
	ID           Identifier
	ActorID      Identifier
	Email        string
	DisplayName  string
	Role         Role
	Capabilities []Capability
}

type UserAdministrationStore interface {
	ListUsers(context.Context, int32, int32) ([]ManagedUser, error)
	FindUserByID(context.Context, Identifier) (ManagedUser, error)
	UpdateUserAccess(context.Context, UpdateUserAccessParams) (ManagedUser, error)
	RevokeAllSessionsForUser(context.Context, Identifier) error
}

type UserProvisionStore interface {
	ProvisionUser(context.Context, ProvisionUserParams) (ManagedUser, error)
}

func (identifier Identifier) String() string {
	encoded := hex.EncodeToString(identifier[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func ParseIdentifier(value string) (Identifier, error) {
	compact := strings.ReplaceAll(strings.TrimSpace(value), "-", "")
	if len(compact) != 32 {
		return Identifier{}, ErrInvalidUserAccess
	}
	decoded, err := hex.DecodeString(compact)
	if err != nil {
		return Identifier{}, ErrInvalidUserAccess
	}
	var identifier Identifier
	copy(identifier[:], decoded)
	return identifier, nil
}

func (service *Service) ListUsers(ctx context.Context, actor Session, limit, offset int32, requestID string) ([]ManagedUser, error) {
	if !actor.User.Role.CanManageUsers() || !actor.User.Active {
		service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAdministrationAccessed, AuditOutcomeDenied, requestID, actor.User.Email)
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	store, ok := service.store.(UserAdministrationStore)
	if !ok {
		service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAdministrationAccessed, AuditOutcomeFailure, requestID, actor.User.Email)
		return nil, fmt.Errorf("%w: administration store is unavailable", ErrInvalidServiceSetup)
	}
	users, err := store.ListUsers(ctx, limit, offset)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAdministrationAccessed, AuditOutcomeFailure, requestID, actor.User.Email)
		return nil, err
	}
	service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAdministrationAccessed, AuditOutcomeSuccess, requestID, actor.User.Email)
	return users, nil
}

func (service *Service) UpdateUserAccess(ctx context.Context, actor Session, params UpdateUserAccessParams, requestID string) (ManagedUser, error) {
	if !actor.User.Role.CanManageUsers() || !actor.User.Active {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, actor.User.Email)
		return ManagedUser{}, ErrForbidden
	}
	if (params.Role != RoleExternal && params.Role != RoleAdmin) || params.Version <= 0 {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, actor.User.Email)
		return ManagedUser{}, ErrInvalidUserAccess
	}
	if actor.User.ID == params.UserID {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, actor.User.Email)
		return ManagedUser{}, ErrSelfAccessChange
	}

	store, ok := service.store.(UserAdministrationStore)
	if !ok {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, actor.User.Email)
		return ManagedUser{}, fmt.Errorf("%w: administration store is unavailable", ErrInvalidServiceSetup)
	}
	target, err := store.FindUserByID(ctx, params.UserID)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, actor.User.Email)
		return ManagedUser{}, err
	}
	if target.User.Role == RoleSuperadmin {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, target.User.Email)
		return ManagedUser{}, ErrProtectedSuperadmin
	}
	displayName := strings.TrimSpace(params.DisplayName)
	email := normalizeEmail(params.Email)
	if displayName == "" {
		displayName = target.User.DisplayName
	}
	if email == "" {
		email = target.User.Email
	}
	params.DisplayName = displayName
	params.Email = email
	roleChanged := target.User.Role != params.Role || target.User.Active != params.Active
	emailChanged := email != target.User.Email
	if !roleChanged && !emailChanged && displayName == target.User.DisplayName {
		return target, nil
	}

	updated, err := store.UpdateUserAccess(ctx, params)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, target.User.Email)
		return ManagedUser{}, err
	}
	service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeSuccess, requestID, updated.User.Email)
	if roleChanged || emailChanged {
		if err := store.RevokeAllSessionsForUser(ctx, params.UserID); err != nil {
			service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventSessionRevoked, AuditOutcomeFailure, requestID, target.User.Email)
			return ManagedUser{}, fmt.Errorf("revoke changed user sessions: %w", err)
		}
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventSessionRevoked, AuditOutcomeSuccess, requestID, updated.User.Email)
	}
	if emailChanged && service.allowlistStore != nil {
		if err := service.allowlistStore.AddAllowedEmail(ctx, email, &actor.User.ID); err != nil {
			return ManagedUser{}, err
		}
		if err := service.allowlistStore.RemoveAllowedEmail(ctx, target.User.Email); err != nil {
			return ManagedUser{}, err
		}
	}
	return updated, nil
}

func (service *Service) ProvisionUser(ctx context.Context, actor Session, params ProvisionUserParams, requestID string) (ManagedUser, error) {
	if !actor.User.Role.CanManageUsers() || !actor.User.Active {
		service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, actor.User.Email)
		return ManagedUser{}, ErrForbidden
	}
	params.Email = normalizeEmail(params.Email)
	params.DisplayName = strings.TrimSpace(params.DisplayName)
	// Name is optional at provision time; first Google login fills it. Until then
	// use the email so the non-empty DB constraint stays satisfied.
	if params.DisplayName == "" {
		params.DisplayName = params.Email
	}
	if params.Email == "" || len(params.DisplayName) > 200 || len(params.Email) > 320 ||
		(params.Role != RoleExternal && params.Role != RoleAdmin) {
		service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, actor.User.Email)
		return ManagedUser{}, ErrInvalidUserAccess
	}
	capabilities, err := provisionCapabilities(params.Role, params.Capabilities)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, params.Email)
		return ManagedUser{}, err
	}
	params.Capabilities = capabilities

	store, ok := service.store.(UserProvisionStore)
	if !ok {
		service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, params.Email)
		return ManagedUser{}, fmt.Errorf("%w: provision store is unavailable", ErrInvalidServiceSetup)
	}
	userID, err := NewIdentifier()
	if err != nil {
		return ManagedUser{}, fmt.Errorf("generate user id: %w", err)
	}
	params.ID = userID
	params.ActorID = actor.User.ID
	created, err := store.ProvisionUser(ctx, params)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &params.ID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, params.Email)
		return ManagedUser{}, err
	}
	service.recordAudit(ctx, &actor.User.ID, &created.User.ID, AuditEventUserAccessChanged, AuditOutcomeSuccess, requestID, created.User.Email)
	return created, nil
}

func provisionCapabilities(role Role, values []Capability) ([]Capability, error) {
	if role != RoleExternal {
		return nil, nil
	}
	seen := make(map[Capability]struct{}, len(values))
	granted := make([]Capability, 0, len(values))
	for _, value := range values {
		if !value.Valid() {
			return nil, ErrCapabilityConflict
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		granted = append(granted, value)
	}
	if len(granted) == 0 {
		return nil, ErrMemberNeedsCapability
	}
	return granted, nil
}

func (service *Service) DeleteUser(ctx context.Context, actor Session, userID Identifier, requestID string) error {
	if !actor.User.Role.CanManageUsers() || !actor.User.Active {
		service.recordAudit(ctx, &actor.User.ID, &userID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, actor.User.Email)
		return ErrForbidden
	}
	if actor.User.ID == userID {
		service.recordAudit(ctx, &actor.User.ID, &userID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, actor.User.Email)
		return ErrSelfAccessChange
	}
	store, ok := service.store.(interface {
		FindUserByID(context.Context, Identifier) (ManagedUser, error)
		DeleteUser(context.Context, Identifier, string) error
	})
	if !ok {
		service.recordAudit(ctx, &actor.User.ID, &userID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, actor.User.Email)
		return fmt.Errorf("%w: delete store is unavailable", ErrInvalidServiceSetup)
	}
	target, err := store.FindUserByID(ctx, userID)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &userID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, actor.User.Email)
		return err
	}
	if target.User.Role == RoleSuperadmin {
		service.recordAudit(ctx, &actor.User.ID, &userID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, target.User.Email)
		return ErrProtectedSuperadmin
	}
	if err := store.DeleteUser(ctx, userID, target.User.Email); err != nil {
		service.recordAudit(ctx, &actor.User.ID, &userID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, target.User.Email)
		return err
	}
	service.recordAudit(ctx, &actor.User.ID, nil, AuditEventUserAccessChanged, AuditOutcomeSuccess, requestID, target.User.Email)
	return nil
}

package auth

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrForbidden           = errors.New("forbidden")
	ErrInvalidManagedRole  = errors.New("invalid managed role")
	ErrInvalidUserVersion  = errors.New("invalid user version")
	ErrProtectedSuperadmin = errors.New("superadmin access is protected")
	ErrSelfAccessChange    = errors.New("users cannot change their own access")
	ErrUserAccessConflict  = errors.New("user access changed concurrently")
	ErrAdministrationSetup = errors.New("user administration store is not configured")
)

type Permission string

const PermissionManageUsers Permission = "manage_users"

func (role Role) Allows(permission Permission) bool {
	switch permission {
	case PermissionManageUsers:
		return role == RoleAdmin || role == RoleSuperadmin
	default:
		return false
	}
}

type ManagedUser struct {
	ID          Identifier
	Login       string
	DisplayName string
	AvatarURL   string
	Role        Role
	Active      bool
	Version     int64
}

type UserAccessUpdate struct {
	UserID  Identifier
	Role    Role
	Active  bool
	Version int64
}

type administrationStore interface {
	FindManagedUser(context.Context, Identifier) (ManagedUser, error)
	ListManagedUsers(context.Context, int32, int32) ([]ManagedUser, error)
	UpdateManagedUserAccess(context.Context, UserAccessUpdate) (ManagedUser, error)
	RevokeAllSessionsForUser(context.Context, Identifier) error
}

func (service *Service) ListUsers(ctx context.Context, actor User, limit, offset int32, requestID string) ([]ManagedUser, error) {
	if !actor.Role.Allows(PermissionManageUsers) {
		service.recordAudit(ctx, &actor.ID, nil, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, "")
		return nil, ErrForbidden
	}

	store, err := service.administrationStore()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	users, err := store.ListManagedUsers(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list managed users: %w", err)
	}
	return users, nil
}

func (service *Service) UpdateUserAccess(ctx context.Context, actor User, update UserAccessUpdate, requestID string) (ManagedUser, error) {
	store, err := service.administrationStore()
	if err != nil {
		return ManagedUser{}, err
	}

	if !actor.Role.Allows(PermissionManageUsers) {
		service.recordAudit(ctx, &actor.ID, &update.UserID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, "")
		return ManagedUser{}, ErrForbidden
	}
	if update.Role != RoleMember && update.Role != RoleAdmin {
		service.recordAudit(ctx, &actor.ID, &update.UserID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, "")
		return ManagedUser{}, ErrInvalidManagedRole
	}
	if update.Version <= 0 {
		return ManagedUser{}, ErrInvalidUserVersion
	}
	if actor.ID == update.UserID {
		service.recordAudit(ctx, &actor.ID, &update.UserID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, "")
		return ManagedUser{}, ErrSelfAccessChange
	}

	target, err := store.FindManagedUser(ctx, update.UserID)
	if err != nil {
		return ManagedUser{}, err
	}
	if target.Role == RoleSuperadmin {
		service.recordAudit(ctx, &actor.ID, &target.ID, AuditEventUserAccessChanged, AuditOutcomeDenied, requestID, "")
		return ManagedUser{}, ErrProtectedSuperadmin
	}
	if target.Role == update.Role && target.Active == update.Active {
		return target, nil
	}

	updated, err := store.UpdateManagedUserAccess(ctx, update)
	if err != nil {
		if errors.Is(err, ErrUserAccessConflict) {
			return ManagedUser{}, err
		}
		service.recordAudit(ctx, &actor.ID, &target.ID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, "")
		return ManagedUser{}, fmt.Errorf("update managed user access: %w", err)
	}
	if err := store.RevokeAllSessionsForUser(ctx, target.ID); err != nil {
		service.recordAudit(ctx, &actor.ID, &target.ID, AuditEventSessionRevoked, AuditOutcomeFailure, requestID, "")
		return ManagedUser{}, fmt.Errorf("revoke managed user sessions: %w", err)
	}

	service.recordAudit(ctx, &actor.ID, &target.ID, AuditEventUserAccessChanged, AuditOutcomeSuccess, requestID, "")
	service.recordAudit(ctx, &actor.ID, &target.ID, AuditEventSessionRevoked, AuditOutcomeSuccess, requestID, "")
	return updated, nil
}

func (service *Service) administrationStore() (administrationStore, error) {
	store, ok := service.store.(administrationStore)
	if !ok {
		return nil, ErrAdministrationSetup
	}
	return store, nil
}

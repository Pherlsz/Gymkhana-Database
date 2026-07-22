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
)

type ManagedUser struct {
	User    User
	Version int64
}

type UpdateUserAccessParams struct {
	UserID  Identifier
	Role    Role
	Active  bool
	Version int64
}

type UserAdministrationStore interface {
	ListUsers(context.Context, int32, int32) ([]ManagedUser, error)
	FindUserByID(context.Context, Identifier) (ManagedUser, error)
	UpdateUserAccess(context.Context, UpdateUserAccessParams) (ManagedUser, error)
	RevokeAllSessionsForUser(context.Context, Identifier) error
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
	if (params.Role != RoleMember && params.Role != RoleAdmin) || params.Version <= 0 {
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
	if target.User.Role == params.Role && target.User.Active == params.Active {
		return target, nil
	}

	updated, err := store.UpdateUserAccess(ctx, params)
	if err != nil {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeFailure, requestID, target.User.Email)
		return ManagedUser{}, err
	}
	if err := store.RevokeAllSessionsForUser(ctx, params.UserID); err != nil {
		service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventSessionRevoked, AuditOutcomeFailure, requestID, target.User.Email)
		return ManagedUser{}, fmt.Errorf("revoke changed user sessions: %w", err)
	}
	service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventUserAccessChanged, AuditOutcomeSuccess, requestID, updated.User.Email)
	service.recordAudit(ctx, &actor.User.ID, &params.UserID, AuditEventSessionRevoked, AuditOutcomeSuccess, requestID, updated.User.Email)
	return updated, nil
}

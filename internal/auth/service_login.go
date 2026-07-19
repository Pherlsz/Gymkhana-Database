package auth

import (
	"context"
	"errors"
	"fmt"
)

func (service *Service) completeGoogleLogin(ctx context.Context, identity GoogleIdentity, requestID string) (User, error) {
	if identity.Subject == "" || identity.Email == "" || !identity.EmailVerified {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return User{}, errors.New("google returned an invalid or unverified identity")
	}
	store, ok := service.store.(GoogleIdentityStore)
	if !ok {
		return User{}, fmt.Errorf("%w: google identity store is unavailable", ErrInvalidServiceSetup)
	}
	user, err := store.FindUserByGoogleSubject(ctx, identity.Subject)
	if errors.Is(err, ErrUserNotFound) {
		user, err = store.FindUserByEmail(ctx, identity.Email)
	}
	if errors.Is(err, ErrUserNotFound) {
		service.recordAudit(ctx, nil, nil, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Email)
		return User{}, ErrAccessDenied
	}
	if err != nil {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return User{}, fmt.Errorf("load allowlisted user: %w", err)
	}
	if !user.Active || user.Email != identity.Email || (user.GoogleSubject != "" && user.GoogleSubject != identity.Subject) {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Email)
		return User{}, ErrAccessDenied
	}
	user, err = store.UpdateGoogleIdentity(ctx, user.ID, identity)
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return User{}, fmt.Errorf("bind google identity: %w", err)
	}
	return user, nil
}

func (service *Service) completeLegacyLogin(ctx context.Context, identity GoogleIdentity, requestID string) (User, error) {
	if identity.UserID <= 0 || identity.Login == "" || len(service.allowedLogins) == 0 {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
		return User{}, errors.New("oauth provider returned an invalid identity")
	}
	if _, allowed := service.allowedLogins[identity.Login]; !allowed {
		service.recordAudit(ctx, nil, nil, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Login)
		return User{}, ErrAccessDenied
	}
	user, err := service.store.FindUserByGitHubID(ctx, identity.UserID)
	switch {
	case errors.Is(err, ErrUserNotFound):
		role := RoleMember
		if identity.Login == service.superadminLogin {
			role = RoleSuperadmin
		}
		userID, idErr := NewIdentifier()
		if idErr != nil {
			return User{}, fmt.Errorf("generate user id: %w", idErr)
		}
		user, err = service.store.CreateUser(ctx, CreateUserParams{ID: userID, Identity: identity, Role: role})
	case err == nil:
		if !user.Active {
			service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Login)
			return User{}, ErrAccessDenied
		}
		user, err = service.store.UpdateUserIdentity(ctx, user.ID, identity)
	}
	if err != nil {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
		return User{}, fmt.Errorf("persist authenticated user: %w", err)
	}
	return user, nil
}

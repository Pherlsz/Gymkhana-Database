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
	user, err := service.store.FindUserByGoogleSubject(ctx, identity.Subject)
	if errors.Is(err, ErrUserNotFound) {
		user, err = service.store.FindUserByEmail(ctx, identity.Email)
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
	user, err = service.store.UpdateGoogleIdentity(ctx, user.ID, identity)
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return User{}, fmt.Errorf("bind google identity: %w", err)
	}
	return user, nil
}

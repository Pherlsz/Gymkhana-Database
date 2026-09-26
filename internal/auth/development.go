package auth

import (
	"context"
	"errors"
	"fmt"
)

// DevelopmentLoginEmail is the fixed local identity used by Dev Login. It is
// not configurable: staging/production never expose this path.
const DevelopmentLoginEmail = "developer@gymkhana.local"

// DevelopmentLogin creates a normal application session for the local
// development user without contacting Google. HTTP exposure is restricted by
// the server to development environments.
func (service *Service) DevelopmentLogin(ctx context.Context, requestID string) (LoginResult, error) {
	identity := normalizeIdentity(GoogleIdentity{
		Email:       DevelopmentLoginEmail,
		DisplayName: "Development User",
		Subject:     "development",
	})

	if err := service.allowlistStore.AddAllowedEmail(ctx, identity.Email, nil); err != nil {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, fmt.Errorf("ensure development allowlist: %w", err)
	}

	user, err := service.store.FindUserByEmail(ctx, identity.Email)
	switch {
	case errors.Is(err, ErrUserNotFound):
		userID, idErr := NewIdentifier()
		if idErr != nil {
			service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
			return LoginResult{}, fmt.Errorf("generate development user id: %w", idErr)
		}
		user, err = service.store.CreateUser(ctx, CreateUserParams{ID: userID, Identity: identity, Role: RoleSuperadmin})
	case err == nil:
		if !user.Active {
			service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Email)
			return LoginResult{}, ErrAccessDenied
		}
		user, err = service.store.UpdateUserIdentity(ctx, user.ID, identity)
	}
	if err != nil {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, fmt.Errorf("persist development user: %w", err)
	}

	sessionValue, err := NewOpaqueSessionValue()
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, fmt.Errorf("generate development session value: %w", err)
	}
	hash, err := HashOpaqueSessionValue(sessionValue)
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, err
	}
	sessionID, err := NewIdentifier()
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, fmt.Errorf("generate development session id: %w", err)
	}
	expiresAt := SessionExpiresAt(service.now().UTC())
	if err := service.store.CreateSession(ctx, CreateSessionParams{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: hash[:],
		ExpiresAt: expiresAt,
	}); err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, fmt.Errorf("create development session: %w", err)
	}

	service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInSucceeded, AuditOutcomeSuccess, requestID, identity.Email)
	return LoginResult{SessionValue: sessionValue, ExpiresAt: expiresAt, User: user}, nil
}

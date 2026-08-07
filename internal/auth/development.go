package auth

import (
	"context"
	"errors"
	"fmt"
)

// DevelopmentLogin creates a normal application session for the configured
// superadmin without contacting the external OAuth provider. HTTP exposure is
// restricted by the server to development environments.
func (service *Service) DevelopmentLogin(ctx context.Context, requestID string) (LoginResult, error) {
	identity := normalizeIdentity(GoogleIdentity{
		Email:       service.superadminEmail,
		DisplayName: "Development User",
		Subject:     "development",
	})

	allowed := false
	if service.allowlistStore != nil {
		dbAllowed, err := service.allowlistStore.IsEmailAllowed(ctx, identity.Email)
		if err != nil {
			service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
			return LoginResult{}, fmt.Errorf("check development email allowlist: %w", err)
		}
		allowed = dbAllowed
	} else {
		_, allowed = service.allowedEmails[identity.Email]
	}
	if !allowed {
		service.recordAudit(ctx, nil, nil, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Email)
		return LoginResult{}, ErrAccessDenied
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

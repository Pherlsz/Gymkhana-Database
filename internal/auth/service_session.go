package auth

import (
	"context"
	"errors"
	"fmt"
)

func (service *Service) createLoginSession(ctx context.Context, user User, providerLogin, requestID string) (LoginResult, error) {
	sessionValue, err := NewOpaqueSessionValue()
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, providerLogin)
		return LoginResult{}, fmt.Errorf("generate session value: %w", err)
	}
	hash, err := HashOpaqueSessionValue(sessionValue)
	if err != nil {
		return LoginResult{}, err
	}
	sessionID, err := NewIdentifier()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate session id: %w", err)
	}
	now := service.now().UTC()
	expiresAt := SessionExpiresAt(now)
	if err := service.store.CreateSession(ctx, CreateSessionParams{ID: sessionID, UserID: user.ID, TokenHash: hash[:], ExpiresAt: expiresAt}); err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, providerLogin)
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}
	service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInSucceeded, AuditOutcomeSuccess, requestID, providerLogin)
	return LoginResult{SessionValue: sessionValue, ExpiresAt: expiresAt, User: user}, nil
}

func (service *Service) CurrentSession(ctx context.Context, sessionValue string) (Session, error) {
	hash, err := HashOpaqueSessionValue(sessionValue)
	if err != nil {
		return Session{}, ErrUnauthenticated
	}
	session, err := service.store.FindAuthenticatedSession(ctx, hash[:], service.now().UTC())
	if errors.Is(err, ErrSessionNotFound) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, fmt.Errorf("load authenticated session: %w", err)
	}
	if err := service.store.TouchSession(ctx, session.ID); err != nil {
		return Session{}, fmt.Errorf("touch authenticated session: %w", err)
	}
	return session, nil
}

func (service *Service) SignOut(ctx context.Context, sessionValue, requestID string) error {
	if sessionValue == "" {
		return nil
	}
	hash, err := HashOpaqueSessionValue(sessionValue)
	if err != nil {
		return nil
	}
	var actor *Identifier
	if session, sessionErr := service.store.FindAuthenticatedSession(ctx, hash[:], service.now().UTC()); sessionErr == nil {
		actor = &session.User.ID
	}
	if err := service.store.RevokeSessionByTokenHash(ctx, hash[:]); err != nil {
		service.recordAudit(ctx, actor, actor, AuditEventSignOut, AuditOutcomeFailure, requestID, "")
		return fmt.Errorf("revoke session: %w", err)
	}
	service.recordAudit(ctx, actor, actor, AuditEventSignOut, AuditOutcomeSuccess, requestID, "")
	return nil
}

func (service *Service) recordAudit(ctx context.Context, actorUserID, subjectUserID *Identifier, eventType AuditEventType, outcome AuditOutcome, requestID, providerLogin string) {
	event := AuditEvent{ActorUserID: actorUserID, SubjectUserID: subjectUserID, EventType: eventType, Outcome: outcome, RequestID: requestID, ProviderLogin: providerLogin}
	id, err := NewIdentifier()
	if err != nil {
		service.reportAuditFailure(ctx, event, fmt.Errorf("generate audit event id: %w", err))
		return
	}
	event.ID = id
	if err := service.store.RecordAuditEvent(ctx, event); err != nil {
		service.reportAuditFailure(ctx, event, fmt.Errorf("record audit event: %w", err))
	}
}

func (service *Service) reportAuditFailure(ctx context.Context, event AuditEvent, err error) {
	if service.onAuditFailure != nil {
		service.onAuditFailure(ctx, event, err)
	}
}

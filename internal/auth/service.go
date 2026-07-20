package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrAccessDenied        = errors.New("access denied")
	ErrInvalidOAuthCode    = errors.New("invalid oauth code")
	ErrSessionNotFound     = errors.New("session not found")
	ErrUnauthenticated     = errors.New("unauthenticated")
	ErrUserNotFound        = errors.New("user not found")
	ErrInvalidServiceSetup = errors.New("invalid authentication service setup")
)

type Identifier [16]byte

func NewIdentifier() (Identifier, error) {
	var value Identifier
	if _, err := rand.Read(value[:]); err != nil {
		return Identifier{}, err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return value, nil
}

type Session struct {
	ID   Identifier
	User User
}

type CreateUserParams struct {
	ID       Identifier
	Identity GoogleIdentity
	Role     Role
}

type CreateSessionParams struct {
	ID        Identifier
	UserID    Identifier
	TokenHash []byte
	ExpiresAt time.Time
}

type AuditEvent struct {
	ID            Identifier
	ActorUserID   *Identifier
	SubjectUserID *Identifier
	EventType     AuditEventType
	Outcome       AuditOutcome
	RequestID     string
	ProviderEmail string
}

type Store interface {
	FindUserByGoogleSubject(context.Context, string) (User, error)
	FindUserByEmail(context.Context, string) (User, error)
	UpdateGoogleIdentity(context.Context, Identifier, GoogleIdentity) (User, error)
	CreateSession(context.Context, CreateSessionParams) error
	FindAuthenticatedSession(context.Context, []byte, time.Time) (Session, error)
	TouchSession(context.Context, Identifier) error
	RevokeSessionByTokenHash(context.Context, []byte) error
	RecordAuditEvent(context.Context, AuditEvent) error
}

type OAuthProvider interface {
	AuthorizationURL(state string) string
	Exchange(context.Context, string) (GoogleIdentity, error)
}

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	Now            func() time.Time
	OnAuditFailure AuditFailureHandler
}

type Service struct {
	provider       OAuthProvider
	store          Store
	now            func() time.Time
	onAuditFailure AuditFailureHandler
}

type LoginResult struct {
	SessionValue string
	ExpiresAt    time.Time
	User         User
}

func NewService(provider OAuthProvider, store Store, options ServiceOptions) (*Service, error) {
	if provider == nil || store == nil {
		return nil, fmt.Errorf("%w: provider and store are required", ErrInvalidServiceSetup)
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{provider: provider, store: store, now: now, onAuditFailure: options.OnAuditFailure}, nil
}

func (service *Service) BeginLogin() (string, string, error) {
	state, err := NewOpaqueSessionValue()
	if err != nil {
		return "", "", fmt.Errorf("generate oauth state: %w", err)
	}
	return state, service.provider.AuthorizationURL(state), nil
}

func (service *Service) CompleteLogin(ctx context.Context, code, requestID string) (LoginResult, error) {
	if strings.TrimSpace(code) == "" {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, "")
		return LoginResult{}, ErrInvalidOAuthCode
	}
	identity, err := service.provider.Exchange(ctx, code)
	if err != nil {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, "")
		return LoginResult{}, fmt.Errorf("exchange oauth code: %w", err)
	}
	identity = normalizeIdentity(identity)
	user, err := service.completeGoogleLogin(ctx, identity, requestID)
	if err != nil {
		return LoginResult{}, err
	}
	return service.createLoginSession(ctx, user, identity.Email, requestID)
}

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
	ProviderLogin string
}

type Store interface {
	FindUserByGitHubID(context.Context, int64) (User, error)
	CreateUser(context.Context, CreateUserParams) (User, error)
	UpdateUserIdentity(context.Context, Identifier, GitHubIdentity) (User, error)
	CreateSession(context.Context, CreateSessionParams) error
	FindAuthenticatedSession(context.Context, []byte, time.Time) (Session, error)
	TouchSession(context.Context, Identifier) error
	RevokeSessionByTokenHash(context.Context, []byte) error
	RecordAuditEvent(context.Context, AuditEvent) error
}

type GoogleIdentityStore interface {
	FindUserByGoogleSubject(context.Context, string) (User, error)
	FindUserByEmail(context.Context, string) (User, error)
	UpdateGoogleIdentity(context.Context, Identifier, GoogleIdentity) (User, error)
}

type OAuthProvider interface {
	AuthorizationURL(state string) string
	Exchange(context.Context, string) (GoogleIdentity, error)
}

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	AllowedLogins   []string
	SuperadminLogin string
	Now             func() time.Time
	OnAuditFailure  AuditFailureHandler
}

type Service struct {
	provider        OAuthProvider
	store           Store
	allowedLogins   map[string]struct{}
	superadminLogin string
	now             func() time.Time
	onAuditFailure  AuditFailureHandler
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
	allowedLogins := make(map[string]struct{}, len(options.AllowedLogins))
	for _, login := range options.AllowedLogins {
		if normalized := normalizeLogin(login); normalized != "" {
			allowedLogins[normalized] = struct{}{}
		}
	}
	superadminLogin := normalizeLogin(options.SuperadminLogin)
	if len(allowedLogins) > 0 {
		if _, allowed := allowedLogins[superadminLogin]; superadminLogin == "" || !allowed {
			return nil, fmt.Errorf("%w: superadmin login must be allowed", ErrInvalidServiceSetup)
		}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Service{provider: provider, store: store, allowedLogins: allowedLogins, superadminLogin: superadminLogin, now: now, onAuditFailure: options.OnAuditFailure}, nil
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
	var user User
	if identity.Subject != "" || identity.Email != "" {
		user, err = service.completeGoogleLogin(ctx, identity, requestID)
	} else {
		user, err = service.completeLegacyLogin(ctx, identity, requestID)
	}
	if err != nil {
		return LoginResult{}, err
	}
	providerLogin := identity.Email
	if providerLogin == "" {
		providerLogin = identity.Login
	}
	return service.createLoginSession(ctx, user, providerLogin, requestID)
}

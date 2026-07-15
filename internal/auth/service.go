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

type GitHubIdentity struct {
	UserID      int64
	Login       string
	DisplayName string
	AvatarURL   string
}

type User struct {
	ID           Identifier
	GitHubUserID int64
	Login        string
	DisplayName  string
	AvatarURL    string
	Role         Role
	Active       bool
}

type Session struct {
	ID   Identifier
	User User
}

type CreateUserParams struct {
	ID       Identifier
	Identity GitHubIdentity
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

type OAuthProvider interface {
	AuthorizationURL(state string) string
	Exchange(context.Context, string) (GitHubIdentity, error)
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
		normalized := normalizeLogin(login)
		if normalized != "" {
			allowedLogins[normalized] = struct{}{}
		}
	}
	if len(allowedLogins) == 0 {
		return nil, fmt.Errorf("%w: at least one allowed login is required", ErrInvalidServiceSetup)
	}

	superadminLogin := normalizeLogin(options.SuperadminLogin)
	if _, allowed := allowedLogins[superadminLogin]; superadminLogin == "" || !allowed {
		return nil, fmt.Errorf("%w: superadmin login must be allowed", ErrInvalidServiceSetup)
	}

	now := options.Now
	if now == nil {
		now = time.Now
	}

	return &Service{
		provider:        provider,
		store:           store,
		allowedLogins:   allowedLogins,
		superadminLogin: superadminLogin,
		now:             now,
		onAuditFailure:  options.OnAuditFailure,
	}, nil
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
	if identity.UserID <= 0 || identity.Login == "" {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
		return LoginResult{}, errors.New("oauth provider returned an invalid identity")
	}
	if _, allowed := service.allowedLogins[identity.Login]; !allowed {
		service.recordAudit(ctx, nil, nil, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Login)
		return LoginResult{}, ErrAccessDenied
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
			service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
			return LoginResult{}, fmt.Errorf("generate user id: %w", idErr)
		}
		user, err = service.store.CreateUser(ctx, CreateUserParams{ID: userID, Identity: identity, Role: role})
	case err == nil:
		if !user.Active {
			service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Login)
			return LoginResult{}, ErrAccessDenied
		}
		user, err = service.store.UpdateUserIdentity(ctx, user.ID, identity)
	}
	if err != nil {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
		return LoginResult{}, fmt.Errorf("persist authenticated user: %w", err)
	}

	sessionValue, err := NewOpaqueSessionValue()
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
		return LoginResult{}, fmt.Errorf("generate session value: %w", err)
	}
	hash, err := HashOpaqueSessionValue(sessionValue)
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
		return LoginResult{}, err
	}
	sessionID, err := NewIdentifier()
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
		return LoginResult{}, fmt.Errorf("generate session id: %w", err)
	}
	now := service.now().UTC()
	expiresAt := SessionExpiresAt(now)
	if err := service.store.CreateSession(ctx, CreateSessionParams{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: hash[:],
		ExpiresAt: expiresAt,
	}); err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Login)
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInSucceeded, AuditOutcomeSuccess, requestID, identity.Login)
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

func (service *Service) recordAudit(
	ctx context.Context,
	actorUserID *Identifier,
	subjectUserID *Identifier,
	eventType AuditEventType,
	outcome AuditOutcome,
	requestID string,
	providerLogin string,
) {
	event := AuditEvent{
		ActorUserID:   actorUserID,
		SubjectUserID: subjectUserID,
		EventType:     eventType,
		Outcome:       outcome,
		RequestID:     requestID,
		ProviderLogin: providerLogin,
	}
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

func normalizeIdentity(identity GitHubIdentity) GitHubIdentity {
	identity.Login = normalizeLogin(identity.Login)
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	identity.AvatarURL = strings.TrimSpace(identity.AvatarURL)
	if identity.DisplayName == "" {
		identity.DisplayName = identity.Login
	}
	return identity
}

func normalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

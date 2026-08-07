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

type GoogleIdentity struct {
	Email       string
	DisplayName string
	AvatarURL   string
	Subject     string
}

type User struct {
	ID          Identifier
	Email       string
	DisplayName string
	AvatarURL   string
	Role        Role
	Active      bool
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
	FindUserByEmail(context.Context, string) (User, error)
	CreateUser(context.Context, CreateUserParams) (User, error)
	UpdateUserIdentity(context.Context, Identifier, GoogleIdentity) (User, error)
	IsEmailAllowed(context.Context, string) (bool, error)
	CreateSession(context.Context, CreateSessionParams) error
	FindAuthenticatedSession(context.Context, []byte, time.Time) (Session, error)
	TouchSession(context.Context, Identifier) error
	RevokeSessionByTokenHash(context.Context, []byte) error
	RecordAuditEvent(context.Context, AuditEvent) error
}

type AllowlistStore interface {
	IsEmailAllowed(context.Context, string) (bool, error)
	AddAllowedEmail(context.Context, string, *Identifier) error
	RemoveAllowedEmail(context.Context, string) error
	ListAllowedEmails(context.Context) ([]string, error)
}

type OAuthProvider interface {
	AuthorizationURL(state string) string
	Exchange(context.Context, string) (GoogleIdentity, error)
}

type AuditFailureHandler func(context.Context, AuditEvent, error)

type ServiceOptions struct {
	AllowedEmails   []string
	SuperadminEmail string
	AllowlistStore  AllowlistStore
	Now             func() time.Time
	OnAuditFailure  AuditFailureHandler
}

type Service struct {
	provider        OAuthProvider
	store           Store
	allowlistStore  AllowlistStore
	allowedEmails   map[string]struct{}
	superadminEmail string
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

	allowedEmails := make(map[string]struct{}, len(options.AllowedEmails))
	for _, email := range options.AllowedEmails {
		normalized := normalizeEmail(email)
		if normalized != "" {
			allowedEmails[normalized] = struct{}{}
		}
	}
	if len(allowedEmails) == 0 {
		return nil, fmt.Errorf("%w: at least one allowed email is required", ErrInvalidServiceSetup)
	}

	superadminEmail := normalizeEmail(options.SuperadminEmail)
	if _, allowed := allowedEmails[superadminEmail]; superadminEmail == "" || !allowed {
		return nil, fmt.Errorf("%w: superadmin email must be allowed", ErrInvalidServiceSetup)
	}

	now := options.Now
	if now == nil {
		now = time.Now
	}

	return &Service{
		provider:        provider,
		store:           store,
		allowlistStore:  options.AllowlistStore,
		allowedEmails:   allowedEmails,
		superadminEmail: superadminEmail,
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
	if identity.Email == "" {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, errors.New("oauth provider returned an invalid identity")
	}

	// Check DB allowlist first if configured, fall back to in-memory map
	allowed := false
	if service.allowlistStore != nil {
		dbAllowed, err := service.allowlistStore.IsEmailAllowed(ctx, identity.Email)
		if err != nil {
			service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
			return LoginResult{}, fmt.Errorf("check email allowlist: %w", err)
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
		role := RoleExternal
		if identity.Email == service.superadminEmail {
			role = RoleSuperadmin
		}
		userID, idErr := NewIdentifier()
		if idErr != nil {
			service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
			return LoginResult{}, fmt.Errorf("generate user id: %w", idErr)
		}
		user, err = service.store.CreateUser(ctx, CreateUserParams{ID: userID, Identity: identity, Role: role})
	case err == nil:
		if !user.Active {
			service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInDenied, AuditOutcomeDenied, requestID, identity.Email)
			return LoginResult{}, ErrAccessDenied
		}
		user, err = service.store.UpdateUserIdentity(ctx, user.ID, identity)
	}
	if err != nil {
		service.recordAudit(ctx, nil, nil, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, fmt.Errorf("persist authenticated user: %w", err)
	}

	sessionValue, err := NewOpaqueSessionValue()
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, fmt.Errorf("generate session value: %w", err)
	}
	hash, err := HashOpaqueSessionValue(sessionValue)
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, err
	}
	sessionID, err := NewIdentifier()
	if err != nil {
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
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
		service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInFailed, AuditOutcomeFailure, requestID, identity.Email)
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	service.recordAudit(ctx, &user.ID, &user.ID, AuditEventSignInSucceeded, AuditOutcomeSuccess, requestID, identity.Email)
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

func (service *Service) ListAllowedEmails(ctx context.Context, actor Session) ([]string, error) {
	if !actor.User.Role.CanManageUsers() {
		return nil, errors.New("only admins can manage the email allowlist")
	}
	if service.allowlistStore == nil {
		return nil, errors.New("allowlist store not configured")
	}
	return service.allowlistStore.ListAllowedEmails(ctx)
}

func (service *Service) AddAllowedEmail(ctx context.Context, actor Session, email string) error {
	if !actor.User.Role.CanManageUsers() {
		return errors.New("only admins can manage the email allowlist")
	}
	if service.allowlistStore == nil {
		return errors.New("allowlist store not configured")
	}
	email = normalizeEmail(email)
	if email == "" {
		return errors.New("email cannot be empty")
	}
	return service.allowlistStore.AddAllowedEmail(ctx, email, &actor.User.ID)
}

func (service *Service) RemoveAllowedEmail(ctx context.Context, actor Session, email string) error {
	if !actor.User.Role.CanManageUsers() {
		return errors.New("only admins can manage the email allowlist")
	}
	if service.allowlistStore == nil {
		return errors.New("allowlist store not configured")
	}
	email = normalizeEmail(email)
	if email == "" {
		return errors.New("email cannot be empty")
	}
	return service.allowlistStore.RemoveAllowedEmail(ctx, email)
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

func normalizeIdentity(identity GoogleIdentity) GoogleIdentity {
	identity.Email = normalizeEmail(identity.Email)
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	identity.AvatarURL = strings.TrimSpace(identity.AvatarURL)
	if identity.DisplayName == "" {
		identity.DisplayName = identity.Email
	}
	return identity
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

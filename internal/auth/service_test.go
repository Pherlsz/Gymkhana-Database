package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeProvider struct {
	identity GoogleIdentity
	err      error
}

func (provider fakeProvider) AuthorizationURL(state string) string {
	return "https://accounts.google.com/authorize?state=" + state
}

func (provider fakeProvider) Exchange(context.Context, string) (GoogleIdentity, error) {
	return provider.identity, provider.err
}

type fakeStore struct {
	user             User
	findUserError    error
	createdUser      CreateUserParams
	createdSession   CreateSessionParams
	authSession      Session
	authSessionError error
	revokedHash      []byte
	revokeError      error
	audits           []AuditEvent
	auditError       error
}

func (store *fakeStore) FindUserByEmail(context.Context, string) (User, error) {
	return store.user, store.findUserError
}

func (store *fakeStore) CreateUser(_ context.Context, params CreateUserParams) (User, error) {
	store.createdUser = params
	return User{
		ID:          params.ID,
		Email:       params.Identity.Email,
		DisplayName: params.Identity.DisplayName,
		AvatarURL:   params.Identity.AvatarURL,
		Role:        params.Role,
		Active:      true,
	}, nil
}

func (store *fakeStore) UpdateUserIdentity(_ context.Context, id Identifier, identity GoogleIdentity) (User, error) {
	store.user.ID = id
	store.user.Email = identity.Email
	store.user.DisplayName = identity.DisplayName
	store.user.AvatarURL = identity.AvatarURL
	return store.user, nil
}

func (store *fakeStore) CreateSession(_ context.Context, params CreateSessionParams) error {
	store.createdSession = params
	return nil
}

func (store *fakeStore) FindAuthenticatedSession(context.Context, []byte, time.Time) (Session, error) {
	return store.authSession, store.authSessionError
}

func (store *fakeStore) TouchSession(context.Context, Identifier) error { return nil }

func (store *fakeStore) RevokeSessionByTokenHash(_ context.Context, hash []byte) error {
	store.revokedHash = append([]byte(nil), hash...)
	return store.revokeError
}

func (store *fakeStore) RecordAuditEvent(_ context.Context, event AuditEvent) error {
	if store.auditError != nil {
		return store.auditError
	}
	store.audits = append(store.audits, event)
	return nil
}

func TestServiceSignsInProvisionedUser(t *testing.T) {
	now := time.Date(2026, time.July, 14, 18, 0, 0, 0, time.UTC)
	userID, _ := NewIdentifier()
	store := &fakeStore{
		user: User{
			ID: userID, Email: "pherlsz@example.com", DisplayName: "pherlsz@example.com",
			Role: RoleSuperadmin, Active: true,
		},
	}
	service, err := NewService(fakeProvider{identity: GoogleIdentity{
		Email:       " Pherlsz@example.com ",
		DisplayName: "",
		AvatarURL:   "https://example.test/avatar.png",
	}}, store, ServiceOptions{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	state, authorizationURL, err := service.BeginLogin()
	if err != nil || state == "" || authorizationURL == "" {
		t.Fatalf("BeginLogin() = %q, %q, %v", state, authorizationURL, err)
	}
	result, err := service.CompleteLogin(context.Background(), "code", "request-1")
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}
	if result.User.Role != RoleSuperadmin || result.User.Email != "pherlsz@example.com" {
		t.Fatalf("user = %#v", result.User)
	}
	if store.createdUser.Role != "" {
		t.Fatalf("created role = %q, want no bootstrap create", store.createdUser.Role)
	}
	if result.ExpiresAt != now.Add(SessionTTL) || len(store.createdSession.TokenHash) != 32 {
		t.Fatalf("session = %#v", store.createdSession)
	}
	if len(store.audits) != 1 || store.audits[0].EventType != AuditEventSignInSucceeded {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceDeniesLoginUntilProvisioned(t *testing.T) {
	store := &fakeStore{findUserError: ErrUserNotFound}
	service, err := NewService(fakeProvider{identity: GoogleIdentity{
		Email: "member@example.com", DisplayName: "Member",
	}}, store, ServiceOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := service.CompleteLogin(context.Background(), "code", "request-unprovisioned"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("CompleteLogin() error = %v, want %v", err, ErrAccessDenied)
	}
	if store.createdUser.Role != "" {
		t.Fatalf("created role = %q, want no user", store.createdUser.Role)
	}
	if len(store.audits) != 1 || store.audits[0].EventType != AuditEventSignInDenied {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceKeepsProvisionedMemberRoleOnFirstGoogleLogin(t *testing.T) {
	userID, _ := NewIdentifier()
	store := &fakeStore{
		user: User{
			ID: userID, Email: "member@example.com", DisplayName: "Membro", Role: RoleExternal, Active: true,
		},
	}
	service, err := NewService(fakeProvider{identity: GoogleIdentity{
		Email: "member@example.com", DisplayName: "Google Name",
	}}, store, ServiceOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result, err := service.CompleteLogin(context.Background(), "code", "request-provisioned")
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}
	if result.User.Role != RoleExternal || result.User.DisplayName != "Membro" {
		t.Fatalf("user = %#v", result.User)
	}
	if store.createdUser.Role != "" {
		t.Fatalf("created a new user = %#v", store.createdUser)
	}
}

func TestServiceTakesGoogleNameWhenProvisionedNameIsEmailPlaceholder(t *testing.T) {
	userID, _ := NewIdentifier()
	store := &fakeStore{
		user: User{
			ID: userID, Email: "member@example.com", DisplayName: "member@example.com", Role: RoleExternal, Active: true,
		},
	}
	service, err := NewService(fakeProvider{identity: GoogleIdentity{
		Email: "member@example.com", DisplayName: "Google Name",
	}}, store, ServiceOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result, err := service.CompleteLogin(context.Background(), "code", "request-provisioned-name")
	if err != nil {
		t.Fatalf("CompleteLogin() error = %v", err)
	}
	if result.User.DisplayName != "Google Name" {
		t.Fatalf("user = %#v", result.User)
	}
}

func TestCompleteLoginDeniesMissingUser(t *testing.T) {
	store := &fakeStore{findUserError: ErrUserNotFound}
	service, err := NewService(fakeProvider{identity: GoogleIdentity{
		Email: "admin@example.com", DisplayName: "Admin",
	}}, store, ServiceOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := service.CompleteLogin(context.Background(), "code", "request-no-user"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("CompleteLogin() error = %v, want %v", err, ErrAccessDenied)
	}
	if store.createdUser.Role != "" {
		t.Fatalf("created = %#v, want no bootstrap", store.createdUser)
	}
}

func TestServiceRejectsInvalidOAuthCode(t *testing.T) {
	store := &fakeStore{}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := service.CompleteLogin(context.Background(), " ", "request-invalid-code"); !errors.Is(err, ErrInvalidOAuthCode) {
		t.Fatalf("CompleteLogin() error = %v, want %v", err, ErrInvalidOAuthCode)
	}
	if len(store.audits) != 1 || store.audits[0].EventType != AuditEventSignInFailed || store.audits[0].RequestID != "request-invalid-code" {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceDeniesUnknownAndInactiveUsers(t *testing.T) {
	tests := []struct {
		name      string
		identity  GoogleIdentity
		store     *fakeStore
		wantAudit AuditEventType
	}{
		{
			name:      "unknown",
			identity:  GoogleIdentity{Email: "outsider@example.com"},
			store:     &fakeStore{findUserError: ErrUserNotFound},
			wantAudit: AuditEventSignInDenied,
		},
		{
			name:     "inactive",
			identity: GoogleIdentity{Email: "member@example.com"},
			store: &fakeStore{
				user: User{
					Email:  "member@example.com",
					Role:   RoleExternal,
					Active: false,
				},
			},
			wantAudit: AuditEventSignInDenied,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewService(fakeProvider{identity: test.identity}, test.store, ServiceOptions{})
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			if _, err := service.CompleteLogin(context.Background(), "code", "request-2"); !errors.Is(err, ErrAccessDenied) {
				t.Fatalf("CompleteLogin() error = %v, want %v", err, ErrAccessDenied)
			}
			if len(test.store.audits) != 1 || test.store.audits[0].EventType != test.wantAudit {
				t.Fatalf("audits = %#v", test.store.audits)
			}
		})
	}
}

func TestServiceReadsAndRevokesOpaqueSession(t *testing.T) {
	userID, _ := NewIdentifier()
	sessionID, _ := NewIdentifier()
	store := &fakeStore{
		authSession: Session{
			ID:   sessionID,
			User: User{ID: userID, Email: "member@example.com", Role: RoleExternal, Active: true},
		},
	}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	session, err := service.CurrentSession(context.Background(), "opaque-session")
	if err != nil || session.User.Email != "member@example.com" {
		t.Fatalf("CurrentSession() = %#v, %v", session, err)
	}
	if err := service.SignOut(context.Background(), "opaque-session", "request-3"); err != nil {
		t.Fatalf("SignOut() error = %v", err)
	}
	if len(store.revokedHash) != 32 {
		t.Fatalf("revoked hash length = %d", len(store.revokedHash))
	}
	if len(store.audits) != 1 || store.audits[0].EventType != AuditEventSignOut || store.audits[0].Outcome != AuditOutcomeSuccess {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceAuditsSignOutFailure(t *testing.T) {
	userID, _ := NewIdentifier()
	store := &fakeStore{
		authSession: Session{User: User{ID: userID, Email: "member@example.com"}},
		revokeError: errors.New("database unavailable"),
	}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.SignOut(context.Background(), "opaque-session", "request-signout-failure"); err == nil {
		t.Fatal("SignOut() error = nil")
	}
	if len(store.audits) != 1 || store.audits[0].EventType != AuditEventSignOut || store.audits[0].Outcome != AuditOutcomeFailure {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceReportsAuditPersistenceFailure(t *testing.T) {
	store := &fakeStore{auditError: errors.New("audit database unavailable")}
	var reported AuditEvent
	var reportedErr error
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		OnAuditFailure: func(_ context.Context, event AuditEvent, err error) {
			reported = event
			reportedErr = err
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, _ = service.CompleteLogin(context.Background(), "", "request-audit-failure")
	if reported.EventType != AuditEventSignInFailed || reported.RequestID != "request-audit-failure" || reportedErr == nil {
		t.Fatalf("reported = %#v, error = %v", reported, reportedErr)
	}
}

func TestNewServiceRequiresProviderAndStore(t *testing.T) {
	store := &fakeStore{}
	if _, err := NewService(nil, store, ServiceOptions{}); !errors.Is(err, ErrInvalidServiceSetup) {
		t.Fatalf("NewService(nil provider) error = %v, want %v", err, ErrInvalidServiceSetup)
	}
	if _, err := NewService(fakeProvider{}, nil, ServiceOptions{}); !errors.Is(err, ErrInvalidServiceSetup) {
		t.Fatalf("NewService(nil store) error = %v, want %v", err, ErrInvalidServiceSetup)
	}
}

package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeProvider struct {
	identity GitHubIdentity
	err      error
}

func (provider fakeProvider) AuthorizationURL(state string) string {
	return "https://github.example/authorize?state=" + state
}

func (provider fakeProvider) Exchange(context.Context, string) (GitHubIdentity, error) {
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
	audits           []AuditEvent
}

func (store *fakeStore) FindUserByGitHubID(context.Context, int64) (User, error) {
	return store.user, store.findUserError
}

func (store *fakeStore) CreateUser(_ context.Context, params CreateUserParams) (User, error) {
	store.createdUser = params
	return User{
		ID:           params.ID,
		GitHubUserID: params.Identity.UserID,
		Login:        params.Identity.Login,
		DisplayName:  params.Identity.DisplayName,
		AvatarURL:    params.Identity.AvatarURL,
		Role:         params.Role,
		Active:       true,
	}, nil
}

func (store *fakeStore) UpdateUserIdentity(_ context.Context, id Identifier, identity GitHubIdentity) (User, error) {
	store.user.ID = id
	store.user.GitHubUserID = identity.UserID
	store.user.Login = identity.Login
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
	return nil
}

func (store *fakeStore) RecordAuditEvent(_ context.Context, event AuditEvent) error {
	store.audits = append(store.audits, event)
	return nil
}

func TestServiceBootstrapsAllowedSuperadminAndSession(t *testing.T) {
	now := time.Date(2026, time.July, 14, 18, 0, 0, 0, time.UTC)
	store := &fakeStore{findUserError: ErrUserNotFound}
	service, err := NewService(fakeProvider{identity: GitHubIdentity{
		UserID:      38593854,
		Login:       " Pherlsz ",
		DisplayName: "",
		AvatarURL:   "https://example.test/avatar.png",
	}}, store, ServiceOptions{
		AllowedLogins:   []string{"pherlsz", "member"},
		SuperadminLogin: "Pherlsz",
		Now:             func() time.Time { return now },
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
	if result.User.Role != RoleSuperadmin || result.User.Login != "pherlsz" || result.User.DisplayName != "pherlsz" {
		t.Fatalf("user = %#v", result.User)
	}
	if store.createdUser.Role != RoleSuperadmin {
		t.Fatalf("created role = %q", store.createdUser.Role)
	}
	if result.ExpiresAt != now.Add(SessionTTL) || len(store.createdSession.TokenHash) != 32 {
		t.Fatalf("session = %#v", store.createdSession)
	}
	if len(store.audits) != 1 || store.audits[0].EventType != AuditEventSignInSucceeded {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceDeniesUnlistedAndInactiveUsers(t *testing.T) {
	tests := []struct {
		name      string
		identity  GitHubIdentity
		store     *fakeStore
		wantAudit AuditEventType
	}{
		{
			name:      "unlisted",
			identity:  GitHubIdentity{UserID: 20, Login: "outsider"},
			store:     &fakeStore{},
			wantAudit: AuditEventSignInDenied,
		},
		{
			name:     "inactive",
			identity: GitHubIdentity{UserID: 21, Login: "member"},
			store: &fakeStore{user: User{
				GitHubUserID: 21,
				Login:        "member",
				Role:         RoleMember,
				Active:       false,
			}},
			wantAudit: AuditEventSignInDenied,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewService(fakeProvider{identity: test.identity}, test.store, ServiceOptions{
				AllowedLogins:   []string{"admin", "member"},
				SuperadminLogin: "admin",
			})
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
	store := &fakeStore{authSession: Session{
		ID:   sessionID,
		User: User{ID: userID, Login: "member", Role: RoleMember, Active: true},
	}}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		AllowedLogins:   []string{"admin"},
		SuperadminLogin: "admin",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	session, err := service.CurrentSession(context.Background(), "opaque-session")
	if err != nil || session.User.Login != "member" {
		t.Fatalf("CurrentSession() = %#v, %v", session, err)
	}
	if err := service.SignOut(context.Background(), "opaque-session", "request-3"); err != nil {
		t.Fatalf("SignOut() error = %v", err)
	}
	if len(store.revokedHash) != 32 {
		t.Fatalf("revoked hash length = %d", len(store.revokedHash))
	}
	if len(store.audits) != 1 || store.audits[0].EventType != AuditEventSignOut {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceRejectsInvalidSetup(t *testing.T) {
	_, err := NewService(fakeProvider{}, &fakeStore{}, ServiceOptions{
		AllowedLogins:   []string{"member"},
		SuperadminLogin: "admin",
	})
	if !errors.Is(err, ErrInvalidServiceSetup) {
		t.Fatalf("NewService() error = %v", err)
	}
}

package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type m2Provider struct {
	identity auth.GoogleIdentity
}

func (provider *m2Provider) AuthorizationURL(state string) string {
	return "https://github.example/authorize?state=" + state
}

func (provider *m2Provider) Exchange(context.Context, string) (auth.GoogleIdentity, error) {
	return provider.identity, nil
}

type m2SessionRecord struct {
	session   auth.Session
	expiresAt time.Time
	revoked   bool
}

type m2Store struct {
	usersByID    map[auth.Identifier]auth.User
	usersByEmail map[string]auth.Identifier
	versions     map[auth.Identifier]int64
	sessions     map[string]m2SessionRecord
	audits       []auth.AuditEvent
}

func newM2Store() *m2Store {
	return &m2Store{
		usersByID:    make(map[auth.Identifier]auth.User),
		usersByEmail: make(map[string]auth.Identifier),
		versions:     make(map[auth.Identifier]int64),
		sessions:     make(map[string]m2SessionRecord),
	}
}

func (store *m2Store) FindUserByEmail(_ context.Context, email string) (auth.User, error) {
	userID, exists := store.usersByEmail[email]
	if !exists {
		return auth.User{}, auth.ErrUserNotFound
	}
	return store.usersByID[userID], nil
}

func (store *m2Store) CreateUser(_ context.Context, params auth.CreateUserParams) (auth.User, error) {
	user := auth.User{
		ID:          params.ID,
		Email:       params.Identity.Email,
		DisplayName: params.Identity.DisplayName,
		AvatarURL:   params.Identity.AvatarURL,
		Role:        params.Role,
		Active:      true,
	}
	store.usersByID[user.ID] = user
	store.usersByEmail[user.Email] = user.ID
	store.versions[user.ID] = 1
	return user, nil
}

func (store *m2Store) UpdateUserIdentity(_ context.Context, userID auth.Identifier, identity auth.GoogleIdentity) (auth.User, error) {
	user, exists := store.usersByID[userID]
	if !exists {
		return auth.User{}, auth.ErrUserNotFound
	}
	user.Email = identity.Email
	user.DisplayName = identity.DisplayName
	user.AvatarURL = identity.AvatarURL
	store.usersByID[userID] = user
	store.versions[userID]++
	return user, nil
}

func (store *m2Store) CreateSession(_ context.Context, params auth.CreateSessionParams) error {
	user, exists := store.usersByID[params.UserID]
	if !exists {
		return auth.ErrUserNotFound
	}
	store.sessions[string(params.TokenHash)] = m2SessionRecord{
		session:   auth.Session{ID: params.ID, User: user},
		expiresAt: params.ExpiresAt,
	}
	return nil
}

func (store *m2Store) FindAuthenticatedSession(_ context.Context, tokenHash []byte, now time.Time) (auth.Session, error) {
	record, exists := store.sessions[string(tokenHash)]
	if !exists || record.revoked || !record.expiresAt.After(now) {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	user := store.usersByID[record.session.User.ID]
	if !user.Active {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	record.session.User = user
	store.sessions[string(tokenHash)] = record
	return record.session, nil
}

func (store *m2Store) TouchSession(context.Context, auth.Identifier) error { return nil }

func (store *m2Store) RevokeSessionByTokenHash(_ context.Context, tokenHash []byte) error {
	key := string(tokenHash)
	record, exists := store.sessions[key]
	if exists {
		record.revoked = true
		store.sessions[key] = record
	}
	return nil
}

func (store *m2Store) RecordAuditEvent(_ context.Context, event auth.AuditEvent) error {
	store.audits = append(store.audits, event)
	return nil
}

func (store *m2Store) ListUsers(context.Context, int32, int32) ([]auth.ManagedUser, error) {
	users := make([]auth.ManagedUser, 0, len(store.usersByID))
	for id, user := range store.usersByID {
		users = append(users, auth.ManagedUser{User: user, Version: store.versions[id]})
	}
	sort.Slice(users, func(left, right int) bool { return users[left].User.Email < users[right].User.Email })
	return users, nil
}

func (store *m2Store) FindUserByID(_ context.Context, userID auth.Identifier) (auth.ManagedUser, error) {
	user, exists := store.usersByID[userID]
	if !exists {
		return auth.ManagedUser{}, auth.ErrUserNotFound
	}
	return auth.ManagedUser{User: user, Version: store.versions[userID]}, nil
}

func (store *m2Store) UpdateUserAccess(_ context.Context, params auth.UpdateUserAccessParams) (auth.ManagedUser, error) {
	user, exists := store.usersByID[params.UserID]
	if !exists {
		return auth.ManagedUser{}, auth.ErrUserNotFound
	}
	if store.versions[params.UserID] != params.Version {
		return auth.ManagedUser{}, auth.ErrUserAccessConflict
	}
	user.Role = params.Role
	user.Active = params.Active
	store.usersByID[params.UserID] = user
	store.versions[params.UserID]++
	return auth.ManagedUser{User: user, Version: store.versions[params.UserID]}, nil
}

func (store *m2Store) RevokeAllSessionsForUser(_ context.Context, userID auth.Identifier) error {
	for key, record := range store.sessions {
		if record.session.User.ID == userID {
			record.revoked = true
			store.sessions[key] = record
		}
	}
	return nil
}

func TestM2AuthenticationAdministrationAndRevocationFlow(t *testing.T) {
	provider := &m2Provider{identity: auth.GoogleIdentity{Subject: "1", Email: "owner", DisplayName: "Owner"}}
	store := newM2Store()
	service, err := auth.NewService(provider, store, auth.ServiceOptions{
		AllowedEmails:   []string{"owner", "member"},
		SuperadminEmail: "owner",
		Now:             func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	handler := New(authTestLogger(), nil, Options{Auth: service, ApplicationURL: "https://app.example"})

	ownerCookie := m2Login(t, handler)
	provider.identity = auth.GoogleIdentity{Subject: "2", Email: "member", DisplayName: "Member"}
	memberCookie := m2Login(t, handler)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	listRequest.AddCookie(ownerCookie)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var listed adminUsersResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listed); err != nil || len(listed.Users) != 2 {
		t.Fatalf("listed = %#v, error = %v", listed, err)
	}
	var member adminUserResponse
	for _, user := range listed.Users {
		if user.Email == "member" {
			member = user
		}
	}
	if member.ID == "" || member.Role != auth.RoleExternal {
		t.Fatalf("member = %#v", member)
	}

	updateBody := `{"role":"ADMIN","active":true,"version":1}`
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+member.ID+"/access", strings.NewReader(updateBody))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("Origin", "https://app.example")
	updateRequest.AddCookie(ownerCookie)
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	memberSessionRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	memberSessionRequest.AddCookie(memberCookie)
	memberSessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(memberSessionResponse, memberSessionRequest)
	if memberSessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked member session status = %d", memberSessionResponse.Code)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Origin", "https://app.example")
	logoutRequest.AddCookie(ownerCookie)
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResponse.Code)
	}

	ownerSessionRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	ownerSessionRequest.AddCookie(ownerCookie)
	ownerSessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(ownerSessionResponse, ownerSessionRequest)
	if ownerSessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked owner session status = %d", ownerSessionResponse.Code)
	}

	wantEvents := map[auth.AuditEventType]int{
		auth.AuditEventSignInSucceeded:            2,
		auth.AuditEventUserAdministrationAccessed: 1,
		auth.AuditEventUserAccessChanged:          1,
		auth.AuditEventSessionRevoked:             1,
		auth.AuditEventSignOut:                    1,
	}
	for _, event := range store.audits {
		wantEvents[event.EventType]--
	}
	for eventType, remaining := range wantEvents {
		if remaining != 0 {
			t.Fatalf("event %s remaining count = %d; audits = %#v", eventType, remaining, store.audits)
		}
	}
}

func m2Login(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodGet, "/auth/login", nil))
	if loginResponse.Code != http.StatusFound {
		t.Fatalf("login status = %d", loginResponse.Code)
	}
	var stateCookie *http.Cookie
	for _, cookie := range loginResponse.Result().Cookies() {
		if cookie.Name == oauthStateCookieName {
			stateCookie = cookie
		}
	}
	if stateCookie == nil {
		t.Fatal("OAuth state cookie was not issued")
	}

	callbackRequest := httptest.NewRequest(http.MethodGet, "/auth/callback?code=code&state="+stateCookie.Value, nil)
	callbackRequest.AddCookie(stateCookie)
	callbackResponse := httptest.NewRecorder()
	handler.ServeHTTP(callbackResponse, callbackRequest)
	if callbackResponse.Code != http.StatusFound {
		t.Fatalf("callback status = %d, body = %s", callbackResponse.Code, callbackResponse.Body.String())
	}
	for _, cookie := range callbackResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.MaxAge > 0 {
			return cookie
		}
	}
	t.Fatal("application session cookie was not issued")
	return nil
}

var _ auth.Store = (*m2Store)(nil)
var _ auth.UserAdministrationStore = (*m2Store)(nil)

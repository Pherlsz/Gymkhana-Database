package auth

import (
	"context"
	"errors"
	"testing"
)

type fakeAdministrationStore struct {
	fakeStore
	users         []ManagedUser
	target        ManagedUser
	updated       ManagedUser
	updateParams  UpdateUserAccessParams
	revokedUserID Identifier
	updateErr     error
}

func (store *fakeAdministrationStore) ListUsers(context.Context, int32, int32) ([]ManagedUser, error) {
	return store.users, nil
}

func (store *fakeAdministrationStore) FindUserByID(context.Context, Identifier) (ManagedUser, error) {
	if store.target.User.ID == (Identifier{}) {
		return ManagedUser{}, ErrUserNotFound
	}
	return store.target, nil
}

func (store *fakeAdministrationStore) UpdateUserAccess(_ context.Context, params UpdateUserAccessParams) (ManagedUser, error) {
	store.updateParams = params
	if store.updateErr != nil {
		return ManagedUser{}, store.updateErr
	}
	return store.updated, nil
}

func (store *fakeAdministrationStore) RevokeAllSessionsForUser(_ context.Context, userID Identifier) error {
	store.revokedUserID = userID
	return nil
}

func newAdministrationService(t *testing.T, store Store) *Service {
	t.Helper()
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		AllowedLogins:   []string{"owner"},
		SuperadminLogin: "owner",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestAdministrationRequiresAdministrativeRole(t *testing.T) {
	store := &fakeAdministrationStore{}
	service := newAdministrationService(t, store)
	actor := Session{User: User{Role: RoleMember, Active: true}}

	if _, err := service.ListUsers(context.Background(), actor, 100, 0); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ListUsers() error = %v, want %v", err, ErrForbidden)
	}
}

func TestAdministrationRejectsSelfAccessChanges(t *testing.T) {
	actorID, _ := NewIdentifier()
	store := &fakeAdministrationStore{}
	service := newAdministrationService(t, store)
	actor := Session{User: User{ID: actorID, Role: RoleAdmin, Active: true}}

	_, err := service.UpdateUserAccess(context.Background(), actor, UpdateUserAccessParams{
		UserID:  actorID,
		Role:    RoleMember,
		Active:  true,
		Version: 1,
	}, "request-self")
	if !errors.Is(err, ErrSelfAccessChange) {
		t.Fatalf("UpdateUserAccess() error = %v, want %v", err, ErrSelfAccessChange)
	}
}

func TestAdministrationProtectsSuperadminAndRejectsPromotion(t *testing.T) {
	actorID, _ := NewIdentifier()
	targetID, _ := NewIdentifier()
	service := newAdministrationService(t, &fakeAdministrationStore{target: ManagedUser{
		User:    User{ID: targetID, Role: RoleSuperadmin, Active: true},
		Version: 1,
	}})
	actor := Session{User: User{ID: actorID, Role: RoleSuperadmin, Active: true}}

	_, err := service.UpdateUserAccess(context.Background(), actor, UpdateUserAccessParams{
		UserID:  targetID,
		Role:    RoleAdmin,
		Active:  true,
		Version: 1,
	}, "request-protected")
	if !errors.Is(err, ErrProtectedSuperadmin) {
		t.Fatalf("protected superadmin error = %v, want %v", err, ErrProtectedSuperadmin)
	}

	memberID, _ := NewIdentifier()
	_, err = service.UpdateUserAccess(context.Background(), actor, UpdateUserAccessParams{
		UserID:  memberID,
		Role:    RoleSuperadmin,
		Active:  true,
		Version: 1,
	}, "request-promote")
	if !errors.Is(err, ErrInvalidUserAccess) {
		t.Fatalf("superadmin promotion error = %v, want %v", err, ErrInvalidUserAccess)
	}
}

func TestNoOpAccessChangeDoesNotRevokeSessions(t *testing.T) {
	actorID, _ := NewIdentifier()
	targetID, _ := NewIdentifier()
	store := &fakeAdministrationStore{target: ManagedUser{
		User:    User{ID: targetID, Login: "member", Role: RoleMember, Active: true},
		Version: 3,
	}}
	service := newAdministrationService(t, store)
	actor := Session{User: User{ID: actorID, Role: RoleAdmin, Active: true}}

	updated, err := service.UpdateUserAccess(context.Background(), actor, UpdateUserAccessParams{
		UserID:  targetID,
		Role:    RoleMember,
		Active:  true,
		Version: 3,
	}, "request-noop")
	if err != nil || updated.Version != 3 {
		t.Fatalf("UpdateUserAccess() = %#v, %v", updated, err)
	}
	if store.revokedUserID != (Identifier{}) || store.updateParams.UserID != (Identifier{}) {
		t.Fatalf("unexpected write or revocation: %#v, %v", store.updateParams, store.revokedUserID)
	}
}

func TestAccessChangeRevokesSessionsAndRecordsAudit(t *testing.T) {
	actorID, _ := NewIdentifier()
	targetID, _ := NewIdentifier()
	store := &fakeAdministrationStore{
		target:  ManagedUser{User: User{ID: targetID, Login: "member", Role: RoleMember, Active: true}, Version: 3},
		updated: ManagedUser{User: User{ID: targetID, Login: "member", Role: RoleAdmin, Active: true}, Version: 4},
	}
	service := newAdministrationService(t, store)
	actor := Session{User: User{ID: actorID, Role: RoleSuperadmin, Active: true}}

	updated, err := service.UpdateUserAccess(context.Background(), actor, UpdateUserAccessParams{
		UserID:  targetID,
		Role:    RoleAdmin,
		Active:  true,
		Version: 3,
	}, "request-change")
	if err != nil {
		t.Fatalf("UpdateUserAccess() error = %v", err)
	}
	if updated.Version != 4 || store.revokedUserID != targetID {
		t.Fatalf("updated = %#v, revoked = %v", updated, store.revokedUserID)
	}
	if len(store.audits) != 2 || store.audits[0].EventType != AuditEventUserAccessChanged || store.audits[1].EventType != AuditEventSessionRevoked {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestIdentifierRoundTrip(t *testing.T) {
	identifier, _ := NewIdentifier()
	parsed, err := ParseIdentifier(identifier.String())
	if err != nil || parsed != identifier {
		t.Fatalf("ParseIdentifier() = %v, %v", parsed, err)
	}
}

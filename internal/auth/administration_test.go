package auth

import (
	"context"
	"errors"
	"testing"
)

type fakeAdministrationStore struct {
	fakeStore
	users        []ManagedUser
	target       ManagedUser
	findError    error
	update       UserAccessUpdate
	updateResult ManagedUser
	updateError  error
	revokedUser  Identifier
}

func (store *fakeAdministrationStore) FindManagedUser(context.Context, Identifier) (ManagedUser, error) {
	return store.target, store.findError
}

func (store *fakeAdministrationStore) ListManagedUsers(context.Context, int32, int32) ([]ManagedUser, error) {
	return store.users, nil
}

func (store *fakeAdministrationStore) UpdateManagedUserAccess(_ context.Context, update UserAccessUpdate) (ManagedUser, error) {
	store.update = update
	return store.updateResult, store.updateError
}

func (store *fakeAdministrationStore) RevokeAllSessionsForUser(_ context.Context, userID Identifier) error {
	store.revokedUser = userID
	return nil
}

func TestRolePermissionsAreCentralized(t *testing.T) {
	if RoleMember.Allows(PermissionManageUsers) {
		t.Fatal("member can manage users")
	}
	if !RoleAdmin.Allows(PermissionManageUsers) || !RoleSuperadmin.Allows(PermissionManageUsers) {
		t.Fatal("administrative roles cannot manage users")
	}
	if RoleAdmin.Allows(Permission("unknown")) {
		t.Fatal("unknown permission was allowed")
	}
}

func TestServiceListsUsersOnlyForAdministrativeRoles(t *testing.T) {
	service, err := NewService(fakeProvider{}, &fakeAdministrationStore{
		users: []ManagedUser{{Login: "member", Role: RoleMember, Active: true, Version: 1}},
	}, ServiceOptions{AllowedLogins: []string{"admin"}, SuperadminLogin: "admin"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := service.ListUsers(context.Background(), User{Role: RoleMember}, 100, 0, "request-list"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member ListUsers() error = %v", err)
	}
	users, err := service.ListUsers(context.Background(), User{Role: RoleAdmin}, 100, 0, "request-list")
	if err != nil || len(users) != 1 {
		t.Fatalf("admin ListUsers() = %#v, %v", users, err)
	}
}

func TestServiceUpdatesAccessAndRevokesSessions(t *testing.T) {
	actorID, _ := NewIdentifier()
	targetID, _ := NewIdentifier()
	store := &fakeAdministrationStore{
		target:       ManagedUser{ID: targetID, Login: "member", Role: RoleMember, Active: true, Version: 3},
		updateResult: ManagedUser{ID: targetID, Login: "member", Role: RoleAdmin, Active: true, Version: 4},
	}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		AllowedLogins: []string{"admin"}, SuperadminLogin: "admin",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	updated, err := service.UpdateUserAccess(context.Background(), User{ID: actorID, Role: RoleAdmin}, UserAccessUpdate{
		UserID:  targetID,
		Role:    RoleAdmin,
		Active:  true,
		Version: 3,
	}, "request-admin")
	if err != nil {
		t.Fatalf("UpdateUserAccess() error = %v", err)
	}
	if updated.Role != RoleAdmin || store.update.Version != 3 || store.revokedUser != targetID {
		t.Fatalf("updated = %#v, store = %#v", updated, store)
	}
	if len(store.audits) != 2 ||
		store.audits[0].EventType != AuditEventUserAccessChanged ||
		store.audits[1].EventType != AuditEventSessionRevoked {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestServiceProtectsSelfAndSuperadmin(t *testing.T) {
	actorID, _ := NewIdentifier()
	otherID, _ := NewIdentifier()
	service, err := NewService(fakeProvider{}, &fakeAdministrationStore{
		target: ManagedUser{ID: otherID, Role: RoleSuperadmin, Active: true, Version: 1},
	}, ServiceOptions{AllowedLogins: []string{"admin"}, SuperadminLogin: "admin"})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := service.UpdateUserAccess(context.Background(), User{ID: actorID, Role: RoleSuperadmin}, UserAccessUpdate{
		UserID: actorID, Role: RoleAdmin, Active: true, Version: 1,
	}, "request-self"); !errors.Is(err, ErrSelfAccessChange) {
		t.Fatalf("self update error = %v", err)
	}
	if _, err := service.UpdateUserAccess(context.Background(), User{ID: actorID, Role: RoleSuperadmin}, UserAccessUpdate{
		UserID: otherID, Role: RoleAdmin, Active: true, Version: 1,
	}, "request-superadmin"); !errors.Is(err, ErrProtectedSuperadmin) {
		t.Fatalf("superadmin update error = %v", err)
	}
	if _, err := service.UpdateUserAccess(context.Background(), User{ID: actorID, Role: RoleAdmin}, UserAccessUpdate{
		UserID: otherID, Role: RoleSuperadmin, Active: true, Version: 1,
	}, "request-role"); !errors.Is(err, ErrInvalidManagedRole) {
		t.Fatalf("invalid role error = %v", err)
	}
}

func TestServicePreservesOptimisticConflict(t *testing.T) {
	actorID, _ := NewIdentifier()
	targetID, _ := NewIdentifier()
	store := &fakeAdministrationStore{
		target:      ManagedUser{ID: targetID, Role: RoleMember, Active: true, Version: 2},
		updateError: ErrUserAccessConflict,
	}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		AllowedLogins: []string{"admin"}, SuperadminLogin: "admin",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = service.UpdateUserAccess(context.Background(), User{ID: actorID, Role: RoleAdmin}, UserAccessUpdate{
		UserID: targetID, Role: RoleAdmin, Active: true, Version: 1,
	}, "request-conflict")
	if !errors.Is(err, ErrUserAccessConflict) {
		t.Fatalf("UpdateUserAccess() error = %v", err)
	}
}

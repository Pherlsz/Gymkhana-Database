package auth

import (
	"context"
	"testing"
)

type provisionStore struct {
	fakeStore
	params ProvisionUserParams
	called bool
}

func (store *provisionStore) ProvisionUser(_ context.Context, params ProvisionUserParams) (ManagedUser, error) {
	store.called = true
	store.params = params
	return ManagedUser{User: User{
		ID:          params.ID,
		Email:       params.Email,
		DisplayName: params.DisplayName,
		Role:        params.Role,
		Active:      true,
	}, Version: 1}, nil
}

func TestProvisionUserStoresMemberAccess(t *testing.T) {
	store := &provisionStore{}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		AllowedEmails:   []string{"admin@example.com"},
		SuperadminEmail: "admin@example.com",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	actorID, _ := NewIdentifier()
	created, err := service.ProvisionUser(context.Background(), Session{User: User{
		ID: actorID, Email: "admin@example.com", Role: RoleAdmin, Active: true,
	}}, ProvisionUserParams{
		Email:        " Maria@Example.com ",
		DisplayName:  " Maria ",
		Role:         RoleExternal,
		Capabilities: []Capability{CapSearch, CapSearch, CapProfiles},
	}, "request-provision")
	if err != nil {
		t.Fatalf("ProvisionUser() error = %v", err)
	}
	if created.User.Email != "maria@example.com" || created.User.DisplayName != "Maria" {
		t.Fatalf("created = %#v", created.User)
	}
	if !store.called || store.params.Role != RoleExternal || len(store.params.Capabilities) != 2 ||
		store.params.Capabilities[0] != CapSearch || store.params.Capabilities[1] != CapProfiles {
		t.Fatalf("stored = %#v", store.params)
	}
	if store.params.ActorID != actorID || store.params.ID == (Identifier{}) {
		t.Fatalf("ids = %#v", store.params)
	}
}

func TestProvisionUserDropsAdminCapabilities(t *testing.T) {
	store := &provisionStore{}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		AllowedEmails:   []string{"admin@example.com"},
		SuperadminEmail: "admin@example.com",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.ProvisionUser(context.Background(), Session{User: User{
		Email: "admin@example.com", Role: RoleSuperadmin, Active: true,
	}}, ProvisionUserParams{
		Email:        "boss@example.com",
		DisplayName:  "Boss",
		Role:         RoleAdmin,
		Capabilities: []Capability{CapSearch},
	}, "request-provision")
	if err != nil {
		t.Fatalf("ProvisionUser() error = %v", err)
	}
	if len(store.params.Capabilities) != 0 {
		t.Fatalf("capabilities = %#v", store.params.Capabilities)
	}
}

func TestProvisionUserRejectsIncompleteAccess(t *testing.T) {
	store := &provisionStore{}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		AllowedEmails:   []string{"admin@example.com"},
		SuperadminEmail: "admin@example.com",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	actor := Session{User: User{Email: "admin@example.com", Role: RoleAdmin, Active: true}}
	_, err = service.ProvisionUser(context.Background(), actor, ProvisionUserParams{
		Email: "member@example.com", DisplayName: " ", Role: RoleExternal, Capabilities: []Capability{CapSearch},
	}, "request-provision")
	if err != nil || !store.called || store.params.DisplayName != "member@example.com" {
		t.Fatalf("blank name error = %v, called = %v, params = %#v", err, store.called, store.params)
	}
	store.called = false
	_, err = service.ProvisionUser(context.Background(), actor, ProvisionUserParams{
		Email: "member@example.com", DisplayName: "Member", Role: RoleSuperadmin,
	}, "request-provision")
	if err != ErrInvalidUserAccess {
		t.Fatalf("superadmin role error = %v", err)
	}
	_, err = service.ProvisionUser(context.Background(), actor, ProvisionUserParams{
		Email: "member2@example.com", DisplayName: "Member", Role: RoleExternal,
	}, "request-provision")
	if err != ErrMemberNeedsCapability || store.called {
		t.Fatalf("missing capability error = %v, called = %v", err, store.called)
	}
	_, err = service.ProvisionUser(context.Background(), actor, ProvisionUserParams{
		Email: "member@example.com", DisplayName: "Member", Role: RoleExternal, Capabilities: []Capability{"NOPE"},
	}, "request-provision")
	if err != ErrCapabilityConflict {
		t.Fatalf("capability error = %v", err)
	}
	_, err = service.ProvisionUser(context.Background(), Session{User: User{
		Email: "member@example.com", Role: RoleExternal, Active: true,
	}}, ProvisionUserParams{
		Email: "other@example.com", DisplayName: "Other", Role: RoleExternal,
	}, "request-provision")
	if err != ErrForbidden {
		t.Fatalf("member error = %v", err)
	}
}

func TestDeleteUserProtectsSuperadmin(t *testing.T) {
	targetID, _ := NewIdentifier()
	store := &deleteStore{user: ManagedUser{User: User{
		ID: targetID, Email: "owner@example.com", Role: RoleSuperadmin, Active: true,
	}}}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{
		AllowedEmails:   []string{"admin@example.com"},
		SuperadminEmail: "admin@example.com",
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	err = service.DeleteUser(context.Background(), Session{User: User{
		Email: "admin@example.com", Role: RoleAdmin, Active: true,
	}}, targetID, "request-delete")
	if err != ErrProtectedSuperadmin || store.deleted {
		t.Fatalf("error = %v, deleted = %v", err, store.deleted)
	}
}

type deleteStore struct {
	fakeStore
	user    ManagedUser
	deleted bool
}

func (store *deleteStore) FindUserByID(context.Context, Identifier) (ManagedUser, error) {
	return store.user, nil
}

func (store *deleteStore) DeleteUser(context.Context, Identifier, string) error {
	store.deleted = true
	return nil
}

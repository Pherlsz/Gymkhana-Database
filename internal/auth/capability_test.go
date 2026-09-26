package auth

import (
	"context"
	"errors"
	"testing"
)

type capabilityStore struct {
	fakeAdministrationStore
	caps    []Capability
	revoked Capability
}

func (store *capabilityStore) ListCapabilities(context.Context, Identifier) ([]Capability, error) {
	return store.caps, nil
}

func (store *capabilityStore) GrantCapability(context.Context, Identifier, Capability) error {
	return nil
}

func (store *capabilityStore) RevokeCapability(_ context.Context, _ Identifier, cap Capability) error {
	store.revoked = cap
	return nil
}

func TestRevokeCapabilityKeepsAtLeastOneForMember(t *testing.T) {
	userID, _ := NewIdentifier()
	store := &capabilityStore{
		fakeAdministrationStore: fakeAdministrationStore{
			target: ManagedUser{User: User{ID: userID, Email: "member@example.com", Role: RoleExternal, Active: true}},
		},
		caps: []Capability{CapSearch},
	}
	service, err := NewService(fakeProvider{}, store, ServiceOptions{AllowlistStore: store})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	actor := Session{User: User{Email: "admin@example.com", Role: RoleAdmin, Active: true}}
	err = service.RevokeCapability(context.Background(), actor, CapabilityGrant{UserID: userID, Capability: CapSearch}, "revoke-last")
	if !errors.Is(err, ErrMemberNeedsCapability) || store.revoked != "" {
		t.Fatalf("error = %v, revoked = %q", err, store.revoked)
	}

	store.caps = []Capability{CapSearch, CapProfiles}
	if err := service.RevokeCapability(context.Background(), actor, CapabilityGrant{UserID: userID, Capability: CapSearch}, "revoke-one"); err != nil {
		t.Fatalf("RevokeCapability() error = %v", err)
	}
	if store.revoked != CapSearch {
		t.Fatalf("revoked = %q", store.revoked)
	}
}

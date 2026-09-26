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
	listErr       error
	updateErr     error
}

func (store *fakeAdministrationStore) ListUsers(context.Context, int32, int32) ([]ManagedUser, error) {
	return store.users, store.listErr
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

func newAdministrationService(t *testing.T, store *fakeAdministrationStore) *Service {
	t.Helper()
	service, err := NewService(fakeProvider{}, store, ServiceOptions{AllowlistStore: store})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestAdministrationAuditsListAccess(t *testing.T) {
	actorID, _ := NewIdentifier()
	tests := []struct {
		name        string
		actor       Session
		store       *fakeAdministrationStore
		wantErr     error
		wantOutcome AuditOutcome
	}{
		{
			name:        "denied",
			actor:       Session{User: User{ID: actorID, Email: "member@example.com", Role: RoleExternal, Active: true}},
			store:       &fakeAdministrationStore{},
			wantErr:     ErrForbidden,
			wantOutcome: AuditOutcomeDenied,
		},
		{
			name:        "success",
			actor:       Session{User: User{ID: actorID, Email: "owner@example.com", Role: RoleAdmin, Active: true}},
			store:       &fakeAdministrationStore{users: []ManagedUser{{User: User{Email: "member@example.com"}}}},
			wantOutcome: AuditOutcomeSuccess,
		},
		{
			name:        "failure",
			actor:       Session{User: User{ID: actorID, Email: "owner@example.com", Role: RoleAdmin, Active: true}},
			store:       &fakeAdministrationStore{listErr: errors.New("database unavailable")},
			wantOutcome: AuditOutcomeFailure,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newAdministrationService(t, test.store)
			_, err := service.ListUsers(context.Background(), test.actor, 100, 0, "request-list")
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("ListUsers() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && test.name == "success" && err != nil {
				t.Fatalf("ListUsers() error = %v", err)
			}
			if len(test.store.audits) != 1 {
				t.Fatalf("audits = %#v", test.store.audits)
			}
			audit := test.store.audits[0]
			if audit.EventType != AuditEventUserAdministrationAccessed || audit.Outcome != test.wantOutcome || audit.RequestID != "request-list" {
				t.Fatalf("audit = %#v", audit)
			}
		})
	}
}

func TestAdministrationRejectsSelfAccessChanges(t *testing.T) {
	actorID, _ := NewIdentifier()
	store := &fakeAdministrationStore{}
	service := newAdministrationService(t, store)
	actor := Session{User: User{ID: actorID, Email: "admin", Role: RoleAdmin, Active: true}}

	_, err := service.UpdateUserAccess(context.Background(), actor, UpdateUserAccessParams{
		UserID:  actorID,
		Role:    RoleExternal,
		Active:  true,
		Version: 1,
	}, "request-self")
	if !errors.Is(err, ErrSelfAccessChange) {
		t.Fatalf("UpdateUserAccess() error = %v, want %v", err, ErrSelfAccessChange)
	}
	if len(store.audits) != 1 || store.audits[0].Outcome != AuditOutcomeDenied {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestAdministrationProtectsSuperadminAndRejectsPromotion(t *testing.T) {
	actorID, _ := NewIdentifier()
	targetID, _ := NewIdentifier()
	service := newAdministrationService(t, &fakeAdministrationStore{target: ManagedUser{
		User:    User{ID: targetID, Role: RoleSuperadmin, Active: true},
		Version: 1,
	}})
	actor := Session{User: User{ID: actorID, Email: "owner", Role: RoleSuperadmin, Active: true}}

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
		User:    User{ID: targetID, Email: "member", Role: RoleExternal, Active: true},
		Version: 3,
	}}
	service := newAdministrationService(t, store)
	actor := Session{User: User{ID: actorID, Email: "admin", Role: RoleAdmin, Active: true}}

	updated, err := service.UpdateUserAccess(context.Background(), actor, UpdateUserAccessParams{
		UserID:  targetID,
		Role:    RoleExternal,
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

func TestAccessChangeFailureIsAudited(t *testing.T) {
	actorID, _ := NewIdentifier()
	targetID, _ := NewIdentifier()
	store := &fakeAdministrationStore{
		target:    ManagedUser{User: User{ID: targetID, Email: "member", Role: RoleExternal, Active: true}, Version: 3},
		updateErr: ErrUserAccessConflict,
	}
	service := newAdministrationService(t, store)
	actor := Session{User: User{ID: actorID, Email: "owner", Role: RoleSuperadmin, Active: true}}

	_, err := service.UpdateUserAccess(context.Background(), actor, UpdateUserAccessParams{
		UserID: targetID, Role: RoleAdmin, Active: true, Version: 3,
	}, "request-conflict")
	if !errors.Is(err, ErrUserAccessConflict) {
		t.Fatalf("UpdateUserAccess() error = %v", err)
	}
	if len(store.audits) != 1 || store.audits[0].Outcome != AuditOutcomeFailure || store.audits[0].RequestID != "request-conflict" {
		t.Fatalf("audits = %#v", store.audits)
	}
}

func TestAccessChangeRevokesSessionsAndRecordsAudit(t *testing.T) {
	actorID, _ := NewIdentifier()
	targetID, _ := NewIdentifier()
	store := &fakeAdministrationStore{
		target:  ManagedUser{User: User{ID: targetID, Email: "member", Role: RoleExternal, Active: true}, Version: 3},
		updated: ManagedUser{User: User{ID: targetID, Email: "member", Role: RoleAdmin, Active: true}, Version: 4},
	}
	service := newAdministrationService(t, store)
	actor := Session{User: User{ID: actorID, Email: "owner", Role: RoleSuperadmin, Active: true}}

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

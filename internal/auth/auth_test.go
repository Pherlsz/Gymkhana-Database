package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestRolesExposeStablePermissions(t *testing.T) {
	for _, role := range []Role{RoleMember, RoleAdmin, RoleSuperadmin} {
		if !role.Valid() {
			t.Fatalf("role %q is not valid", role)
		}
	}
	if Role("OWNER").Valid() {
		t.Fatal("unexpected role is valid")
	}
	if RoleMember.CanManageUsers() {
		t.Fatal("member can manage users")
	}
	if !RoleAdmin.CanManageUsers() || !RoleSuperadmin.CanManageUsers() {
		t.Fatal("administrative role cannot manage users")
	}
	if RoleMember.CanManageGoogleForms() || !RoleAdmin.CanManageGoogleForms() || !RoleSuperadmin.CanManageGoogleForms() {
		t.Fatal("Google Forms administration permissions are not role-scoped")
	}
	for _, role := range []Role{RoleMember, RoleAdmin, RoleSuperadmin} {
		if !role.CanReadCustomData() || !role.CanReadAttachments() || !role.CanSearch() || !role.CanUseOperations() {
			t.Fatalf("role %q cannot use authorized Search or Operations modules", role)
		}
	}
	if Role("UNKNOWN").CanSearch() || Role("UNKNOWN").CanUseOperations() || Role("UNKNOWN").CanManageGoogleForms() || Role("UNKNOWN").CanReadCustomData() || Role("UNKNOWN").CanReadAttachments() {
		t.Fatal("unknown role received read, Search, or Operations permissions")
	}
}

func TestAuditValuesAreExplicit(t *testing.T) {
	for _, eventType := range []AuditEventType{
		AuditEventSignInSucceeded,
		AuditEventSignInDenied,
		AuditEventSignInFailed,
		AuditEventSignOut,
		AuditEventSessionRevoked,
		AuditEventUserAccessChanged,
	} {
		if !eventType.Valid() {
			t.Fatalf("event type %q is not valid", eventType)
		}
	}
	for _, outcome := range []AuditOutcome{AuditOutcomeSuccess, AuditOutcomeDenied, AuditOutcomeFailure} {
		if !outcome.Valid() {
			t.Fatalf("outcome %q is not valid", outcome)
		}
	}
}

func TestOpaqueSessionValueUsesStrongURLSafeEntropy(t *testing.T) {
	first, err := NewOpaqueSessionValue()
	if err != nil {
		t.Fatalf("NewOpaqueSessionValue() error = %v", err)
	}
	second, err := NewOpaqueSessionValue()
	if err != nil {
		t.Fatalf("NewOpaqueSessionValue() error = %v", err)
	}
	if first == second {
		t.Fatal("generated session values are equal")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil {
		t.Fatalf("decode generated value: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded entropy length = %d, want 32", len(decoded))
	}
}

func TestHashOpaqueSessionValue(t *testing.T) {
	got, err := HashOpaqueSessionValue("opaque-value")
	if err != nil {
		t.Fatalf("HashOpaqueSessionValue() error = %v", err)
	}
	want := sha256.Sum256([]byte("opaque-value"))
	if got != want {
		t.Fatalf("hash = %x, want %x", got, want)
	}
	if _, err := HashOpaqueSessionValue(""); !errors.Is(err, ErrEmptySessionValue) {
		t.Fatalf("empty value error = %v, want %v", err, ErrEmptySessionValue)
	}
}

func TestSessionExpiresAfterTwentyFourHours(t *testing.T) {
	issuedAt := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)
	if got, want := SessionExpiresAt(issuedAt), issuedAt.Add(24*time.Hour); !got.Equal(want) {
		t.Fatalf("SessionExpiresAt() = %s, want %s", got, want)
	}
}

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"
)

const SessionTTL = 24 * time.Hour

var ErrEmptySessionValue = errors.New("session value cannot be empty")

type Role string

const (
	RoleMember     Role = "MEMBER"
	RoleAdmin      Role = "ADMIN"
	RoleSuperadmin Role = "SUPERADMIN"
)

func (role Role) Valid() bool {
	switch role {
	case RoleMember, RoleAdmin, RoleSuperadmin:
		return true
	default:
		return false
	}
}

func (role Role) CanManageUsers() bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

func (role Role) CanReadProfiles() bool {
	return role.Valid()
}

func (role Role) CanWriteProfiles() bool {
	return role.Valid()
}

func (role Role) CanDeleteProfiles() bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

func (role Role) CanReadDocuments() bool {
	return role.Valid()
}

func (role Role) CanWriteDocuments() bool {
	return role.Valid()
}

func (role Role) CanDeleteDocuments() bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

func (role Role) CanManageDocumentTypes() bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

func (role Role) CanManageDocumentCurrentUse() bool {
	return role.Valid()
}

func (role Role) CanReadBills() bool {
	return role.Valid()
}

func (role Role) CanWriteBills() bool {
	return role.Valid()
}

func (role Role) CanDeleteBills() bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

func (role Role) CanManageBillTypes() bool {
	return role == RoleAdmin || role == RoleSuperadmin
}

func (role Role) CanManageBillCurrentUse() bool {
	return role.Valid()
}

type AuditEventType string

const (
	AuditEventSignInSucceeded            AuditEventType = "SIGN_IN_SUCCEEDED"
	AuditEventSignInDenied               AuditEventType = "SIGN_IN_DENIED"
	AuditEventSignInFailed               AuditEventType = "SIGN_IN_FAILED"
	AuditEventSignOut                    AuditEventType = "SIGN_OUT"
	AuditEventSessionRevoked             AuditEventType = "SESSION_REVOKED"
	AuditEventUserAdministrationAccessed AuditEventType = "USER_ADMINISTRATION_ACCESSED"
	AuditEventUserAccessChanged          AuditEventType = "USER_ACCESS_CHANGED"
)

func (eventType AuditEventType) Valid() bool {
	switch eventType {
	case AuditEventSignInSucceeded,
		AuditEventSignInDenied,
		AuditEventSignInFailed,
		AuditEventSignOut,
		AuditEventSessionRevoked,
		AuditEventUserAdministrationAccessed,
		AuditEventUserAccessChanged:
		return true
	default:
		return false
	}
}

type AuditOutcome string

const (
	AuditOutcomeSuccess AuditOutcome = "SUCCESS"
	AuditOutcomeDenied  AuditOutcome = "DENIED"
	AuditOutcomeFailure AuditOutcome = "FAILURE"
)

func (outcome AuditOutcome) Valid() bool {
	switch outcome {
	case AuditOutcomeSuccess, AuditOutcomeDenied, AuditOutcomeFailure:
		return true
	default:
		return false
	}
}

func NewOpaqueSessionValue() (string, error) {
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(entropy[:]), nil
}

func HashOpaqueSessionValue(value string) ([sha256.Size]byte, error) {
	if value == "" {
		return [sha256.Size]byte{}, ErrEmptySessionValue
	}
	return sha256.Sum256([]byte(value)), nil
}

func SessionExpiresAt(issuedAt time.Time) time.Time {
	return issuedAt.Add(SessionTTL)
}

package auth

import "strings"

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
}

type User struct {
	ID            Identifier
	GoogleSubject string
	Email         string
	DisplayName   string
	AvatarURL     string
	Role          Role
	Active        bool
}

func normalizeIdentity(identity GoogleIdentity) GoogleIdentity {
	identity.Subject = strings.TrimSpace(identity.Subject)
	identity.Email = normalizeEmail(identity.Email)
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	identity.AvatarURL = strings.TrimSpace(identity.AvatarURL)
	if identity.DisplayName == "" {
		identity.DisplayName = identity.Email
	}
	return identity
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

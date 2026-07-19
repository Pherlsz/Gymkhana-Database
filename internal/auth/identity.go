package auth

import "strings"

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string

	UserID int64
	Login  string
}

type GitHubIdentity = GoogleIdentity

type User struct {
	ID            Identifier
	GoogleSubject string
	Email         string
	Login         string
	DisplayName   string
	AvatarURL     string
	Role          Role
	Active        bool
	GitHubUserID  int64
}

func normalizeIdentity(identity GoogleIdentity) GoogleIdentity {
	identity.Subject = strings.TrimSpace(identity.Subject)
	identity.Email = normalizeLogin(identity.Email)
	identity.Login = normalizeLogin(identity.Login)
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	identity.AvatarURL = strings.TrimSpace(identity.AvatarURL)
	if identity.Email != "" {
		identity.Login = identity.Email
	}
	if identity.DisplayName == "" {
		identity.DisplayName = identity.Login
	}
	return identity
}

func normalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

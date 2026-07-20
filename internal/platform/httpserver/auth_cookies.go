package httpserver

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	oauthStateCookieName = "gymkhana_oauth_state"
	sessionCookieName    = "gymkhana_session"
	oauthStateTTL        = 10 * time.Minute
)

func setOAuthStateCookie(w http.ResponseWriter, value string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookieName, Value: value, Path: "/auth/callback",
		MaxAge: int(oauthStateTTL.Seconds()), HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookieName, Path: "/auth/callback", MaxAge: -1,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func setSessionCookie(w http.ResponseWriter, value string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: value, Path: "/", Expires: expiresAt,
		MaxAge: int(auth.SessionTTL.Seconds()), HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func equalSecretValues(left, right string) bool {
	if left == "" || len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

package httpserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	oauthStateCookieName = "gymkhana_oauth_state"
	sessionCookieName    = "gymkhana_session"
	oauthStateTTL        = 10 * time.Minute
)

type authenticationService interface {
	BeginLogin() (string, string, error)
	CompleteLogin(context.Context, string, string) (auth.LoginResult, error)
	CurrentSession(context.Context, string) (auth.Session, error)
	SignOut(context.Context, string, string) error
}

type developmentAuthenticationService interface {
	DevelopmentLogin(context.Context, string) (auth.LoginResult, error)
}

type authSessionResponse struct {
	Authenticated bool              `json:"authenticated"`
	User          *authUserResponse `json:"user,omitempty"`
}

type authUserResponse struct {
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Role        auth.Role `json:"role"`
}

func registerAuthRoutes(
	mux *http.ServeMux,
	logger *slog.Logger,
	service authenticationService,
	development bool,
	secureCookies bool,
	applicationURL string,
) {
	mux.HandleFunc("GET /auth/login", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Authentication is not configured"})
			return
		}
		state, authorizationURL, err := service.BeginLogin()
		if err != nil {
			logger.Error("begin authentication", "request_id", requestIDFromContext(r.Context()), "error", err)
			writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Authentication could not be started"})
			return
		}
		setOAuthStateCookie(w, state, secureCookies)
		http.Redirect(w, r, authorizationURL, http.StatusFound)
	})

	mux.HandleFunc("GET /auth/callback", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Authentication is not configured"})
			return
		}
		stateCookie, err := r.Cookie(oauthStateCookieName)
		state := r.URL.Query().Get("state")
		clearOAuthStateCookie(w, secureCookies)
		if err != nil || !equalSecretValues(stateCookie.Value, state) {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeInvalidOAuthState, Message: "Authentication state is invalid or expired"})
			return
		}

		result, err := service.CompleteLogin(r.Context(), r.URL.Query().Get("code"), requestIDFromContext(r.Context()))
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrAccessDenied):
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "This Google account is not allowed"})
			case errors.Is(err, auth.ErrInvalidOAuthCode):
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Authentication code is missing"})
			default:
				logger.Error("complete authentication", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusBadGateway, Code: ErrorCodeAuthProvider, Message: "Google authentication failed"})
			}
			return
		}

		setSessionCookie(w, result.SessionValue, result.ExpiresAt, secureCookies)
		http.Redirect(w, r, applicationURL, http.StatusFound)
	})

	// The bypass is registered only when the application was composed for a
	// local/test environment. Cookie transport settings are deliberately not
	// used as an environment proxy.
	if development {
		completeDevelopmentLogin := func(w http.ResponseWriter, r *http.Request) bool {
			if service == nil {
				writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Authentication is not configured"})
				return false
			}
			developmentService, ok := service.(developmentAuthenticationService)
			if !ok {
				writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Development authentication is unavailable"})
				return false
			}
			result, err := developmentService.DevelopmentLogin(r.Context(), requestIDFromContext(r.Context()))
			if err != nil {
				if errors.Is(err, auth.ErrAccessDenied) {
					writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Development account is not allowed"})
					return false
				}
				logger.Error("complete development authentication", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Development authentication failed"})
				return false
			}
			setSessionCookie(w, result.SessionValue, result.ExpiresAt, false)
			return true
		}
		mux.HandleFunc("POST /api/auth/dev-login", func(w http.ResponseWriter, r *http.Request) {
			if completeDevelopmentLogin(w, r) {
				w.WriteHeader(http.StatusNoContent)
			}
		})
		mux.HandleFunc("GET /api/auth/dev-login", func(w http.ResponseWriter, r *http.Request) {
			if completeDevelopmentLogin(w, r) {
				http.Redirect(w, r, "/", http.StatusFound)
			}
		})
	}

	mux.HandleFunc("GET /api/auth/session", func(w http.ResponseWriter, r *http.Request) {
		session, problem := authenticatedSession(r, service)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		writeJSON(w, http.StatusOK, authSessionResponse{
			Authenticated: true,
			User:          authUser(session.User),
		})
	})

	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Authentication is not configured"})
			return
		}
		cookie, _ := r.Cookie(sessionCookieName)
		if cookie != nil {
			if err := service.SignOut(r.Context(), cookie.Value, requestIDFromContext(r.Context())); err != nil {
				logger.Error("sign out", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Sign out failed"})
				return
			}
		}
		clearSessionCookie(w, secureCookies)
		w.WriteHeader(http.StatusNoContent)
	})
}

func authenticatedSession(r *http.Request, service authenticationService) (auth.Session, *Problem) {
	if service == nil {
		return auth.Session{}, &Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "Authentication is not configured"}
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return auth.Session{}, &Problem{Status: http.StatusUnauthorized, Code: ErrorCodeUnauthorized, Message: "Authentication is required"}
	}
	session, err := service.CurrentSession(r.Context(), cookie.Value)
	if errors.Is(err, auth.ErrUnauthenticated) {
		return auth.Session{}, &Problem{Status: http.StatusUnauthorized, Code: ErrorCodeUnauthorized, Message: "Authentication is required"}
	}
	if err != nil {
		return auth.Session{}, &Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Authentication could not be verified"}
	}
	return session, nil
}

func authUser(user auth.User) *authUserResponse {
	return &authUserResponse{
		Email:       user.Email,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Role:        user.Role,
	}
}

func setOAuthStateCookie(w http.ResponseWriter, value string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    value,
		Path:     "/auth/callback",
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearOAuthStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Path:     "/auth/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func setSessionCookie(w http.ResponseWriter, value string, expiresAt time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func equalSecretValues(left, right string) bool {
	if left == "" || len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

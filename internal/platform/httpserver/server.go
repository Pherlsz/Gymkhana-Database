package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const DefaultMaxBodyBytes int64 = 1 << 20

const (
	corsAllowedMethods = "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders = "Accept, Content-Type"
)

type Options struct {
	MaxBodyBytes   int64
	Auth           authenticationService
	Profile        profileService
	SecureCookies  bool
	ApplicationURL string
}

type healthResponse struct {
	Status    string `json:"status"`
	RequestID string `json:"request_id,omitempty"`
}

func New(logger *slog.Logger, pool *pgxpool.Pool, options ...Options) http.Handler {
	settings := Options{MaxBodyBytes: DefaultMaxBodyBytes, ApplicationURL: "/"}
	if len(options) > 0 {
		settings = options[0]
		if settings.MaxBodyBytes <= 0 {
			settings.MaxBodyBytes = DefaultMaxBodyBytes
		}
		if settings.ApplicationURL == "" {
			settings.ApplicationURL = "/"
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", RequestID: requestIDFromContext(r.Context())})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable", RequestID: requestIDFromContext(r.Context())})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable", RequestID: requestIDFromContext(r.Context())})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", RequestID: requestIDFromContext(r.Context())})
	})
	registerAuthRoutes(mux, logger, settings.Auth, settings.SecureCookies, settings.ApplicationURL)
	registerAdministrationRoutes(mux, logger, settings.Auth)
	registerProfileRoutes(mux, logger, settings.Auth, settings.Profile)
	mux.HandleFunc("/", fallbackHandler)
	applicationOrigin := absoluteOrigin(settings.ApplicationURL)
	return requestIDMiddleware(recoverMiddleware(logger, securityHeaders(bodyLimitMiddleware(settings.MaxBodyBytes, browserOriginMiddleware(applicationOrigin, mux)))))
}

type contextKey string

const requestIDKey contextKey = "request-id"

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, requestID)))
	})
}
func bodyLimitMiddleware(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}
func browserOriginMiddleware(applicationOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		trustedOrigin := applicationOrigin != "" && origin == applicationOrigin
		if trustedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", applicationOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			if !trustedOrigin {
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Request origin is not allowed"})
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if applicationOrigin != "" && !isSafeMethod(r.Method) && !trustedOrigin {
			writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Request origin is not allowed"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
func absoluteOrigin(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return ""
	}
	return (&url.URL{Scheme: strings.ToLower(parsed.Scheme), Host: strings.ToLower(parsed.Host)}).String()
}
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "request_id", requestIDFromContext(r.Context()), "panic", recovered, "stack", string(debug.Stack()))
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "An internal error occurred"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}
func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(value[:])
}

package httpserver

import (
	"context"
	"net/http"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

// capabilityChecker is satisfied by *auth.PostgresStore.
type capabilityChecker interface {
	UserHasCapability(ctx context.Context, userID auth.Identifier, cap auth.Capability) (bool, error)
}

// requireCapability wraps an http.HandlerFunc with a capability gate.
// ADMIN/SUPERADMIN bypass. EXTERNAL users need an explicit grant.
// When checker is nil, the gate is disabled (routes work as before).
func requireCapability(cap auth.Capability, checker capabilityChecker, authService authenticationService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// No checker configured — pass through (backward compat / tests)
		if checker == nil {
			next(w, r)
			return
		}
		session, problem := authenticatedSession(r, authService)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if session.User.Role == auth.RoleAdmin || session.User.Role == auth.RoleSuperadmin {
			next(w, r)
			return
		}
		if session.User.Role == auth.RoleExternal {
			has, err := checker.UserHasCapability(r.Context(), session.User.ID, cap)
			if err != nil {
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to check capabilities"})
				return
			}
			if !has {
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Insufficient capabilities"})
				return
			}
			next(w, r)
			return
		}
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Unknown role"})
	}
}

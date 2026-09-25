package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type administrationService interface {
	ListUsers(context.Context, auth.Session, int32, int32, string) ([]auth.ManagedUser, error)
	ProvisionUser(context.Context, auth.Session, auth.ProvisionUserParams, string) (auth.ManagedUser, error)
	DeleteUser(context.Context, auth.Session, auth.Identifier, string) error
	UpdateUserAccess(context.Context, auth.Session, auth.UpdateUserAccessParams, string) (auth.ManagedUser, error)
	GrantCapability(context.Context, auth.Session, auth.CapabilityGrant, string) error
	RevokeCapability(context.Context, auth.Session, auth.CapabilityGrant, string) error
	ListCapabilities(context.Context, auth.Session, auth.Identifier, string) ([]auth.Capability, error)
	ListAllowedEmails(context.Context, auth.Session) ([]string, error)
	AddAllowedEmail(context.Context, auth.Session, string) error
	RemoveAllowedEmail(context.Context, auth.Session, string) error
}

type adminUsersResponse struct {
	Users []adminUserResponse `json:"users"`
}

type adminUserResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Role        auth.Role `json:"role"`
	Active      bool      `json:"active"`
	Version     int64     `json:"version"`
}

type updateUserAccessRequest struct {
	Role        auth.Role `json:"role"`
	Active      bool      `json:"active"`
	Version     int64     `json:"version"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

func registerAdministrationRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService) {
	administration, _ := authentication.(administrationService)

	mux.HandleFunc("GET /api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		limit, parseProblem := parseBoundedInt32(r.URL.Query().Get("limit"), 100, 1, 100)
		if parseProblem != nil {
			writeProblem(w, r, *parseProblem)
			return
		}
		offset, parseProblem := parseBoundedInt32(r.URL.Query().Get("offset"), 0, 0, 1_000_000)
		if parseProblem != nil {
			writeProblem(w, r, *parseProblem)
			return
		}

		users, err := administration.ListUsers(r.Context(), actor, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			if errors.Is(err, auth.ErrForbidden) {
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Administrative access is required"})
				return
			}
			logger.Error("list application users", "request_id", requestIDFromContext(r.Context()), "error", err)
			writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Application users could not be loaded"})
			return
		}

		response := adminUsersResponse{Users: make([]adminUserResponse, 0, len(users))}
		for _, user := range users {
			response.Users = append(response.Users, adminUser(user))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("POST /api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		var request struct {
			Email        string            `json:"email"`
			DisplayName  string            `json:"display_name"`
			Role         auth.Role         `json:"role"`
			Capabilities []auth.Capability `json:"capabilities"`
		}
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}

		created, err := administration.ProvisionUser(r.Context(), actor, auth.ProvisionUserParams{
			Email:        request.Email,
			DisplayName:  request.DisplayName,
			Role:         request.Role,
			Capabilities: request.Capabilities,
		}, requestIDFromContext(r.Context()))
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Administrative access is required"})
			case errors.Is(err, auth.ErrUserAlreadyExists):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A user with this email already exists"})
			case errors.Is(err, auth.ErrMemberNeedsCapability):
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "A member needs a role and at least one permission"})
			case errors.Is(err, auth.ErrInvalidUserAccess), errors.Is(err, auth.ErrCapabilityConflict):
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "User access values are invalid"})
			default:
				logger.Error("provision application user", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "User could not be created"})
			}
			return
		}
		writeJSON(w, http.StatusCreated, adminUser(created))
	})

	mux.HandleFunc("DELETE /api/admin/users/{userID}", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}
		userID, err := auth.ParseIdentifier(r.PathValue("userID"))
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "User identifier is invalid"})
			return
		}
		if err := administration.DeleteUser(r.Context(), actor, userID, requestIDFromContext(r.Context())); err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Administrative access is required"})
			case errors.Is(err, auth.ErrUserNotFound):
				writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Application user was not found"})
			case errors.Is(err, auth.ErrSelfAccessChange):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "You cannot change your own access"})
			case errors.Is(err, auth.ErrProtectedSuperadmin):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "The superadmin access is protected"})
			case errors.Is(err, auth.ErrUserInUse):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "User has linked records"})
			default:
				logger.Error("delete application user", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "User could not be deleted"})
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("PATCH /api/admin/users/{userID}/access", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		userID, err := auth.ParseIdentifier(r.PathValue("userID"))
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "User identifier is invalid"})
			return
		}
		var request updateUserAccessRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}

		updated, err := administration.UpdateUserAccess(r.Context(), actor, auth.UpdateUserAccessParams{
			UserID:      userID,
			Role:        request.Role,
			Active:      request.Active,
			Version:     request.Version,
			DisplayName: request.DisplayName,
			Email:       request.Email,
		}, requestIDFromContext(r.Context()))
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "This user access change is not allowed"})
			case errors.Is(err, auth.ErrUserNotFound):
				writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Application user was not found"})
			case errors.Is(err, auth.ErrUserAccessConflict):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "User access changed since it was loaded"})
			case errors.Is(err, auth.ErrSelfAccessChange):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "You cannot change your own access"})
			case errors.Is(err, auth.ErrProtectedSuperadmin):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "The superadmin access is protected"})
			case errors.Is(err, auth.ErrInvalidUserAccess):
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "User access values are invalid"})
			default:
				logger.Error("update application user access", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "User access could not be updated"})
			}
			return
		}
		writeJSON(w, http.StatusOK, adminUser(updated))
	})

	mux.HandleFunc("POST /api/admin/users/{userID}/capabilities", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		userID, err := auth.ParseIdentifier(r.PathValue("userID"))
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "User identifier is invalid"})
			return
		}

		var req struct {
			Capability auth.Capability `json:"capability"`
		}
		if problem := DecodeJSON(w, r, &req); problem != nil {
			writeProblem(w, r, *problem)
			return
		}

		if err := administration.GrantCapability(r.Context(), actor, auth.CapabilityGrant{
			UserID:     userID,
			Capability: req.Capability,
		}, requestIDFromContext(r.Context())); err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Administrative access is required"})
			case errors.Is(err, auth.ErrSelfAccessChange):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Cannot grant capabilities to yourself"})
			case errors.Is(err, auth.ErrCapabilityConflict):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Cannot grant capabilities to non-EXTERNAL users"})
			case errors.Is(err, auth.ErrCapabilityAlreadyGranted):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Capability already granted"})
			case errors.Is(err, auth.ErrUserNotFound):
				writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "User not found"})
			default:
				logger.Error("grant capability", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to grant capability"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"message":    "Capability granted successfully",
			"user_id":    userID.String(),
			"capability": req.Capability,
		})
	})

	mux.HandleFunc("DELETE /api/admin/users/{userID}/capabilities/{capability}", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		userID, err := auth.ParseIdentifier(r.PathValue("userID"))
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "User identifier is invalid"})
			return
		}

		cap := auth.Capability(r.PathValue("capability"))
		if !cap.Valid() {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Invalid capability"})
			return
		}

		if err := administration.RevokeCapability(r.Context(), actor, auth.CapabilityGrant{
			UserID:     userID,
			Capability: cap,
		}, requestIDFromContext(r.Context())); err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Administrative access is required"})
			case errors.Is(err, auth.ErrSelfAccessChange):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "Cannot revoke capabilities from yourself"})
			case errors.Is(err, auth.ErrMemberNeedsCapability):
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "A member needs at least one permission"})
			case errors.Is(err, auth.ErrCapabilityNotFound):
				writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Capability not found"})
			default:
				logger.Error("revoke capability", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to revoke capability"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"message":    "Capability revoked successfully",
			"user_id":    userID.String(),
			"capability": cap,
		})
	})

	mux.HandleFunc("GET /api/admin/users/{userID}/capabilities", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		userID, err := auth.ParseIdentifier(r.PathValue("userID"))
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "User identifier is invalid"})
			return
		}

		capabilities, err := administration.ListCapabilities(r.Context(), actor, userID, requestIDFromContext(r.Context()))
		if err != nil {
			if errors.Is(err, auth.ErrForbidden) {
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Administrative access is required"})
				return
			}
			logger.Error("list capabilities", "request_id", requestIDFromContext(r.Context()), "error", err)
			writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to list capabilities"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"user_id":      userID.String(),
			"capabilities": capabilities,
		})
	})

	mux.HandleFunc("GET /api/admin/allowed-emails", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		emails, err := administration.ListAllowedEmails(r.Context(), actor)
		if err != nil {
			logger.Error("list allowed emails", "request_id", requestIDFromContext(r.Context()), "error", err)
			writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to list allowed emails"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"emails": emails,
		})
	})

	mux.HandleFunc("POST /api/admin/allowed-emails", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		var request struct {
			Email string `json:"email"`
		}
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}

		if err := administration.AddAllowedEmail(r.Context(), actor, request.Email); err != nil {
			logger.Error("add allowed email", "request_id", requestIDFromContext(r.Context()), "error", err)
			writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to add email to allowlist"})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"message": "Email added to allowlist",
			"email":   request.Email,
		})
	})

	mux.HandleFunc("DELETE /api/admin/allowed-emails/{email}", func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		email := r.PathValue("email")
		if email == "" {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Email is required"})
			return
		}

		if err := administration.RemoveAllowedEmail(r.Context(), actor, email); err != nil {
			logger.Error("remove allowed email", "request_id", requestIDFromContext(r.Context()), "error", err)
			writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Failed to remove email from allowlist"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"message": "Email removed from allowlist",
			"email":   email,
		})
	})
}

func adminUser(user auth.ManagedUser) adminUserResponse {
	return adminUserResponse{
		ID:          user.User.ID.String(),
		Email:       user.User.Email,
		DisplayName: user.User.DisplayName,
		AvatarURL:   user.User.AvatarURL,
		Role:        user.User.Role,
		Active:      user.User.Active,
		Version:     user.Version,
	}
}

func parseBoundedInt32(value string, fallback, minimum, maximum int32) (int32, *Problem) {
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < int64(minimum) || parsed > int64(maximum) {
		return 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Pagination values are invalid"}
	}
	return int32(parsed), nil
}

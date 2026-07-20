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
	UpdateUserAccess(context.Context, auth.Session, auth.UpdateUserAccessParams, string) (auth.ManagedUser, error)
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
	Role    auth.Role `json:"role"`
	Active  bool      `json:"active"`
	Version int64     `json:"version"`
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
			UserID: userID, Role: request.Role, Active: request.Active, Version: request.Version,
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
}

func adminUser(user auth.ManagedUser) adminUserResponse {
	return adminUserResponse{
		ID: user.User.ID.String(), Email: user.User.Email, DisplayName: user.User.DisplayName,
		AvatarURL: user.User.AvatarURL, Role: user.User.Role, Active: user.User.Active, Version: user.Version,
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

package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type userAdministrationService interface {
	ListUsers(context.Context, auth.User, int32, int32, string) ([]auth.ManagedUser, error)
	UpdateUserAccess(context.Context, auth.User, auth.UserAccessUpdate, string) (auth.ManagedUser, error)
}

type managedUserResponse struct {
	ID          string    `json:"id"`
	Login       string    `json:"login"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Role        auth.Role `json:"role"`
	Active      bool      `json:"active"`
	Version     int64     `json:"version"`
	Protected   bool      `json:"protected"`
}

type managedUsersResponse struct {
	Users []managedUserResponse `json:"users"`
}

type userAccessRequest struct {
	Role    auth.Role `json:"role"`
	Active  bool      `json:"active"`
	Version int64     `json:"version"`
}

func registerAdministrationRoutes(
	mux *http.ServeMux,
	logger *slog.Logger,
	authentication authenticationService,
	administration userAdministrationService,
) {
	mux.HandleFunc("GET /api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		session, problem := authorizedSession(r, authentication, auth.PermissionManageUsers)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if administration == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeAuthUnavailable, Message: "User administration is not configured"})
			return
		}

		limit, limitProblem := queryInt32(r, "limit", 100, 1, 200)
		if limitProblem != nil {
			writeProblem(w, r, *limitProblem)
			return
		}
		offset, offsetProblem := queryInt32(r, "offset", 0, 0, 1_000_000)
		if offsetProblem != nil {
			writeProblem(w, r, *offsetProblem)
			return
		}

		users, err := administration.ListUsers(r.Context(), session.User, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			if errors.Is(err, auth.ErrForbidden) {
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "This account cannot manage users"})
				return
			}
			logger.Error("list managed users", "request_id", requestIDFromContext(r.Context()), "error", err)
			writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Users could not be loaded"})
			return
		}

		response := managedUsersResponse{Users: make([]managedUserResponse, 0, len(users))}
		for _, user := range users {
			response.Users = append(response.Users, managedUser(user))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("PATCH /api/admin/users/{userID}/access", func(w http.ResponseWriter, r *http.Request) {
		session, problem := authorizedSession(r, authentication, auth.PermissionManageUsers)
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
		var request userAccessRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}

		updated, err := administration.UpdateUserAccess(r.Context(), session.User, auth.UserAccessUpdate{
			UserID:  userID,
			Role:    request.Role,
			Active:  request.Active,
			Version: request.Version,
		}, requestIDFromContext(r.Context()))
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrForbidden):
				writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "This account cannot manage users"})
			case errors.Is(err, auth.ErrUserNotFound):
				writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "User was not found"})
			case errors.Is(err, auth.ErrUserAccessConflict):
				writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "User access changed; reload and try again"})
			case errors.Is(err, auth.ErrInvalidManagedRole), errors.Is(err, auth.ErrInvalidUserVersion):
				writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "User access values are invalid"})
			case errors.Is(err, auth.ErrSelfAccessChange):
				writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "You cannot change your own access"})
			case errors.Is(err, auth.ErrProtectedSuperadmin):
				writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "The active superadmin is protected"})
			default:
				logger.Error("update managed user", "request_id", requestIDFromContext(r.Context()), "error", err)
				writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "User access could not be updated"})
			}
			return
		}

		writeJSON(w, http.StatusOK, managedUser(updated))
	})
}

func authorizedSession(r *http.Request, service authenticationService, permission auth.Permission) (auth.Session, *Problem) {
	session, problem := authenticatedSession(r, service)
	if problem != nil {
		return auth.Session{}, problem
	}
	if !session.User.Role.Allows(permission) {
		return auth.Session{}, &Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "This account is not authorized for this resource"}
	}
	return session, nil
}

func managedUser(user auth.ManagedUser) managedUserResponse {
	return managedUserResponse{
		ID:          user.ID.String(),
		Login:       user.Login,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Role:        user.Role,
		Active:      user.Active,
		Version:     user.Version,
		Protected:   user.Role == auth.RoleSuperadmin,
	}
}

func queryInt32(r *http.Request, name string, fallback, minimum, maximum int32) (int32, *Problem) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value < int64(minimum) || value > int64(maximum) {
		return 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Query parameter " + name + " is invalid"}
	}
	return int32(value), nil
}

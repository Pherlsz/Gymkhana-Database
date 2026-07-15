package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
)

type ErrorCode string

const (
	ErrorCodeBadRequest        ErrorCode = "bad_request"
	ErrorCodeInvalidJSON       ErrorCode = "invalid_json"
	ErrorCodeUnsupportedMedia  ErrorCode = "unsupported_media_type"
	ErrorCodeRequestTooLarge   ErrorCode = "request_too_large"
	ErrorCodeUnauthorized      ErrorCode = "unauthorized"
	ErrorCodeForbidden         ErrorCode = "forbidden"
	ErrorCodeConflict          ErrorCode = "conflict"
	ErrorCodeInvalidOAuthState ErrorCode = "invalid_oauth_state"
	ErrorCodeAuthProvider      ErrorCode = "auth_provider_error"
	ErrorCodeAuthUnavailable   ErrorCode = "auth_unavailable"
	ErrorCodeNotFound          ErrorCode = "not_found"
	ErrorCodeMethodNotAllowed  ErrorCode = "method_not_allowed"
	ErrorCodeInternal          ErrorCode = "internal_error"
)

type Problem struct {
	Status  int
	Code    ErrorCode
	Message string
}

type errorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type errorResponse struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, destination any) *Problem {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &Problem{Status: http.StatusUnsupportedMediaType, Code: ErrorCodeUnsupportedMedia, Message: "Content-Type must be application/json"}
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return &Problem{Status: http.StatusRequestEntityTooLarge, Code: ErrorCodeRequestTooLarge, Message: "Request body exceeds the configured limit"}
		}
		if errors.Is(err, io.EOF) {
			return &Problem{Status: http.StatusBadRequest, Code: ErrorCodeInvalidJSON, Message: "Request body must contain one JSON value"}
		}
		return &Problem{Status: http.StatusBadRequest, Code: ErrorCodeInvalidJSON, Message: "Request body contains invalid JSON"}
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &Problem{Status: http.StatusBadRequest, Code: ErrorCodeInvalidJSON, Message: "Request body must contain exactly one JSON value"}
	}

	return nil
}

func writeProblem(w http.ResponseWriter, r *http.Request, problem Problem) {
	writeJSON(w, problem.Status, errorResponse{
		Error:     errorBody{Code: problem.Code, Message: problem.Message},
		RequestID: requestIDFromContext(r.Context()),
	})
}

func fallbackHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/health/") || strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/api/") {
		writeProblem(w, r, Problem{Status: http.StatusMethodNotAllowed, Code: ErrorCodeMethodNotAllowed, Message: "HTTP method is not allowed for this resource"})
		return
	}
	writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Resource was not found"})
}

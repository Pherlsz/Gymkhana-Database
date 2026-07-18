package aichat

import "errors"

var (
	ErrForbidden         = errors.New("AI Chat is forbidden")
	ErrNotFound          = errors.New("AI Chat resource was not found")
	ErrConflict          = errors.New("AI Chat resource changed concurrently")
	ErrInvalidInput      = errors.New("AI Chat input is invalid")
	ErrInvalidState      = errors.New("AI Chat state transition is invalid")
	ErrRateLimited       = errors.New("AI Chat rate limit exceeded")
	ErrQuotaExceeded     = errors.New("AI Chat usage quota exceeded")
	ErrUnavailable       = errors.New("AI Chat is unavailable")
	ErrTimeout           = errors.New("AI Chat run timed out")
	ErrCancelled         = errors.New("AI Chat run was cancelled")
	ErrMalformedProvider = errors.New("AI Chat provider output is invalid")
	ErrStaleContext      = errors.New("AI Chat result context is stale")
	ErrToolFailed        = errors.New("AI Chat tool failed")
	ErrUnsafeResult      = errors.New("AI Chat tool returned an unsafe result")
	ErrInvalidSetup      = errors.New("AI Chat service setup is invalid")
)

package matching

import "errors"

var (
	ErrCancelled           = errors.New("matching analysis cancelled")
	ErrConflict            = errors.New("matching resource changed concurrently")
	ErrDependencyConflict  = errors.New("profile dependencies conflict")
	ErrForbidden           = errors.New("matching operation forbidden")
	ErrInvalidConfirmation = errors.New("merge confirmation is invalid")
	ErrInvalidInput        = errors.New("matching input is invalid")
	ErrInvalidState        = errors.New("matching resource state is invalid")
	ErrNotFound            = errors.New("matching resource not found")
	ErrRateLimited         = errors.New("matching rate limit exceeded")
	ErrStalePreview        = errors.New("merge preview is stale")
	ErrTimeout             = errors.New("matching analysis timed out")
)

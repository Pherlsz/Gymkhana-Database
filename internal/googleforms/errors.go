package googleforms

import "errors"

var (
	ErrDisabled          = errors.New("google forms integration is disabled")
	ErrForbidden         = errors.New("google forms access is forbidden")
	ErrNotFound          = errors.New("google forms resource was not found")
	ErrConflict          = errors.New("google forms resource conflict")
	ErrInvalidInput      = errors.New("invalid google forms input")
	ErrInvalidState      = errors.New("invalid google forms state")
	ErrOAuthState        = errors.New("invalid google forms oauth state")
	ErrOAuthScopes       = errors.New("required google forms scopes were not granted")
	ErrNeedsReauth       = errors.New("google forms connection requires reauthorization")
	ErrSchemaDrift       = errors.New("google forms schema changed")
	ErrUnsupportedForm   = errors.New("google form contains unsupported questions")
	ErrProvider          = errors.New("google forms provider failed")
	ErrProviderRetryable = errors.New("google forms provider request is retryable")
	ErrRateLimited       = errors.New("google forms rate limit exceeded")
	ErrCancelled         = errors.New("google forms synchronization was cancelled")
)

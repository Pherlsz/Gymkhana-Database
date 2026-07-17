package queryengine

import "errors"

var (
	ErrForbidden        = errors.New("query engine is forbidden")
	ErrNotFound         = errors.New("query execution was not found")
	ErrExpired          = errors.New("query result expired")
	ErrInvalidPlan      = errors.New("query plan is invalid")
	ErrStaleCatalog     = errors.New("query catalog is stale")
	ErrCostLimit        = errors.New("query plan exceeds the cost limit")
	ErrRateLimited      = errors.New("query execution rate limit exceeded")
	ErrConflict         = errors.New("query execution conflicts with active work")
	ErrTimeout          = errors.New("query execution timed out")
	ErrCancelled        = errors.New("query execution was cancelled")
	ErrUnsafeResult     = errors.New("query result violated the authorized catalog")
	ErrInvalidSetup     = errors.New("query engine setup is invalid")
	ErrReadOnlyRequired = errors.New("query execution must be read only")
)

package operations

import "errors"

var (
	ErrCancelled           = errors.New("operation cancelled")
	ErrConflict            = errors.New("operation changed concurrently")
	ErrDecisionRequired    = errors.New("import decisions are required")
	ErrExpired             = errors.New("operation expired")
	ErrForbidden           = errors.New("operation forbidden")
	ErrInvalidConfirmation = errors.New("invalid operation confirmation")
	ErrInvalidInput        = errors.New("invalid operation input")
	ErrInvalidMapping      = errors.New("invalid import mapping")
	ErrInvalidState        = errors.New("invalid operation state")
	ErrInvalidServiceSetup = errors.New("invalid operation service setup")
	ErrNotFound            = errors.New("operation not found")
	ErrQuotaExceeded       = errors.New("operation quota exceeded")
	ErrRateLimited         = errors.New("operation rate limit exceeded")
	ErrStalePreview        = errors.New("import preview is stale")
	ErrUnsupportedWorkbook = errors.New("unsupported workbook")
	ErrWorkbookLimit       = errors.New("workbook limit exceeded")
)

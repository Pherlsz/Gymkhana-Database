package ocr

import "errors"

var (
	ErrCancelled         = errors.New("OCR job was cancelled")
	ErrConflict          = errors.New("OCR state changed concurrently")
	ErrForbidden         = errors.New("OCR operation is forbidden")
	ErrInvalidInput      = errors.New("OCR input is invalid")
	ErrInvalidSetup      = errors.New("OCR service setup is invalid")
	ErrInvalidState      = errors.New("OCR state does not allow this operation")
	ErrMalformedProvider = errors.New("OCR provider output is invalid")
	ErrNotFound          = errors.New("OCR resource was not found")
	ErrQuotaExceeded     = errors.New("OCR usage quota was exceeded")
	ErrRateLimited       = errors.New("OCR request rate was exceeded")
	ErrStaleTarget       = errors.New("OCR target changed after review")
	ErrTimeout           = errors.New("OCR extraction timed out")
	ErrUnavailable       = errors.New("OCR extraction is unavailable")
	ErrUnsafeSource      = errors.New("OCR source is unsupported or unsafe")
)

package attachment

import "errors"

var (
	ErrForbidden            = errors.New("attachment operation is forbidden")
	ErrInvalidOwner         = errors.New("attachment owner is invalid")
	ErrInvalidInput         = errors.New("attachment input is invalid")
	ErrInvalidFileName      = errors.New("attachment filename is invalid")
	ErrInvalidMIME          = errors.New("attachment MIME is invalid")
	ErrInvalidSize          = errors.New("attachment size is invalid")
	ErrUnsupportedFile      = errors.New("attachment file signature is unsupported")
	ErrMIMEMismatch         = errors.New("attachment declared MIME does not match detected content")
	ErrUploadIntentNotFound = errors.New("attachment upload intent was not found")
	ErrUploadIntentExpired  = errors.New("attachment upload intent expired")
	ErrUploadIntentConsumed = errors.New("attachment upload intent was already consumed")
	ErrUploadObjectNotFound = errors.New("attachment upload object was not found")
	ErrAttachmentNotFound   = errors.New("attachment was not found")
	ErrConflict             = errors.New("attachment version conflict")
	ErrInvalidState         = errors.New("attachment lifecycle state is invalid")
	ErrInvalidConfirmation  = errors.New("attachment confirmation is invalid")
	ErrStorageUnavailable   = errors.New("attachment storage is unavailable")
	ErrInvalidServiceSetup  = errors.New("attachment service setup is invalid")
)

type FieldError struct {
	Field string
	Code  string
}

type ValidationError struct {
	Fields []FieldError
}

func (err *ValidationError) Error() string {
	return "attachment validation failed"
}

func (err *ValidationError) add(field, code string) {
	err.Fields = append(err.Fields, FieldError{Field: field, Code: code})
}

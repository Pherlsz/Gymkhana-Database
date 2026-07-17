package customdata

import "errors"

const DeleteConfirmation = "Confirmar"

var (
	ErrForbidden              = errors.New("custom data operation is forbidden")
	ErrInvalidConfirmation    = errors.New("custom data delete confirmation is invalid")
	ErrInvalidListOptions     = errors.New("custom data list options are invalid")
	ErrInvalidTarget          = errors.New("custom data target is invalid")
	ErrInvalidServiceSetup    = errors.New("custom data service setup is invalid")
	ErrNotFound               = errors.New("custom data resource not found")
	ErrConflict               = errors.New("custom data resource changed concurrently")
	ErrTechnicalKeyConflict   = errors.New("custom data technical key is already used")
	ErrTechnicalKeyImmutable  = errors.New("custom data technical key is immutable")
	ErrDefinitionInUse        = errors.New("custom field definition is in use")
	ErrDefinitionChangeUnsafe = errors.New("custom field definition change would invalidate stored values")
	ErrOptionInUse            = errors.New("custom field option is in use")
	ErrEntityTypeInUse        = errors.New("custom entity type is in use")
	ErrEntityTypeInactive     = errors.New("custom entity type is inactive")
	ErrDefinitionInactive     = errors.New("custom field definition is inactive")
	ErrReferenceNotFound      = errors.New("custom data reference was not found")
	ErrCardinalityConflict    = errors.New("custom entity profile cardinality conflict")
)

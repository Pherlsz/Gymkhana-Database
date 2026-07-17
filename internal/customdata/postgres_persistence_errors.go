package customdata

import "errors"

func mapKnownOrPersistence(operation string, err error) error {
	known := []error{ErrNotFound, ErrConflict, ErrTechnicalKeyConflict, ErrTechnicalKeyImmutable, ErrDefinitionInUse,
		ErrDefinitionChangeUnsafe, ErrOptionInUse, ErrEntityTypeInUse, ErrEntityTypeInactive, ErrDefinitionInactive,
		ErrReferenceNotFound, ErrCardinalityConflict, ErrInvalidTarget}
	for _, candidate := range known {
		if errors.Is(err, candidate) {
			return candidate
		}
	}
	return mapPersistenceError(operation, err)
}

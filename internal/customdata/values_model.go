package customdata

import "time"

type ValueTargetKind string

const (
	ValueTargetProfile      ValueTargetKind = "PROFILE"
	ValueTargetDocument     ValueTargetKind = "DOCUMENT"
	ValueTargetBill         ValueTargetKind = "BILL"
	ValueTargetCustomEntity ValueTargetKind = "CUSTOM_ENTITY"
)

func (value ValueTargetKind) Valid() bool {
	switch value {
	case ValueTargetProfile, ValueTargetDocument, ValueTargetBill, ValueTargetCustomEntity:
		return true
	default:
		return false
	}
}

type TargetReference struct {
	Kind ValueTargetKind
	ID   Identifier
}

func (value TargetReference) Valid() bool { return value.Kind.Valid() && !value.ID.IsZero() }

type StoredValue struct {
	ID        Identifier
	Target    TargetReference
	Input     ValueInput
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ValueSet struct {
	Target  TargetReference
	Values  []StoredValue
	Version int64
}

type Entity struct {
	ID                 Identifier
	TypeID             Identifier
	OwnerProfileID     *Identifier
	ProfileCardinality ProfileCardinality
	Values             []StoredValue
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type EntityListOptions struct {
	TypeID         Identifier
	OwnerProfileID *Identifier
	Limit          int32
	Offset         int32
}

type EntityPage struct {
	Entities       []Entity
	Total          int64
	TypeID         Identifier
	OwnerProfileID *Identifier
	Limit          int32
	Offset         int32
}

func normalizeEntityListOptions(value EntityListOptions) (EntityListOptions, error) {
	if value.Limit == 0 {
		value.Limit = 100
	}
	if value.TypeID.IsZero() || value.Limit < 1 || value.Limit > 1000 || value.Offset < 0 || (value.OwnerProfileID != nil && value.OwnerProfileID.IsZero()) {
		return EntityListOptions{}, ErrInvalidListOptions
	}
	return value, nil
}

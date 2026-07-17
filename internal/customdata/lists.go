package customdata

import "strings"

type SortOrder string

const (
	SortAscending  SortOrder = "asc"
	SortDescending SortOrder = "desc"
)

func (value SortOrder) Valid() bool { return value == SortAscending || value == SortDescending }

type EntityTypeSortField string

const (
	EntityTypeSortLabel     EntityTypeSortField = "label"
	EntityTypeSortCreatedAt EntityTypeSortField = "created_at"
	EntityTypeSortUpdatedAt EntityTypeSortField = "updated_at"
)

func (value EntityTypeSortField) Valid() bool {
	return value == EntityTypeSortLabel || value == EntityTypeSortCreatedAt || value == EntityTypeSortUpdatedAt
}

type EntityTypeFilters struct {
	Label  string
	Active *bool
}

type EntityTypeListOptions struct {
	Limit     int32
	Offset    int32
	SortField EntityTypeSortField
	SortOrder SortOrder
	Filters   EntityTypeFilters
}

type EntityTypePage struct {
	Types     []EntityType
	Total     int64
	Limit     int32
	Offset    int32
	SortField EntityTypeSortField
	SortOrder SortOrder
	Filters   EntityTypeFilters
}

type FieldDefinitionSortField string

const (
	FieldDefinitionSortLabel     FieldDefinitionSortField = "label"
	FieldDefinitionSortKey       FieldDefinitionSortField = "technical_key"
	FieldDefinitionSortCreatedAt FieldDefinitionSortField = "created_at"
	FieldDefinitionSortUpdatedAt FieldDefinitionSortField = "updated_at"
)

func (value FieldDefinitionSortField) Valid() bool {
	return value == FieldDefinitionSortLabel || value == FieldDefinitionSortKey || value == FieldDefinitionSortCreatedAt || value == FieldDefinitionSortUpdatedAt
}

type FieldDefinitionFilters struct {
	TargetKind TargetKind
	TargetID   Identifier
	Label      string
	Active     *bool
}

type FieldDefinitionListOptions struct {
	Limit     int32
	Offset    int32
	SortField FieldDefinitionSortField
	SortOrder SortOrder
	Filters   FieldDefinitionFilters
}

type FieldDefinitionPage struct {
	Definitions []FieldDefinition
	Total       int64
	Limit       int32
	Offset      int32
	SortField   FieldDefinitionSortField
	SortOrder   SortOrder
	Filters     FieldDefinitionFilters
}

func normalizeEntityTypeListOptions(value EntityTypeListOptions) (EntityTypeListOptions, error) {
	if value.Limit == 0 {
		value.Limit = 100
	}
	if value.SortField == "" {
		value.SortField = EntityTypeSortLabel
	}
	if value.SortOrder == "" {
		value.SortOrder = SortAscending
	}
	value.Filters.Label = strings.TrimSpace(value.Filters.Label)
	if value.Limit < 1 || value.Limit > 1000 || value.Offset < 0 || !value.SortField.Valid() || !value.SortOrder.Valid() {
		return EntityTypeListOptions{}, ErrInvalidListOptions
	}
	return value, nil
}

func normalizeFieldDefinitionListOptions(value FieldDefinitionListOptions) (FieldDefinitionListOptions, error) {
	if value.Limit == 0 {
		value.Limit = 100
	}
	if value.SortField == "" {
		value.SortField = FieldDefinitionSortLabel
	}
	if value.SortOrder == "" {
		value.SortOrder = SortAscending
	}
	value.Filters.Label = strings.TrimSpace(value.Filters.Label)
	if value.Limit < 1 || value.Limit > 1000 || value.Offset < 0 || !value.SortField.Valid() || !value.SortOrder.Valid() || !value.Filters.TargetKind.Valid() {
		return FieldDefinitionListOptions{}, ErrInvalidListOptions
	}
	if value.Filters.TargetKind == TargetProfile {
		if !value.Filters.TargetID.IsZero() {
			return FieldDefinitionListOptions{}, ErrInvalidListOptions
		}
	} else if value.Filters.TargetID.IsZero() {
		return FieldDefinitionListOptions{}, ErrInvalidListOptions
	}
	return value, nil
}

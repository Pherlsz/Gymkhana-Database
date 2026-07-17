package httpserver

import (
	"context"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
)

type customDataService interface {
	ListEntityTypes(context.Context, auth.Session, customdata.EntityTypeListOptions) (customdata.EntityTypePage, error)
	GetEntityType(context.Context, auth.Session, customdata.Identifier) (customdata.EntityType, error)
	CreateEntityType(context.Context, auth.Session, customdata.EntityTypeValues, string) (customdata.EntityType, error)
	UpdateEntityType(context.Context, auth.Session, customdata.Identifier, int64, customdata.EntityTypeValues, string) (customdata.EntityType, error)
	DeleteEntityType(context.Context, auth.Session, customdata.Identifier, int64, string, string) error
	ListFieldDefinitions(context.Context, auth.Session, customdata.FieldDefinitionListOptions) (customdata.FieldDefinitionPage, error)
	GetFieldDefinition(context.Context, auth.Session, customdata.Identifier) (customdata.FieldDefinition, error)
	CreateFieldDefinition(context.Context, auth.Session, customdata.FieldDefinitionValues, string) (customdata.FieldDefinition, error)
	UpdateFieldDefinition(context.Context, auth.Session, customdata.Identifier, int64, customdata.FieldDefinitionValues, string) (customdata.FieldDefinition, error)
	DeleteFieldDefinition(context.Context, auth.Session, customdata.Identifier, int64, string, string) error
	ListOptions(context.Context, auth.Session, customdata.Identifier) ([]customdata.Option, error)
	CreateOption(context.Context, auth.Session, customdata.Identifier, customdata.OptionValues, string) (customdata.Option, error)
	UpdateOption(context.Context, auth.Session, customdata.Identifier, customdata.Identifier, int64, customdata.OptionValues, string) (customdata.Option, error)
	DeleteOption(context.Context, auth.Session, customdata.Identifier, customdata.Identifier, int64, string, string) error
	GetValues(context.Context, auth.Session, customdata.TargetReference) (customdata.ValueSet, error)
	ReplaceValues(context.Context, auth.Session, customdata.TargetReference, int64, []customdata.ValueInput, string) (customdata.ValueSet, error)
	ListEntities(context.Context, auth.Session, customdata.EntityListOptions) (customdata.EntityPage, error)
	GetEntity(context.Context, auth.Session, customdata.Identifier) (customdata.Entity, error)
	CreateEntity(context.Context, auth.Session, customdata.Identifier, *customdata.Identifier, []customdata.ValueInput, string) (customdata.Entity, error)
	UpdateEntity(context.Context, auth.Session, customdata.Identifier, int64, []customdata.ValueInput, string) (customdata.Entity, error)
	DeleteEntity(context.Context, auth.Session, customdata.Identifier, int64, string, string) error
}

type customEntityTypeRequest struct {
	TechnicalKey       string                        `json:"technical_key"`
	Label              string                        `json:"label"`
	Active             bool                          `json:"active"`
	ProfileCardinality customdata.ProfileCardinality `json:"profile_cardinality"`
	Version            int64                         `json:"version,omitempty"`
}

type customFieldRequest struct {
	TargetKind      customdata.TargetKind `json:"target_kind"`
	TargetID        string                `json:"target_id,omitempty"`
	TechnicalKey    string                `json:"technical_key"`
	Label           string                `json:"label"`
	FieldKind       customdata.FieldKind  `json:"field_kind"`
	Required        bool                  `json:"required"`
	Active          bool                  `json:"active"`
	MinimumLength   int                   `json:"minimum_length,omitempty"`
	MaximumLength   int                   `json:"maximum_length,omitempty"`
	ValidationRegex string                `json:"validation_regex,omitempty"`
	MinimumDecimal  string                `json:"minimum_decimal,omitempty"`
	MaximumDecimal  string                `json:"maximum_decimal,omitempty"`
	Version         int64                 `json:"version,omitempty"`
}

type customOptionRequest struct {
	TechnicalKey string `json:"technical_key"`
	Label        string `json:"label"`
	Active       bool   `json:"active"`
	SortOrder    int    `json:"sort_order"`
	Version      int64  `json:"version,omitempty"`
}

type customDeleteRequest struct {
	Version      int64  `json:"version"`
	Confirmation string `json:"confirmation"`
}

type customValuesRequest struct {
	Version int64                `json:"version"`
	Values  []customValueRequest `json:"values"`
}

type customEntityRequest struct {
	EntityTypeID   string               `json:"entity_type_id,omitempty"`
	OwnerProfileID string               `json:"owner_profile_id,omitempty"`
	Version        int64                `json:"version,omitempty"`
	Values         []customValueRequest `json:"values"`
}

type customValueRequest struct {
	FieldDefinitionID string               `json:"field_definition_id"`
	FieldKind         customdata.FieldKind `json:"field_kind"`
	Text              string               `json:"text,omitempty"`
	Integer           *int64               `json:"integer,omitempty"`
	Decimal           string               `json:"decimal,omitempty"`
	Boolean           *bool                `json:"boolean,omitempty"`
	CivilDate         string               `json:"civil_date,omitempty"`
	CivilMonth        string               `json:"civil_month,omitempty"`
	OptionIDs         []string             `json:"option_ids,omitempty"`
}

type customEntityTypeResponse struct {
	ID                 string                        `json:"id"`
	TechnicalKey       string                        `json:"technical_key"`
	Label              string                        `json:"label"`
	Active             bool                          `json:"active"`
	ProfileCardinality customdata.ProfileCardinality `json:"profile_cardinality"`
	Version            int64                         `json:"version"`
	CreatedAt          time.Time                     `json:"created_at"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

type customFieldResponse struct {
	ID              string                `json:"id"`
	TargetKind      customdata.TargetKind `json:"target_kind"`
	TargetID        string                `json:"target_id,omitempty"`
	TechnicalKey    string                `json:"technical_key"`
	Label           string                `json:"label"`
	FieldKind       customdata.FieldKind  `json:"field_kind"`
	Required        bool                  `json:"required"`
	Active          bool                  `json:"active"`
	MinimumLength   int                   `json:"minimum_length,omitempty"`
	MaximumLength   int                   `json:"maximum_length,omitempty"`
	ValidationRegex string                `json:"validation_regex,omitempty"`
	MinimumDecimal  string                `json:"minimum_decimal,omitempty"`
	MaximumDecimal  string                `json:"maximum_decimal,omitempty"`
	Version         int64                 `json:"version"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

type customOptionResponse struct {
	ID                string    `json:"id"`
	FieldDefinitionID string    `json:"field_definition_id"`
	TechnicalKey      string    `json:"technical_key"`
	Label             string    `json:"label"`
	Active            bool      `json:"active"`
	SortOrder         int       `json:"sort_order"`
	Version           int64     `json:"version"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type customStoredValueResponse struct {
	ID                string               `json:"id"`
	FieldDefinitionID string               `json:"field_definition_id"`
	FieldKind         customdata.FieldKind `json:"field_kind"`
	Text              string               `json:"text,omitempty"`
	Integer           *int64               `json:"integer,omitempty"`
	Decimal           string               `json:"decimal,omitempty"`
	Boolean           *bool                `json:"boolean,omitempty"`
	CivilDate         string               `json:"civil_date,omitempty"`
	CivilMonth        string               `json:"civil_month,omitempty"`
	OptionIDs         []string             `json:"option_ids,omitempty"`
	Version           int64                `json:"version"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
}

type customValueSetResponse struct {
	TargetKind customdata.ValueTargetKind  `json:"target_kind"`
	TargetID   string                      `json:"target_id"`
	Values     []customStoredValueResponse `json:"values"`
	Version    int64                       `json:"version"`
}

type customEntityResponse struct {
	ID                 string                        `json:"id"`
	EntityTypeID       string                        `json:"entity_type_id"`
	OwnerProfileID     string                        `json:"owner_profile_id,omitempty"`
	ProfileCardinality customdata.ProfileCardinality `json:"profile_cardinality"`
	Values             []customStoredValueResponse   `json:"values"`
	Version            int64                         `json:"version"`
	CreatedAt          time.Time                     `json:"created_at"`
	UpdatedAt          time.Time                     `json:"updated_at"`
}

type customPageMeta struct {
	Total     int64  `json:"total"`
	Limit     int32  `json:"limit"`
	Offset    int32  `json:"offset"`
	SortField string `json:"sort_field,omitempty"`
	SortOrder string `json:"sort_order,omitempty"`
}

type customEntityTypePageResponse struct {
	Types []customEntityTypeResponse `json:"types"`
	Page  customPageMeta             `json:"page"`
}

type customFieldPageResponse struct {
	Fields []customFieldResponse `json:"fields"`
	Page   customPageMeta        `json:"page"`
}

type customOptionPageResponse struct {
	Options []customOptionResponse `json:"options"`
}

type customEntityPageResponse struct {
	Entities []customEntityResponse `json:"entities"`
	Page     customPageMeta         `json:"page"`
}

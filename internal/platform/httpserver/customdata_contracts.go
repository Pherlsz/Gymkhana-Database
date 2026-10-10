package httpserver

import (
	"context"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
)

type customDataService interface {
	ListFieldDefinitions(context.Context, auth.Session, customdata.FieldDefinitionListOptions) (customdata.FieldDefinitionPage, error)
	ListOptions(context.Context, auth.Session, customdata.Identifier) ([]customdata.Option, error)
	GetValues(context.Context, auth.Session, customdata.TargetReference) (customdata.ValueSet, error)
	ReplaceValues(context.Context, auth.Session, customdata.TargetReference, int64, []customdata.ValueInput, string) (customdata.ValueSet, error)
}

type customValuesRequest struct {
	Version int64                `json:"version"`
	Values  []customValueRequest `json:"values"`
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

type customPageMeta struct {
	Total     int64  `json:"total"`
	Limit     int32  `json:"limit"`
	Offset    int32  `json:"offset"`
	SortField string `json:"sort_field,omitempty"`
	SortOrder string `json:"sort_order,omitempty"`
}

type customFieldPageResponse struct {
	Fields []customFieldResponse `json:"fields"`
	Page   customPageMeta        `json:"page"`
}

type customOptionPageResponse struct {
	Options []customOptionResponse `json:"options"`
}

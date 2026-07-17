package httpserver

import "github.com/Pherlsz/Gymkhana-Database/internal/customdata"

func customEntityTypeFromDomain(value customdata.EntityType) customEntityTypeResponse {
	return customEntityTypeResponse{ID: value.ID.String(), TechnicalKey: value.Values.TechnicalKey, Label: value.Values.Label, Active: value.Values.Active, ProfileCardinality: value.Values.ProfileCardinality, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func customFieldFromDomain(value customdata.FieldDefinition) customFieldResponse {
	targetID := ""
	if !value.Values.TargetID.IsZero() {
		targetID = value.Values.TargetID.String()
	}
	return customFieldResponse{ID: value.ID.String(), TargetKind: value.Values.TargetKind, TargetID: targetID, TechnicalKey: value.Values.TechnicalKey, Label: value.Values.Label, FieldKind: value.Values.Kind, Required: value.Values.Required, Active: value.Values.Active, MinimumLength: value.Values.MinimumLength, MaximumLength: value.Values.MaximumLength, ValidationRegex: value.Values.ValidationRegex, MinimumDecimal: value.Values.MinimumDecimal, MaximumDecimal: value.Values.MaximumDecimal, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func customOptionFromDomain(value customdata.Option) customOptionResponse {
	return customOptionResponse{ID: value.ID.String(), FieldDefinitionID: value.FieldDefinitionID.String(), TechnicalKey: value.Values.TechnicalKey, Label: value.Values.Label, Active: value.Values.Active, SortOrder: value.Values.SortOrder, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func customStoredValueFromDomain(value customdata.StoredValue) customStoredValueResponse {
	options := make([]string, 0, len(value.Input.OptionIDs))
	for _, id := range value.Input.OptionIDs {
		options = append(options, id.String())
	}
	return customStoredValueResponse{ID: value.ID.String(), FieldDefinitionID: value.Input.FieldDefinitionID.String(), FieldKind: value.Input.Kind, Text: value.Input.Text, Integer: value.Input.Integer, Decimal: value.Input.Decimal, Boolean: value.Input.Boolean, CivilDate: value.Input.CivilDate, CivilMonth: value.Input.CivilMonth, OptionIDs: options, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func customValueSetFromDomain(value customdata.ValueSet) customValueSetResponse {
	response := customValueSetResponse{TargetKind: value.Target.Kind, TargetID: value.Target.ID.String(), Version: value.Version}
	for _, item := range value.Values {
		response.Values = append(response.Values, customStoredValueFromDomain(item))
	}
	return response
}

func customEntityFromDomain(value customdata.Entity) customEntityResponse {
	response := customEntityResponse{ID: value.ID.String(), EntityTypeID: value.TypeID.String(), ProfileCardinality: value.ProfileCardinality, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if value.OwnerProfileID != nil {
		response.OwnerProfileID = value.OwnerProfileID.String()
	}
	for _, item := range value.Values {
		response.Values = append(response.Values, customStoredValueFromDomain(item))
	}
	return response
}

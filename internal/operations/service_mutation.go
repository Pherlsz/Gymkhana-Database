package operations

import (
	"context"
	"strconv"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type rowNormalizer func(ctx context.Context, service *Service, effective map[string]string) (Mutation, error)

func normalizeProfileMutation(_ context.Context, _ *Service, effective map[string]string) (Mutation, error) {
	values, err := profileValuesFromImport(effective)
	if err != nil {
		return Mutation{}, err
	}
	normalized, err := profile.Normalize(values)
	if err != nil {
		return Mutation{}, err
	}
	return Mutation{Module: ModuleProfiles, Profile: &normalized}, nil
}

func normalizeDocumentMutation(ctx context.Context, service *Service, effective map[string]string) (Mutation, error) {
	ownerID, err := profile.ParseIdentifier(effective["owner_profile_id"])
	if err != nil {
		return Mutation{}, err
	}
	typeID, err := document.ParseIdentifier(effective["document_type_id"])
	if err != nil {
		return Mutation{}, err
	}
	definition, err := service.store.DocumentType(ctx, typeID)
	if err != nil {
		return Mutation{}, err
	}
	normalized, err := document.Normalize(document.Values{OwnerProfileID: ownerID, TypeID: typeID,
		Identifier: effective["identifier_value"], DocumentDate: effective["document_date"],
		Notes: effective["notes"], Medium: document.Medium(effective["medium"])}, definition)
	if err != nil {
		return Mutation{}, err
	}
	return Mutation{Module: ModuleDocuments, Document: &normalized}, nil
}

func normalizeBillMutation(ctx context.Context, service *Service, effective map[string]string) (Mutation, error) {
	ownerID, err := profile.ParseIdentifier(effective["owner_profile_id"])
	if err != nil {
		return Mutation{}, err
	}
	typeID, err := bill.ParseIdentifier(effective["bill_type_id"])
	if err != nil {
		return Mutation{}, err
	}
	definition, err := service.store.BillType(ctx, typeID)
	if err != nil {
		return Mutation{}, err
	}
	normalized, err := bill.Normalize(bill.Values{OwnerProfileID: ownerID, TypeID: typeID,
		PrintedHolderName: effective["printed_holder_name"], PrintedAddress: effective["printed_address"],
		Reference: effective["reference_value"], Competence: effective["competence"], Amount: effective["amount"],
		Currency: effective["currency"], Notes: effective["notes"], Medium: bill.Medium(effective["medium"])}, definition)
	if err != nil {
		return Mutation{}, err
	}
	return Mutation{Module: ModuleBills, Bill: &normalized}, nil
}

var moduleNormalizers = map[Module]rowNormalizer{
	ModuleProfiles:  normalizeProfileMutation,
	ModuleDocuments: normalizeDocumentMutation,
	ModuleBills:     normalizeBillMutation,
}

func (service *Service) mutationForRow(ctx context.Context, module Module, values map[string]string, customFields []CustomFieldDefinition) (Mutation, int64, error) {
	effective := make(map[string]string, len(values))
	for field, value := range values {
		effective[field] = value
	}
	targetID, _, action, err := service.resolveTarget(ctx, module, values)
	if err != nil {
		return Mutation{}, 0, err
	}
	var canonicalVersion int64
	if action == ActionUpdate {
		current, err := service.store.CanonicalValues(ctx, module, *targetID)
		if err != nil {
			return Mutation{}, 0, err
		}
		canonicalVersion, err = strconv.ParseInt(current["version"], 10, 64)
		if err != nil || canonicalVersion <= 0 {
			return Mutation{}, 0, ErrInvalidState
		}
		for field, value := range current {
			if _, mapped := effective[field]; !mapped {
				effective[field] = value
			}
		}
	}
	custom, err := normalizeCustomValues(effective, customFields)
	if err != nil {
		return Mutation{}, 0, err
	}
	normalizer, ok := moduleNormalizers[module]
	if !ok {
		return Mutation{}, 0, ErrInvalidInput
	}
	mutation, err := normalizer(ctx, service, effective)
	if err != nil {
		return Mutation{}, 0, err
	}
	mutation.CustomValues = custom
	return mutation, canonicalVersion, nil
}

func normalizeCustomValues(values map[string]string, fields []CustomFieldDefinition) ([]CustomValueMutation, error) {
	allowed := make(map[string]CustomFieldDefinition, len(fields))
	for _, field := range fields {
		allowed[field.Field.ID] = field
	}
	for key := range values {
		if strings.HasPrefix(key, CustomFieldPrefix) {
			if _, ok := allowed[key]; !ok {
				return nil, ErrInvalidMapping
			}
		}
	}
	result := make([]CustomValueMutation, 0, len(fields))
	for _, field := range fields {
		raw, mapped := values[field.Field.ID]
		if !mapped {
			continue
		}
		input, err := customValueInput(field.Definition, raw)
		if err != nil {
			return nil, err
		}
		result = append(result, CustomValueMutation{Definition: field.Definition, Value: input})
	}
	return result, nil
}

func customValueInput(definition customdata.FieldDefinition, raw string) (customdata.ValueInput, error) {
	value := strings.TrimSpace(raw)
	input := customdata.ValueInput{FieldDefinitionID: definition.ID, Kind: definition.Values.Kind}
	switch definition.Values.Kind {
	case customdata.FieldText, customdata.FieldLongText, customdata.FieldEmail, customdata.FieldPhone:
		input.Text = value
	case customdata.FieldInteger:
		if value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return customdata.ValueInput{}, ErrInvalidInput
			}
			input.Integer = &parsed
		}
	case customdata.FieldDecimal:
		input.Decimal = value
	case customdata.FieldBoolean:
		if value != "" {
			parsed, err := strconv.ParseBool(strings.ToLower(value))
			if err != nil {
				return customdata.ValueInput{}, ErrInvalidInput
			}
			input.Boolean = &parsed
		}
	case customdata.FieldCivilDate:
		input.CivilDate = value
	case customdata.FieldCivilMonth:
		input.CivilMonth = value
	default:
		return customdata.ValueInput{}, ErrInvalidMapping
	}
	normalized, err := customdata.NormalizeValue(input, definition)
	if err != nil {
		return customdata.ValueInput{}, err
	}
	return normalized, nil
}

func (service *Service) resolveTarget(ctx context.Context, module Module, values map[string]string) (*Identifier, int64, Action, error) {
	id, version, action, err := targetFromValues(values)
	if err != nil || action != ActionCreate || module != ModuleProfiles {
		return id, version, action, err
	}
	digits := profile.CanonicalCPFDigits(values["cpf"])
	if digits == "" {
		return nil, 0, ActionCreate, nil
	}
	matches, err := service.store.FindProfileIDsByCPF(ctx, digits)
	if err != nil {
		return nil, 0, "", err
	}
	if len(matches) == 0 {
		return nil, 0, ActionCreate, nil
	}
	if len(matches) > 1 {
		return nil, 0, "", ErrAmbiguousCPF
	}
	return &matches[0], 0, ActionUpdate, nil
}

func targetFromValues(values map[string]string) (*Identifier, int64, Action, error) {
	rawID := strings.TrimSpace(values["record_id"])
	rawVersion := strings.TrimSpace(values["version"])
	if rawID == "" && rawVersion == "" {
		return nil, 0, ActionCreate, nil
	}
	if rawID == "" || rawVersion == "" {
		return nil, 0, "", ErrInvalidInput
	}
	id, err := ParseIdentifier(rawID)
	if err != nil {
		return nil, 0, "", err
	}
	version, err := strconv.ParseInt(rawVersion, 10, 64)
	if err != nil || version <= 0 {
		return nil, 0, "", ErrInvalidInput
	}
	return &id, version, ActionUpdate, nil
}

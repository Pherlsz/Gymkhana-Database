package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/fingerprint"
	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/bill"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
)

type profileTargetService interface {
	Get(context.Context, auth.Session, profile.Identifier) (profile.Profile, error)
	Update(context.Context, auth.Session, profile.Identifier, int64, profile.Values, string) (profile.Profile, error)
}

type documentTargetService interface {
	Get(context.Context, auth.Session, document.Identifier) (document.Document, error)
	Update(context.Context, auth.Session, document.Identifier, int64, document.Values, string) (document.Document, error)
}

type billTargetService interface {
	Get(context.Context, auth.Session, bill.Identifier) (bill.Bill, error)
	Update(context.Context, auth.Session, bill.Identifier, int64, bill.Values, string) (bill.Bill, error)
}

type customTargetService interface {
	GetFieldDefinition(context.Context, auth.Session, customdata.Identifier) (customdata.FieldDefinition, error)
	GetValues(context.Context, auth.Session, customdata.TargetReference) (customdata.ValueSet, error)
	ReplaceValues(context.Context, auth.Session, customdata.TargetReference, int64, []customdata.ValueInput, string) (customdata.ValueSet, error)
}

type DomainTargetGateway struct {
	profiles  profileTargetService
	documents documentTargetService
	bills     billTargetService
	custom    customTargetService
}

func NewDomainTargetGateway(profiles profileTargetService, documents documentTargetService, bills billTargetService, custom customTargetService) (*DomainTargetGateway, error) {
	if profiles == nil || documents == nil || bills == nil || custom == nil {
		return nil, ErrInvalidSetup
	}
	return &DomainTargetGateway{profiles: profiles, documents: documents, bills: bills, custom: custom}, nil
}

func (gateway *DomainTargetGateway) Catalog(ctx context.Context, actor auth.Session, owner attachment.OwnerReference) (Catalog, error) {
	if gateway == nil || !owner.Valid() || !actor.User.Active {
		return Catalog{}, ErrForbidden
	}
	var fields []FieldSchema
	switch owner.Kind {
	case attachment.OwnerDocument:
		value, err := gateway.documents.Get(ctx, actor, document.Identifier(owner.ID))
		if err != nil {
			return Catalog{}, targetError(err)
		}
		fields = append(fields, documentFields(value)...)
		ownerProfile, err := gateway.profiles.Get(ctx, actor, value.Values.OwnerProfileID)
		if err != nil {
			return Catalog{}, targetError(err)
		}
		fields = append(fields, profileFields(ownerProfile)...)
	case attachment.OwnerBill:
		value, err := gateway.bills.Get(ctx, actor, bill.Identifier(owner.ID))
		if err != nil {
			return Catalog{}, targetError(err)
		}
		fields = append(fields, billFields(value)...)
		ownerProfile, err := gateway.profiles.Get(ctx, actor, value.Values.OwnerProfileID)
		if err != nil {
			return Catalog{}, targetError(err)
		}
		fields = append(fields, profileFields(ownerProfile)...)
	case attachment.OwnerCustomField:
		field, err := gateway.customField(ctx, actor, owner)
		if err != nil {
			return Catalog{}, err
		}
		fields = append(fields, field)
	default:
		return Catalog{}, ErrInvalidInput
	}
	if len(fields) == 0 || len(fields) > MaximumSuggestions {
		return Catalog{}, ErrInvalidSetup
	}
	sort.Slice(fields, func(left, right int) bool { return fields[left].Key < fields[right].Key })
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if !validFieldSchema(field) {
			return Catalog{}, ErrInvalidSetup
		}
		if _, duplicate := seen[field.Key]; duplicate {
			return Catalog{}, ErrInvalidSetup
		}
		seen[field.Key] = struct{}{}
	}
	payload, err := json.Marshal(fields)
	if err != nil {
		return Catalog{}, fmt.Errorf("fingerprint OCR target catalog: %w", err)
	}
	return Catalog{Fields: fields, Fingerprint: [32]byte(fingerprint.Sum(payload))}, nil
}

func (gateway *DomainTargetGateway) CurrentField(ctx context.Context, actor auth.Session, field FieldSchema) (CurrentField, error) {
	if gateway == nil || !validFieldSchema(field) || !actor.User.Active {
		return CurrentField{}, ErrForbidden
	}
	switch field.Target.Kind {
	case TargetProfile:
		value, err := gateway.profiles.Get(ctx, actor, profile.Identifier(field.Target.ID))
		if err != nil {
			return CurrentField{}, targetError(err)
		}
		current, ok := profileFieldValue(value.Values, field.Key)
		if !ok {
			return CurrentField{}, ErrInvalidInput
		}
		return CurrentField{Value: current, Version: value.Version}, nil
	case TargetDocument:
		value, err := gateway.documents.Get(ctx, actor, document.Identifier(field.Target.ID))
		if err != nil {
			return CurrentField{}, targetError(err)
		}
		current, ok := documentFieldValue(value.Values, field.Key)
		if !ok {
			return CurrentField{}, ErrInvalidInput
		}
		return CurrentField{Value: current, Version: value.Version}, nil
	case TargetBill:
		value, err := gateway.bills.Get(ctx, actor, bill.Identifier(field.Target.ID))
		if err != nil {
			return CurrentField{}, targetError(err)
		}
		current, ok := billFieldValue(value.Values, field.Key)
		if !ok {
			return CurrentField{}, ErrInvalidInput
		}
		return CurrentField{Value: current, Version: value.Version}, nil
	case TargetCustomField:
		target, definitionID, err := customFieldReferences(field)
		if err != nil {
			return CurrentField{}, err
		}
		values, err := gateway.custom.GetValues(ctx, actor, target)
		if err != nil {
			return CurrentField{}, targetError(err)
		}
		for _, stored := range values.Values {
			if stored.Input.FieldDefinitionID == definitionID {
				return CurrentField{Value: customValueString(stored.Input), Version: values.Version}, nil
			}
		}
		return CurrentField{Version: values.Version}, nil
	default:
		return CurrentField{}, ErrInvalidInput
	}
}

func (gateway *DomainTargetGateway) ApplyTarget(ctx context.Context, actor auth.Session, target TargetReference, expectedVersion int64, changes []ApprovedChange, requestID string) (int64, error) {
	if gateway == nil || !target.Valid() || expectedVersion < 1 || len(changes) == 0 || len(changes) > MaximumSuggestions || !actor.User.Active {
		return 0, ErrInvalidInput
	}
	for _, change := range changes {
		if change.Field.Target != target || change.SuggestionID.IsZero() || !validFieldSchema(change.Field) {
			return 0, ErrInvalidInput
		}
		if _, err := normalizeValue(change.Field.Kind, change.Value, !change.Field.Required); err != nil {
			return 0, err
		}
	}
	switch target.Kind {
	case TargetProfile:
		value, err := gateway.profiles.Get(ctx, actor, profile.Identifier(target.ID))
		if err != nil {
			return 0, targetError(err)
		}
		if value.Version != expectedVersion {
			return 0, ErrStaleTarget
		}
		for _, change := range changes {
			if !setProfileField(&value.Values, change.Field.Key, change.Value) {
				return 0, ErrInvalidInput
			}
		}
		updated, err := gateway.profiles.Update(ctx, actor, value.ID, expectedVersion, value.Values, requestID)
		if err != nil {
			return 0, targetError(err)
		}
		return updated.Version, nil
	case TargetDocument:
		value, err := gateway.documents.Get(ctx, actor, document.Identifier(target.ID))
		if err != nil {
			return 0, targetError(err)
		}
		if value.Version != expectedVersion {
			return 0, ErrStaleTarget
		}
		for _, change := range changes {
			if !setDocumentField(&value.Values, change.Field.Key, change.Value) {
				return 0, ErrInvalidInput
			}
		}
		updated, err := gateway.documents.Update(ctx, actor, value.ID, expectedVersion, value.Values, requestID)
		if err != nil {
			return 0, targetError(err)
		}
		return updated.Version, nil
	case TargetBill:
		value, err := gateway.bills.Get(ctx, actor, bill.Identifier(target.ID))
		if err != nil {
			return 0, targetError(err)
		}
		if value.Version != expectedVersion {
			return 0, ErrStaleTarget
		}
		for _, change := range changes {
			if !setBillField(&value.Values, change.Field.Key, change.Value) {
				return 0, ErrInvalidInput
			}
		}
		updated, err := gateway.bills.Update(ctx, actor, value.ID, expectedVersion, value.Values, requestID)
		if err != nil {
			return 0, targetError(err)
		}
		return updated.Version, nil
	case TargetCustomField:
		return gateway.applyCustomFields(ctx, actor, target, expectedVersion, changes, requestID)
	default:
		return 0, ErrInvalidInput
	}
}

func (gateway *DomainTargetGateway) customField(ctx context.Context, actor auth.Session, owner attachment.OwnerReference) (FieldSchema, error) {
	definitionID := customdata.Identifier(owner.FieldDefinitionID)
	definition, err := gateway.custom.GetFieldDefinition(ctx, actor, definitionID)
	if err != nil {
		return FieldSchema{}, targetError(err)
	}
	kind, ok := customValueKind(definition.Values.Kind)
	keyPrefix, targetMatches := customTargetKey(owner.CustomTargetKind, definition.Values.TargetKind)
	if !ok || !definition.Values.Active || !targetMatches {
		return FieldSchema{}, ErrInvalidInput
	}
	target, err := customTargetReference(owner.CustomTargetKind, owner.ID)
	if err != nil {
		return FieldSchema{}, err
	}
	values, err := gateway.custom.GetValues(ctx, actor, target)
	if err != nil {
		return FieldSchema{}, targetError(err)
	}
	return FieldSchema{
		Key: keyPrefix + definition.ID.String(), Label: definition.Values.Label, Kind: kind,
		Required: definition.Values.Required,
		Target:   TargetReference{Kind: TargetCustomField, ID: Identifier(owner.ID)}, TargetVersion: values.Version,
	}, nil
}

func (gateway *DomainTargetGateway) applyCustomFields(ctx context.Context, actor auth.Session, target TargetReference, expectedVersion int64, changes []ApprovedChange, requestID string) (int64, error) {
	if len(changes) == 0 {
		return 0, ErrInvalidInput
	}
	customTarget, _, err := customFieldReferences(changes[0].Field)
	if err != nil {
		return 0, err
	}
	values, err := gateway.custom.GetValues(ctx, actor, customTarget)
	if err != nil {
		return 0, targetError(err)
	}
	if values.Version != expectedVersion {
		return 0, ErrStaleTarget
	}
	inputs := make([]customdata.ValueInput, 0, len(values.Values)+len(changes))
	positions := make(map[customdata.Identifier]int, len(values.Values))
	for _, stored := range values.Values {
		positions[stored.Input.FieldDefinitionID] = len(inputs)
		inputs = append(inputs, stored.Input)
	}
	for _, change := range changes {
		candidateTarget, definitionID, err := customFieldReferences(change.Field)
		if err != nil || candidateTarget != customTarget {
			return 0, ErrInvalidInput
		}
		definition, err := gateway.custom.GetFieldDefinition(ctx, actor, definitionID)
		if err != nil {
			return 0, targetError(err)
		}
		input, err := customValueInput(definition, change.Value)
		if err != nil {
			return 0, err
		}
		if position, exists := positions[definitionID]; exists {
			inputs[position] = input
		} else {
			positions[definitionID] = len(inputs)
			inputs = append(inputs, input)
		}
	}
	updated, err := gateway.custom.ReplaceValues(ctx, actor, customTarget, expectedVersion, inputs, requestID)
	if err != nil {
		return 0, targetError(err)
	}
	return updated.Version, nil
}

func profileFields(value profile.Profile) []FieldSchema {
	target := TargetReference{Kind: TargetProfile, ID: Identifier(value.ID)}
	return []FieldSchema{
		{Key: "profile.full_name", Label: "Nome completo", Kind: ValueText, Required: true, Target: target, TargetVersion: value.Version},
		{Key: "profile.social_name", Label: "Nome social", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.cpf", Label: "CPF", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.email", Label: "E-mail", Kind: ValueEmail, Target: target, TargetVersion: value.Version},
		{Key: "profile.mobile_phone", Label: "Celular", Kind: ValuePhone, Target: target, TargetVersion: value.Version},
		{Key: "profile.landline_phone", Label: "Telefone", Kind: ValuePhone, Target: target, TargetVersion: value.Version},
		{Key: "profile.address.street", Label: "Logradouro", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.address.number", Label: "Número", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.address.complement", Label: "Complemento", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.address.neighborhood", Label: "Bairro", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.address.city", Label: "Cidade", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.address.state", Label: "UF", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.address.postal_code", Label: "CEP", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "profile.notes", Label: "Observações", Kind: ValueLongText, Target: target, TargetVersion: value.Version},
	}
}

func documentFields(value document.Document) []FieldSchema {
	target := TargetReference{Kind: TargetDocument, ID: Identifier(value.ID)}
	return []FieldSchema{
		{Key: "document.identifier", Label: value.Type.Values.Label, Kind: ValueText, Required: true, Target: target, TargetVersion: value.Version},
		{Key: "document.document_date", Label: "Data do documento", Kind: ValueCivilDate, Required: value.Type.Values.DateRequired, Target: target, TargetVersion: value.Version},
		{Key: "document.notes", Label: "Observações do documento", Kind: ValueLongText, Target: target, TargetVersion: value.Version},
	}
}

func billFields(value bill.Bill) []FieldSchema {
	target := TargetReference{Kind: TargetBill, ID: Identifier(value.ID)}
	return []FieldSchema{
		{Key: "bill.printed_holder_name", Label: "Titular impresso", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "bill.printed_address", Label: "Endereço impresso", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "bill.reference", Label: "Referência", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "bill.competence", Label: "Competência", Kind: ValueCivilMonth, Target: target, TargetVersion: value.Version},
		{Key: "bill.amount", Label: "Valor", Kind: ValueDecimal, Target: target, TargetVersion: value.Version},
		{Key: "bill.currency", Label: "Moeda", Kind: ValueText, Target: target, TargetVersion: value.Version},
		{Key: "bill.notes", Label: "Observações da conta", Kind: ValueLongText, Target: target, TargetVersion: value.Version},
	}
}

func profileFieldValue(values profile.Values, key string) (string, bool) {
	switch key {
	case "profile.full_name":
		return values.FullName, true
	case "profile.social_name":
		return values.SocialName, true
	case "profile.cpf":
		return values.CPF, true
	case "profile.email":
		return values.Email, true
	case "profile.mobile_phone":
		return values.MobilePhone, true
	case "profile.landline_phone":
		return values.LandlinePhone, true
	case "profile.address.street":
		return values.Address.Street, true
	case "profile.address.number":
		return values.Address.Number, true
	case "profile.address.complement":
		return values.Address.Complement, true
	case "profile.address.neighborhood":
		return values.Address.Neighborhood, true
	case "profile.address.city":
		return values.Address.City, true
	case "profile.address.state":
		return values.Address.State, true
	case "profile.address.postal_code":
		return values.Address.PostalCode, true
	case "profile.notes":
		return values.Notes, true
	default:
		return "", false
	}
}

func setProfileField(values *profile.Values, key, value string) bool {
	switch key {
	case "profile.full_name":
		values.FullName = value
	case "profile.social_name":
		values.SocialName = value
	case "profile.cpf":
		values.CPF = value
	case "profile.email":
		values.Email = value
	case "profile.mobile_phone":
		values.MobilePhone = value
	case "profile.landline_phone":
		values.LandlinePhone = value
	case "profile.address.street":
		values.Address.Street = value
	case "profile.address.number":
		values.Address.Number = value
	case "profile.address.complement":
		values.Address.Complement = value
	case "profile.address.neighborhood":
		values.Address.Neighborhood = value
	case "profile.address.city":
		values.Address.City = value
	case "profile.address.state":
		values.Address.State = value
	case "profile.address.postal_code":
		values.Address.PostalCode = value
	case "profile.notes":
		values.Notes = value
	default:
		return false
	}
	return true
}

func documentFieldValue(values document.Values, key string) (string, bool) {
	switch key {
	case "document.identifier":
		return values.Identifier, true
	case "document.document_date":
		return values.DocumentDate, true
	case "document.notes":
		return values.Notes, true
	default:
		return "", false
	}
}

func setDocumentField(values *document.Values, key, value string) bool {
	switch key {
	case "document.identifier":
		values.Identifier = value
	case "document.document_date":
		values.DocumentDate = value
	case "document.notes":
		values.Notes = value
	default:
		return false
	}
	return true
}

func billFieldValue(values bill.Values, key string) (string, bool) {
	switch key {
	case "bill.printed_holder_name":
		return values.PrintedHolderName, true
	case "bill.printed_address":
		return values.PrintedAddress, true
	case "bill.reference":
		return values.Reference, true
	case "bill.competence":
		return values.Competence, true
	case "bill.amount":
		return values.Amount, true
	case "bill.currency":
		return values.Currency, true
	case "bill.notes":
		return values.Notes, true
	default:
		return "", false
	}
}

func setBillField(values *bill.Values, key, value string) bool {
	switch key {
	case "bill.printed_holder_name":
		values.PrintedHolderName = value
	case "bill.printed_address":
		values.PrintedAddress = value
	case "bill.reference":
		values.Reference = value
	case "bill.competence":
		values.Competence = value
	case "bill.amount":
		values.Amount = value
	case "bill.currency":
		values.Currency = value
	case "bill.notes":
		values.Notes = value
	default:
		return false
	}
	return true
}

func customTargetReference(kind attachment.CustomTargetKind, id attachment.Identifier) (customdata.TargetReference, error) {
	mapped := customdata.TargetReference{ID: customdata.Identifier(id)}
	switch kind {
	case attachment.CustomTargetProfile:
		mapped.Kind = customdata.ValueTargetProfile
	case attachment.CustomTargetDocument:
		mapped.Kind = customdata.ValueTargetDocument
	case attachment.CustomTargetBill:
		mapped.Kind = customdata.ValueTargetBill
	case attachment.CustomTargetCustomEntity:
		mapped.Kind = customdata.ValueTargetCustomEntity
	default:
		return customdata.TargetReference{}, ErrInvalidInput
	}
	return mapped, nil
}

func customFieldReferences(field FieldSchema) (customdata.TargetReference, customdata.Identifier, error) {
	if field.Target.Kind != TargetCustomField {
		return customdata.TargetReference{}, customdata.Identifier{}, ErrInvalidInput
	}
	parts := strings.Split(field.Key, ".")
	if len(parts) != 3 || parts[0] != "custom" {
		return customdata.TargetReference{}, customdata.Identifier{}, ErrInvalidInput
	}
	definitionID, err := customdata.ParseIdentifier(parts[2])
	if err != nil {
		return customdata.TargetReference{}, customdata.Identifier{}, ErrInvalidInput
	}
	target := customdata.TargetReference{ID: customdata.Identifier(field.Target.ID)}
	switch parts[1] {
	case "profile":
		target.Kind = customdata.ValueTargetProfile
	case "document":
		target.Kind = customdata.ValueTargetDocument
	case "bill":
		target.Kind = customdata.ValueTargetBill
	case "entity":
		target.Kind = customdata.ValueTargetCustomEntity
	default:
		return customdata.TargetReference{}, customdata.Identifier{}, ErrInvalidInput
	}
	return target, definitionID, nil
}

func customTargetKey(ownerKind attachment.CustomTargetKind, definitionKind customdata.TargetKind) (string, bool) {
	switch ownerKind {
	case attachment.CustomTargetProfile:
		return "custom.profile.", definitionKind == customdata.TargetProfile
	case attachment.CustomTargetDocument:
		return "custom.document.", definitionKind == customdata.TargetDocumentType
	case attachment.CustomTargetBill:
		return "custom.bill.", definitionKind == customdata.TargetBillType
	case attachment.CustomTargetCustomEntity:
		return "custom.entity.", definitionKind == customdata.TargetCustomEntityType
	default:
		return "", false
	}
}

func customValueKind(kind customdata.FieldKind) (ValueKind, bool) {
	switch kind {
	case customdata.FieldText:
		return ValueText, true
	case customdata.FieldLongText:
		return ValueLongText, true
	case customdata.FieldInteger:
		return ValueInteger, true
	case customdata.FieldDecimal:
		return ValueDecimal, true
	case customdata.FieldBoolean:
		return ValueBoolean, true
	case customdata.FieldCivilDate:
		return ValueCivilDate, true
	case customdata.FieldCivilMonth:
		return ValueCivilMonth, true
	case customdata.FieldEmail:
		return ValueEmail, true
	case customdata.FieldPhone:
		return ValuePhone, true
	default:
		return "", false
	}
}

func customValueString(input customdata.ValueInput) string {
	switch input.Kind {
	case customdata.FieldText, customdata.FieldLongText, customdata.FieldEmail, customdata.FieldPhone:
		return input.Text
	case customdata.FieldInteger:
		if input.Integer != nil {
			return strconv.FormatInt(*input.Integer, 10)
		}
	case customdata.FieldDecimal:
		return input.Decimal
	case customdata.FieldBoolean:
		if input.Boolean != nil {
			return strconv.FormatBool(*input.Boolean)
		}
	case customdata.FieldCivilDate:
		return input.CivilDate
	case customdata.FieldCivilMonth:
		return input.CivilMonth
	}
	return ""
}

func customValueInput(definition customdata.FieldDefinition, value string) (customdata.ValueInput, error) {
	input := customdata.ValueInput{FieldDefinitionID: definition.ID, Kind: definition.Values.Kind}
	switch definition.Values.Kind {
	case customdata.FieldText, customdata.FieldLongText, customdata.FieldEmail, customdata.FieldPhone:
		input.Text = value
	case customdata.FieldInteger:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return customdata.ValueInput{}, ErrInvalidInput
		}
		input.Integer = &parsed
	case customdata.FieldDecimal:
		input.Decimal = value
	case customdata.FieldBoolean:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return customdata.ValueInput{}, ErrInvalidInput
		}
		input.Boolean = &parsed
	case customdata.FieldCivilDate:
		input.CivilDate = value
	case customdata.FieldCivilMonth:
		input.CivilMonth = value
	default:
		return customdata.ValueInput{}, ErrInvalidInput
	}
	return input, nil
}

func targetError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, profile.ErrForbidden), errors.Is(err, document.ErrForbidden), errors.Is(err, bill.ErrForbidden), errors.Is(err, customdata.ErrForbidden):
		return ErrForbidden
	case errors.Is(err, profile.ErrNotFound), errors.Is(err, document.ErrNotFound), errors.Is(err, bill.ErrNotFound), errors.Is(err, customdata.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, profile.ErrConflict), errors.Is(err, document.ErrConflict), errors.Is(err, bill.ErrConflict), errors.Is(err, customdata.ErrConflict):
		return ErrStaleTarget
	}
	var profileValidation *profile.ValidationError
	var documentValidation *document.ValidationError
	var billValidation *bill.ValidationError
	var customValidation *customdata.ValidationError
	if errors.As(err, &profileValidation) || errors.As(err, &documentValidation) || errors.As(err, &billValidation) || errors.As(err, &customValidation) {
		return ErrInvalidInput
	}
	return err
}

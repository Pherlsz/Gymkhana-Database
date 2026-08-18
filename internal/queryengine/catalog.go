package queryengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type sqlEntityDefinition struct {
	Public            EntityDefinition
	FromTemplate      string
	IDExpression      string
	LabelExpression   string
	UpdatedExpression string
	BaseCondition     string
}

type sqlFieldDefinition struct {
	Public     FieldDefinition
	Expression string
}

type sqlRelationDefinition struct {
	Public          RelationDefinition
	JoinCondition   string
	TargetEntityKey string
}

type resolvedCatalog struct {
	Public    Catalog
	Entities  map[string]sqlEntityDefinition
	Fields    map[string]sqlFieldDefinition
	Relations map[string]sqlRelationDefinition
}

func loadCatalog(ctx context.Context, store Store, role auth.Role) (resolvedCatalog, error) {
	if !role.Valid() || !role.CanSearch() {
		return resolvedCatalog{}, ErrForbidden
	}
	dynamic, err := store.CatalogDefinitions(ctx)
	if err != nil {
		return resolvedCatalog{}, err
	}
	resolved := resolvedCatalog{
		Entities: make(map[string]sqlEntityDefinition),
		Fields:   make(map[string]sqlFieldDefinition), Relations: make(map[string]sqlRelationDefinition),
	}
	if role.CanReadProfiles() {
		resolved.addEntity(sqlEntityDefinition{Public: EntityDefinition{Key: "profiles", Label: "Pessoas", Kind: "profile", Navigable: true, DefaultSort: "profile.full_name"},
			FromTemplate: "profiles {root}", IDExpression: "{root}.id::text", LabelExpression: "{root}.full_name", UpdatedExpression: "{root}.updated_at"})
	}
	if role.CanReadDocuments() {
		resolved.addEntity(sqlEntityDefinition{Public: EntityDefinition{Key: "documents", Label: "Documentos", Kind: "document", Navigable: true, DefaultSort: "document.identifier"},
			FromTemplate: `documents {root}
JOIN document_presences {presence} ON {presence}.id={root}.presence_id
JOIN document_types {type} ON {type}.id={presence}.document_type_id
JOIN profiles {owner} ON {owner}.id={presence}.profile_id
LEFT JOIN document_current_uses {current} ON {current}.document_id={root}.id
LEFT JOIN profiles {holder} ON {holder}.id={current}.holder_profile_id`,
			IDExpression: "{root}.id::text", LabelExpression: "concat({type}.label, ' · ', COALESCE({presence}.identifier_value, ''))", UpdatedExpression: "{root}.updated_at"})
	}
	if role.CanReadBills() {
		resolved.addEntity(sqlEntityDefinition{Public: EntityDefinition{Key: "bills", Label: "Contas e comprovantes", Kind: "bill", Navigable: true, DefaultSort: "bill.updated_at"},
			FromTemplate: `bills {root}
JOIN bill_types {type} ON {type}.id={root}.bill_type_id
JOIN profiles {owner} ON {owner}.id={root}.owner_profile_id
LEFT JOIN bill_current_uses {current} ON {current}.bill_id={root}.id
LEFT JOIN profiles {holder} ON {holder}.id={current}.holder_profile_id`,
			IDExpression: "{root}.id::text", LabelExpression: "concat({type}.label, ' · ', COALESCE({root}.reference_value, {root}.competence, {root}.id::text))", UpdatedExpression: "{root}.updated_at"})
	}
	if role.CanReadAttachments() {
		resolved.addEntity(sqlEntityDefinition{Public: EntityDefinition{Key: "attachments", Label: "Anexos", Kind: "attachment", Navigable: false, DefaultSort: "attachment.updated_at"},
			FromTemplate: "attachments {root}", IDExpression: "{root}.id::text", LabelExpression: "{root}.original_filename", UpdatedExpression: "{root}.updated_at", BaseCondition: "{root}.lifecycle_state='ACTIVE'"})
	}

	entityTypeByID := make(map[string]DynamicEntityDefinition, len(dynamic.Entities))
	if role.CanReadCustomData() {
		for _, value := range dynamic.Entities {
			if !validUUID(value.ID) || !validLogicalSegment(value.TechnicalKey) || strings.TrimSpace(value.Label) == "" {
				continue
			}
			key := "custom_entity." + value.ID
			labelLiteral := quoteSQLLiteral(truncateRunes(value.Label, 120))
			resolved.addEntity(sqlEntityDefinition{Public: EntityDefinition{Key: key, Label: truncateRunes(value.Label, 120), Kind: key, Navigable: true, DefaultSort: key + ".updated_at"},
				FromTemplate: `custom_entities {root}
JOIN custom_entity_types {type} ON {type}.id={root}.custom_entity_type_id
LEFT JOIN profiles {owner} ON {owner}.id={root}.owner_profile_id`,
				IDExpression: "{root}.id::text", LabelExpression: fmt.Sprintf("concat(%s, ' · ', left({root}.id::text, 8))", labelLiteral),
				UpdatedExpression: "{root}.updated_at", BaseCondition: fmt.Sprintf("{root}.custom_entity_type_id='%s'::uuid", value.ID)})
			entityTypeByID[value.ID] = value
		}
	}

	addStaticFields(&resolved)
	if role.CanReadCustomData() {
		for _, field := range dynamic.Fields {
			resolved.addDynamicField(field, entityTypeByID)
		}
	}
	addRelations(&resolved, entityTypeByID)
	resolved.finalize()
	return resolved, nil
}

func addStaticFields(catalog *resolvedCatalog) {
	fields := []sqlFieldDefinition{
		field("profile.id", "profiles", "ID", ValueIdentifier, false, true, true, true, "{root}.id"),
		field("profile.full_name", "profiles", "Nome completo", ValueText, false, true, true, true, "{root}.full_name"),
		field("profile.social_name", "profiles", "Nome social", ValueText, true, true, true, true, "{root}.social_name"),
		field("profile.cpf", "profiles", "CPF", ValueIdentifier, true, true, true, true, `(SELECT presence.identifier_value FROM document_presences presence JOIN document_types document_type ON document_type.id=presence.document_type_id WHERE presence.profile_id={root}.id AND document_type.technical_key='cpf' AND presence.claim='informed_number' LIMIT 1)`),
		field("profile.email", "profiles", "E-mail", ValueText, true, true, true, true, "{root}.email"),
		field("profile.mobile_phone", "profiles", "Celular", ValueIdentifier, true, true, true, true, "{root}.mobile_phone"),
		field("profile.landline_phone", "profiles", "Telefone", ValueIdentifier, true, true, true, true, "{root}.landline_phone"),
		field("profile.address_street", "profiles", "Logradouro", ValueText, true, true, true, true, "{root}.address_street"),
		field("profile.address_number", "profiles", "Número", ValueText, true, true, true, true, "{root}.address_number"),
		field("profile.address_complement", "profiles", "Complemento", ValueText, true, true, true, false, "{root}.address_complement"),
		field("profile.address_neighborhood", "profiles", "Bairro", ValueText, true, true, true, true, "{root}.address_neighborhood"),
		field("profile.address_city", "profiles", "Cidade", ValueText, true, true, true, true, "{root}.address_city"),
		field("profile.address_state", "profiles", "UF", ValueEnum, true, true, true, true, "{root}.address_state"),
		field("profile.address_postal_code", "profiles", "CEP", ValueIdentifier, true, true, true, true, "{root}.address_postal_code"),
		field("profile.notes", "profiles", "Observações", ValueLongText, true, true, true, false, "{root}.notes"),
		field("profile.team", "profiles", "Equipe", ValueText, true, true, true, true, "{root}.team"),
		field("profile.club_membership", "profiles", "Sócio clube", ValueText, true, true, true, true, "{root}.club_membership"),
		field("profile.membership_type", "profiles", "Categoria de sócio", ValueText, true, true, true, true, "{root}.membership_type"),
		field("profile.place_of_origin", "profiles", "Naturalidade", ValueText, true, true, true, true, "{root}.place_of_origin"),
		field("profile.birth_country", "profiles", "País de nascimento", ValueText, true, true, true, true, "{root}.birth_country"),
		field("profile.parents_wedding_date", "profiles", "Casamento dos pais", ValueCivilDate, true, true, true, true, "{root}.parents_wedding_date"),
		field("profile.supermarket_club", "profiles", "Clube de supermercado", ValueText, true, true, true, true, "{root}.supermarket_club"),
		field("profile.pet", "profiles", "Animal", ValueText, true, true, true, true, "{root}.pet"),
		field("profile.travel_countries", "profiles", "Viagem", ValueLongText, true, true, true, false, "{root}.travel_countries"),
		field("profile.card_brand", "profiles", "Bandeira do cartão", ValueText, true, true, true, true, "{root}.card_brand"),
		field("profile.card_bank", "profiles", "Banco do cartão", ValueText, true, true, true, true, "{root}.card_bank"),
		field("profile.birth_date", "profiles", "Data de nascimento", ValueCivilDate, true, true, true, true, "{root}.birth_date"),
		field("profile.gender", "profiles", "Sexo", ValueText, true, true, true, true, "{root}.gender"),
		field("profile.blood_type", "profiles", "Tipo sanguíneo", ValueText, true, true, true, true, "{root}.blood_type"),
		field("profile.nationality", "profiles", "Nacionalidade", ValueText, true, true, true, true, "{root}.nationality"),
		field("profile.birth_city", "profiles", "Cidade de nascimento", ValueText, true, true, true, true, "{root}.birth_city"),
		field("profile.marital_status", "profiles", "Estado civil", ValueText, true, true, true, true, "{root}.marital_status"),
		field("profile.wedding_date", "profiles", "Data de casamento", ValueCivilDate, true, true, true, true, "{root}.wedding_date"),
		field("profile.father_name", "profiles", "Nome do pai", ValueText, true, true, true, true, "{root}.father_name"),
		field("profile.father_birth_date", "profiles", "Nascimento do pai", ValueCivilDate, true, true, true, true, "{root}.father_birth_date"),
		field("profile.mother_name", "profiles", "Nome da mãe", ValueText, true, true, true, true, "{root}.mother_name"),
		field("profile.mother_birth_date", "profiles", "Nascimento da mãe", ValueCivilDate, true, true, true, true, "{root}.mother_birth_date"),
		field("profile.health_plan", "profiles", "Plano de saúde", ValueText, true, true, true, true, "{root}.health_plan"),
		field("profile.blood_donor", "profiles", "Doador de sangue", ValueBoolean, true, true, true, true, "{root}.blood_donor"),
		field("profile.organ_donor", "profiles", "Doador de órgãos", ValueBoolean, true, true, true, true, "{root}.organ_donor"),
		field("profile.sector", "profiles", "Setor", ValueText, true, true, true, true, "{root}.sector"),
		field("profile.collections", "profiles", "Coleções", ValueLongText, true, true, true, false, "{root}.collections"),
		field("profile.vehicle_model", "profiles", "Modelo do veículo", ValueText, true, true, true, true, "{root}.vehicle_model"),
		field("profile.vehicle_color", "profiles", "Cor do veículo", ValueText, true, true, true, true, "{root}.vehicle_color"),
		field("profile.vehicle_plate", "profiles", "Placa", ValueIdentifier, true, true, true, true, "{root}.vehicle_plate"),
		field("profile.vehicle_year", "profiles", "Ano do veículo", ValueInteger, true, true, true, true, "{root}.vehicle_year"),
		field("profile.updated_at", "profiles", "Atualizado em", ValueTimestamp, false, true, true, true, "{root}.updated_at"),

		field("document.id", "documents", "ID", ValueIdentifier, false, true, true, true, "{root}.id"),
		field("document.type", "documents", "Tipo", ValueText, false, true, true, true, "{type}.label"),
		field("document.identifier", "documents", "Identificador", ValueIdentifier, false, true, true, true, "{presence}.identifier_value"),
		field("document.date", "documents", "Data", ValueCivilDate, true, true, true, true, "{root}.document_date"),
		field("document.valid_until", "documents", "Validade", ValueCivilDate, true, true, true, true, "{root}.valid_until"),
		field("document.notes", "documents", "Observações", ValueLongText, true, true, true, false, "{root}.notes"),
		field("document.medium", "documents", "Meio", ValueEnum, false, true, true, true, "{root}.medium"),
		field("document.idle_custody", "documents", "Guarda", ValueEnum, true, true, true, true, "{root}.idle_custody"),
		field("document.owner_profile_id", "documents", "ID da pessoa proprietária", ValueIdentifier, false, true, true, true, "{presence}.profile_id"),
		field("document.owner_name", "documents", "Pessoa proprietária", ValueText, false, true, true, true, "{owner}.full_name"),
		field("document.current_holder_id", "documents", "ID da pessoa em uso", ValueIdentifier, true, true, true, true, "{current}.holder_profile_id"),
		field("document.current_holder_name", "documents", "Pessoa em uso", ValueText, true, true, true, true, "{holder}.full_name"),
		field("document.updated_at", "documents", "Atualizado em", ValueTimestamp, false, true, true, true, "{root}.updated_at"),

		field("bill.id", "bills", "ID", ValueIdentifier, false, true, true, true, "{root}.id"),
		field("bill.type", "bills", "Tipo", ValueText, false, true, true, true, "{type}.label"),
		field("bill.printed_holder_name", "bills", "Titular impresso", ValueText, true, true, true, true, "{root}.printed_holder_name"),
		field("bill.printed_address", "bills", "Endereço impresso", ValueText, true, true, true, true, "{root}.printed_address"),
		field("bill.reference", "bills", "Referência", ValueIdentifier, true, true, true, true, "{root}.reference_value"),
		field("bill.competence", "bills", "Competência", ValueCivilMonth, true, true, true, true, "{root}.competence"),
		field("bill.amount", "bills", "Valor", ValueDecimal, true, true, true, true, "{root}.amount"),
		field("bill.currency", "bills", "Moeda", ValueEnum, true, true, true, true, "{root}.currency"),
		field("bill.notes", "bills", "Observações", ValueLongText, true, true, true, false, "{root}.notes"),
		field("bill.medium", "bills", "Meio", ValueEnum, false, true, true, true, "{root}.medium"),
		field("bill.idle_custody", "bills", "Guarda", ValueEnum, true, true, true, true, "{root}.idle_custody"),
		field("bill.owner_profile_id", "bills", "ID da pessoa proprietária", ValueIdentifier, false, true, true, true, "{root}.owner_profile_id"),
		field("bill.owner_name", "bills", "Pessoa proprietária", ValueText, false, true, true, true, "{owner}.full_name"),
		field("bill.current_holder_id", "bills", "ID da pessoa em uso", ValueIdentifier, true, true, true, true, "{current}.holder_profile_id"),
		field("bill.current_holder_name", "bills", "Pessoa em uso", ValueText, true, true, true, true, "{holder}.full_name"),
		field("bill.updated_at", "bills", "Atualizado em", ValueTimestamp, false, true, true, true, "{root}.updated_at"),

		field("attachment.id", "attachments", "ID", ValueIdentifier, false, true, true, true, "{root}.id"),
		field("attachment.filename", "attachments", "Nome do arquivo", ValueText, false, true, true, true, "{root}.original_filename"),
		field("attachment.declared_mime", "attachments", "Tipo declarado", ValueText, false, true, true, true, "{root}.declared_mime"),
		field("attachment.detected_mime", "attachments", "Tipo detectado", ValueText, false, true, true, true, "{root}.detected_mime"),
		field("attachment.byte_size", "attachments", "Tamanho em bytes", ValueInteger, false, true, true, true, "{root}.byte_size"),
		field("attachment.owner_kind", "attachments", "Tipo de vínculo", ValueEnum, false, true, true, true, "{root}.owner_kind"),
		field("attachment.updated_at", "attachments", "Atualizado em", ValueTimestamp, false, true, true, true, "{root}.updated_at"),
	}
	for _, value := range fields {
		if _, exists := catalog.Entities[value.Public.Entity]; exists {
			catalog.addField(value)
		}
	}
	for entityKey := range catalog.Entities {
		if !strings.HasPrefix(entityKey, "custom_entity.") {
			continue
		}
		catalog.addField(field(entityKey+".id", entityKey, "ID", ValueIdentifier, false, true, true, true, "{root}.id"))
		catalog.addField(field(entityKey+".owner_profile_id", entityKey, "ID da pessoa proprietária", ValueIdentifier, true, true, true, true, "{root}.owner_profile_id"))
		catalog.addField(field(entityKey+".owner_name", entityKey, "Pessoa proprietária", ValueText, true, true, true, true, "{owner}.full_name"))
		catalog.addField(field(entityKey+".updated_at", entityKey, "Atualizado em", ValueTimestamp, false, true, true, true, "{root}.updated_at"))
	}
}

func (catalog *resolvedCatalog) addDynamicField(value DynamicFieldDefinition, entityTypes map[string]DynamicEntityDefinition) {
	if !validUUID(value.ID) || !validLogicalSegment(value.TechnicalKey) || strings.TrimSpace(value.Label) == "" {
		return
	}
	entityKey := ""
	targetColumn := ""
	switch value.TargetKind {
	case "PROFILE":
		entityKey, targetColumn = "profiles", "profile_id"
	case "DOCUMENT_TYPE":
		if !validUUID(value.DocumentTypeID) {
			return
		}
		entityKey, targetColumn = "documents", "document_id"
	case "BILL_TYPE":
		if !validUUID(value.BillTypeID) {
			return
		}
		entityKey, targetColumn = "bills", "bill_id"
	case "CUSTOM_ENTITY_TYPE":
		if _, ok := entityTypes[value.CustomEntityTypeID]; !ok {
			return
		}
		entityKey, targetColumn = "custom_entity."+value.CustomEntityTypeID, "custom_entity_id"
	default:
		return
	}
	if _, ok := catalog.Entities[entityKey]; !ok {
		return
	}
	kind, ok := customValueKind(value.FieldKind)
	if !ok {
		return
	}
	expression := customValueExpression(value.ID, targetColumn, kind)
	definition := field("custom."+value.ID, entityKey, truncateRunes(value.Label, 160), kind, true, true, true, kind != ValueLongText, expression)
	definition.Public.Options = normalizedOptions(value.Options)
	catalog.addField(definition)
}

func addRelations(catalog *resolvedCatalog, entityTypes map[string]DynamicEntityDefinition) {
	relations := []sqlRelationDefinition{
		relation("profile.documents", "profiles", "documents", "Documentos da pessoa", CardinalityMany, "{to.presence}.profile_id={from.root}.id"),
		relation("profile.bills", "profiles", "bills", "Contas da pessoa", CardinalityMany, "{to.root}.owner_profile_id={from.root}.id"),
		relation("profile.attachments", "profiles", "attachments", "Anexos da pessoa", CardinalityMany, "{to.root}.custom_profile_id={from.root}.id"),
		relation("document.owner", "documents", "profiles", "Pessoa proprietária", CardinalityOne, "{to.root}.id={from.presence}.profile_id"),
		relation("document.current_holder", "documents", "profiles", "Pessoa em uso", CardinalityOne, "{to.root}.id={from.current}.holder_profile_id"),
		relation("bill.owner", "bills", "profiles", "Pessoa proprietária", CardinalityOne, "{to.root}.id={from.root}.owner_profile_id"),
		relation("bill.current_holder", "bills", "profiles", "Pessoa em uso", CardinalityOne, "{to.root}.id={from.current}.holder_profile_id"),
		relation("document.attachments", "documents", "attachments", "Anexos do documento", CardinalityMany, "({to.root}.document_id={from.root}.id OR {to.root}.custom_document_id={from.root}.id)"),
		relation("bill.attachments", "bills", "attachments", "Anexos da conta", CardinalityMany, "({to.root}.bill_id={from.root}.id OR {to.root}.custom_bill_id={from.root}.id)"),
	}
	for _, value := range relations {
		if _, from := catalog.Entities[value.Public.FromEntity]; !from {
			continue
		}
		if _, to := catalog.Entities[value.Public.ToEntity]; !to {
			continue
		}
		catalog.addRelation(value)
	}
	for id, value := range entityTypes {
		entityKey := "custom_entity." + id
		if _, ok := catalog.Entities[entityKey]; !ok || value.ProfileCardinality == "" {
			continue
		}
		catalog.addRelation(relation("profile.custom_entity."+id, "profiles", entityKey, value.Label, CardinalityMany, "{to.root}.owner_profile_id={from.root}.id"))
		catalog.addRelation(relation(entityKey+".owner", entityKey, "profiles", "Pessoa proprietária", CardinalityOne, "{to.root}.id={from.root}.owner_profile_id"))
		if _, attachments := catalog.Entities["attachments"]; attachments {
			catalog.addRelation(relation(entityKey+".attachments", entityKey, "attachments", "Anexos", CardinalityMany, "{to.root}.custom_entity_id={from.root}.id"))
		}
	}
}

func (catalog *resolvedCatalog) finalize() {
	catalog.Public.Entities = make([]EntityDefinition, 0, len(catalog.Entities))
	for _, value := range catalog.Entities {
		catalog.Public.Entities = append(catalog.Public.Entities, value.Public)
	}
	sort.Slice(catalog.Public.Entities, func(i, j int) bool { return catalog.Public.Entities[i].Key < catalog.Public.Entities[j].Key })
	catalog.Public.Fields = make([]FieldDefinition, 0, len(catalog.Fields))
	for _, value := range catalog.Fields {
		catalog.Public.Fields = append(catalog.Public.Fields, value.Public)
	}
	sort.Slice(catalog.Public.Fields, func(i, j int) bool {
		if catalog.Public.Fields[i].Entity != catalog.Public.Fields[j].Entity {
			return catalog.Public.Fields[i].Entity < catalog.Public.Fields[j].Entity
		}
		if catalog.Public.Fields[i].Label != catalog.Public.Fields[j].Label {
			return catalog.Public.Fields[i].Label < catalog.Public.Fields[j].Label
		}
		return catalog.Public.Fields[i].Key < catalog.Public.Fields[j].Key
	})
	catalog.Public.Relations = make([]RelationDefinition, 0, len(catalog.Relations))
	for _, value := range catalog.Relations {
		catalog.Public.Relations = append(catalog.Public.Relations, value.Public)
	}
	sort.Slice(catalog.Public.Relations, func(i, j int) bool { return catalog.Public.Relations[i].Key < catalog.Public.Relations[j].Key })
	catalog.Public.Operators = operatorCatalog()
	catalog.Public.Limits = CatalogLimits{MaximumProjections: MaximumProjections, MaximumFilterNodes: MaximumFilterNodes,
		MaximumFilterDepth: MaximumFilterDepth, MaximumRelationDepth: MaximumRelationDepth,
		MaximumPredicateValues: MaximumPredicateValues, MaximumSortFields: MaximumSortFields,
		MaximumRows: MaximumRows, MaximumPageSize: MaximumPageSize}
	encoded, _ := json.Marshal(catalog.Public)
	digest := sha256.Sum256(encoded)
	catalog.Public.Version = hex.EncodeToString(digest[:])
}

func (catalog *resolvedCatalog) addEntity(value sqlEntityDefinition) {
	if value.Public.Key == "" || value.FromTemplate == "" {
		return
	}
	catalog.Entities[value.Public.Key] = value
}

func (catalog *resolvedCatalog) addField(value sqlFieldDefinition) {
	if value.Public.Key == "" || value.Expression == "" {
		return
	}
	if _, duplicate := catalog.Fields[value.Public.Key]; duplicate {
		return
	}
	catalog.Fields[value.Public.Key] = value
}

func (catalog *resolvedCatalog) addRelation(value sqlRelationDefinition) {
	if value.Public.Key == "" || value.JoinCondition == "" {
		return
	}
	catalog.Relations[value.Public.Key] = value
}

func field(key, entity, label string, kind ValueKind, nullable, projectable, filterable, sortable bool, expression string) sqlFieldDefinition {
	operators := operatorsForKind(kind)
	if !nullable {
		operators = removeOperators(operators, OperatorIsNull, OperatorNotNull)
	}
	return sqlFieldDefinition{Public: FieldDefinition{Key: key, Entity: entity, Label: label, Kind: kind, Nullable: nullable,
		Projectable: projectable, Filterable: filterable, Sortable: sortable, Operators: operators}, Expression: expression}
}

func removeOperators(values []Operator, excluded ...Operator) []Operator {
	result := make([]Operator, 0, len(values))
	for _, value := range values {
		if !containsOperator(excluded, value) {
			result = append(result, value)
		}
	}
	return result
}

func relation(key, from, to, label string, cardinality RelationCardinality, condition string) sqlRelationDefinition {
	return sqlRelationDefinition{Public: RelationDefinition{Key: key, FromEntity: from, ToEntity: to, Label: truncateRunes(label, 160), Cardinality: cardinality},
		JoinCondition: condition, TargetEntityKey: to}
}

func operatorsForKind(kind ValueKind) []Operator {
	nulls := []Operator{OperatorIsNull, OperatorNotNull}
	switch kind {
	case ValueText, ValueLongText, ValueIdentifier, ValueEnum, ValueCivilMonth:
		return append([]Operator{OperatorEqual, OperatorNotEqual, OperatorContains, OperatorStartsWith, OperatorIn}, nulls...)
	case ValueInteger, ValueDecimal, ValueCivilDate, ValueTimestamp:
		return append([]Operator{OperatorEqual, OperatorNotEqual, OperatorGreater, OperatorGreaterEq, OperatorLess, OperatorLessEq, OperatorBetween, OperatorIn}, nulls...)
	case ValueBoolean:
		return append([]Operator{OperatorEqual, OperatorNotEqual}, nulls...)
	default:
		return nulls
	}
}

func operatorCatalog() []OperatorDefinition {
	return []OperatorDefinition{
		{Key: OperatorEqual, Label: "é igual a", MinimumValues: 1, MaximumValues: 1},
		{Key: OperatorNotEqual, Label: "é diferente de", MinimumValues: 1, MaximumValues: 1},
		{Key: OperatorContains, Label: "contém", MinimumValues: 1, MaximumValues: 1},
		{Key: OperatorStartsWith, Label: "começa com", MinimumValues: 1, MaximumValues: 1},
		{Key: OperatorGreater, Label: "é maior que", MinimumValues: 1, MaximumValues: 1},
		{Key: OperatorGreaterEq, Label: "é maior ou igual a", MinimumValues: 1, MaximumValues: 1},
		{Key: OperatorLess, Label: "é menor que", MinimumValues: 1, MaximumValues: 1},
		{Key: OperatorLessEq, Label: "é menor ou igual a", MinimumValues: 1, MaximumValues: 1},
		{Key: OperatorBetween, Label: "está entre", MinimumValues: 2, MaximumValues: 2},
		{Key: OperatorIn, Label: "está em", MinimumValues: 1, MaximumValues: MaximumPredicateValues},
		{Key: OperatorIsNull, Label: "está vazio", MinimumValues: 0, MaximumValues: 0},
		{Key: OperatorNotNull, Label: "não está vazio", MinimumValues: 0, MaximumValues: 0},
	}
}

func customValueKind(value string) (ValueKind, bool) {
	switch value {
	case "TEXT", "EMAIL", "PHONE":
		return ValueText, true
	case "LONG_TEXT":
		return ValueLongText, true
	case "INTEGER":
		return ValueInteger, true
	case "DECIMAL":
		return ValueDecimal, true
	case "BOOLEAN":
		return ValueBoolean, true
	case "CIVIL_DATE":
		return ValueCivilDate, true
	case "CIVIL_MONTH":
		return ValueCivilMonth, true
	case "SINGLE_SELECT", "MULTI_SELECT":
		return ValueEnum, true
	default:
		return "", false
	}
}

func customValueExpression(definitionID, targetColumn string, kind ValueKind) string {
	if kind == ValueEnum {
		return fmt.Sprintf(`(SELECT string_agg(option_value.technical_key, ',' ORDER BY option_value.sort_order, option_value.technical_key, option_value.id)
FROM custom_field_values custom_value
JOIN custom_field_value_options selected_option ON selected_option.custom_field_value_id=custom_value.id
JOIN custom_field_options option_value ON option_value.id=selected_option.option_id
WHERE custom_value.%s={root}.id AND custom_value.field_definition_id='%s'::uuid)`, targetColumn, definitionID)
	}
	column := map[ValueKind]string{ValueText: "text_value", ValueLongText: "text_value", ValueInteger: "integer_value",
		ValueDecimal: "decimal_value", ValueBoolean: "boolean_value", ValueCivilDate: "civil_date_value", ValueCivilMonth: "civil_month_value"}[kind]
	return fmt.Sprintf("(SELECT custom_value.%s FROM custom_field_values custom_value WHERE custom_value.%s={root}.id AND custom_value.field_definition_id='%s'::uuid)", column, targetColumn, definitionID)
}

func normalizedOptions(values []OptionDefinition) []OptionDefinition {
	result := make([]OptionDefinition, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validLogicalSegment(value.Key) || strings.TrimSpace(value.Label) == "" {
			continue
		}
		if _, duplicate := seen[value.Key]; duplicate {
			continue
		}
		seen[value.Key] = struct{}{}
		result = append(result, OptionDefinition{Key: value.Key, Label: truncateRunes(value.Label, 120)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func validUUID(value string) bool {
	_, err := auth.ParseIdentifier(value)
	return err == nil
}

func validLogicalSegment(value string) bool {
	if len(value) < 2 || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value[1:] {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func quoteSQLLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

func truncateRunes(value string, maximum int) string {
	value = strings.TrimSpace(strings.ToValidUTF8(value, ""))
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}

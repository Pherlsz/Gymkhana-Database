package operations

import "github.com/Pherlsz/Gymkhana-Database/internal/auth"

const CustomFieldPrefix = "custom:"

type FieldKind string

const (
	FieldText       FieldKind = "TEXT"
	FieldIdentifier FieldKind = "IDENTIFIER"
	FieldInteger    FieldKind = "INTEGER"
	FieldDecimal    FieldKind = "DECIMAL"
	FieldBoolean    FieldKind = "BOOLEAN"
	FieldCivilDate  FieldKind = "CIVIL_DATE"
	FieldCivilMonth FieldKind = "CIVIL_MONTH"
)

type Field struct {
	ID         string
	Label      string
	Kind       FieldKind
	Required   bool
	Importable bool
	Exportable bool
}

type ModuleCatalog struct {
	ID            Module
	Label         string
	CanImport     bool
	CanExport     bool
	CanDuplicate  bool
	CanDelete     bool
	CanBulkDelete bool
	Fields        []Field
}

func Catalog(role auth.Role) []ModuleCatalog {
	if !role.Valid() {
		return nil
	}
	return []ModuleCatalog{
		{
			ID: ModuleProfiles, Label: ModuleProfiles.Label(), CanImport: role.CanWriteProfiles(), CanExport: role.CanReadProfiles(), CanDuplicate: role.CanWriteProfiles(), CanDelete: role.CanDeleteProfiles(), CanBulkDelete: role.CanDeleteProfiles(),
			Fields: withProfileDocumentFields([]Field{
				{ID: "record_id", Label: "ID do registro", Kind: FieldIdentifier, Importable: true, Exportable: true},
				{ID: "version", Label: "Versão", Kind: FieldInteger, Importable: true, Exportable: true},
				{ID: "full_name", Label: "Nome completo", Kind: FieldText, Required: true, Importable: true, Exportable: true},
				{ID: "social_name", Label: "Nome social", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "cpf", Label: "CPF", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "email", Label: "E-mail", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "mobile_phone", Label: "Celular", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "landline_phone", Label: "Telefone", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "address_street", Label: "Logradouro", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "address_number", Label: "Número", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "address_complement", Label: "Complemento", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "address_neighborhood", Label: "Bairro", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "address_city", Label: "Cidade", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "address_state", Label: "UF", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "address_postal_code", Label: "CEP", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "notes", Label: "Observações", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "birth_date", Label: "Data de nascimento", Kind: FieldCivilDate, Importable: true, Exportable: true},
				{ID: "gender", Label: "Sexo", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "blood_type", Label: "Tipo sanguíneo", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "nationality", Label: "Nacionalidade", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "birth_city", Label: "Cidade de nascimento", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "marital_status", Label: "Estado civil", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "wedding_date", Label: "Data de casamento", Kind: FieldCivilDate, Importable: true, Exportable: true},
				{ID: "father_name", Label: "Nome do pai", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "father_birth_date", Label: "Nascimento do pai", Kind: FieldCivilDate, Importable: true, Exportable: true},
				{ID: "mother_name", Label: "Nome da mãe", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "mother_birth_date", Label: "Nascimento da mãe", Kind: FieldCivilDate, Importable: true, Exportable: true},
				{ID: "health_plan", Label: "Plano de saúde", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "blood_donor", Label: "Doador de sangue", Kind: FieldBoolean, Importable: true, Exportable: true},
				{ID: "organ_donor", Label: "Doador de órgãos", Kind: FieldBoolean, Importable: true, Exportable: true},
				{ID: "team", Label: "Equipe", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "sector", Label: "Setor", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "collections", Label: "Coleções", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "vehicle_model", Label: "Modelo do veículo", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "vehicle_color", Label: "Cor do veículo", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "vehicle_plate", Label: "Placa", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "vehicle_year", Label: "Ano do veículo", Kind: FieldInteger, Importable: true, Exportable: true},
				{ID: "club_membership", Label: "Clube", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "membership_type", Label: "Tipo de sócio", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "place_of_origin", Label: "Naturalidade", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "birth_country", Label: "País de nascimento", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "parents_wedding_date", Label: "Casamento dos pais", Kind: FieldCivilDate, Importable: true, Exportable: true},
				{ID: "supermarket_club", Label: "Clube de supermercado", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "pet", Label: "Animal", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "travel_countries", Label: "Viagens", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "card_brand", Label: "Bandeira do cartão", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "card_bank", Label: "Banco do cartão", Kind: FieldText, Importable: true, Exportable: true},
			}),
		},
		{
			ID: ModuleDocuments, Label: ModuleDocuments.Label(), CanImport: role.CanWriteDocuments(), CanExport: role.CanReadDocuments(), CanDuplicate: role.CanWriteDocuments(), CanDelete: role.CanDeleteDocuments(), CanBulkDelete: role.CanDeleteDocuments(),
			Fields: []Field{
				{ID: "record_id", Label: "ID do registro", Kind: FieldIdentifier, Importable: true, Exportable: true},
				{ID: "version", Label: "Versão", Kind: FieldInteger, Importable: true, Exportable: true},
				{ID: "owner_profile_id", Label: "ID da pessoa proprietária", Kind: FieldIdentifier, Required: true, Importable: true, Exportable: true},
				{ID: "document_type_id", Label: "ID do tipo", Kind: FieldIdentifier, Required: true, Importable: true, Exportable: true},
				{ID: "identifier_value", Label: "Identificador", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "document_date", Label: "Data", Kind: FieldCivilDate, Importable: true, Exportable: true},
				{ID: "notes", Label: "Observações", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "medium", Label: "Meio", Kind: FieldText, Required: true, Importable: true, Exportable: true},
			},
		},
		{
			ID: ModuleBills, Label: ModuleBills.Label(), CanImport: role.CanWriteBills(), CanExport: role.CanReadBills(), CanDuplicate: role.CanWriteBills(), CanDelete: role.CanDeleteBills(), CanBulkDelete: role.CanDeleteBills(),
			Fields: []Field{
				{ID: "record_id", Label: "ID do registro", Kind: FieldIdentifier, Importable: true, Exportable: true},
				{ID: "version", Label: "Versão", Kind: FieldInteger, Importable: true, Exportable: true},
				{ID: "owner_profile_id", Label: "ID da pessoa proprietária", Kind: FieldIdentifier, Required: true, Importable: true, Exportable: true},
				{ID: "bill_type_id", Label: "ID do tipo", Kind: FieldIdentifier, Required: true, Importable: true, Exportable: true},
				{ID: "printed_holder_name", Label: "Nome impresso", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "printed_address", Label: "Endereço impresso", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "reference_value", Label: "Referência", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "competence", Label: "Competência", Kind: FieldCivilMonth, Importable: true, Exportable: true},
				{ID: "amount", Label: "Valor", Kind: FieldDecimal, Importable: true, Exportable: true},
				{ID: "currency", Label: "Moeda", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "notes", Label: "Observações", Kind: FieldText, Importable: true, Exportable: true},
				{ID: "medium", Label: "Meio", Kind: FieldText, Required: true, Importable: true, Exportable: true},
			},
		},
	}
}

func withProfileDocumentFields(fields []Field) []Field {
	extras := []Field{
		{ID: "document:rg", Label: "RG", Kind: FieldText, Importable: true},
		{ID: "document:rg:date", Label: "RG (data)", Kind: FieldCivilDate, Importable: true},
		{ID: "document:cnh", Label: "CNH", Kind: FieldText, Importable: true},
		{ID: "document:cnh:date", Label: "CNH (data)", Kind: FieldCivilDate, Importable: true},
		{ID: "document:voter_id", Label: "Título de eleitor", Kind: FieldText, Importable: true},
		{ID: "document:ctps", Label: "CTPS", Kind: FieldText, Importable: true},
		{ID: "document:pis", Label: "PIS", Kind: FieldText, Importable: true},
		{ID: "document:sus_card", Label: "Cartão SUS", Kind: FieldText, Importable: true},
		{ID: "document:passport", Label: "Passaporte", Kind: FieldText, Importable: true},
		{ID: "document:citizen_card", Label: "Cartão cidadão", Kind: FieldText, Importable: true},
		{ID: "document:oab", Label: "OAB", Kind: FieldText, Importable: true},
		{ID: "document:crea", Label: "CREA", Kind: FieldText, Importable: true},
		{ID: "document:coren", Label: "COREN", Kind: FieldText, Importable: true},
		{ID: "document:crm", Label: "CRM", Kind: FieldText, Importable: true},
		{ID: "document:cro", Label: "CRO", Kind: FieldText, Importable: true},
		{ID: "document:cref", Label: "CREF", Kind: FieldText, Importable: true},
		{ID: "document:tri", Label: "TRI", Kind: FieldText, Importable: true},
		{ID: "document:teu", Label: "TEU", Kind: FieldText, Importable: true},
		{ID: "document:generic", Label: "Documento (identificar)", Kind: FieldText, Importable: true},
	}
	return append(fields, extras...)
}

func moduleCatalog(role auth.Role, module Module) (ModuleCatalog, bool) {
	for _, candidate := range Catalog(role) {
		if candidate.ID == module {
			return candidate, true
		}
	}
	return ModuleCatalog{}, false
}

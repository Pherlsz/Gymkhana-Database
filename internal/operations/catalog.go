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
			Fields: []Field{
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
			},
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

func moduleCatalog(role auth.Role, module Module) (ModuleCatalog, bool) {
	for _, candidate := range Catalog(role) {
		if candidate.ID == module {
			return candidate, true
		}
	}
	return ModuleCatalog{}, false
}

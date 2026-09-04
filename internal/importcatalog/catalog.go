// Package importcatalog implements Orchestration §16.2 product rules: column header
// aliases, explicit discards, and document-label remaps. Value normalization
// remains in Gymkhana-Core and domain services at execute time.
package importcatalog

import (
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

const discardSentinel = "__discard__"

// Module identifies the operations import target table.
type Module string

const (
	ModuleProfiles  Module = "PROFILES"
	ModuleDocuments Module = "DOCUMENTS"
	ModuleBills     Module = "BILLS"
)

// Suggestion is a proposed mapping for one spreadsheet column.
type Suggestion struct {
	TargetField string
	Discard     bool
}

// SuggestColumn returns a catalog mapping for a source header within a module.
func SuggestColumn(module Module, sourceHeader string) Suggestion {
	folded := normalize.SearchText(sourceHeader)
	if folded == "" {
		return Suggestion{}
	}
	if _, discard := discardHeaders[folded]; discard {
		return Suggestion{TargetField: discardSentinel, Discard: true}
	}
	table := aliasTable(module)
	if target, ok := table[folded]; ok {
		return Suggestion{TargetField: target}
	}
	return Suggestion{}
}

// ApplyHeaders returns target fields aligned with headers (same length).
func ApplyHeaders(module Module, headers []string) []string {
	targets := make([]string, len(headers))
	for index, header := range headers {
		suggestion := SuggestColumn(module, header)
		if suggestion.Discard {
			targets[index] = discardSentinel
			continue
		}
		targets[index] = suggestion.TargetField
	}
	return targets
}

// IsDiscard reports whether a stored target marks an explicit discard column.
func IsDiscard(targetField string) bool {
	return targetField == discardSentinel
}

// VisibleTarget returns empty for discard sentinels so APIs omit discarded columns from mapping UI.
func VisibleTarget(targetField string) string {
	if IsDiscard(targetField) {
		return ""
	}
	return targetField
}

var discardHeaders = map[string]struct{}{
	"idade": {}, "signo": {}, "soma digitos cpf": {}, "soma digitos": {},
	"quem indicou": {}, "horario nascimento": {}, "peculiaridade": {},
	"link anexo": {}, "anexo link": {}, "calculado": {}, "derivado": {},
}

func aliasTable(module Module) map[string]string {
	switch module {
	case ModuleProfiles:
		return profileAliases
	case ModuleDocuments:
		return documentAliases
	case ModuleBills:
		return billAliases
	default:
		return nil
	}
}

var profileAliases = map[string]string{
	"nome": "full_name", "name": "full_name", "nome completo": "full_name",
	"nome social": "social_name",
	"cpf": "cpf",
	"email": "email", "e mail": "email",
	"celular": "mobile_phone", "telefone celular": "mobile_phone",
	"residencial": "landline_phone", "fone comercial": "landline_phone",
	"telefone": "landline_phone", "fone": "landline_phone",
	"endereco": "address_street", "logradouro": "address_street",
	"numero": "address_number", "n": "address_number",
	"complemento": "address_complement", "bloco": "address_complement",
	"apto": "address_complement", "predio": "address_complement",
	"bairro": "address_neighborhood",
	"cidade": "address_city", "cidade reside": "address_city", "cidade residencia": "address_city",
	"uf": "address_state", "estado": "address_state",
	"cep": "address_postal_code",
	"observacoes": "notes", "obs": "notes", "notas": "notes",
	"record id": "record_id", "id registro": "record_id", "id": "record_id",
	"versao": "version", "version": "version",
}

var documentAliases = map[string]string{
	"identificador": "identifier_value", "numero": "identifier_value", "numero documento": "identifier_value",
	"data": "document_date", "data documento": "document_date", "validade": "document_date",
	"observacoes": "notes", "obs": "notes",
	"meio": "medium", "medium": "medium",
	"id pessoa": "owner_profile_id", "id proprietario": "owner_profile_id", "owner profile id": "owner_profile_id",
	"id tipo": "document_type_id", "tipo id": "document_type_id", "document type id": "document_type_id",
	"record id": "record_id", "versao": "version",
}

var billAliases = map[string]string{
	"referencia": "reference_value", "conta": "reference_value", "numero conta": "reference_value",
	"competencia": "competence",
	"valor": "amount", "amount": "amount",
	"moeda": "currency", "currency": "currency",
	"nome impresso": "printed_holder_name", "titular": "printed_holder_name", "nome titular": "printed_holder_name",
	"endereco impresso": "printed_address", "endereco": "printed_address",
	"observacoes": "notes", "obs": "notes",
	"meio": "medium",
	"id pessoa": "owner_profile_id", "id proprietario": "owner_profile_id",
	"id tipo": "bill_type_id", "tipo id": "bill_type_id",
	"record id": "record_id", "versao": "version",
}

// FoldHeader exposes catalog folding for tests.
func FoldHeader(header string) string {
	return strings.TrimSpace(normalize.SearchText(header))
}

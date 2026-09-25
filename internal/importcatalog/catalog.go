// Package importcatalog implements Orchestration §16.2 product rules: column header
// aliases, explicit discards, and document-label remaps. Value normalization
// remains in Gymkhana-Core and domain services at execute time.
package importcatalog

import (
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

const DiscardSentinel = "__discard__"

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
	if strings.HasSuffix(folded, " presenca") {
		return Suggestion{TargetField: DiscardSentinel, Discard: true}
	}
	if strings.HasPrefix(folded, "soma ") {
		return Suggestion{TargetField: DiscardSentinel, Discard: true}
	}
	if _, discard := discardHeaders[folded]; discard {
		return Suggestion{TargetField: DiscardSentinel, Discard: true}
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
			targets[index] = DiscardSentinel
			continue
		}
		targets[index] = suggestion.TargetField
	}
	return targets
}

// IsDiscard reports whether a stored target marks an explicit discard column.
func IsDiscard(targetField string) bool {
	return targetField == DiscardSentinel
}

// VisibleTarget returns empty for discard sentinels so APIs omit discarded columns from mapping UI.
func VisibleTarget(targetField string) string {
	if IsDiscard(targetField) {
		return ""
	}
	return targetField
}

var discardHeaders = map[string]struct{}{
	"idade": {}, "signo": {},
	"soma digitos cpf": {}, "soma digitos": {}, "soma rg": {}, "soma cpf": {},
	"soma do nome": {}, "soma nome": {}, "conta digito": {},
	"quem indicou": {}, "horario nascimento": {}, "peculiaridade": {},
	"link anexo": {}, "anexo link": {}, "calculado": {}, "derivado": {},
	"carimbo de data hora": {}, "timestamp": {},
	"arquivo": {}, "aba": {}, "linha origem": {},
	"cpf presenca": {}, "cpf emissao": {},
	"em caso de dados de familiar podemos ligar em qualquer horario": {},
	"n": {}, "no": {}, "item": {}, "indice": {}, "seq": {}, "ordem": {},
	"titulo zona": {}, "titulo secao": {}, "ctps serie": {},
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
	"cpf":         "cpf", "cpf cgc": "cpf",
	"email": "email", "e mail": "email",
	"celular": "mobile_phone", "telefone celular": "mobile_phone", "fone celular": "mobile_phone",
	"celular whatsapp": "mobile_phone", "whatsapp": "mobile_phone",
	"residencial": "landline_phone", "fone comercial": "landline_phone", "comercial": "landline_phone",
	"telefone": "landline_phone", "fone": "landline_phone", "telefone fixo": "landline_phone",
	"telefone residencial": "landline_phone", "fone residencial": "landline_phone", "fixo": "landline_phone",
	"endereco": "address_street", "logradouro": "address_street", "rua": "address_street",
	"numero": "address_number", "numero casa": "address_number", "n casa": "address_number",
	"complemento": "address_complement", "bloco": "address_complement",
	"apto": "address_complement", "predio": "address_complement",
	"bairro": "address_neighborhood",
	"cidade": "address_city", "cidade reside": "address_city", "cidade residencia": "address_city",
	"municipio": "address_city",
	"uf":        "address_state", "estado": "address_state",
	"cep":         "address_postal_code",
	"observacoes": "notes", "obs": "notes", "notas": "notes",
	"sexo": "gender", "genero": "gender", "gender": "gender",
	"tipo sanguineo": "blood_type", "sangue": "blood_type",
	"estado civil":    "marital_status",
	"data nascimento": "birth_date", "data de nascimento": "birth_date",
	"nascimento": "birth_date", "birth date": "birth_date",
	"cidade nascimento": "birth_city", "cidade de nascimento": "birth_city",
	"naturalidade": "place_of_origin", "natural de": "place_of_origin",
	"pais nascimento": "birth_country", "pais de nascimento": "birth_country",
	"pais":          "birth_country",
	"nacionalidade": "nationality", "nationality": "nationality",
	"pai": "father_name", "nome pai": "father_name", "nome do pai": "father_name",
	"nasc pai": "father_birth_date", "nascimento pai": "father_birth_date",
	"mae": "mother_name", "nome mae": "mother_name", "nome da mae": "mother_name",
	"nasc mae": "mother_birth_date", "nascimento mae": "mother_birth_date",
	"casamento": "wedding_date", "data casamento": "wedding_date",
	"casamento pais": "parents_wedding_date", "casamento dos pais": "parents_wedding_date",
	"equipe": "team", "time": "team",
	"setor":       "sector",
	"socio clube": "club_membership", "clube": "club_membership",
	"voce e socio de algum clube": "club_membership",
	"membership type":             "membership_type", "tipo socio": "membership_type",
	"clube supermercado": "supermarket_club",
	"colecao":            "collections", "colecoes": "collections",
	"animal": "pet", "animais": "pet",
	"viagem": "travel_countries", "paises viagem": "travel_countries",
	"plano saude": "health_plan", "convenio": "health_plan",
	"doador sangue": "blood_donor", "doador de sangue": "blood_donor",
	"doador orgaos": "organ_donor", "doador de orgaos": "organ_donor",
	"cartao bandeira": "card_brand", "bandeira cartao": "card_brand",
	"cartao banco": "card_bank", "banco cartao": "card_bank",
	"veiculo modelo": "vehicle_model", "modelo veiculo": "vehicle_model",
	"veiculo cor": "vehicle_color", "cor veiculo": "vehicle_color",
	"veiculo placa": "vehicle_plate", "placa": "vehicle_plate",
	"veiculo ano": "vehicle_year", "ano veiculo": "vehicle_year",
	"record id": "record_id", "id registro": "record_id",
	"versao": "version", "version": "version",
	"rg": DocumentFieldPrefix + "rg", "identidade": DocumentFieldPrefix + "rg",
	"rg numero": DocumentFieldPrefix + "rg", "numero rg": DocumentFieldPrefix + "rg",
	"rg emissao": DocumentFieldPrefix + "rg:date", "data rg": DocumentFieldPrefix + "rg:date",
	"cnh": DocumentFieldPrefix + "cnh", "cnh numero": DocumentFieldPrefix + "cnh",
	"cnh emissao": DocumentFieldPrefix + "cnh:date", "data cnh": DocumentFieldPrefix + "cnh:date",
	"cnh numero e data da primeira emissao": DocumentFieldPrefix + "cnh",
	"titulo":                                DocumentFieldPrefix + "voter_id", "titulo eleitor": DocumentFieldPrefix + "voter_id",
	"titulo de eleitor": DocumentFieldPrefix + "voter_id",
	"ctps":              DocumentFieldPrefix + "ctps",
	"pis":               DocumentFieldPrefix + "pis", "pasep": DocumentFieldPrefix + "pis",
	"nit": DocumentFieldPrefix + "pis", "nis": DocumentFieldPrefix + "pis",
	"sus": DocumentFieldPrefix + "sus_card", "cns": DocumentFieldPrefix + "sus_card",
	"cartao sus":     DocumentFieldPrefix + "sus_card",
	"passaporte":     DocumentFieldPrefix + "passport",
	"cartao cidadao": DocumentFieldPrefix + "citizen_card",
	"estudante":      DocumentFieldPrefix + "generic", "carteirinha": DocumentFieldPrefix + "generic",
	"formacao": DocumentFieldPrefix + "generic",
	"oab":      DocumentFieldPrefix + "oab", "crea": DocumentFieldPrefix + "crea",
	"coren": DocumentFieldPrefix + "coren", "crm": DocumentFieldPrefix + "crm",
	"cro": DocumentFieldPrefix + "cro", "cref": DocumentFieldPrefix + "cref",
	"tri": DocumentFieldPrefix + "tri", "teu": DocumentFieldPrefix + "teu",
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
	"valor":       "amount", "amount": "amount",
	"moeda": "currency", "currency": "currency",
	"nome impresso": "printed_holder_name", "titular": "printed_holder_name", "nome titular": "printed_holder_name",
	"endereco impresso": "printed_address", "endereco": "printed_address",
	"observacoes": "notes", "obs": "notes",
	"meio":      "medium",
	"id pessoa": "owner_profile_id", "id proprietario": "owner_profile_id",
	"id tipo": "bill_type_id", "tipo id": "bill_type_id",
	"record id": "record_id", "versao": "version",
}

// FoldHeader exposes catalog folding for tests.
func FoldHeader(header string) string {
	return strings.TrimSpace(normalize.SearchText(header))
}

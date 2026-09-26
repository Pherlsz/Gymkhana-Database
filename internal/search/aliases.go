package search

import (
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

// Autocomplete inserts Portuguese tokens. English keys stay in fieldAliases /
// moduleAliases for Assistente and API queries only.

type TokenKind string

const (
	TokenOperator    TokenKind = "operator"
	TokenConjunction TokenKind = "conjunction"
	TokenTypeSugar   TokenKind = "type_sugar"
	TokenExclude     TokenKind = "exclude"
)

type QueryToken struct {
	Token       string
	Insert      string
	Kind        TokenKind
	Description string
}

type fieldAlias struct {
	keys []string
}

type moduleAlias struct {
	module Module
}

type typeSugarSpec struct {
	kind        string
	description string
	tokens      []string
}

var operatorTokens = []QueryToken{
	{Token: "tipo", Insert: "tipo:", Kind: TokenOperator, Description: "Tipo de documento ou conta"},
	{Token: "identificador", Insert: "identificador:", Kind: TokenOperator, Description: "Número informado ou referência"},
	{Token: "nome", Insert: "nome:", Kind: TokenOperator, Description: "Nome completo"},
	{Token: "cidade", Insert: "cidade:", Kind: TokenOperator, Description: "Cidade"},
	{Token: "bairro", Insert: "bairro:", Kind: TokenOperator, Description: "Bairro"},
	{Token: "uf", Insert: "uf:", Kind: TokenOperator, Description: "UF"},
	{Token: "cep", Insert: "cep:", Kind: TokenOperator, Description: "CEP"},
	{Token: "logradouro", Insert: "logradouro:", Kind: TokenOperator, Description: "Logradouro"},
	{Token: "email", Insert: "email:", Kind: TokenOperator, Description: "E-mail"},
	{Token: "equipe", Insert: "equipe:", Kind: TokenOperator, Description: "Equipe de gincana"},
	{Token: "em", Insert: "em:", Kind: TokenOperator, Description: "Restringir ao módulo"},
	{Token: "em_uso", Insert: "em_uso:", Kind: TokenOperator, Description: "Pessoa em uso"},
	{Token: "competencia", Insert: "competencia:", Kind: TokenOperator, Description: "Competência da conta"},
	{Token: "valor", Insert: "valor:", Kind: TokenOperator, Description: "Valor da conta"},
	{Token: "meio", Insert: "meio:", Kind: TokenOperator, Description: "Físico ou digital"},
	{Token: "data", Insert: "data:", Kind: TokenOperator, Description: "Data do documento"},
	{Token: "observacoes", Insert: "observacoes:", Kind: TokenOperator, Description: "Observações"},
	{Token: "OU", Insert: "OU ", Kind: TokenConjunction, Description: "Qualquer um dos lados"},
	{Token: "-", Insert: "-", Kind: TokenExclude, Description: "Excluir termo"},
}

// typeSugarSpecs map Portuguese (and synonym) tokens to document_types.technical_key.
// Core kinds use CanonicalDocument; product extras (tri/teu/cref) do not.
var typeSugarSpecs = []typeSugarSpec{
	{kind: string(normalize.DocumentCPF), description: "Tipo CPF e identificador", tokens: []string{"cpf"}},
	{kind: string(normalize.DocumentCNPJ), description: "Tipo CNPJ e identificador", tokens: []string{"cnpj"}},
	{kind: string(normalize.DocumentRG), description: "Tipo RG e identificador", tokens: []string{"rg", "identidade"}},
	{kind: string(normalize.DocumentCNH), description: "Tipo CNH e identificador", tokens: []string{"cnh"}},
	{kind: string(normalize.DocumentCTPS), description: "Tipo CTPS e identificador", tokens: []string{"ctps"}},
	{kind: string(normalize.DocumentPIS), description: "Tipo PIS e identificador", tokens: []string{"pis", "pasep", "nit", "nis"}},
	{kind: string(normalize.DocumentVoterID), description: "Tipo título de eleitor e identificador", tokens: []string{"titulo", "eleitor"}},
	{kind: string(normalize.DocumentPassport), description: "Tipo passaporte e identificador", tokens: []string{"passaporte", "passport"}},
	{kind: string(normalize.DocumentSUSCard), description: "Tipo cartão SUS e identificador", tokens: []string{"sus", "cns"}},
	{kind: string(normalize.DocumentOAB), description: "Tipo OAB e identificador", tokens: []string{"oab"}},
	{kind: string(normalize.DocumentCREA), description: "Tipo CREA e identificador", tokens: []string{"crea"}},
	{kind: string(normalize.DocumentCOREN), description: "Tipo COREN e identificador", tokens: []string{"coren"}},
	{kind: string(normalize.DocumentCRM), description: "Tipo CRM e identificador", tokens: []string{"crm"}},
	{kind: string(normalize.DocumentCRO), description: "Tipo CRO e identificador", tokens: []string{"cro"}},
	{kind: string(normalize.DocumentStudentID), description: "Tipo carteira estudantil e identificador", tokens: []string{"estudantil", "carteirinha"}},
	{kind: string(normalize.DocumentCitizenCard), description: "Tipo cartão cidadão e identificador", tokens: []string{"cidadao"}},
	{kind: string(normalize.DocumentBirthCertificate), description: "Tipo certidão de nascimento e identificador", tokens: []string{"nascimento"}},
	{kind: string(normalize.DocumentMarriageCertificate), description: "Tipo certidão de casamento e identificador", tokens: []string{"casamento"}},
	{kind: "tri", description: "Tipo TRI e identificador", tokens: []string{"tri"}},
	{kind: "teu", description: "Tipo TEU e identificador", tokens: []string{"teu"}},
	{kind: "cref", description: "Tipo CREF e identificador", tokens: []string{"cref"}},
}

var queryTokens []QueryToken
var typeSugars map[string]string

func init() {
	queryTokens = append([]QueryToken{}, operatorTokens...)
	typeSugars = make(map[string]string, len(typeSugarSpecs)*2)
	for _, spec := range typeSugarSpecs {
		typeSugars[spec.kind] = spec.kind
		typeSugars[foldToken(spec.kind)] = spec.kind
		for _, token := range spec.tokens {
			typeSugars[foldToken(token)] = spec.kind
			queryTokens = append(queryTokens, QueryToken{
				Token:       token,
				Insert:      token + ":",
				Kind:        TokenTypeSugar,
				Description: spec.description,
			})
		}
	}
}

var fieldAliases = map[string]fieldAlias{
	"tipo":           {keys: []string{"document.type", "bill.type", "profile.document_identifier"}},
	"type":           {keys: []string{"document.type", "bill.type", "profile.document_identifier"}},
	"identificador":  {keys: []string{"document.identifier", "bill.reference", "profile.document_identifier"}},
	"identifier":     {keys: []string{"document.identifier", "bill.reference", "profile.document_identifier"}},
	"id":             {keys: []string{"document.identifier", "bill.reference", "profile.document_identifier"}},
	"nome":           {keys: []string{"profile.full_name"}},
	"name":           {keys: []string{"profile.full_name"}},
	"cidade":         {keys: []string{"profile.address_city"}},
	"city":           {keys: []string{"profile.address_city"}},
	"bairro":         {keys: []string{"profile.address_neighborhood"}},
	"neighborhood":   {keys: []string{"profile.address_neighborhood"}},
	"uf":             {keys: []string{"profile.address_state"}},
	"state":          {keys: []string{"profile.address_state"}},
	"cep":            {keys: []string{"profile.address_postal_code"}},
	"postal_code":    {keys: []string{"profile.address_postal_code"}},
	"zip":            {keys: []string{"profile.address_postal_code"}},
	"logradouro":     {keys: []string{"profile.address_street"}},
	"street":         {keys: []string{"profile.address_street"}},
	"email":          {keys: []string{"profile.email"}},
	"mail":           {keys: []string{"profile.email"}},
	"equipe":         {keys: []string{"profile.team"}},
	"team":           {keys: []string{"profile.team"}},
	"em_uso":         {keys: []string{"document.current_holder", "bill.current_holder"}},
	"holder":         {keys: []string{"document.current_holder", "bill.current_holder"}},
	"current_holder": {keys: []string{"document.current_holder", "bill.current_holder"}},
	"competencia":    {keys: []string{"bill.competence"}},
	"competence":     {keys: []string{"bill.competence"}},
	"valor":          {keys: []string{"bill.amount"}},
	"amount":         {keys: []string{"bill.amount"}},
	"meio":           {keys: []string{"document.medium", "bill.medium"}},
	"medium":         {keys: []string{"document.medium", "bill.medium"}},
	"data":           {keys: []string{"document.date"}},
	"date":           {keys: []string{"document.date"}},
	"observacoes":    {keys: []string{"profile.notes", "document.notes", "bill.notes"}},
	"notes":          {keys: []string{"profile.notes", "document.notes", "bill.notes"}},
}

var moduleAliases = map[string]moduleAlias{
	"pessoas":     {module: ModuleProfiles},
	"profiles":    {module: ModuleProfiles},
	"documentos":  {module: ModuleDocuments},
	"documents":   {module: ModuleDocuments},
	"contas":      {module: ModuleBills},
	"bills":       {module: ModuleBills},
	"anexos":      {module: ModuleAttachments},
	"attachments": {module: ModuleAttachments},
	"custom_data": {module: ModuleCustomData},
}

var physicalPrefixes = []string{
	"profiles.", "documents.", "bills.", "attachments.", "document_presences.",
	"document_types.", "bill_types.", "custom_field_values.", "custom_field_definitions.",
	"custom_entities.", "app_users.", "search_rate_limits.",
}

func foldToken(value string) string {
	return strings.TrimSpace(normalize.SearchText(value))
}

func QueryTokens() []QueryToken {
	out := make([]QueryToken, len(queryTokens))
	copy(out, queryTokens)
	return out
}

func isPhysicalName(token string) bool {
	folded := strings.ToLower(strings.TrimSpace(token))
	for _, prefix := range physicalPrefixes {
		if strings.HasPrefix(folded, prefix) {
			return true
		}
	}
	return false
}

func lookupFieldAlias(token string) (fieldAlias, bool) {
	alias, ok := fieldAliases[foldToken(token)]
	return alias, ok
}

func lookupModuleAlias(token string) (Module, bool) {
	alias, ok := moduleAliases[foldToken(token)]
	if !ok {
		return "", false
	}
	return alias.module, true
}

func lookupTypeSugar(token string) (string, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(token))
	if kind, ok := typeSugars[trimmed]; ok {
		return kind, true
	}
	kind, ok := typeSugars[foldToken(token)]
	return kind, ok
}

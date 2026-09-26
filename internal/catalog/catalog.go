// Package catalog holds Gymkhana-Database value lists (teams, sectors, clubs,
// vehicles) and their formatters. Core keeps only reusable primitives.
package catalog

import (
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

type labeledRule struct {
	label   string
	needles []string
}

var teamPhrases = []labeledRule{
	{"TNC", []string{"tamo nessa por cerveja", "tamo nesse por cerveja", "equipe tnc"}},
	{"Guardiões do Vale", []string{"guardioes do vale", "guardioes vale", "gurdioes vale"}},
	{"Águia de Fogo", []string{"aguia de fogo", "aguia fogo"}},
	{"Moço do Borogodó", []string{"moco do borogodo", "moco borogodo"}},
	{"Laranja Mecânica", []string{"laranja mecanica"}},
	{"Caipira Funil", []string{"caipira funil"}},
	{"Curto Circuito", []string{"curto circuito"}},
	{"Tatu Cascalho", []string{"tatu cascalho"}},
	{"Tatu Pelado", []string{"tatu pelado", "tatu pelados"}},
	{"Anjos da Aldeia", []string{"anjos da aldeia", "anjos da aldeira"}},
	{"Trio Elétrico", []string{"trio eletrico", "trio eletro"}},
	{"Força Tarefa", []string{"forca tarefa"}},
	{"Maragatos", []string{"equipe maragatos"}},
	{"Piratas", []string{"equipe piratas", "equi piratas", "equi pirata"}},
	{"Tendeu", []string{"equi tendeu"}},
	{"NERD", []string{"n e r d", "equipe n e r d"}},
	{"Doze", []string{"equi doze"}},
	{"Revolução", []string{"revolucao vacaria"}},
}

var teamTokens = map[string]string{
	"tnc": "TNC", "tnconha": "TNC", "aguia": "Águia de Fogo",
	"gv": "Guardiões do Vale", "gurdioes": "Guardiões do Vale",
	"atropelando": "Atropelando", "azzurra": "Azzurra", "azzura": "Azzurra",
	"avassaladores": "Avassaladores", "tendeu": "Tendeu",
	"moco": "Mocó do Borogodó", "doze": "Doze", "revolucao": "Revolução",
	"vikings": "Vikings", "vks": "Vikings", "atari": "Atari",
	"nerd": "NERD", "detenidos": "Detenidos", "equivoco": "Equívoco",
	"maragatos": "Maragatos", "piratas": "Piratas", "pirata": "Piratas",
	"urbanos": "Urbanos", "templarios": "Templários", "avalon": "Avalon",
	"paulista": "Paulista", "stefani": "Stefani", "tnt": "TNT",
}

var sectorLabels = map[string]string{
	"rua": "Rua", "diversas": "Diversas", "artistica": "Artística",
	"construcao": "Construção", "midias": "Mídias", "esportiva": "Esportiva",
	"charadas": "Charadas", "objetos": "Objetos", "fechamento": "Fechamento",
	"secretaria": "Secretaria",
}

var sectorAliases = map[string]string{
	"rua": "rua", "diversas": "diversas", "diversa": "diversas",
	"artistica": "artistica", "maquiagem": "artistica", "artes": "artistica",
	"esportiva": "esportiva", "esporte": "esportiva",
	"charadas": "charadas", "objetos": "objetos",
	"fechamento": "fechamento", "secretaria": "secretaria",
	"construcao": "construcao", "obras": "construcao", "criacao": "construcao",
	"midia": "midias", "midias": "midias",
}

var notFootball = []string{
	"iate", "piratini", "clubinho", "campestre", "piscina", "sesc",
	"tiradentes", "tira dentes", "candeias", "nautico",
}

var footballTypos = map[string]string{
	"internacio": "Internacional", "internacioal": "Internacional",
	"inter": "Internacional",
}

var footballOther = []labeledRule{
	{"Chapecoense", []string{"chapecoense", "chape"}},
	{"Juventude", []string{"juventude"}},
	{"Flamengo", []string{"flamengo"}},
	{"Fluminense", []string{"fluminense"}},
	{"Vasco", []string{"vasco"}},
	{"Botafogo", []string{"botafogo"}},
	{"Corinthians", []string{"corinthians", "timao"}},
	{"Palmeiras", []string{"palmeiras"}},
	{"Cruzeiro", []string{"cruzeiro"}},
	{"Bahia", []string{"bahia"}},
	{"Criciúma", []string{"criciuma"}},
	{"Avaí", []string{"avai"}},
}

var planRules = []labeledRule{
	{"Unimed", []string{"unimed"}},
	{"IPE Saúde", []string{"ipe saude", "ipergs", "ipe"}},
	{"Bradesco Saúde", []string{"bradesco saude"}},
	{"Hapvida", []string{"hapvida", "notredame", "intermedica"}},
	{"Amil", []string{"amil"}},
	{"Cabergs", []string{"cabergs"}},
	{"GEAP", []string{"geap"}},
	{"SulAmérica", []string{"sulamerica"}},
}

var cardBrands = []labeledRule{
	{"Banricompras", []string{"banricompras", "banri compras"}},
	{"Mastercard", []string{"mastercard", "master card", "master"}},
	{"Visa", []string{"visa"}},
	{"Hipercard", []string{"hipercard", "hiper"}},
	{"Elo", []string{"elo", "ello"}},
}

var cardBanks = []labeledRule{
	{"Banrisul", []string{"banrisul"}},
	{"Nubank", []string{"nubank", "nu bank"}},
	{"Itaú", []string{"itau"}},
	{"Bradesco", []string{"bradesco", "bradescard"}},
	{"Caixa", []string{"caixa economica", "caixa", "cef"}},
	{"Santander", []string{"santander"}},
	{"Sicredi", []string{"sicredi"}},
}

var collectionRules = []labeledRule{
	{"Moedas", []string{"moeda", "moedas"}},
	{"Selos", []string{"selo", "selos"}},
	{"Livros", []string{"livro", "livros"}},
	{"Discos", []string{"vinil", "disco", "discos", "cd", "cds"}},
}

var animalRules = []labeledRule{
	{"Cachorro", []string{"cachorro", "cachorros", "cao", "caes", "dog", "poodle"}},
	{"Gato", []string{"gato", "gatos", "gata"}},
	{"Pássaro", []string{"passaro", "passaros", "canario", "calopsita", "periquito"}},
}

// Club is the persisted football membership: Inter, Grêmio, or Outro plus name.
type Club struct {
	Club  string
	Other string
}

// FormatTeam returns the canonical gincana team, or empty.
func FormatTeam(value string) string {
	if isAbsent(value) {
		return ""
	}
	key := normalize.SearchText(value)
	if label, ok := firstLabeled(key, teamPhrases); ok {
		return label
	}
	return teamTokens[key]
}

// FormatSector returns closed sector labels, semicolon-separated.
func FormatSector(value string) string {
	if isAbsent(value) {
		return ""
	}
	key := normalize.SearchText(value)
	var found []string
	seen := map[string]bool{}
	for _, token := range strings.Fields(key) {
		alias, ok := sectorAliases[token]
		if !ok {
			continue
		}
		label := sectorLabels[alias]
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		found = append(found, label)
	}
	return strings.Join(found, "; ")
}

// FormatClub returns Internacional, Grêmio, or Outro with the other team name.
func FormatClub(value string) Club {
	if isAbsent(value) {
		return Club{}
	}
	key := normalize.SearchText(value)
	for _, blocked := range notFootball {
		if strings.Contains(key, blocked) && !strings.Contains(key, "gremio") && !strings.Contains(key, "internacional") {
			return Club{}
		}
	}
	if label, ok := footballTypos[key]; ok {
		return Club{Club: label}
	}
	main := ""
	if strings.Contains(key, "gremio") {
		main = "Grêmio"
	}
	if strings.Contains(key, "internacional") || hasWord(key, "inter") {
		if main == "" {
			main = "Internacional"
		}
	}
	var others []string
	for _, rule := range footballOther {
		for _, needle := range rule.needles {
			if hasNeedle(key, needle) {
				others = append(others, rule.label)
				break
			}
		}
	}
	if main != "" {
		return Club{Club: main, Other: strings.Join(others, "; ")}
	}
	if len(others) > 0 {
		return Club{Club: "Outro", Other: strings.Join(others, "; ")}
	}
	return Club{}
}

// FormatHealthPlan returns known operators, semicolon-separated.
func FormatHealthPlan(value string) string {
	if isAbsent(value) {
		return ""
	}
	key := normalize.SearchText(value)
	if hasWord(key, "sus") {
		return ""
	}
	return joinLabeled(key, planRules)
}

// Card is bandeira plus issuing bank.
type Card struct {
	Brand string
	Bank  string
}

// FormatCard returns brand and bank from a free-text card cell.
func FormatCard(value string) Card {
	if isAbsent(value) {
		return Card{}
	}
	key := normalize.SearchText(value)
	return Card{
		Brand: joinLabeled(key, cardBrands),
		Bank:  joinLabeled(key, cardBanks),
	}
}

// FormatCollection returns known collection types.
func FormatCollection(value string) string {
	if isAbsent(value) {
		return ""
	}
	return joinLabeled(normalize.SearchText(value), collectionRules)
}

// FormatAnimal returns known animal types.
func FormatAnimal(value string) string {
	if isAbsent(value) {
		return ""
	}
	return joinLabeled(normalize.SearchText(value), animalRules)
}

func firstLabeled(key string, rules []labeledRule) (string, bool) {
	blob := " " + key + " "
	for _, rule := range rules {
		for _, needle := range rule.needles {
			if strings.Contains(blob, " "+needle+" ") || key == needle {
				return rule.label, true
			}
		}
	}
	return "", false
}

func joinLabeled(key string, rules []labeledRule) string {
	var found []string
	seen := map[string]bool{}
	for _, rule := range rules {
		if seen[rule.label] {
			continue
		}
		for _, needle := range rule.needles {
			if hasNeedle(key, needle) {
				seen[rule.label] = true
				found = append(found, rule.label)
				break
			}
		}
	}
	return strings.Join(found, "; ")
}

func hasNeedle(key, needle string) bool {
	if len(needle) <= 3 {
		return hasWord(key, needle) || key == needle
	}
	return strings.Contains(key, needle)
}

func hasWord(key, word string) bool {
	return strings.Contains(" "+key+" ", " "+word+" ")
}

func isAbsent(value string) bool {
	key := normalize.SearchText(value)
	if key == "" || absentValues[key] {
		return true
	}
	return strings.HasPrefix(key, "nao ") || strings.HasPrefix(key, "nenhum")
}

var absentValues = map[string]bool{
	"nao": true, "n": true, "nao tenho": true, "nao possuo": true, "nao tem": true,
	"nenhum": true, "nenhuma": true, "nao possui": true, "nops": true, "no": true,
	"nunca": true, "sem": true, "nao sei": true, "0": true,
}

var membershipValues = map[string]string{
	"cartao": "cartao", "socio": "socio", "socia": "socio",
	"cadastro": "cadastro", "so cadas": "cadastro", "so cadastro": "cadastro",
	"menor": "infantil", "infantil": "infantil",
	"senior": "senior",
}

// FormatMembershipType returns cartao, socio, cadastro, infantil, or senior.
func FormatMembershipType(value string) string {
	if isAbsent(value) {
		return ""
	}
	return membershipValues[normalize.SearchText(value)]
}

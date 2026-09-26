package document

import (
	"regexp"
	"strings"
	"unicode"
)

type IdentifierAction string

const (
	IdentifierKeep   IdentifierAction = "keep"
	IdentifierDelete IdentifierAction = "delete"
)

type IdentifierClassification struct {
	Action IdentifierAction
	Number string
	State  string
}

var (
	brazilStates = map[string]struct{}{
		"AC": {}, "AL": {}, "AP": {}, "AM": {}, "BA": {}, "CE": {}, "DF": {}, "ES": {}, "GO": {},
		"MA": {}, "MT": {}, "MS": {}, "MG": {}, "PA": {}, "PB": {}, "PR": {}, "PE": {}, "PI": {},
		"RJ": {}, "RN": {}, "RS": {}, "RO": {}, "RR": {}, "SC": {}, "SP": {}, "SE": {}, "TO": {},
	}
	refusalPattern         = regexp.MustCompile(`^(nao|nao tenho|nao possuo|nao tenho isso|perdi|perdida|perdido|n)$`)
	possessionPattern      = regexp.MustCompile(`^(sim|possuo|tenho|sim possuo|tenho sim|possuo sim)$`)
	labeledNumberPattern   = regexp.MustCompile(`(?i)\b(oab|crea|carteirinha|cartao|cnh|rg|cpf|passaporte|titulo|sus|pis|pasep|nit|nis|ctps)\b`)
	slashStatePattern      = regexp.MustCompile(`(?i)[/ ]([a-z]{2})\b`)
	numberTokenPattern     = regexp.MustCompile(`[0-9][0-9.\-/]*`)
	cleanIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9./-]*$`)
)

func ClassifyIdentifier(raw string) IdentifierClassification {
	trimmed := strings.TrimSpace(strings.ToValidUTF8(raw, ""))
	if trimmed == "" {
		return IdentifierClassification{Action: IdentifierKeep}
	}
	folded := foldIdentifier(trimmed)
	if refusalPattern.MatchString(folded) {
		return IdentifierClassification{Action: IdentifierDelete}
	}
	state := extractState(trimmed)
	if possessionPattern.MatchString(folded) {
		return IdentifierClassification{Action: IdentifierKeep, State: state}
	}
	if cleanIdentifierPattern.MatchString(trimmed) && !labeledNumberPattern.MatchString(trimmed) {
		return IdentifierClassification{Action: IdentifierKeep, Number: trimmed, State: state}
	}
	number := extractDocumentNumber(trimmed)
	if number != "" {
		return IdentifierClassification{Action: IdentifierKeep, Number: number, State: state}
	}
	if possessionWords(folded) {
		return IdentifierClassification{Action: IdentifierKeep, State: state}
	}
	return IdentifierClassification{Action: IdentifierDelete}
}

func ApplyIdentifierClassification(raw string) (string, IdentifierClassification, error) {
	classified := ClassifyIdentifier(raw)
	if classified.Action == IdentifierDelete {
		return "", classified, &ValidationError{Fields: []FieldError{{Field: "identifier_value", Code: "refused"}}}
	}
	return classified.Number, classified, nil
}

func foldIdentifier(value string) string {
	normalized := strings.ToLower(value)
	var builder strings.Builder
	lastSpace := true
	for _, r := range normalized {
		switch {
		case r == 'ã' || r == 'á' || r == 'à' || r == 'â':
			r = 'a'
		case r == 'é' || r == 'ê':
			r = 'e'
		case r == 'í':
			r = 'i'
		case r == 'ó' || r == 'ô' || r == 'õ':
			r = 'o'
		case r == 'ú' || r == 'ü':
			r = 'u'
		case r == 'ç':
			r = 'c'
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			builder.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func possessionWords(folded string) bool {
	return strings.Contains(folded, "sim") || strings.Contains(folded, "possuo") || strings.Contains(folded, "tenho")
}

func extractState(value string) string {
	matches := slashStatePattern.FindAllStringSubmatch(strings.ToUpper(value), -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		candidate := match[1]
		if _, ok := brazilStates[candidate]; ok {
			return candidate
		}
	}
	return ""
}

func extractDocumentNumber(value string) string {
	best := ""
	for _, token := range numberTokenPattern.FindAllString(value, -1) {
		cleaned := strings.ReplaceAll(token, ".", "")
		cleaned = strings.Trim(cleaned, "-/")
		if cleaned == "" {
			continue
		}
		if digitCount(cleaned) > digitCount(best) || (digitCount(cleaned) == digitCount(best) && len(cleaned) > len(best)) {
			best = cleaned
		}
	}
	return best
}

func digitCount(value string) int {
	count := 0
	for _, r := range value {
		if r >= '0' && r <= '9' {
			count++
		}
	}
	return count
}

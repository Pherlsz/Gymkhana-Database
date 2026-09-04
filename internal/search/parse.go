package search

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

type CompareOp string

const (
	CompareContains CompareOp = "contains"
	ComparePrefix   CompareOp = "prefix"
	ComparePhrase   CompareOp = "phrase"
	CompareGT       CompareOp = "gt"
	CompareGTE      CompareOp = "gte"
	CompareLT       CompareOp = "lt"
	CompareLTE      CompareOp = "lte"
	CompareBetween  CompareOp = "between"
)

type Atom struct {
	Exclude    bool
	FieldToken string
	Value      string
	Value2     string
	Compare    CompareOp
	TypeSugar  string
}

type AndClause struct {
	Atoms []Atom
}

type ParsedQuery struct {
	Branches []AndClause
	Modules  []Module
}

func ParseQuery(raw string) (ParsedQuery, *ValidationError) {
	validation := &ValidationError{}
	raw = strings.TrimSpace(strings.ToValidUTF8(raw, ""))
	if raw == "" {
		validation.add("q", "required")
		return ParsedQuery{}, validation
	}
	if utf8.RuneCountInString(raw) > MaxQueryLength {
		validation.add("q", "too_long")
		return ParsedQuery{}, validation
	}
	tokens, err := tokenize(raw)
	if err != nil {
		validation.add("q", err.Error())
		return ParsedQuery{}, validation
	}
	parsed := ParsedQuery{Branches: []AndClause{{}}}
	for _, token := range tokens {
		if token.kind == tokOR {
			if len(parsed.Branches[len(parsed.Branches)-1].Atoms) == 0 {
				validation.add("q", "invalid_syntax")
				continue
			}
			parsed.Branches = append(parsed.Branches, AndClause{})
			continue
		}
		atom, modules, atomErr := token.atom()
		if atomErr != "" {
			validation.add("q", atomErr)
			continue
		}
		if len(modules) > 0 {
			parsed.Modules = appendUniqueModules(parsed.Modules, modules...)
			continue
		}
		if isPhysicalName(atom.FieldToken) {
			validation.add("q", "physical_name")
			continue
		}
		last := &parsed.Branches[len(parsed.Branches)-1]
		last.Atoms = append(last.Atoms, atom)
	}
	if len(parsed.Branches) > 0 && len(parsed.Branches[len(parsed.Branches)-1].Atoms) == 0 && len(parsed.Modules) == 0 {
		validation.add("q", "invalid_syntax")
	}
	clauseCount := 0
	for _, branch := range parsed.Branches {
		clauseCount += len(branch.Atoms)
	}
	if clauseCount > MaxTerms {
		validation.add("q", "too_many")
	}
	if len(validation.Fields) > 0 {
		return ParsedQuery{}, validation
	}
	cleaned := ParsedQuery{Modules: parsed.Modules}
	for _, branch := range parsed.Branches {
		if len(branch.Atoms) == 0 {
			continue
		}
		cleaned.Branches = append(cleaned.Branches, branch)
	}
	if len(cleaned.Branches) == 0 && len(cleaned.Modules) == 0 {
		validation.add("q", "required")
		return ParsedQuery{}, validation
	}
	if len(cleaned.Branches) == 0 {
		cleaned.Branches = []AndClause{{}}
	}
	return cleaned, nil
}

type tokKind int

const (
	tokWord tokKind = iota
	tokOR
)

type rawToken struct {
	kind    tokKind
	exclude bool
	quoted  bool
	text    string
}

func tokenize(raw string) ([]rawToken, error) {
	tokens := make([]rawToken, 0)
	runes := []rune(raw)
	i := 0
	for i < len(runes) {
		if unicode.IsSpace(runes[i]) {
			i++
			continue
		}
		exclude := false
		if runes[i] == '-' && i+1 < len(runes) && !unicode.IsSpace(runes[i+1]) {
			exclude = true
			i++
		}
		if runes[i] == '/' {
			i++
			if i >= len(runes) {
				break
			}
		}
		quoted := i < len(runes) && runes[i] == '"'
		text, next, err := readToken(runes, i)
		if err != nil {
			return nil, err
		}
		i = next
		if text == "" {
			continue
		}
		if !exclude && !quoted && isOrToken(text) {
			tokens = append(tokens, rawToken{kind: tokOR})
			continue
		}
		tokens = append(tokens, rawToken{kind: tokWord, exclude: exclude, quoted: quoted, text: text})
	}
	return tokens, nil
}

func readToken(runes []rune, i int) (string, int, error) {
	if i >= len(runes) {
		return "", i, nil
	}
	if runes[i] == '"' {
		return readQuoted(runes, i)
	}
	start := i
	colon := -1
	for i < len(runes) && !unicode.IsSpace(runes[i]) {
		if runes[i] == ':' && colon < 0 {
			colon = i
		}
		i++
	}
	text := string(runes[start:i])
	if colon > start && i < len(runes) && unicode.IsSpace(runes[i]) {
		// keep unquoted field values as a single token (no spaces)
	}
	if colon > start && colon+1 < i && runes[colon+1] == '"' {
		quoted, next, err := readQuoted(runes, colon+1)
		if err != nil {
			return "", i, err
		}
		return string(runes[start:colon+1]) + quoted, next, nil
	}
	return text, i, nil
}

func readQuoted(runes []rune, i int) (string, int, error) {
	if i >= len(runes) || runes[i] != '"' {
		return "", i, nil
	}
	i++
	var b strings.Builder
	for i < len(runes) {
		if runes[i] == '\\' && i+1 < len(runes) {
			b.WriteRune(runes[i+1])
			i += 2
			continue
		}
		if runes[i] == '"' {
			return b.String(), i + 1, nil
		}
		b.WriteRune(runes[i])
		i++
	}
	return "", i, errUnclosedQuote
}

const errUnclosedQuote = invalidSyntax("unclosed_quote")

type invalidSyntax string

func (err invalidSyntax) Error() string { return string(err) }

func isOrToken(text string) bool {
	folded := foldToken(text)
	return folded == "ou" || strings.EqualFold(text, "OR")
}

func (token rawToken) atom() (Atom, []Module, string) {
	compare := CompareContains
	if token.quoted {
		compare = ComparePhrase
	}
	atom := Atom{Exclude: token.exclude, Compare: compare, Value: token.text}
	field, value, hasField := strings.Cut(token.text, ":")
	if !hasField {
		return atom, nil, validateAtomValue(atom)
	}
	field = strings.TrimPrefix(field, "/")
	if foldToken(field) == "em" || foldToken(field) == "in" {
		module, ok := lookupModuleAlias(value)
		if !ok {
			return Atom{}, nil, "unknown_module"
		}
		return Atom{}, []Module{module}, ""
	}
	atom.FieldToken = field
	atom.Value = value
	if sugar, ok := lookupTypeSugar(field); ok {
		atom.TypeSugar = sugar
	}
	applyValueShape(&atom)
	if code := validateAtomValue(atom); code != "" {
		return Atom{}, nil, code
	}
	return atom, nil, ""
}

func applyValueShape(atom *Atom) {
	value := atom.Value
	if strings.HasPrefix(value, `"`) {
		unquoted := strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`)
		atom.Value = unquoted
		if atom.Compare == CompareContains {
			atom.Compare = ComparePhrase
		}
		value = unquoted
	}
	if strings.HasSuffix(value, "*") && !strings.Contains(value, "..") {
		atom.Value = strings.TrimSuffix(value, "*")
		atom.Compare = ComparePrefix
		return
	}
	if left, right, ok := strings.Cut(value, ".."); ok && left != "" && right != "" {
		atom.Value = left
		atom.Value2 = right
		atom.Compare = CompareBetween
		return
	}
	switch {
	case strings.HasPrefix(value, ">="):
		atom.Value = strings.TrimPrefix(value, ">=")
		atom.Compare = CompareGTE
	case strings.HasPrefix(value, "<="):
		atom.Value = strings.TrimPrefix(value, "<=")
		atom.Compare = CompareLTE
	case strings.HasPrefix(value, ">"):
		atom.Value = strings.TrimPrefix(value, ">")
		atom.Compare = CompareGT
	case strings.HasPrefix(value, "<"):
		atom.Value = strings.TrimPrefix(value, "<")
		atom.Compare = CompareLT
	}
}

func validateAtomValue(atom Atom) string {
	if atom.FieldToken == "" && atom.Value == "" {
		return "empty"
	}
	if atom.TypeSugar == "" && atom.Value == "" {
		return "empty"
	}
	if utf8.RuneCountInString(atom.Value) > MaxTermLength || utf8.RuneCountInString(atom.Value2) > MaxTermLength {
		return "too_long"
	}
	for _, character := range atom.Value + atom.Value2 {
		if unicode.IsControl(character) {
			return "invalid_characters"
		}
	}
	return ""
}

func appendUniqueModules(existing []Module, extras ...Module) []Module {
	seen := make(map[Module]struct{}, len(existing)+len(extras))
	out := make([]Module, 0, len(existing)+len(extras))
	for _, module := range existing {
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}
		out = append(out, module)
	}
	for _, module := range extras {
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}
		out = append(out, module)
	}
	return out
}

func foldValue(value string) string {
	return normalize.SearchText(value)
}

func digitValue(value string) string {
	return normalize.Digits(value)
}

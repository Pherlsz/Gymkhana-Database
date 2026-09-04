package search

import (
	"strings"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
)

type resolvedAtom struct {
	Atom
	FieldKeys []string
	Kind      string
}

type resolvedQuery struct {
	Branches [][]resolvedAtom
	Modules  []Module
}

func resolveParsed(parsed ParsedQuery, catalog Catalog) (resolvedQuery, *ValidationError) {
	validation := &ValidationError{}
	byKey := make(map[string]FieldDefinition, len(catalog.Fields))
	byFoldedLabel := map[string][]string{}
	for _, field := range catalog.Fields {
		byKey[field.Key] = field
		folded := foldToken(field.Label)
		byFoldedLabel[folded] = append(byFoldedLabel[folded], field.Key)
		if cut, ok := strings.CutPrefix(field.Key, "custom."); ok && cut != "" {
			byFoldedLabel[foldToken(cut)] = append(byFoldedLabel[foldToken(cut)], field.Key)
		}
	}
	resolved := resolvedQuery{Modules: parsed.Modules}
	for _, branch := range parsed.Branches {
		atoms := make([]resolvedAtom, 0, len(branch.Atoms)*2)
		for _, atom := range branch.Atoms {
			expanded := expandTypeSugar(atom)
			for _, item := range expanded {
				bound, bindErr := bindAtom(item, catalog, byKey, byFoldedLabel)
				if bindErr != nil {
					validation.Fields = append(validation.Fields, bindErr.Fields...)
					continue
				}
				atoms = append(atoms, bound)
			}
		}
		if len(atoms) > 0 {
			resolved.Branches = append(resolved.Branches, atoms)
		}
	}
	if len(validation.Fields) > 0 {
		return resolvedQuery{}, validation
	}
	return resolved, nil
}

func expandTypeSugar(atom Atom) []Atom {
	if atom.TypeSugar == "" {
		return []Atom{atom}
	}
	tipo := Atom{Exclude: atom.Exclude, FieldToken: "tipo", Value: atom.TypeSugar, Compare: CompareContains, TypeSugar: atom.TypeSugar}
	if strings.TrimSpace(atom.Value) == "" {
		return []Atom{tipo}
	}
	ident := Atom{
		Exclude:    atom.Exclude,
		FieldToken: "identificador",
		Value:      atom.Value,
		Value2:     atom.Value2,
		Compare:    atom.Compare,
		TypeSugar:  atom.TypeSugar,
	}
	return []Atom{tipo, ident}
}

func bindAtom(atom Atom, catalog Catalog, byKey map[string]FieldDefinition, byFoldedLabel map[string][]string) (resolvedAtom, *ValidationError) {
	validation := &ValidationError{}
	if atom.FieldToken == "" {
		return resolvedAtom{Atom: atom}, nil
	}
	if isPhysicalName(atom.FieldToken) {
		validation.add("q", "physical_name")
		return resolvedAtom{}, validation
	}
	keys := bindFieldKeys(atom.FieldToken, byKey, byFoldedLabel)
	if len(keys) == 0 {
		validation.Fields = append(validation.Fields, FieldError{
			Field:  "q",
			Code:   "unknown_field",
			Detail: suggestField(atom.FieldToken, catalog),
		})
		return resolvedAtom{}, validation
	}
	kind := "text"
	if field, ok := byKey[keys[0]]; ok {
		kind = field.Kind
	}
	return resolvedAtom{Atom: atom, FieldKeys: keys, Kind: kind}, nil
}

func bindFieldKeys(token string, byKey map[string]FieldDefinition, byFoldedLabel map[string][]string) []string {
	if _, ok := byKey[token]; ok {
		return []string{token}
	}
	if alias, ok := lookupFieldAlias(token); ok {
		keys := make([]string, 0, len(alias.keys))
		for _, key := range alias.keys {
			if _, exists := byKey[key]; exists {
				keys = append(keys, key)
			}
		}
		return keys
	}
	return uniqueStrings(byFoldedLabel[foldToken(token)])
}

func suggestField(token string, catalog Catalog) string {
	folded := foldToken(token)
	matches := make([]string, 0, 5)
	for _, field := range catalog.Fields {
		label := foldToken(field.Label)
		if strings.HasPrefix(label, folded) || strings.HasPrefix(foldToken(strings.TrimPrefix(field.Key, "profile.")), folded) {
			matches = append(matches, field.Label)
		}
	}
	for _, queryToken := range queryTokens {
		if strings.HasPrefix(foldToken(queryToken.Token), folded) {
			matches = append(matches, queryToken.Token)
		}
	}
	matches = uniqueStrings(matches)
	if len(matches) == 0 {
		return ""
	}
	if len(matches) > 5 {
		matches = matches[:5]
	}
	return strings.Join(matches, ", ")
}

func compileBranch(atoms []resolvedAtom) (includes, excludes []TermSpec, terms []string) {
	for _, atom := range atoms {
		spec := termSpecFor(atom)
		if atom.Exclude {
			excludes = append(excludes, spec)
			continue
		}
		includes = append(includes, spec)
		terms = append(terms, spec.Term)
	}
	return includes, excludes, uniqueStrings(terms)
}

func termSpecFor(atom resolvedAtom) TermSpec {
	compare := atom.Compare
	if compare == "" {
		compare = CompareContains
	}
	values := compileValues(atom)
	if len(values) == 0 {
		values = []string{atom.Value}
	}
	term := values[0]
	if utf8.RuneCountInString(term) == 0 {
		term = atom.Value
		values = []string{atom.Value}
	}
	patterns := make([]string, 0, len(values))
	for _, value := range values {
		patterns = append(patterns, literalPattern(value, compare))
	}
	patterns = uniqueStrings(patterns)
	keys := atom.FieldKeys
	if keys == nil {
		keys = []string{}
	}
	return TermSpec{
		Term:      term,
		Pattern:   patterns[0],
		Patterns:  patterns,
		FieldKeys: keys,
		Exclude:   atom.Exclude,
		Compare:   string(compare),
		Value:     values[0],
		Value2:    atom.Value2,
	}
}

func compileValues(atom resolvedAtom) []string {
	raw := strings.TrimSpace(atom.Value)
	values := make([]string, 0, 8)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}

	foldedField := foldToken(atom.FieldToken)
	if foldedField == "tipo" || foldedField == "type" {
		if kind, ok := lookupTypeSugar(raw); ok {
			add(kind)
		}
		add(foldValue(raw))
		return uniqueStrings(values)
	}

	kind := atom.TypeSugar
	if kind == "" {
		kind, _ = lookupTypeSugar(atom.FieldToken)
	}
	if kind != "" && normalize.KnownDocumentKind(normalize.DocumentKind(kind)) {
		if canonical, err := normalize.CanonicalDocument(normalize.DocumentKind(kind), raw); err == nil {
			add(canonical)
		}
	}

	switch {
	case foldedField == "cep" || foldedField == "postal_code" || foldedField == "zip" || hasFieldKey(atom, "profile.address_postal_code"):
		if cep, err := normalize.CanonicalCEP(raw); err == nil {
			add(cep)
			if formatted := normalize.FormatCEP(raw); formatted != "" {
				add(formatted)
			}
		}
		add(digitValue(raw))
	case atom.Kind == "email" || foldedField == "email" || foldedField == "mail" || hasFieldKey(atom, "profile.email"):
		if email, err := normalize.CanonicalEmail(raw); err == nil {
			add(email)
		}
		add(foldValue(raw))
	case atom.Kind == "phone" || hasFieldKey(atom, "profile.mobile_phone") || hasFieldKey(atom, "profile.landline_phone"):
		if phone, err := normalize.CanonicalBrazilPhone(raw); err == nil {
			add(phone)
			add(strings.TrimPrefix(phone, "+"))
		}
		add(digitValue(raw))
	case atom.Kind == "identifier" || isIdentifierField(foldedField):
		add(normalize.Alphanumeric(raw))
		add(digitValue(raw))
		add(foldValue(raw))
	default:
		if atom.FieldToken == "" {
			for _, match := range normalize.IdentifyDocumentMatches(raw) {
				add(match.Canonical)
			}
			add(digitValue(raw))
		}
		add(foldValue(raw))
	}

	if len(values) == 0 {
		add(raw)
	}
	return uniqueStrings(values)
}

func isIdentifierField(folded string) bool {
	switch folded {
	case "identificador", "identifier", "id", "cep":
		return true
	default:
		_, ok := lookupTypeSugar(folded)
		return ok
	}
}

func hasFieldKey(atom resolvedAtom, key string) bool {
	for _, candidate := range atom.FieldKeys {
		if candidate == key {
			return true
		}
	}
	return false
}

func literalPattern(value string, compare CompareOp) string {
	escaped := escapeLike(value)
	switch compare {
	case ComparePrefix:
		return escaped + "%"
	case ComparePhrase, CompareContains:
		return "%" + escaped + "%"
	default:
		return "%" + escaped + "%"
	}
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(strings.ToLower(value))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func simplifyLookup(parsed ParsedQuery) ParsedQuery {
	cleaned := ParsedQuery{}
	for _, branch := range parsed.Branches {
		atoms := make([]Atom, 0, len(branch.Atoms))
		for _, atom := range branch.Atoms {
			if atom.Exclude {
				continue
			}
			if atom.FieldToken == "" || foldToken(atom.FieldToken) == "nome" || atom.TypeSugar == string(normalize.DocumentCPF) || foldToken(atom.FieldToken) == "name" {
				atoms = append(atoms, atom)
			}
		}
		if len(atoms) > 0 {
			cleaned.Branches = append(cleaned.Branches, AndClause{Atoms: atoms})
		}
	}
	if len(cleaned.Branches) == 0 {
		return parsed
	}
	return cleaned
}

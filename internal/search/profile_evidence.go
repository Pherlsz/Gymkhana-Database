package search

import (
	"strings"
	"unicode/utf8"
)

// ProfileHit is one field that matched a people search. Weight is the field
// weight from the search catalog, not the relevance score.
type ProfileHit struct {
	ProfileID  string
	FieldKey   string
	FieldLabel string
	Display    string
	Weight     int32
}

// HiddenFieldEvidence is the highest-weight hidden field for one person.
type HiddenFieldEvidence struct {
	FieldKey           string
	Label              string
	Snippet            string
	OtherHiddenMatches int
}

// defaultPeopleColumns matches the people sheet's default visible keys.
// notes is intentionally absent.
var defaultPeopleColumns = []string{
	"full_name", "documents", "cpf", "rg", "street", "number", "city", "postal_code",
	"email", "mobile", "birth_date", "team", "sector",
	"voter_id", "cnh", "ctps", "ctps_series", "pis", "crea", "oab", "student_id",
	"sus_card", "citizen_card", "passport",
}

// profileColumnFields maps a table column key to the search field keys that
// column shows. Document identifier hits do not carry a technical key, so the
// documents and cpf columns both cover those field keys.
var profileColumnFields = map[string][]string{
	"full_name":        {"profile.full_name"},
	"social_name":      {"profile.social_name"},
	"email":            {"profile.email"},
	"mobile":           {"profile.mobile_phone"},
	"landline":         {"profile.landline_phone"},
	"street":           {"profile.address_street"},
	"number":           {"profile.address_number"},
	"complement":       {"profile.address_complement"},
	"neighborhood":     {"profile.address_neighborhood"},
	"city":             {"profile.address_city"},
	"state":            {"profile.address_state"},
	"postal_code":      {"profile.address_postal_code"},
	"notes":            {"profile.notes"},
	"team":             {"profile.team"},
	"club_membership":  {"profile.club_membership"},
	"place_of_origin":  {"profile.place_of_origin"},
	"birth_country":    {"profile.birth_country"},
	"supermarket_club": {"profile.supermarket_club"},
	"pet":              {"profile.pet"},
	"travel_countries": {"profile.travel_countries"},
	"card_brand":       {"profile.card_brand"},
	"card_bank":        {"profile.card_bank"},
	"documents": {
		"profile.document_identifier", "document.identifier", "document.type",
		"document.date", "document.notes", "document.medium", "document.current_holder",
	},
	"cpf": {"profile.document_identifier", "document.identifier"},
}

// PartitionProfileIDs stably splits sortedIDs into people who match a visible
// column, then people who only match a hidden field. Evidence is set only for
// the hidden-only group. allowed drops field keys the caller cannot search;
// nil keeps every hit. useDefault applies the people sheet's default columns
// when the client did not send any.
func PartitionProfileIDs(sortedIDs []string, hits []ProfileHit, columns []string, useDefault bool, allowed map[string]struct{}, term string) ([]string, map[string]HiddenFieldEvidence) {
	visibleFields := visibleSearchFields(columns, useDefault)
	grouped := map[string][]ProfileHit{}
	for _, hit := range hits {
		if hit.ProfileID == "" || hit.FieldKey == "" {
			continue
		}
		if allowed != nil {
			if _, ok := allowed[hit.FieldKey]; !ok {
				continue
			}
		}
		grouped[hit.ProfileID] = append(grouped[hit.ProfileID], hit)
	}
	if sortedIDs == nil {
		seen := map[string]struct{}{}
		sortedIDs = make([]string, 0)
		for _, hit := range hits {
			if hit.ProfileID == "" {
				continue
			}
			if _, ok := seen[hit.ProfileID]; ok {
				continue
			}
			seen[hit.ProfileID] = struct{}{}
			sortedIDs = append(sortedIDs, hit.ProfileID)
		}
	}
	visible := make([]string, 0, len(sortedIDs))
	hidden := make([]string, 0)
	evidence := map[string]HiddenFieldEvidence{}
	for _, id := range sortedIDs {
		profileHits := grouped[id]
		if matchIsVisible(profileHits, visibleFields) || len(hiddenHits(profileHits, visibleFields)) == 0 {
			visible = append(visible, id)
			continue
		}
		hidden = append(hidden, id)
		if ev, ok := hiddenEvidence(profileHits, visibleFields, term); ok {
			evidence[id] = ev
		}
	}
	return append(visible, hidden...), evidence
}

func ProfileIDsFromHits(hits []ProfileHit) []string {
	seen := map[string]struct{}{}
	ids := make([]string, 0)
	for _, hit := range hits {
		if hit.ProfileID == "" {
			continue
		}
		if _, ok := seen[hit.ProfileID]; ok {
			continue
		}
		seen[hit.ProfileID] = struct{}{}
		ids = append(ids, hit.ProfileID)
	}
	return ids
}

func visibleSearchFields(columns []string, useDefault bool) map[string]struct{} {
	if useDefault {
		columns = defaultPeopleColumns
	}
	fields := map[string]struct{}{}
	addColumn := func(column string) {
		column = strings.TrimSpace(column)
		if column == "" {
			return
		}
		if strings.HasPrefix(column, "custom:") {
			fields["custom."+strings.TrimPrefix(column, "custom:")] = struct{}{}
			return
		}
		for _, key := range profileColumnFields[column] {
			fields[key] = struct{}{}
		}
	}
	addColumn("full_name")
	for _, column := range columns {
		addColumn(column)
	}
	return fields
}

func matchIsVisible(hits []ProfileHit, visible map[string]struct{}) bool {
	for _, hit := range hits {
		if _, ok := visible[hit.FieldKey]; ok {
			return true
		}
	}
	return false
}

func hiddenHits(hits []ProfileHit, visible map[string]struct{}) []ProfileHit {
	out := make([]ProfileHit, 0)
	for _, hit := range hits {
		if _, ok := visible[hit.FieldKey]; ok {
			continue
		}
		out = append(out, hit)
	}
	return out
}

func hiddenEvidence(hits []ProfileHit, visible map[string]struct{}, term string) (HiddenFieldEvidence, bool) {
	hidden := hiddenHits(hits, visible)
	if len(hidden) == 0 {
		return HiddenFieldEvidence{}, false
	}
	best := hidden[0]
	fields := map[string]struct{}{}
	for _, hit := range hidden {
		fields[hit.FieldKey] = struct{}{}
		if hit.Weight > best.Weight || (hit.Weight == best.Weight && hit.FieldKey < best.FieldKey) {
			best = hit
		}
	}
	others := len(fields) - 1
	if others < 0 {
		others = 0
	}
	return HiddenFieldEvidence{
		FieldKey:           best.FieldKey,
		Label:              best.FieldLabel,
		Snippet:            searchSnippet(best.Display, term),
		OtherHiddenMatches: others,
	}, true
}

func searchSnippet(display, term string) string {
	display = strings.TrimSpace(display)
	term = strings.TrimSpace(term)
	if display == "" {
		return ""
	}
	if term == "" {
		return clipRunes(display, 48)
	}
	lowerDisplay := strings.ToLower(display)
	lowerTerm := strings.ToLower(term)
	idx := strings.Index(lowerDisplay, lowerTerm)
	matchLen := len(lowerTerm)
	if idx < 0 {
		for _, word := range strings.Fields(lowerTerm) {
			if word == "" {
				continue
			}
			if found := strings.Index(lowerDisplay, word); found >= 0 {
				idx = found
				matchLen = len(word)
				break
			}
		}
	}
	if idx < 0 || idx+matchLen > len(display) {
		return clipRunes(display, 48)
	}
	start := utf8.RuneCountInString(display[:idx])
	end := utf8.RuneCountInString(display[:idx+matchLen])
	runes := []rune(display)
	if start < 0 || end > len(runes) || start > end {
		return clipRunes(display, 48)
	}
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "\u2026" + snippet
	}
	if end < len(runes) {
		snippet += "\u2026"
	}
	return snippet
}

func clipRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "\u2026"
}

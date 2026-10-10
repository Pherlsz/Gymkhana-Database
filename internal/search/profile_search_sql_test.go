package search

import (
	"strings"
	"testing"
)

func TestRenderProfileSearchSQLSkipsMissingArchiveColumns(t *testing.T) {
	columns := map[string]struct{}{
		"full_name": {}, "social_name": {}, "email": {}, "mobile_phone": {},
		"landline_phone": {}, "address_street": {}, "address_number": {},
		"address_complement": {}, "address_neighborhood": {}, "address_city": {},
		"address_state": {}, "address_postal_code": {}, "notes": {}, "team": {},
		"club_membership": {},
	}
	values, prefilter := renderProfileSearchSQL(columns)
	for _, column := range []string{"place_of_origin", "birth_country", "supermarket_club", "pet", "travel_countries", "card_brand", "card_bank"} {
		if strings.Contains(values, "profile."+column) {
			t.Fatalf("values included missing column %s:\n%s", column, values)
		}
		if strings.Contains(prefilter, "profile."+column) {
			t.Fatalf("prefilter included missing column %s:\n%s", column, prefilter)
		}
	}
	if !strings.Contains(values, "profile.full_name") {
		t.Fatal("values omitted required full_name")
	}
	if !strings.Contains(prefilter, "profile.full_name ILIKE pattern ESCAPE '\\'") {
		t.Fatalf("prefilter missing full_name: %s", prefilter)
	}
}

func TestRenderProfileSearchSQLIncludesArchiveColumnsWhenPresent(t *testing.T) {
	columns := allProfileSearchColumns()
	values, prefilter := renderProfileSearchSQL(columns)
	for _, column := range []string{"place_of_origin", "birth_country", "card_bank"} {
		if !strings.Contains(values, "profile."+column) {
			t.Fatalf("values omitted present column %s", column)
		}
	}
	if !strings.Contains(prefilter, "COALESCE(profile.place_of_origin, '') ILIKE pattern ESCAPE '\\'") {
		t.Fatalf("prefilter omitted place_of_origin: %s", prefilter)
	}
}

func TestBindProfileSearchSQLReplacesTokens(t *testing.T) {
	values, prefilter := renderProfileSearchSQL(allProfileSearchColumns())
	sql := bindProfileSearchSQL(searchBodyTemplate, values, prefilter) + searchProfileHitSQLTail
	if strings.Contains(sql, profileValuesToken) || strings.Contains(sql, profilePrefilterToken) {
		t.Fatal("compiled SQL still contains bind tokens")
	}
	if !strings.Contains(sql, "('profile.full_name', 'Nome completo', profile.full_name, profile.full_name, 80)") {
		t.Fatal("compiled SQL missing full_name value row")
	}
}

func TestValidProfileSearchColumnRejectsUnsafeIdentifiers(t *testing.T) {
	if !validProfileSearchColumn("place_of_origin") {
		t.Fatal("place_of_origin should be allowed")
	}
	for _, column := range []string{"", "FullName", "full-name", "full_name;drop", "full_name as x"} {
		if validProfileSearchColumn(column) {
			t.Fatalf("accepted unsafe column %q", column)
		}
	}
}

func allProfileSearchColumns() map[string]struct{} {
	columns := make(map[string]struct{}, len(profileSearchFields))
	for _, field := range profileSearchFields {
		columns[field.Column] = struct{}{}
	}
	return columns
}

func compiledSearchSQLForTest() (search, ids, profileIDs, hits string) {
	values, prefilter := renderProfileSearchSQL(allProfileSearchColumns())
	body := bindProfileSearchSQL(searchBodyTemplate, values, prefilter)
	return body + searchSQLTail, body + searchIDSQLTail, body + searchProfileIDSQLTail, body + searchProfileHitSQLTail
}

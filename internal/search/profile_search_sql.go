package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	profileValuesToken    = "__PROFILE_FIELD_VALUES__"
	profilePrefilterToken = "__PROFILE_PREFILTER__"
)

type profileSearchField struct {
	Key       string
	Label     string
	Column    string
	Weight    int
	Prefilter bool
}

// Physical Profile columns the search SQL may read. Extra archive columns from
// migration 033 are skipped when the connected database has not applied them.
var profileSearchFields = []profileSearchField{
	{Key: "profile.full_name", Label: "Nome completo", Column: "full_name", Weight: 80, Prefilter: true},
	{Key: "profile.social_name", Label: "Nome social", Column: "social_name", Weight: 70, Prefilter: true},
	{Key: "profile.email", Label: "E-mail", Column: "email", Weight: 60, Prefilter: true},
	{Key: "profile.mobile_phone", Label: "Celular", Column: "mobile_phone", Weight: 55, Prefilter: true},
	{Key: "profile.landline_phone", Label: "Telefone", Column: "landline_phone", Weight: 50, Prefilter: false},
	{Key: "profile.address_street", Label: "Logradouro", Column: "address_street", Weight: 35, Prefilter: true},
	{Key: "profile.address_number", Label: "Número", Column: "address_number", Weight: 25, Prefilter: false},
	{Key: "profile.address_complement", Label: "Complemento", Column: "address_complement", Weight: 20, Prefilter: false},
	{Key: "profile.address_neighborhood", Label: "Bairro", Column: "address_neighborhood", Weight: 30, Prefilter: false},
	{Key: "profile.address_city", Label: "Cidade", Column: "address_city", Weight: 40, Prefilter: true},
	{Key: "profile.address_state", Label: "UF", Column: "address_state", Weight: 20, Prefilter: false},
	{Key: "profile.address_postal_code", Label: "CEP", Column: "address_postal_code", Weight: 45, Prefilter: false},
	{Key: "profile.notes", Label: "Observações", Column: "notes", Weight: 10, Prefilter: true},
	{Key: "profile.team", Label: "Equipe da gincana", Column: "team", Weight: 40, Prefilter: true},
	{Key: "profile.club_membership", Label: "Sócio de clube de futebol", Column: "club_membership", Weight: 35, Prefilter: false},
	{Key: "profile.place_of_origin", Label: "Naturalidade", Column: "place_of_origin", Weight: 30, Prefilter: true},
	{Key: "profile.birth_country", Label: "País de nascimento", Column: "birth_country", Weight: 25, Prefilter: true},
	{Key: "profile.supermarket_club", Label: "Clube de supermercado", Column: "supermarket_club", Weight: 25, Prefilter: false},
	{Key: "profile.pet", Label: "Animal", Column: "pet", Weight: 20, Prefilter: false},
	{Key: "profile.travel_countries", Label: "Viagem", Column: "travel_countries", Weight: 20, Prefilter: false},
	{Key: "profile.card_brand", Label: "Bandeira do cartão", Column: "card_brand", Weight: 15, Prefilter: false},
	{Key: "profile.card_bank", Label: "Banco do cartão", Column: "card_bank", Weight: 15, Prefilter: false},
}

func renderProfileSearchSQL(columns map[string]struct{}) (values, prefilter string) {
	valueParts := make([]string, 0, len(profileSearchFields))
	predParts := make([]string, 0, 8)
	for _, field := range profileSearchFields {
		if !validProfileSearchColumn(field.Column) {
			continue
		}
		if _, ok := columns[field.Column]; !ok {
			continue
		}
		label := strings.ReplaceAll(field.Label, "'", "''")
		valueParts = append(valueParts, fmt.Sprintf(
			"    ('%s', '%s', profile.%s, profile.%s, %d)",
			field.Key, label, field.Column, field.Column, field.Weight,
		))
		if !field.Prefilter {
			continue
		}
		if field.Column == "full_name" {
			predParts = append(predParts, "profile.full_name ILIKE pattern ESCAPE '\\'")
			continue
		}
		predParts = append(predParts, fmt.Sprintf("COALESCE(profile.%s, '') ILIKE pattern ESCAPE '\\'", field.Column))
	}
	if len(valueParts) == 0 {
		valueParts = append(valueParts, "    ('profile.full_name', 'Nome completo', profile.full_name, profile.full_name, 80)")
		predParts = append(predParts, "profile.full_name ILIKE pattern ESCAPE '\\'")
	}
	return strings.Join(valueParts, ",\n"), strings.Join(predParts, "\n           OR ")
}

func validProfileSearchColumn(column string) bool {
	if column == "" {
		return false
	}
	for _, character := range column {
		if character != '_' && (character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func bindProfileSearchSQL(template, values, prefilter string) string {
	return strings.ReplaceAll(strings.ReplaceAll(template, profileValuesToken, values), profilePrefilterToken, prefilter)
}

func loadProfileColumns(ctx context.Context, pool *pgxpool.Pool) (map[string]struct{}, error) {
	rows, err := pool.Query(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = 'profiles'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make(map[string]struct{}, len(profileSearchFields))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns[name] = struct{}{}
	}
	return columns, rows.Err()
}

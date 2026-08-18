package queryengine

// portugueseTextCollation is created by database/migrations/035_portuguese_text_sort.sql.
// ICU pt-BR sorts accented letters with their base letter (Á with A), matching the
// SPA's localeCompare(..., "pt-BR") and the people-grid A→Z / Z→A funnel.
const portugueseTextCollation = "gymkhana_pt_br"

func orderExpression(sqlExpression string, kind ValueKind, direction string) string {
	if kind == ValueText || kind == ValueLongText {
		sqlExpression += " COLLATE " + portugueseTextCollation
	}
	return sqlExpression + " " + direction + " NULLS LAST"
}

package queryengine

import (
	"strings"
	"unicode"
)

const portugueseAccentFrom = "áàâãäéèêëíìîïóòôõöúùûüçÁÀÂÃÄÉÈÊËÍÌÎÏÓÒÔÕÖÚÙÛÜÇ"
const portugueseAccentTo = "aaaaaeeeeiiiiooooouuuucAAAAAEEEEIIIIOOOOOUUUUC"

func foldPortugueseSQL(expression string) string {
	return "lower(translate((" + expression + ")::text,'" + portugueseAccentFrom + "','" + portugueseAccentTo + "'))"
}

func foldPortugueseText(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, character := range value {
		if folded, ok := foldRune(character); ok {
			builder.WriteRune(unicode.ToLower(folded))
			continue
		}
		builder.WriteRune(unicode.ToLower(character))
	}
	return builder.String()
}

func FoldClubToken(value string) string {
	var builder strings.Builder
	for _, character := range foldPortugueseText(value) {
		if character >= 'a' && character <= 'z' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

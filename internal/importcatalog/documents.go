package importcatalog

import (
	"strings"

	"github.com/Pherlsz/Gymkhana-Core/normalize"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
)

const DocumentFieldPrefix = "document:"

// DocumentSidecar is one wide-sheet document column group keyed by catalog technical_key.
type DocumentSidecar struct {
	TypeKey    string
	Identifier string
	Date       string
}

func IsDocumentField(target string) bool {
	return strings.HasPrefix(target, DocumentFieldPrefix)
}

func ParseDocumentField(target string) (typeKey, facet string, ok bool) {
	if !IsDocumentField(target) {
		return "", "", false
	}
	rest := strings.TrimPrefix(target, DocumentFieldPrefix)
	typeKey, facet, _ = strings.Cut(rest, ":")
	if typeKey == "" {
		return "", "", false
	}
	return typeKey, facet, true
}

func CollectDocumentSidecars(values map[string]string) []DocumentSidecar {
	byType := map[string]*DocumentSidecar{}
	var order []string
	for field, raw := range values {
		typeKey, facet, ok := ParseDocumentField(field)
		if !ok {
			continue
		}
		item, exists := byType[typeKey]
		if !exists {
			item = &DocumentSidecar{TypeKey: typeKey}
			byType[typeKey] = item
			order = append(order, typeKey)
		}
		switch facet {
		case "", "identifier":
			identifier, date := SplitIdentifierDate(raw)
			item.Identifier = identifier
			if item.Date == "" {
				item.Date = date
			}
		case "date":
			item.Date = strings.TrimSpace(raw)
		}
	}
	result := make([]DocumentSidecar, 0, len(order))
	for _, key := range order {
		item := *byType[key]
		if item.Identifier == "" && item.Date == "" {
			continue
		}
		result = append(result, item)
	}
	return result
}

// ResolveDocumentType keeps a labeled column on its declared type. Generic
// columns (formacao) only remap when the value itself names OAB/CREA/COREN/etc.
// Invalid checksums do not promote an unlabeled number to another kind.
func ResolveDocumentType(declaredKey, raw string) string {
	if declaredKey == "" || declaredKey == "cpf" {
		return ""
	}
	if declaredKey != "generic" {
		return declaredKey
	}
	classified := document.ClassifyIdentifier(raw)
	if classified.Action == document.IdentifierDelete {
		return ""
	}
	match, err := normalize.IdentifyDocument(raw)
	if err != nil {
		return ""
	}
	if match.Kind == normalize.DocumentCPF || match.Kind == normalize.DocumentCNPJ {
		return ""
	}
	return string(match.Kind)
}

func CanonicalSidecarIdentifier(typeKey, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if classified := document.ClassifyIdentifier(trimmed); classified.Action == document.IdentifierDelete {
		return ""
	}
	canonical, err := normalize.CanonicalDocument(normalize.DocumentKind(typeKey), trimmed)
	if err != nil {
		return trimmed
	}
	return canonical
}

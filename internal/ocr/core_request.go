package ocr

import (
	"encoding/json"
	"strings"

	coreocr "github.com/Pherlsz/Gymkhana-Core/ocr"
	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
)

type portableObjectSchema struct {
	Type                 string                      `json:"type"`
	Properties           map[string]portableProperty `json:"properties"`
	Required             []string                    `json:"required"`
	AdditionalProperties bool                        `json:"additionalProperties"`
}

type portableProperty struct {
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

func coreExtractionRequest(attachmentID attachment.Identifier, mime string, catalog Catalog) (coreocr.ExtractionRequest, error) {
	schema, err := targetSchema(catalog.Fields)
	if err != nil {
		return coreocr.ExtractionRequest{}, err
	}
	return coreocr.ExtractionRequest{
		Mode: coreocr.ModeSchemaGuided,
		Sources: []coreocr.SourceRef{{
			ID:        attachmentID.String(),
			Modality:  sourceModality(mime),
			MediaType: mime,
		}},
		TargetSchema:  schema,
		MaxCandidates: MaximumSuggestions,
	}, nil
}

func sourceModality(mime string) coreocr.SourceModality {
	if mime == "application/pdf" {
		return coreocr.SourceDocument
	}
	return coreocr.SourceImage
}

func targetSchema(fields []FieldSchema) (json.RawMessage, error) {
	schema := portableObjectSchema{
		Type:                 "object",
		Properties:           make(map[string]portableProperty, len(fields)),
		Required:             make([]string, 0, len(fields)),
		AdditionalProperties: false,
	}
	for _, field := range fields {
		if !validFieldSchema(field) {
			return nil, ErrInvalidSetup
		}
		schema.Properties[field.Key] = portableProperty{Type: schemaType(field.Kind), Title: field.Label}
		schema.Required = append(schema.Required, field.Key)
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func schemaType(kind ValueKind) string {
	switch kind {
	case ValueInteger:
		return "integer"
	case ValueBoolean:
		return "boolean"
	default:
		return "string"
	}
}

func fieldPointer(key string) string {
	return "/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(key)
}

func fieldKeyFromPointer(pointer string) (string, bool) {
	if pointer == "" || pointer[0] != '/' || strings.Contains(pointer[1:], "/") {
		return "", false
	}
	return strings.NewReplacer("~1", "/", "~0", "~").Replace(pointer[1:]), true
}

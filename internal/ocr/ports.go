package ocr

import (
	"context"
	"io"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

type SourceGateway interface {
	GetForProcessing(context.Context, auth.Session, attachment.Identifier) (attachment.Attachment, error)
	OpenForProcessing(context.Context, auth.Session, attachment.Identifier) (attachment.Attachment, io.ReadCloser, error)
}

type CurrentField struct {
	Value   string
	Version int64
}

type ApprovedChange struct {
	SuggestionID Identifier
	Field        FieldSchema
	Value        string
}

type TargetGateway interface {
	Catalog(context.Context, auth.Session, attachment.OwnerReference) (Catalog, error)
	CurrentField(context.Context, auth.Session, FieldSchema) (CurrentField, error)
	ApplyTarget(context.Context, auth.Session, TargetReference, int64, []ApprovedChange, string) (int64, error)
}

type ProviderField struct {
	Key      string
	Label    string
	Kind     ValueKind
	Required bool
}

type ExtractionRequest struct {
	SchemaVersion string
	MIME          string
	ByteSize      int64
	SHA256        [32]byte
	PageCount     int
	PixelCount    int64
	Fields        []ProviderField
	Source        io.Reader
}

type ProviderSuggestion struct {
	FieldKey string
	Value    string
	Evidence Evidence
}

type ExtractionResponse struct {
	Suggestions []ProviderSuggestion
	Usage       int64
}

type Extractor interface {
	Extract(context.Context, ExtractionRequest) (ExtractionResponse, error)
}

type Jobs interface {
	EnqueueExtraction(context.Context, Identifier) (int64, error)
	Cancel(context.Context, int64) error
}

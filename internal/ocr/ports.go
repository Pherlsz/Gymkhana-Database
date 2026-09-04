package ocr

import (
	"context"
	"io"

	coreocr "github.com/Pherlsz/Gymkhana-Core/ocr"
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

// ExtractionInput is the host-owned extractor call. Request is the Core OCR
// contract (no bytes). Source bytes stay on the Database side of the port.
type ExtractionInput struct {
	Request coreocr.ExtractionRequest
	Fields  []FieldSchema
	Source  io.Reader
}

// ExtractionOutput is provider-produced Core OCR plus Database-owned usage.
type ExtractionOutput struct {
	Result coreocr.ExtractionResult
	Usage  int64
}

type Extractor interface {
	Extract(context.Context, ExtractionInput) (ExtractionOutput, error)
}

type Jobs interface {
	EnqueueExtraction(context.Context, Identifier) (int64, error)
	Cancel(context.Context, int64) error
}

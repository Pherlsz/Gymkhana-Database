package attachment

import (
	"context"
	"io"
	"time"
)

type ObjectStore interface {
	PresignUpload(context.Context, string, time.Duration) (SignedRequest, error)
	PresignDownload(context.Context, string, string, string, time.Duration) (SignedRequest, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

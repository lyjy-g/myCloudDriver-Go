package plugin

import (
	"context"
	"io"
)

type MultipartStore interface {
	InitiateMultipartUpload(ctx context.Context, key Key, opts PutOptions) (uploadID string, err error)
	UploadPart(ctx context.Context, key Key, uploadID string, partNumber int, r io.Reader) (etag string, err error)
	CompleteMultipartUpload(ctx context.Context, key Key, uploadID string, parts []CompletedPart) (ObjectInfo, error)
	AbortMultipartUpload(ctx context.Context, key Key, uploadID string) error
}

type CompletedPart struct {
	PartNumber int
	ETag       string
}

package plugin

import (
	"context"
	"io"
	"time"
)

type Capability string

const (
	CapabilityBasic     Capability = "basic"
	CapabilityMultipart Capability = "multipart"
	CapabilitySignedURL Capability = "signed_url"
)

type PlatformIdentifier string

const (
	PlatformIdentifierLocal PlatformIdentifier = "local"
	PlatformIdentifierS3    PlatformIdentifier = "s3"
	PlatformIdentifierOSS   PlatformIdentifier = "oss"
)

type Key string

type PutOptions struct {
	ContentType   string
	ContentLength *int64
	Metadata      map[string]string
}

type ObjectInfo struct {
	Key          Key
	Size         int64
	ContentType  string
	ETag         string
	LastModified *time.Time
}

type Store interface {
	Put(ctx context.Context, key Key, r io.Reader, opts PutOptions) (ObjectInfo, error)
	Get(ctx context.Context, key Key) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key Key) error
	Stat(ctx context.Context, key Key) (ObjectInfo, error)
}

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

type SignedURLStore interface {
	PresignGet(ctx context.Context, key Key, expire time.Duration) (string, error)
	PresignPut(ctx context.Context, key Key, expire time.Duration) (string, error)
}

type HealthChecker interface {
	Check(ctx context.Context) error
}

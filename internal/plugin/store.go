package plugin

import (
	"context"
	"io"
	"time"
)

// Capability 表示驱动能力类型。
type Capability string

const (
	// CapabilityBasic 表示基础对象读写能力。
	CapabilityBasic Capability = "basic"
	// CapabilityMultipart 表示分片上传能力。
	CapabilityMultipart Capability = "multipart"
	// CapabilitySignedURL 表示预签名 URL 能力。
	CapabilitySignedURL Capability = "signed_url"
)

// PlatformIdentifier 表示存储平台标识。
type PlatformIdentifier string

const (
	// PlatformIdentifierLocal 表示本地存储。
	PlatformIdentifierLocal PlatformIdentifier = "local"
	// PlatformIdentifierS3 表示 S3 兼容存储。
	PlatformIdentifierS3 PlatformIdentifier = "s3"
	// PlatformIdentifierOSS 表示阿里云 OSS。
	PlatformIdentifierOSS PlatformIdentifier = "oss"
)

// Key 表示对象键。
type Key string

// PutOptions 表示对象写入可选参数。
type PutOptions struct {
	ContentType   string
	ContentLength *int64
	Metadata      map[string]string
}

// ObjectInfo 表示对象元信息。
type ObjectInfo struct {
	Key          Key
	Size         int64
	ContentType  string
	ETag         string
	LastModified *time.Time
}

// Store 定义基础对象存储接口。
type Store interface {
	Put(ctx context.Context, key Key, r io.Reader, opts PutOptions) (ObjectInfo, error)
	Get(ctx context.Context, key Key) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key Key) error
	Stat(ctx context.Context, key Key) (ObjectInfo, error)
}

// MultipartStore 定义分片上传扩展接口。
type MultipartStore interface {
	InitiateMultipartUpload(ctx context.Context, key Key, opts PutOptions) (uploadID string, err error)
	UploadPart(ctx context.Context, key Key, uploadID string, partNumber int, r io.Reader) (etag string, err error)
	CompleteMultipartUpload(ctx context.Context, key Key, uploadID string, parts []CompletedPart) (ObjectInfo, error)
	AbortMultipartUpload(ctx context.Context, key Key, uploadID string) error
}

// CompletedPart 表示完成分片上传时的分片信息。
type CompletedPart struct {
	PartNumber int
	ETag       string
}

// SignedURLStore 定义预签名 URL 扩展接口。
type SignedURLStore interface {
	PresignGet(ctx context.Context, key Key, expire time.Duration) (string, error)
	PresignPut(ctx context.Context, key Key, expire time.Duration) (string, error)
}

// HealthChecker 定义驱动健康检查接口。
type HealthChecker interface {
	Check(ctx context.Context) error
}

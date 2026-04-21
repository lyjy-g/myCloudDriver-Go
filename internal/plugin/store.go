package plugin

import (
	"context"
	"io"
)

type Key string

type PutOptions struct {
	ContentType string
	Metadata    map[string]string
}

type ObjectInfo struct {
	Key         Key
	Size        int64
	ContentType string
	ETag        string
}

type Store interface {
	Put(ctx context.Context, key Key, r io.Reader, opts PutOptions) (ObjectInfo, error)
	Get(ctx context.Context, key Key) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key Key) error
	Stat(ctx context.Context, key Key) (ObjectInfo, error)
}

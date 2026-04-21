package plugin

import (
	"context"
	"time"
)

type SignedURLStore interface {
	PresignGet(ctx context.Context, key Key, expire time.Duration) (string, error)
	PresignPut(ctx context.Context, key Key, expire time.Duration) (string, error)
}

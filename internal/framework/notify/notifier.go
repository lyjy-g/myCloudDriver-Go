package notify

import "context"

// Notifier 表示通知发送能力。
type Notifier interface {
	Send(ctx context.Context, target, content string) error
}

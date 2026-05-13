package history

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	historyKeyPrefix = "agent:history"
	historyTTL       = 7 * 24 * time.Hour
	pageSize         = 10
)

// Entry 表示一条历史对话记录。
type Entry struct {
	TraceID   string    `json:"traceId"`
	Query     string    `json:"query"`
	Summary   string    `json:"summary"`
	Intent    string    `json:"intent"`
	Mode      string    `json:"mode"`
	Source    string    `json:"source"`
	ItemCount int       `json:"itemCount"`
	CreatedAt time.Time `json:"createdAt"`
}

// Service 基于 Redis 的对话历史存储。
type Service struct {
	rdb redis.Cmdable
}

func NewService(rdb redis.Cmdable) *Service {
	return &Service{rdb: rdb}
}

// Record 记录一次对话。
func (s *Service) Record(ctx context.Context, userID, workspaceID string, entry *Entry) error {
	if s.rdb == nil || entry == nil {
		return nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	key := historyKey(userID, workspaceID)
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	pipe := s.rdb.Pipeline()
	score := float64(entry.CreatedAt.UnixNano())
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: entry.TraceID})
	pipe.HSet(ctx, historyHashKey(key), entry.TraceID, data)
	// 设置 TTL：每次写入刷新整个 key 的过期时间
	pipe.Expire(ctx, key, historyTTL)
	pipe.Expire(ctx, historyHashKey(key), historyTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// List 返回最近的 n 条历史记录。beforeTraceId 不为空时，返回比该记录更早的 n 条。
func (s *Service) List(ctx context.Context, userID, workspaceID string, beforeTraceID string, n int) ([]Entry, error) {
	if n <= 0 {
		n = pageSize
	}
	key := historyKey(userID, workspaceID)
	var entries []Entry

	if beforeTraceID == "" {
		// 最新的 n 条
		results, err := s.rdb.ZRevRangeWithScores(ctx, key, 0, int64(n-1)).Result()
		if err != nil {
			return nil, err
		}
		entries = make([]Entry, 0, len(results))
		for _, z := range results {
			traceID, _ := z.Member.(string)
			entry, err := s.getEntry(ctx, key, traceID)
			if err != nil {
				continue
			}
			entries = append(entries, *entry)
		}
		return entries, nil
	}

	// 获取 beforeTraceID 的 score
	score := s.rdb.ZScore(ctx, key, beforeTraceID).Val()
	if score == 0 {
		return s.List(ctx, userID, workspaceID, "", n)
	}

	results, err := s.rdb.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    fmt.Sprintf("%.0f", score-1),
		Offset: 0,
		Count:  int64(n),
	}).Result()
	if err != nil {
		return nil, err
	}
	entries = make([]Entry, 0, len(results))
	for _, z := range results {
		traceID, _ := z.Member.(string)
		entry, err := s.getEntry(ctx, key, traceID)
		if err != nil {
			continue
		}
		entries = append(entries, *entry)
	}
	return entries, nil
}

// HasMore 检查在 beforeTraceID 之前是否还有更多记录。
func (s *Service) HasMore(ctx context.Context, userID, workspaceID, beforeTraceID string) (bool, error) {
	score := s.rdb.ZScore(ctx, historyKey(userID, workspaceID), beforeTraceID).Val()
	if score == 0 {
		return false, nil
	}
	cnt, err := s.rdb.ZCount(ctx, historyKey(userID, workspaceID), "-inf", fmt.Sprintf("%.0f", score-1)).Result()
	return cnt > 0, err
}

func (s *Service) getEntry(ctx context.Context, key, traceID string) (*Entry, error) {
	data, err := s.rdb.HGet(ctx, historyHashKey(key), traceID).Bytes()
	if err != nil {
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func historyKey(userID, workspaceID string) string {
	return fmt.Sprintf("%s:%s:%s", historyKeyPrefix, userID, workspaceID)
}

func historyHashKey(key string) string {
	return key + ":data"
}

// EnsureTTL 清理已过期但未删除的 key（启动时调用一次）。
func (s *Service) EnsureTTL(ctx context.Context) {
	if s.rdb == nil {
		return
	}
	iter := s.rdb.Scan(ctx, 0, historyKeyPrefix+":*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		if strings.HasSuffix(key, ":data") {
			continue
		}
		s.rdb.Expire(ctx, key, historyTTL)
		s.rdb.Expire(ctx, historyHashKey(key), historyTTL)
	}
}

// Delete 删除指定对话记录。
func (s *Service) Delete(ctx context.Context, userID, workspaceID, traceID string) error {
	key := historyKey(userID, workspaceID)
	pipe := s.rdb.Pipeline()
	pipe.ZRem(ctx, key, traceID)
	pipe.HDel(ctx, historyHashKey(key), traceID)
	_, err := pipe.Exec(ctx)
	return err
}

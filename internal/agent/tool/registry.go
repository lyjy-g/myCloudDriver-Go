package tool

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CallContext 是工具调用上下文。
type CallContext struct {
	TraceID          string
	UserID           string
	WorkspaceID      string
	StorageSettingID string
	Query            string
}

// ToolResult 是工具统一返回。
type ToolResult struct {
	Source string
	Items  []any
	Info   string
}

// Tool 定义检索工具接口。
type Tool interface {
	Name() string
	Call(ctx context.Context, callCtx CallContext) (ToolResult, error)
}

// Registry 维护白名单工具。
type Registry struct {
	tools map[string]Tool
}

func NewRegistry(tools ...Tool) *Registry {
	m := make(map[string]Tool, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		m[t.Name()] = t
	}
	return &Registry{tools: m}
}

func (r *Registry) MustAllowed(name string) error {
	if _, ok := r.tools[name]; !ok {
		return fmt.Errorf("tool not allowed: %s", name)
	}
	return nil
}

func (r *Registry) Call(ctx context.Context, name string, callCtx CallContext, timeout time.Duration) (ToolResult, error) {
	tool, ok := r.tools[name]
	if !ok {
		return ToolResult{}, fmt.Errorf("tool not allowed: %s", name)
	}
	if timeout <= 0 {
		timeout = 1800 * time.Millisecond
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return tool.Call(cctx, callCtx)
}

func NormalizeKeyword(query string) string {
	q := strings.TrimSpace(query)
	// 仅移除句子开头的提问语气词和结尾标点，保留内容关键词（如"配置"、"文件"）
	repl := []string{"请问", "帮我", "我想", "我要", "有没有", "有哪些", "哪些", "最近", "一些"}
	for _, p := range repl {
		q = strings.ReplaceAll(q, p, "")
	}
	q = strings.TrimSpace(q)
	// 去除尾部标点语气词
	for strings.HasSuffix(q, "？") || strings.HasSuffix(q, "?") || strings.HasSuffix(q, "。") || strings.HasSuffix(q, "的") || strings.HasSuffix(q, "了") {
		q = strings.TrimSuffix(q, "？")
		q = strings.TrimSuffix(q, "?")
		q = strings.TrimSuffix(q, "。")
		q = strings.TrimSuffix(q, "的")
		q = strings.TrimSuffix(q, "了")
		q = strings.TrimSpace(q)
	}
	return q
}

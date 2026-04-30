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
	repl := []string{"最近", "上传", "访问", "分享", "文件", "配置", "空间", "查询", "搜索", "有哪些", "有没有"}
	for _, p := range repl {
		q = strings.ReplaceAll(q, p, "")
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	return q
}

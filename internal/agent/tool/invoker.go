package tool

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Invoker 统一工具调用入口，处理超时、日志、权限。
type Invoker struct {
	registry *Registry
	guard    *Guard
}

func NewInvoker(registry *Registry) *Invoker {
	return &Invoker{
		registry: registry,
		guard:    NewGuard(),
	}
}

// InvokeResult 工具调用统一结果。
type InvokeResult struct {
	ToolName  string
	Result    ToolResult
	LatencyMs int64
	Error     error
}

// Call 执行工具调用。超时默认 3s。
func (iv *Invoker) Call(ctx context.Context, toolName string, callCtx CallContext, timeout time.Duration) InvokeResult {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	started := time.Now()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("tool.invoke panic tool=%s panic=%v", toolName, r)
		}
	}()

	// 权限校验
	if err := iv.guard.Check(ctx, toolName, callCtx); err != nil {
		return InvokeResult{
			ToolName:  toolName,
			LatencyMs: time.Since(started).Milliseconds(),
			Error:     fmt.Errorf("permission denied: %w", err),
		}
	}
	// 白名单校验
	if err := iv.registry.MustAllowed(toolName); err != nil {
		return InvokeResult{
			ToolName:  toolName,
			LatencyMs: time.Since(started).Milliseconds(),
			Error:     fmt.Errorf("tool not allowed: %w", err),
		}
	}
	// 执行
	result, err := iv.registry.Call(ctx, toolName, callCtx, timeout)
	latency := time.Since(started).Milliseconds()
	log.Printf("tool.invoke tool=%s latency_ms=%d err=%v", toolName, latency, err)
	return InvokeResult{
		ToolName:  toolName,
		Result:    result,
		LatencyMs: latency,
		Error:     err,
	}
}

// CallMulti 并行调用多个工具（当前为串行实现，后续可优化为并发）。
func (iv *Invoker) CallMulti(ctx context.Context, toolNames []string, callCtx CallContext, timeout time.Duration) []InvokeResult {
	results := make([]InvokeResult, 0, len(toolNames))
	for _, name := range toolNames {
		results = append(results, iv.Call(ctx, name, callCtx, timeout))
	}
	return results
}

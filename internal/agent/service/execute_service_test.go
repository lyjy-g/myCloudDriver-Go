package service

import (
	"context"
	"testing"

	agentllm "myclouddrive-go/internal/agent/llm"
	agentmodel "myclouddrive-go/internal/agent/model"
	agenttool "myclouddrive-go/internal/agent/tool"
)

type fakeTool struct {
	name string
}

func (f *fakeTool) Name() string { return f.name }
func (f *fakeTool) Call(ctx context.Context, callCtx agenttool.CallContext) (agenttool.ToolResult, error) {
	return agenttool.ToolResult{
		Source: "test",
		Items: []any{
			map[string]any{"ok": true, "tool": f.name},
		},
		Info: "ok",
	}, nil
}

type fakeLLM struct{}

func (f *fakeLLM) Name() string  { return "fake" }
func (f *fakeLLM) Model() string { return "fake-model" }
func (f *fakeLLM) DecideTools(ctx context.Context, query string) (agentllm.Decision, error) {
	return agentllm.Decision{Intent: "test", Tools: []string{"tool.share.create"}}, nil
}
func (f *fakeLLM) Summarize(ctx context.Context, query string, decision agentllm.Decision, items []any) (string, error) {
	return "ok", nil
}
func (f *fakeLLM) SummarizeStream(ctx context.Context, query string, decision agentllm.Decision, items []any, onToken func(string)) error {
	onToken("ok")
	return nil
}

func TestExecuteModePlanConfirmExecute(t *testing.T) {
	registry := agenttool.NewRegistry(&fakeTool{name: "tool.share.create"})
	svc := New(registry, &fakeLLM{}, nil)
	ctx := context.Background()
	callCtx := agenttool.CallContext{
		TraceID:          "agt_test_execute",
		UserID:           "u1",
		WorkspaceID:      "ws1",
		StorageSettingID: "stg1",
		Query:            "帮我创建分享",
	}

	// 第一步：execute mode 先返回 plan（写操作需确认）
	resp, err := svc.executeMode(ctx, agentmodel.QueryRequest{Query: "帮我创建分享"}, callCtx.TraceID, "创建分享", []string{"tool.share.create"}, "storage_setting", callCtx)
	if err != nil {
		t.Fatalf("executeMode failed: %v", err)
	}
	if resp == nil || len(resp.Items) == 0 {
		t.Fatalf("executeMode should return plan response")
	}
	if !resp.Partial {
		t.Fatalf("plan stage should be partial before confirm")
	}

	// 第二步：confirm 后真正执行工具
	confirmResp, err := svc.ConfirmExecute(ctx, callCtx.TraceID)
	if err != nil {
		t.Fatalf("ConfirmExecute failed: %v", err)
	}
	if confirmResp == nil {
		t.Fatalf("confirm response should not be nil")
	}
	if len(confirmResp.ToolResults) != 1 || confirmResp.ToolResults[0].Status != "ok" {
		t.Fatalf("unexpected tool results: %+v", confirmResp.ToolResults)
	}
	if len(confirmResp.Items) == 0 {
		t.Fatalf("confirm response should include tool items")
	}
}

package test

import (
	"context"
	"testing"

	agentmodel "myclouddrive-go/internal/agent/model"
	agenttool "myclouddrive-go/internal/agent/tool"
	agentutils "myclouddrive-go/internal/agent/utils"
	filemodel "myclouddrive-go/internal/file/model"
	filesvc "myclouddrive-go/internal/file/service"
	sharesvc "myclouddrive-go/internal/share/service"
)

func TestToolRegistryRegistration(t *testing.T) {
	fs := filesvc.NewFileService(nil, nil)
	r := agenttool.NewRegistry(
		agenttool.NewFileListTool(fs),
		agenttool.NewFileSearchTool(fs),
		agenttool.NewFileStatsTool(fs),
		agenttool.NewFileTrashListTool(fs),
		agenttool.NewFileRankTool(fs),
	)

	for _, name := range []string{
		"tool.file.list", "tool.file.search", "tool.file.stats",
		"tool.file.trash.list", "tool.file.rank",
	} {
		if err := r.MustAllowed(name); err != nil {
			t.Errorf("tool %s should be allowed: %v", name, err)
		}
	}
	if err := r.MustAllowed("tool.share.list"); err == nil {
		t.Error("tool.share.list should not be registered in file-only registry")
	}
}

func TestFileSearchTool(t *testing.T) {
	fs := filesvc.NewFileService(nil, nil)
	// 创建一些测试文件
	fs.CreateDirectory(context.Background(), "root", "testdir", "")
	tool := agenttool.NewFileSearchTool(fs)
	cc := agenttool.CallContext{
		TraceID:          "test_trace",
		UserID:           "test_user",
		WorkspaceID:      "ws_test",
		StorageSettingID: "",
		Query:            "最近上传了什么文件",
	}
	result, err := tool.Call(context.Background(), cc)
	if err != nil {
		t.Fatalf("file.search failed: %v", err)
	}
	if result.Source != "file" {
		t.Errorf("expected source=file, got %s", result.Source)
	}
}

func TestFileStatsTool(t *testing.T) {
	fs := filesvc.NewFileService(nil, nil)
	tool := agenttool.NewFileStatsTool(fs)
	result, err := tool.Call(context.Background(), agenttool.CallContext{
		TraceID: "test", Query: "统计pdf文件",
	})
	if err != nil {
		t.Fatalf("file.stats failed: %v", err)
	}
	if len(result.Items) == 0 {
		t.Error("expected stats output")
	}
}

func TestFileRankTool(t *testing.T) {
	fs := filesvc.NewFileService(nil, nil)
	fs.CreateDirectory(context.Background(), "root", "docs", "")
	tool := agenttool.NewFileRankTool(fs)
	result, err := tool.Call(context.Background(), agenttool.CallContext{
		TraceID: "test", Query: "最近比较重要的文件",
	})
	if err != nil {
		t.Fatalf("file.rank failed: %v", err)
	}
	if result.Source != "file_rank" {
		t.Errorf("expected source=file_rank, got %s", result.Source)
	}
}

func TestFileTrashTool(t *testing.T) {
	fs := filesvc.NewFileService(nil, nil)
	tool := agenttool.NewFileTrashListTool(fs)
	result, err := tool.Call(context.Background(), agenttool.CallContext{
		TraceID: "test", Query: "回收站有哪些文件",
	})
	if err != nil {
		t.Fatalf("file.trash.list failed: %v", err)
	}
	if result.Source != "trash" {
		t.Errorf("expected source=trash, got %s", result.Source)
	}
}

func TestPlannerBuildPlan(t *testing.T) {
	fs := filesvc.NewFileService(nil, nil)
	r := agenttool.NewRegistry(
		agenttool.NewFileSearchTool(fs),
		agenttool.NewFileStatsTool(fs),
	)
	p := agentutils.NewPlanner(r)
	cc := agenttool.CallContext{
		TraceID: "test", UserID: "u1", WorkspaceID: "ws1", Query: "搜索pdf文件",
	}
	plan, err := p.BuildPlan("搜索pdf文件", "search_files", []string{"tool.file.search", "tool.file.stats"}, cc)
	if err != nil {
		t.Fatalf("build plan failed: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(plan.Steps))
	}
	if plan.Risk != agentmodel.RiskRead {
		t.Errorf("search should be READ risk, got %s", plan.Risk)
	}
	if agentutils.NeedsConfirmation(plan) {
		t.Error("read-only plan should not need confirmation")
	}
}

func TestPlannerDangerousPlan(t *testing.T) {
	fs := filesvc.NewFileService(nil, nil)
	ss := sharesvc.NewService(nil, nil)
	r := agenttool.NewRegistry(
		agenttool.NewFileSearchTool(fs),
		agenttool.NewShareRevokeTool(ss),
	)
	p := agentutils.NewPlanner(r)
	cc := agenttool.CallContext{
		TraceID: "test", UserID: "u1", WorkspaceID: "ws1", Query: "撤销分享shr_123",
	}
	plan, err := p.BuildPlan("撤销分享shr_123", "revoke_share", []string{"tool.file.search", "tool.share.revoke"}, cc)
	if err != nil {
		t.Fatalf("build plan failed: %v", err)
	}
	if plan.Risk != agentmodel.RiskDanger {
		t.Errorf("revoke should be DANGER risk, got %s", plan.Risk)
	}
	if !agentutils.NeedsConfirmation(plan) {
		t.Error("dangerous plan should need confirmation")
	}
	if !agentutils.HasDangerousSteps(plan) {
		t.Error("plan with revoke should have dangerous steps")
	}
}

func TestNormalizeKeyword(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"最近上传了哪些文件", "了哪些"},
		{"配置中心", "中心"},
		{"PDF文档", "PDF文档"},
		{"有哪些分享", ""},
		{"hello world", "hello world"},
	}
	for _, c := range cases {
		got := agenttool.NormalizeKeyword(c.input)
		if got != c.expected {
			t.Errorf("NormalizeKeyword(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

// 编译时接口检查
var _ = agentmodel.ExecutionPlan{}
var _ = agentmodel.ExecuteRequest{}
var _ = agentmodel.ShareStatEntry{}
var _ = agentmodel.RiskRead
var _ = filemodel.FileItem{}
var _ = filesvc.NewFileService
var _ = sharesvc.NewService
var _ = agentutils.NewPlanner
var _ = agenttool.CallContext{}

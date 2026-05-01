package tool

import (
	"context"
	"fmt"
	"strings"

	agentmodel "myclouddrive-go/internal/agent/model"
	"myclouddrive-go/internal/framework/code"
)

// GuardConfig 权限守卫配置。
type GuardConfig struct {
	RequireAuth      bool
	RequireWorkspace bool
	AllowedRoles     []string
}

// Guard 工具调用前的权限校验。
type Guard struct {
	configs map[string]GuardConfig
}

func NewGuard() *Guard {
	g := &Guard{configs: make(map[string]GuardConfig)}
	// 默认所有只读工具需要认证 + workspace
	readCfg := GuardConfig{RequireAuth: true, RequireWorkspace: true}
	for _, s := range AllToolSchemas() {
		switch s.RiskLevel {
		case "READ":
			g.configs[s.Name] = readCfg
		case "WRITE":
			g.configs[s.Name] = GuardConfig{RequireAuth: true, RequireWorkspace: true}
		case "DANGER":
			g.configs[s.Name] = GuardConfig{RequireAuth: true, RequireWorkspace: true}
		case "EXPORT":
			g.configs[s.Name] = GuardConfig{RequireAuth: true, RequireWorkspace: true}
		case "CROSS_WS":
			g.configs[s.Name] = GuardConfig{RequireAuth: true, RequireWorkspace: true}
		}
	}
	return g
}

// Check 校验工具调用的权限。
func (g *Guard) Check(ctx context.Context, toolName string, callCtx CallContext) error {
	cfg, ok := g.configs[toolName]
	if !ok {
		return fmt.Errorf("unknown tool: %s", toolName)
	}
	if cfg.RequireAuth && strings.TrimSpace(callCtx.UserID) == "" {
		return code.New(code.NoPermission, "login required for tool: "+toolName)
	}
	if cfg.RequireWorkspace && strings.TrimSpace(callCtx.WorkspaceID) == "" {
		return code.New(code.BadRequest, "workspace required for tool: "+toolName)
	}
	return nil
}

// RiskLevel 返回工具的风险等级。
func (g *Guard) RiskLevel(toolName string) agentmodel.RiskLevel {
	cfg, ok := g.configs[toolName]
	if !ok {
		return agentmodel.RiskRead
	}
	switch {
	case toolName == agentmodel.ToolShareRevoke:
		return agentmodel.RiskDanger
	case cfg.RequireAuth && cfg.RequireWorkspace:
		return agentmodel.RiskRead
	default:
		return agentmodel.RiskRead
	}
}

// IsReadOnly 判断是否为只读操作。
func (g *Guard) IsReadOnly(toolName string) bool {
	return g.RiskLevel(toolName) == agentmodel.RiskRead
}

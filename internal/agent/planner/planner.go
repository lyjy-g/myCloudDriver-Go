package planner

import (
	"fmt"
	"strings"
	"time"

	agentmodel "myclouddrive-go/internal/agent/model"
	agenttool "myclouddrive-go/internal/agent/tool"
)

// Planner 把用户意图拆成执行计划。
type Planner struct {
	registry *agenttool.Registry
}

func NewPlanner(registry *agenttool.Registry) *Planner {
	return &Planner{registry: registry}
}

// BuildPlan 根据 LLM 决策生成执行计划。
func (p *Planner) BuildPlan(query, intent string, tools []string, callCtx agenttool.CallContext) (agentmodel.ExecutionPlan, error) {
	if len(tools) == 0 {
		planID := fmt.Sprintf("plan_%d", time.Now().UnixNano())
		steps := []agentmodel.ExecutionStep{
			{
				Index:       1,
				Description: "未匹配到可执行工具，建议改写指令或补充文件/目录标识",
				ToolName:    "",
				Risk:        agentmodel.RiskRead,
			},
		}
		return agentmodel.ExecutionPlan{
			PlanID:  planID,
			Steps:   steps,
			Summary: "当前问题未匹配到可执行工具，已降级为只读建议，不会执行写操作。",
			Risk:    agentmodel.RiskRead,
		}, nil
	}
	risk := agentmodel.RiskRead
	steps := make([]agentmodel.ExecutionStep, 0, len(tools))
	for i, name := range tools {
		stepRisk := classifyToolRisk(name)
		if priorityOrder(stepRisk) > priorityOrder(risk) {
			risk = stepRisk
		}
		steps = append(steps, agentmodel.ExecutionStep{
			Index:       i + 1,
			Description: describeStep(name, query),
			ToolName:    name,
			Risk:        stepRisk,
		})
	}
	planID := fmt.Sprintf("plan_%d", time.Now().UnixNano())
	return agentmodel.ExecutionPlan{
		PlanID:  planID,
		Steps:   steps,
		Summary: buildSummary(intent, steps, callCtx.Query),
		Risk:    risk,
	}, nil
}

func buildSummary(intent string, steps []agentmodel.ExecutionStep, query string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("我将执行以下操作（意图: %s）：\n", intent))
	for _, s := range steps {
		b.WriteString(fmt.Sprintf("%d. [%s] %s\n", s.Index, s.Risk, s.Description))
	}
	total := len(steps)
	b.WriteString(fmt.Sprintf("\n共 %d 步。", total))
	if total > 3 {
		b.WriteString(" 如操作数量较多，建议分批次执行。")
	}
	return b.String()
}

func describeStep(toolName, query string) string {
	m := map[string]string{
		"tool.file.list":          "查询当前空间的文件列表",
		"tool.file.search":        fmt.Sprintf("搜索文件：%s", query),
		"tool.file.stats":         "统计文件的类型和大小分布",
		"tool.file.trash.list":    "列出回收站中的文件",
		"tool.file.rank":          "按重要性排序文件",
		"tool.file.rename":        "重命名指定文件",
		"tool.file.create_dir":    "在根目录创建新目录",
		"tool.file.move":          "移动文件到目标目录",
		"tool.file.delete":        "删除文件或目录到回收站",
		"tool.file.restore":       "从回收站恢复最近文件",
		"tool.file.rebuild_index": "重建文件索引",
		"tool.share.list":         "查询我的分享链接列表",
		"tool.share.search":       fmt.Sprintf("搜索分享：%s", query),
		"tool.share.records":      "查询分享的访问记录",
		"tool.share.stats":        "统计分享的查看和下载数据",
		"tool.share.revoke":       "撤销指定的分享链接",
		"tool.share.create":       "创建新的分享链接",
	}
	if d, ok := m[toolName]; ok {
		return d
	}
	return fmt.Sprintf("调用工具 %s", toolName)
}

var toolRiskMap = map[string]agentmodel.RiskLevel{
	"tool.file.list":          agentmodel.RiskRead,
	"tool.file.search":        agentmodel.RiskRead,
	"tool.file.stats":         agentmodel.RiskRead,
	"tool.file.trash.list":    agentmodel.RiskRead,
	"tool.file.rank":          agentmodel.RiskRead,
	"tool.file.rename":        agentmodel.RiskWrite,
	"tool.file.create_dir":    agentmodel.RiskWrite,
	"tool.file.move":          agentmodel.RiskWrite,
	"tool.file.delete":        agentmodel.RiskDanger,
	"tool.file.restore":       agentmodel.RiskWrite,
	"tool.file.rebuild_index": agentmodel.RiskWrite,
	"tool.share.list":         agentmodel.RiskRead,
	"tool.share.search":       agentmodel.RiskRead,
	"tool.share.records":      agentmodel.RiskRead,
	"tool.share.stats":        agentmodel.RiskRead,
	"tool.share.revoke":       agentmodel.RiskDanger,
	"tool.share.create":       agentmodel.RiskWrite,
}

func classifyToolRisk(name string) agentmodel.RiskLevel {
	if r, ok := toolRiskMap[name]; ok {
		return r
	}
	return agentmodel.RiskRead
}

func priorityOrder(r agentmodel.RiskLevel) int {
	switch r {
	case agentmodel.RiskRead:
		return 0
	case agentmodel.RiskWrite:
		return 1
	case agentmodel.RiskExport:
		return 2
	case agentmodel.RiskDanger:
		return 3
	case agentmodel.RiskCrossWS:
		return 4
	default:
		return 0
	}
}

// NeedsConfirmation 判断计划是否需要用户二次确认。
func NeedsConfirmation(plan agentmodel.ExecutionPlan) bool {
	if plan.Risk == agentmodel.RiskRead {
		return false
	}
	return true
}

// HasDangerousSteps 判断是否包含不可逆操作。
func HasDangerousSteps(plan agentmodel.ExecutionPlan) bool {
	for _, s := range plan.Steps {
		if s.Risk == agentmodel.RiskDanger || s.Risk == agentmodel.RiskCrossWS {
			return true
		}
	}
	return false
}

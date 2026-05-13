package workflow

import (
	"encoding/json"
	"fmt"
)

// NodeType 节点类型。
type NodeType string

const (
	NodeTool          NodeType = "tool"
	NodeCondition     NodeType = "condition"
	NodeLLM           NodeType = "llm"
	NodeManualConfirm NodeType = "manual_confirm"
)

// Node 工作流节点。
type Node struct {
	ID       string          `json:"id"`
	Type     NodeType        `json:"type"`
	Label    string          `json:"label"`
	Config   json.RawMessage `json:"config"`
	Next     string          `json:"next,omitempty"`     // 默认下一节点
	NextOnOK string          `json:"nextOnOk,omitempty"` // 条件分支（真）
	NextOnNO string          `json:"nextOnNo,omitempty"` // 条件分支（假）
}

// ToolConfig 工具节点配置。
type ToolConfig struct {
	ToolName string         `json:"toolName"`
	Params   map[string]any `json:"params,omitempty"`
}

// ConditionConfig 条件节点配置。
type ConditionConfig struct {
	Field    string `json:"field"`    // 检查的字段，如 "items.length", "status"
	Operator string `json:"operator"` // "gt", "eq", "contains", "empty"
	Value    string `json:"value"`    // 比较值
}

// LLMConfig LLM 节点配置。
type LLMConfig struct {
	Prompt    string `json:"prompt"`
	MaxTokens int    `json:"maxTokens,omitempty"`
	OutputKey string `json:"outputKey"` // 结果存入上下文的 key
}

// ManualConfirmConfig 人工确认节点配置。
type ManualConfirmConfig struct {
	Message       string `json:"message"`
	TimeoutSec    int    `json:"timeoutSec"`
	DefaultChoice bool   `json:"defaultChoice"`
}

// Definition 工作流定义（JSON DSL）。
type Definition struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Nodes       []Node         `json:"nodes"`
	StartNodeID string         `json:"startNodeId"`
	Vars        map[string]any `json:"vars,omitempty"`
}

// Validate 校验工作流定义。
func (d *Definition) Validate() error {
	if len(d.Nodes) == 0 {
		return fmt.Errorf("workflow must have at least one node")
	}
	if d.StartNodeID == "" {
		return fmt.Errorf("startNodeId is required")
	}
	nodeIDs := make(map[string]bool, len(d.Nodes))
	for _, n := range d.Nodes {
		if n.ID == "" {
			return fmt.Errorf("node id is required")
		}
		if nodeIDs[n.ID] {
			return fmt.Errorf("duplicate node id: %s", n.ID)
		}
		nodeIDs[n.ID] = true
		switch n.Type {
		case NodeTool, NodeCondition, NodeLLM, NodeManualConfirm:
		default:
			return fmt.Errorf("unknown node type: %s", n.Type)
		}
	}
	if !nodeIDs[d.StartNodeID] {
		return fmt.Errorf("startNodeId %s not found in nodes", d.StartNodeID)
	}
	return nil
}

// ParseDefinition 从 JSON 解析工作流定义。
func ParseDefinition(raw []byte) (*Definition, error) {
	var d Definition
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("parse workflow definition: %w", err)
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

// ExampleWorkflow 返回一个示例工作流 JSON。
func ExampleWorkflow() *Definition {
	return &Definition{
		ID:          "wf_example",
		Name:        "文件扫描并创建分享",
		Description: "扫描最近上传的PDF文件，如果超过10个则自动创建分享",
		StartNodeID: "n1",
		Nodes: []Node{
			{
				ID:    "n1",
				Type:  NodeTool,
				Label: "搜索最近PDF文件",
				Config: mustMarshal(ToolConfig{
					ToolName: "tool.file.search",
					Params:   map[string]any{"fileType": "pdf", "limit": 50},
				}),
				Next: "n2",
			},
			{
				ID:    "n2",
				Type:  NodeCondition,
				Label: "文件数量是否大于10",
				Config: mustMarshal(ConditionConfig{
					Field: "items.length", Operator: "gt", Value: "10",
				}),
				NextOnOK: "n3",
				NextOnNO: "n4",
			},
			{
				ID:    "n3",
				Type:  NodeLLM,
				Label: "生成分享名称建议",
				Config: mustMarshal(LLMConfig{
					Prompt: "根据这些PDF文件名生成一个分享名称", OutputKey: "shareName",
				}),
				Next: "n5",
			},
			{
				ID:    "n4",
				Type:  NodeLLM,
				Label: "告知用户文件较少",
				Config: mustMarshal(LLMConfig{
					Prompt: "告知用户PDF文件数量较少无需分享", OutputKey: "summary",
				}),
			},
			{
				ID:    "n5",
				Type:  NodeManualConfirm,
				Label: "确认创建分享",
				Config: mustMarshal(ManualConfirmConfig{
					Message: "是否确认创建分享链接？", TimeoutSec: 300, DefaultChoice: false,
				}),
			},
		},
	}
}

func mustMarshal(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}

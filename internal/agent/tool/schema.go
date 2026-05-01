package tool

// ToolSchema 定义工具的入参/出参 schema（用于 LLM function calling 和文档生成）。
type ToolSchema struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	RiskLevel   string      `json:"riskLevel"`
	Parameters  []ParamDef  `json:"parameters"`
	Returns     []ReturnDef `json:"returns"`
}

// ParamDef 参数定义。
type ParamDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
}

// ReturnDef 返回值定义。
type ReturnDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// AllToolSchemas 返回所有已注册工具的 schema。
func AllToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			Name: "tool.file.list", Description: "列出当前工作空间的文件列表", Category: "file", RiskLevel: "READ",
			Parameters: []ParamDef{
				{Name: "parentId", Type: "string", Description: "父目录ID"},
				{Name: "keyword", Type: "string", Description: "文件名关键词"},
			},
			Returns: []ReturnDef{
				{Name: "items", Type: "array", Description: "文件列表"},
			},
		},
		{
			Name: "tool.file.search", Description: "按条件检索文件，支持文件类型、大小范围等过滤", Category: "file", RiskLevel: "READ",
			Parameters: []ParamDef{
				{Name: "keyword", Type: "string", Description: "搜索关键词"},
				{Name: "fileType", Type: "string", Description: "文件类型，如 pdf/docx"},
				{Name: "limit", Type: "int", Description: "返回条数上限", Default: "20"},
			},
			Returns: []ReturnDef{
				{Name: "items", Type: "array", Description: "文件列表"},
			},
		},
		{
			Name: "tool.file.stats", Description: "统计文件分布，按类型和大小分桶", Category: "file", RiskLevel: "READ",
			Parameters: []ParamDef{
				{Name: "fileType", Type: "string", Description: "可选，按类型过滤统计"},
			},
			Returns: []ReturnDef{
				{Name: "stats", Type: "object", Description: "统计结果，含 totalFiles/totalSize/byType/bySizeBucket"},
			},
		},
		{
			Name: "tool.file.trash.list", Description: "查询回收站中的文件列表", Category: "file", RiskLevel: "READ",
			Returns: []ReturnDef{
				{Name: "items", Type: "array", Description: "已删除文件列表"},
			},
		},
		{
			Name: "tool.file.rank", Description: "按重要性评分排序文件（面试亮点）", Category: "file", RiskLevel: "READ",
			Returns: []ReturnDef{
				{Name: "items", Type: "array", Description: "按 importanceScore 降序排列"},
			},
		},
		{
			Name: "tool.share.list", Description: "查询我的分享链接列表", Category: "share", RiskLevel: "READ",
			Returns: []ReturnDef{
				{Name: "items", Type: "array", Description: "分享列表"},
			},
		},
		{
			Name: "tool.share.search", Description: "搜索分享链接，按热度排序", Category: "share", RiskLevel: "READ",
			Returns: []ReturnDef{
				{Name: "items", Type: "array", Description: "分享列表，按下载+查看次数排序"},
			},
		},
		{
			Name: "tool.share.records", Description: "查询单个分享的访问记录", Category: "share", RiskLevel: "READ",
			Parameters: []ParamDef{
				{Name: "shareId", Type: "string", Description: "分享ID，未提供时自动取最新分享"},
			},
			Returns: []ReturnDef{
				{Name: "items", Type: "array", Description: "访问记录列表"},
			},
		},
		{
			Name: "tool.share.stats", Description: "分享统计：总查看/下载次数、Top5热度分享", Category: "share", RiskLevel: "READ",
			Returns: []ReturnDef{
				{Name: "stats", Type: "object", Description: "统计结果"},
			},
		},
		{
			Name: "tool.share.create", Description: "创建新的分享链接（需plan确认）", Category: "share", RiskLevel: "WRITE",
			Parameters: []ParamDef{
				{Name: "fileIds", Type: "array", Required: true, Description: "要分享的文件ID列表"},
				{Name: "expireHours", Type: "int", Description: "有效期（小时）", Default: "24"},
				{Name: "allowDownload", Type: "bool", Description: "是否允许下载", Default: "true"},
			},
		},
		{
			Name: "tool.share.revoke", Description: "撤销分享链接", Category: "share", RiskLevel: "DANGER",
			Parameters: []ParamDef{
				{Name: "shareId", Type: "string", Required: true, Description: "要撤销的分享ID"},
			},
		},
		{
			Name: "tool.rag.search", Description: "在知识库中检索相关内容（混合召回）", Category: "rag", RiskLevel: "READ",
			Returns: []ReturnDef{
				{Name: "items", Type: "array", Description: "检索结果chunk列表，含相关性分数"},
			},
		},
		{
			Name: "tool.workflow", Description: "启动或管理工作流编排", Category: "workflow", RiskLevel: "WRITE",
			Returns: []ReturnDef{
				{Name: "run", Type: "object", Description: "工作流执行记录"},
			},
		},
	}
}

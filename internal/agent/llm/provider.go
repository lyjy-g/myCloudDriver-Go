package llm

import "context"

// Decision 表示 LLM 的工具决策结果。
type Decision struct {
	Intent string   `json:"intent"`
	Tools  []string `json:"tools"`
}

// Provider 定义 LLM 能力。
type Provider interface {
	Name() string
	Model() string
	DecideTools(ctx context.Context, query string) (Decision, error)
	Summarize(ctx context.Context, query string, decision Decision, items []any) (string, error)
	// SummarizeStream 流式摘要，通过 onToken 回传逐步生成的文本。
	SummarizeStream(ctx context.Context, query string, decision Decision, items []any, onToken func(string)) error
}

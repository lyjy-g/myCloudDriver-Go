package tool

import (
	"context"
	"fmt"
	"strings"

	"myclouddrive-go/internal/agent/rag"
)

// RAGSearchTool 知识库检索工具。
type RAGSearchTool struct {
	retriever *rag.Retriever
}

func NewRAGSearchTool(retriever *rag.Retriever) *RAGSearchTool {
	return &RAGSearchTool{retriever: retriever}
}
func (t *RAGSearchTool) Name() string { return "tool.rag.search" }

func (t *RAGSearchTool) Call(ctx context.Context, callCtx CallContext) (ToolResult, error) {
	if t == nil || t.retriever == nil {
		return ToolResult{}, fmt.Errorf("rag retriever unavailable")
	}
	q := strings.TrimSpace(callCtx.Query)
	results, err := t.retriever.Search(ctx, rag.HybridQuery{
		Query:        q,
		TopK:         5,
		VectorWeight: 0.5,
	})
	if err != nil {
		return ToolResult{}, err
	}
	items := make([]any, 0, len(results))
	for _, r := range results {
		items = append(items, map[string]any{
			"chunkId":    r.ChunkID,
			"documentId": r.DocumentID,
			"text":       r.Text,
			"score":      fmt.Sprintf("%.3f", r.Score),
			"source":     r.Source,
		})
	}
	return ToolResult{Source: "rag", Items: items, Info: fmt.Sprintf("rag_results=%d", len(items))}, nil
}

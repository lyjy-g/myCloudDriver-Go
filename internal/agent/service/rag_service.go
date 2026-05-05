package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentllm "myclouddrive-go/internal/agent/llm"
	agentmodel "myclouddrive-go/internal/agent/model"
	agentdb "myclouddrive-go/internal/agent/model/dbmodel"
	agentrag "myclouddrive-go/internal/agent/rag"
	agenttool "myclouddrive-go/internal/agent/tool"
	"myclouddrive-go/internal/framework/code"
)

func ragNamespace(workspaceID, kbID string) string {
	return strings.TrimSpace(workspaceID) + "::kb::" + strings.TrimSpace(kbID)
}

func (s *AgentService) ragMode(ctx context.Context, req agentmodel.QueryRequest, traceID, intent string, callCtx agenttool.CallContext) (*agentmodel.QueryResponse, error) {
	if s.ragIndexer == nil || s.ragRetriever == nil || s.runSvc == nil || s.runSvc.db == nil {
		return nil, code.New(code.InternalError, "rag service unavailable")
	}
	kbID := strings.TrimSpace(req.KbID)
	if kbID == "" {
		return nil, code.New(code.BadRequest, "kbId is required in rag mode")
	}

	var kb agentdb.Knowledge
	if err := s.runSvc.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", kbID, callCtx.WorkspaceID).
		First(&kb).Error; err != nil {
		return nil, code.New(code.BadRequest, "knowledge not found in workspace")
	}

	ns := ragNamespace(callCtx.WorkspaceID, kbID)
	if err := s.rebuildNamespaceIndex(ctx, ns, kbID); err != nil {
		return nil, err
	}

	results, err := s.ragRetriever.Search(ctx, agentrag.HybridQuery{
		Namespace:    ns,
		Query:        strings.TrimSpace(req.Query),
		TopK:         10,
		VectorWeight: 0.5,
		KeywordBoost: 1.2,
	})
	if err != nil {
		return nil, code.New(code.InternalError, "rag search failed: "+err.Error())
	}

	items := make([]any, 0, len(results))
	for _, r := range results {
		items = append(items, map[string]any{
			"chunkId":    r.ChunkID,
			"documentId": r.DocumentID,
			"text":       r.Text,
			"score":      r.Score,
			"source":     r.Source,
			"metadata":   r.Metadata,
		})
	}

	decision := agentmodel.ToolResult{Tool: "tool.rag.search", Status: "ok", LatencyMs: 0}
	summary, sumErr := s.llm.Summarize(ctx, req.Query, agentllm.Decision{Intent: intent, Tools: []string{"tool.rag.search"}}, items)
	if sumErr != nil {
		summary = fmt.Sprintf("rag 命中 %d 条", len(items))
	}
	return &agentmodel.QueryResponse{
		TraceID:     traceID,
		RouteMode:   "rag",
		Provider:    s.llm.Name(),
		Model:       s.llm.Model(),
		Intent:      intent,
		Sources:     []string{"rag"},
		Items:       items,
		Summary:     summary,
		ToolResults: []agentmodel.ToolResult{decision},
		Partial:     false,
		CreatedAt:   time.Now(),
	}, nil
}

func (s *AgentService) rebuildNamespaceIndex(ctx context.Context, namespace, kbID string) error {
	var rows []agentdb.KnowledgeDocumentChunk
	if err := s.runSvc.db.WithContext(ctx).
		Where("knowledge_base_id = ?", kbID).
		Order("id asc").
		Find(&rows).Error; err != nil {
		return code.New(code.InternalError, "load knowledge chunks failed: "+err.Error())
	}
	chunks := make([]agentrag.Chunk, 0, len(rows))
	for _, row := range rows {
		meta := map[string]string{}
		if strings.TrimSpace(row.MetadataJSON) != "" {
			_ = json.Unmarshal([]byte(row.MetadataJSON), &meta)
		}
		chunks = append(chunks, agentrag.Chunk{
			ID:         fmt.Sprintf("kb_%s_ck_%d", kbID, row.ID),
			DocumentID: row.FileID,
			Index:      int(row.ChunkNo),
			Text:       row.Content,
			Metadata:   meta,
		})
	}
	if err := s.ragIndexer.IndexNamespace(ctx, namespace, chunks); err != nil {
		return code.New(code.InternalError, "index namespace failed: "+err.Error())
	}
	return nil
}

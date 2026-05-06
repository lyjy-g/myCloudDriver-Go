package module

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"myclouddrive-go/internal/agent/api"
	agenthistory "myclouddrive-go/internal/agent/history"
	agentllm "myclouddrive-go/internal/agent/llm"
	agentdb "myclouddrive-go/internal/agent/model/dbmodel"
	agentrag "myclouddrive-go/internal/agent/rag"
	agentsvc "myclouddrive-go/internal/agent/service"
	agenttool "myclouddrive-go/internal/agent/tool"
	"myclouddrive-go/internal/app"
	filesvc "myclouddrive-go/internal/file/service"
	pluginsvc "myclouddrive-go/internal/plugin/service"
	sharesvc "myclouddrive-go/internal/share/service"
	storagesvc "myclouddrive-go/internal/storage/service"
)

// Module 是 Agent 检索模块。
type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return "agent"
}

func (m *Module) Models() []any {
	return []any{
		&agentdb.AgentAction{},
		&agentdb.AgentActionStep{},
		&agentdb.AgentTool{},
		&agentdb.AgentToolCall{},
		&agentdb.AgentPromptTemplate{},
		&agentdb.Knowledge{},
		&agentdb.KnowledgeFile{},
		&agentdb.KnowledgeImportTask{},
		&agentdb.KnowledgeDocumentChunk{},
		&agentdb.WorkflowDefinition{},
		&agentdb.WorkflowRun{},
		&agentdb.WorkflowNodeRun{},
	}
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, deps *app.Dependencies) error {
	storageService := storagesvc.NewService(deps.DB, deps.Redis, 30*time.Second, pluginsvc.GetRunManager(deps.DB))
	fileService := filesvc.NewFileService(storageService, deps.DB, deps.Redis)
	shareService := sharesvc.NewService(deps.DB, storageService)

	registry := agenttool.NewRegistry(
		agenttool.NewFileListTool(fileService),
		agenttool.NewFileSearchTool(fileService),
		agenttool.NewFileStatsTool(fileService),
		agenttool.NewFileTrashListTool(fileService),
		agenttool.NewFileRankTool(fileService),
		agenttool.NewFileRenameTool(fileService),
		agenttool.NewFileCreateDirTool(fileService),
		agenttool.NewFileMoveTool(fileService),
		agenttool.NewFileDeleteTool(fileService),
		agenttool.NewFileRestoreTool(fileService),
		agenttool.NewFileRebuildIndexTool(),
		agenttool.NewShareListTool(shareService),
		agenttool.NewShareSearchTool(shareService),
		agenttool.NewShareRecordsTool(shareService),
		agenttool.NewShareStatsTool(shareService),
		agenttool.NewShareRevokeTool(shareService),
		agenttool.NewShareCreateTool(shareService, fileService),
		agenttool.NewTransferStatusTool(fileService),
	)
	if strings.EqualFold(strings.TrimSpace(deps.Config.LLM.Provider), "deepseek") && strings.TrimSpace(deps.Config.LLM.APIKey) == "" {
		return fmt.Errorf("llm.api_key is empty ")
	}
	if strings.TrimSpace(deps.Config.Embedding.APIKey) == "" {
		return fmt.Errorf("embedding.api_key is empty")
	}
	llmProvider := agentllm.NewDeepSeekProvider(
		deps.Config.LLM.BaseURL,
		deps.Config.LLM.APIKey,
		deps.Config.LLM.Model,
		time.Duration(deps.Config.LLM.TimeoutMs)*time.Millisecond,
	)
	historySvc := agenthistory.NewService(deps.Redis)
	runSvc := agentsvc.NewRunService(deps.DB)
	embedder := agentrag.NewHTTPEmbedder(
		deps.Config.Embedding.Provider,
		deps.Config.Embedding.BaseURL,
		deps.Config.Embedding.APIKey,
		deps.Config.Embedding.Model,
		deps.Config.Embedding.Dims,
		time.Duration(deps.Config.Embedding.TimeoutMs)*time.Millisecond,
	)
	retriever := agentrag.NewRetriever(embedder)
	indexer := agentrag.NewIndexer(embedder, retriever)
	svc := agentsvc.New(registry, llmProvider, historySvc, runSvc, fileService, indexer, retriever)
	api.RegisterRoutes(mux, svc)
	return nil
}

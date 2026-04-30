package module

import (
	"context"
	"log"
	"net/http"
	"time"

	"myclouddrive-go/internal/agent/api"
	agentaudit "myclouddrive-go/internal/agent/audit"
	agentllm "myclouddrive-go/internal/agent/llm"
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
	return nil
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, deps *app.Dependencies) error {
	storageService := storagesvc.NewService(deps.DB, deps.Redis, 30*time.Second, pluginsvc.GetRunManager(deps.DB))
	fileService := filesvc.NewFileService(storageService, deps.DB, deps.Redis)
	shareService := sharesvc.NewService(deps.DB, storageService)

	registry := agenttool.NewRegistry(
		agenttool.NewFileListTool(fileService),
		agenttool.NewShareListTool(shareService),
		agenttool.NewShareRecordsTool(shareService),
	)
	llmProvider := agentllm.NewDeepSeekProvider(
		deps.Config.LLM.BaseURL,
		deps.Config.LLM.APIKey,
		deps.Config.LLM.Model,
		time.Duration(deps.Config.LLM.TimeoutMs)*time.Millisecond,
	)
	audit := agentaudit.NewLogger(deps.DB)
	svc := agentsvc.New(registry, audit, llmProvider)
	if err := svc.EnsureSchema(context.Background()); err != nil {
		log.Printf("agent ensure schema failed: %v", err)
	}
	api.RegisterRoutes(mux, svc)
	return nil
}

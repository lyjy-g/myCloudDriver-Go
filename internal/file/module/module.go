package module

import (
	"time"

	"github.com/gin-gonic/gin"
	"myclouddrive-go/internal/app"
	"myclouddrive-go/internal/file/api"
	"myclouddrive-go/internal/file/service"
	pluginsvc "myclouddrive-go/internal/plugin/service"
	storagesvc "myclouddrive-go/internal/storage/service"
)

// Module 是 file 服务模块占位。
type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return "file"
}

func (m *Module) Models() []any {
	return nil
}

func (m *Module) RegisterRoutes(router gin.IRouter, deps *app.Dependencies) error {
	storageService := storagesvc.NewService(deps.DB, deps.Redis, 30*time.Second, pluginsvc.GetRunManager(deps.DB))
	fileService := service.NewFileService(storageService, deps.DB, deps.Redis)
	api.RegisterRoutes(router, fileService)
	return nil
}

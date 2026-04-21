package module

import (
	"github.com/gin-gonic/gin"

	"myclouddrive-go/internal/app"
	"myclouddrive-go/internal/share/api"
	"myclouddrive-go/internal/share/service"
)

// Module 是 share 服务模块占位。
type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return "share"
}

func (m *Module) Models() []any {
	return nil
}

func (m *Module) RegisterRoutes(engine *gin.Engine, _ *app.Dependencies) error {
	api.RegisterRoutes(engine, service.NewPlaceholderService())
	return nil
}

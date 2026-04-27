package module

import (
	"net/http"

	"myclouddrive-go/internal/app"
	pluginsvc "myclouddrive-go/internal/plugin/service"
)

// Module 是插件运行时模块，负责插件系统初始化。
type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return "plugin"
}

func (m *Module) Models() []any {
	return nil
}

func (m *Module) RegisterRoutes(_ *http.ServeMux, deps *app.Dependencies) error {
	pluginsvc.Init(deps.DB)
	return nil
}

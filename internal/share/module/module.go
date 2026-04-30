package module

import (
	"net/http"
	"time"

	"myclouddrive-go/internal/app"
	pluginsvc "myclouddrive-go/internal/plugin/service"
	"myclouddrive-go/internal/share/api"
	"myclouddrive-go/internal/share/service"
	storagesvc "myclouddrive-go/internal/storage/service"
)

// Module 是 share 服务模块。
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

func (m *Module) RegisterRoutes(mux *http.ServeMux, deps *app.Dependencies) error {
	storageService := storagesvc.NewService(deps.DB, deps.Redis, 30*time.Second, pluginsvc.GetRunManager(deps.DB))
	api.RegisterRoutes(mux, service.NewService(deps.DB, storageService))
	return nil
}

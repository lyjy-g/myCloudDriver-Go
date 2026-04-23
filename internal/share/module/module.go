package module

import (
	"net/http"

	"myclouddrive-go/internal/app"
	"myclouddrive-go/internal/share/api"
	"myclouddrive-go/internal/share/service"
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
	api.RegisterRoutes(mux, service.NewService(deps.DB))
	return nil
}

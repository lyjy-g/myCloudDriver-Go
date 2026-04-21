package module

import (
	"net/http"

	"myclouddrive-go/internal/app"
	"myclouddrive-go/internal/user/api"
	"myclouddrive-go/internal/user/service"
)

// Module 是 user 服务模块占位。
type Module struct{}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return "user"
}

func (m *Module) Models() []any {
	return nil
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, _ *app.Dependencies) error {
	api.RegisterRoutes(mux, service.NewPlaceholderService())
	return nil
}

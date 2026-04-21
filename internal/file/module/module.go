package module

import (
	"net/http"

	"myclouddrive-go/internal/app"
	"myclouddrive-go/internal/file/api"
	"myclouddrive-go/internal/file/service"
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

func (m *Module) RegisterRoutes(mux *http.ServeMux, _ *app.Dependencies) error {
	api.RegisterRoutes(mux, service.NewPlaceholderService())
	return nil
}

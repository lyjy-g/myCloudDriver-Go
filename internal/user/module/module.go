package module

import (
	"net/http"
	"os"

	"myclouddrive-go/internal/app"
	"myclouddrive-go/internal/framework/security"
	"myclouddrive-go/internal/user/api"
	"myclouddrive-go/internal/user/service"
)

// Module 是 user 服务模块。
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

func (m *Module) RegisterRoutes(mux *http.ServeMux, deps *app.Dependencies) error {
	jwtSvc := security.NewJWTService(os.Getenv("MYCLOUDDRIVE_JWT_SECRET"))
	api.RegisterRoutes(mux, service.NewUserService(deps.DB, deps.Redis, jwtSvc))
	return nil
}

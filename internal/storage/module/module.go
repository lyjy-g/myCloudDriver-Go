package module

import (
	"net/http"
	"time"

	"myclouddrive-go/internal/app"
	storageapi "myclouddrive-go/internal/storage/api"
	"myclouddrive-go/internal/storage/model/model"
	"myclouddrive-go/internal/storage/service"
)

// Module 是 storage 服务模块。
type Module struct {
	cacheTTL time.Duration
}

func New() *Module {
	return &Module{cacheTTL: 30 * time.Second}
}

func (m *Module) Name() string {
	return "storage"
}

func (m *Module) Models() []any {
	return []any{&model.StoragePlatform{}, &model.StorageSetting{}}
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, deps *app.Dependencies) error {
	svc := service.NewService(deps.DB, deps.Redis, m.cacheTTL)
	storageapi.RegisterRoutes(mux, svc)
	return nil
}

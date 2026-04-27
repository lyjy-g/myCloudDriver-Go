package module

import (
	"net/http"
	"time"

	"myclouddrive-go/internal/app"
	pluginsvc "myclouddrive-go/internal/plugin/service"
	storageapi "myclouddrive-go/internal/storage/api"
	dbmodel "myclouddrive-go/internal/storage/model/dbmodel"
	storageService "myclouddrive-go/internal/storage/service"
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
	return []any{&dbmodel.StoragePlatform{}, &dbmodel.StorageSetting{}}
}

func (m *Module) RegisterRoutes(mux *http.ServeMux, deps *app.Dependencies) error {
	svc := storageService.NewService(deps.DB, deps.Redis, m.cacheTTL, pluginsvc.GetRuntime(deps.DB))
	storageapi.RegisterRoutes(mux, svc)
	return nil
}

package app

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"myclouddrive-go/internal/framework/cache"
	"myclouddrive-go/internal/framework/config"
	"myclouddrive-go/internal/framework/orm"
)

// Module 定义服务模块扩展点。
// 后续新增 user/file/share 等服务时，只需要实现该接口并在 main 中注册。
type Module interface {
	Name() string
	Models() []any
	RegisterRoutes(mux *http.ServeMux, deps *Dependencies) error
}

// Dependencies 是模块可使用的基础设施依赖集合。
type Dependencies struct {
	Config *config.Config
	DB     *gorm.DB
	Redis  redis.Cmdable
}

// Server 表示应用服务实例。
type Server struct {
	httpServer *http.Server
}

// NewServer 构建带模块能力的标准库 HTTP 服务。
func NewServer(configPath string, modules ...Module) (*Server, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	db, err := orm.NewGormDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init gorm: %w", err)
	}

	rdb, err := cache.NewRedisClient(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("warn: redis unavailable, fallback without cache: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	deps := &Dependencies{Config: cfg, DB: db, Redis: rdb}
	for _, module := range modules {
		if err = module.RegisterRoutes(mux, deps); err != nil {
			return nil, fmt.Errorf("register module %s: %w", module.Name(), err)
		}
		log.Printf("module registered: %s", module.Name())
	}

	handler := recoverMiddleware(accessLogMiddleware(mux))
	return &Server{httpServer: &http.Server{Addr: cfg.HTTP.Addr, Handler: handler}}, nil
}

// Run 启动 HTTP 服务。
func (s *Server) Run() error {
	log.Printf("http api listening on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func accessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v\n%s", rec, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

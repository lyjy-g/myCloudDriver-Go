package app

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
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
	RegisterRoutes(engine *gin.Engine, deps *Dependencies) error
}

// Dependencies 是模块可使用的基础设施依赖集合。
type Dependencies struct {
	Config *config.Config
	DB     *gorm.DB
	Redis  redis.Cmdable
}

// Server 表示应用服务实例。
type Server struct {
	engine *gin.Engine
	addr   string
}

// NewServer 构建带模块能力的 Gin 服务。
func NewServer(configPath string, modules ...Module) (*Server, error) {

	//读取mysql/redis配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	//创建gorm
	db, err := orm.NewGormDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init gorm: %w", err)
	}

	//创建redis
	rdb, err := cache.NewRedisClient(cfg.Redis)
	if err != nil {
		return nil, fmt.Errorf("warn: redis unavailable, fallback without cache: %v", err)
	}

	ginEngine := gin.New()
	ginEngine.Use(gin.Logger(), gin.Recovery())
	ginEngine.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	//创建`type Dependencies struct`把实例放进去
	depens := &Dependencies{Config: cfg, DB: db, Redis: rdb}

	//遍历模块列表
	for _, module := range modules {
		if err = module.RegisterRoutes(ginEngine, depens); err != nil {
			return nil, fmt.Errorf("register module %s: %w", module.Name(), err)
		}
		log.Printf("module registered: %s", module.Name())
	}

	return &Server{engine: ginEngine, addr: cfg.HTTP.Addr}, nil
}

// Run 启动 HTTP 服务。
func (s *Server) Run() error {
	log.Printf("gin api listening on %s", s.addr)
	return s.engine.Run(s.addr)
}

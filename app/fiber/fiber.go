package fiber

import (
	"context"
	"strconv"

	"github.com/xsxdot/aio/pkg/distributed/election"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/internal/authmanager"
	"github.com/xsxdot/aio/internal/cache/server"
	configService "github.com/xsxdot/aio/internal/config"
	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/internal/monitoring"
	monitorapi "github.com/xsxdot/aio/internal/monitoring/api"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
	"go.uber.org/zap"
)

// Server 代表Fiber Web服务器
type Server struct {
	app      *fiber.App
	config   *ApiConfig
	logger   *zap.Logger
	status   consts.ComponentStatus
	shutdown chan struct{}
}

func (s *Server) RegisterMetadata() (bool, int, map[string]string) {
	return false, 0, nil
}

// ApiConfig 代表Fiber服务器实例配置
type ApiConfig struct {
	ListenAddr string `json:"listen_addr" yaml:"listen_addr"`
}

// NewFiberServer 创建并配置一个新的Fiber服务器
func NewFiberServer() *Server {
	fiberApp := fiber.New(fiber.Config{
		AppName:      "AIO API Server",
		ErrorHandler: customErrorHandler,
	})

	// 添加中间件
	fiberApp.Use(recover.New())     // 从恐慌中恢复
	fiberApp.Use(fiberlogger.New()) // 使用fiber的logger中间件

	fiberApp.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
		//AllowCredentials: true,
	}))

	return &Server{
		app:      fiberApp,
		logger:   common.GetLogger().GetZapLogger(consts.ComponentFiberServer),
		shutdown: make(chan struct{}),
	}
}

func (s *Server) Name() string {
	return consts.ComponentFiberServer
}

func (s *Server) Status() consts.ComponentStatus {
	return s.status
}

// GetClientConfig 实现Component接口，返回客户端配置
func (s *Server) GetClientConfig() (bool, *config.ClientConfig) {
	return false, nil
}

// DefaultConfig 返回组件的默认配置
func (s *Server) DefaultConfig(config *config.BaseConfig) interface{} {
	return s.genConfig(config)
}

func (s *Server) genConfig(config *config.BaseConfig) *ApiConfig {
	ip := config.Network.LocalIp
	if config.Network.HttpAllowExternal {
		ip = "0.0.0.0"
	}
	return &ApiConfig{ListenAddr: ip + ":" + strconv.Itoa(config.Network.HttpPort)}
}

func (s *Server) Init(config *config.BaseConfig, body []byte) error {
	a := s.genConfig(config)
	s.config = a
	s.status = consts.StatusInitialized
	return nil
}

func (s *Server) Restart(ctx context.Context) error {
	err := s.Stop(ctx)
	if err != nil {
		return err
	}
	return s.Start(ctx)
}

// Start 启动Fiber服务器
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("启动Fiber API服务器", zap.String("地址", s.config.ListenAddr))
	go func() {
		err := s.app.Listen(s.config.ListenAddr)
		if err != nil {
			panic(err)
		}
	}()
	s.status = consts.StatusRunning
	return nil
}

// Stop 停止Fiber服务器
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("关闭Fiber API服务器")
	err := s.app.Shutdown()
	if err != nil {
		return err
	}
	s.status = consts.StatusStopped
	return err
}

// GetApp 返回底层Fiber应用实例
func (s *Server) GetApp() *fiber.App {
	return s.app
}

// RegisterAuthManagerAPI 注册AuthManager API路由
func (s *Server) RegisterAuthManagerAPI(authManager *authmanager.AuthManager) {
	if authManager == nil {
		s.logger.Warn("AuthManager为空，跳过API注册")
		return
	}

	api := authmanager.NewAPI(authManager)
	api.RegisterRoutes(s.app)
	s.logger.Info("已注册AuthManager API路由")
}

// RegisterEtcdAPI 注册ETCD API路由
func (s *Server) RegisterEtcdAPI(etcdServer *etcd.EtcdServer, etcdClient *etcd.EtcdClient) {
	if etcdClient == nil {
		s.logger.Warn("EtcdClient为空，跳过API注册")
		return
	}

	api := etcd.NewAPI(etcdServer, etcdClient, s.logger)
	api.SetupRoutes(s.app)
	s.logger.Info("已注册ETCD API路由")
}

// RegisterCacheServerAPI 注册缓存服务器API路由
func (s *Server) RegisterCacheServerAPI(cacheServer *server.Server) {
	if cacheServer == nil {
		s.logger.Warn("缓存服务器为空，跳过API注册")
		return
	}

	api := server.NewAPI(cacheServer, s.logger)
	api.SetupRoutes(s.app)
	s.logger.Info("已注册缓存服务器API路由")
}

// RegisterConfigAPI 注册配置中心API路由
func (s *Server) RegisterConfigAPI(configSer *configService.Service) {
	if configSer == nil {
		s.logger.Warn("配置中心服务为空，跳过API注册")
		return
	}

	api := configService.NewAPI(configSer, s.logger)
	api.SetupRoutes(s.app)
	s.logger.Info("已注册配置中心API路由")
}

// RegisterDistributedAPI 注册分布式组件API路由
func (s *Server) RegisterDistributedAPI(manager discovery.DiscoveryService, election election.ElectionService) {
	if manager == nil {
		s.logger.Warn("分布式组件管理器为空，跳过API注册")
		return
	}

	api := distributed.NewAPI(manager, election, s.logger)

	api.SetupRoutes(s.app)
	s.logger.Info("已注册分布式组件API路由")
}

// RegisterMonitorAPI 注册监控API路由
func (s *Server) RegisterMonitorAPI(monitorService *monitoring.Monitor) {
	if monitorService == nil {
		s.logger.Warn("监控服务为空，跳过API注册")
		return
	}

	api := monitorapi.NewAPI(monitorService.GetStorage(), monitorService.GetAlertManager(), monitorService.GetNotifierManager(), s.logger)
	api.SetupRoutes(s.app)
	s.logger.Info("已注册监控API路由")
}

// 自定义错误处理程序
func customErrorHandler(c *fiber.Ctx, err error) error {
	// 默认状态码为500 Internal Server Error
	code := fiber.StatusInternalServerError

	// 检查是否是Fiber错误
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	// 返回JSON格式的错误
	return c.Status(code).JSON(fiber.Map{
		"error": true,
		"msg":   err.Error(),
	})
}

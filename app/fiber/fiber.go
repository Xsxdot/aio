package fiber

import (
	"context"
	"strconv"

	config2 "github.com/xsxdot/aio/pkg/config"

	"github.com/xsxdot/aio/pkg/distributed/election"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/internal/authmanager"
	"github.com/xsxdot/aio/internal/cache/server"
	"github.com/xsxdot/aio/internal/certmanager"
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
	app              *fiber.App
	config           *ApiConfig
	logger           *zap.Logger
	status           consts.ComponentStatus
	shutdown         chan struct{}
	router           fiber.Router
	baseAuthHandler  func(c *fiber.Ctx) error
	adminRoleHandler func(c *fiber.Ctx) error
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
		router:   fiberApp.Group("/api"),
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

func (s *Server) GetRouter() (fiber.Router, func(c *fiber.Ctx) error, func(c *fiber.Ctx) error) {
	return s.router, s.baseAuthHandler, s.adminRoleHandler
}

func (s *Server) genConfig(config *config.BaseConfig) *ApiConfig {
	ip := config.Network.LocalIP
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

// RegisterStaticFiles 配置静态文件服务，用于提供Vue打包的前端页面
func (s *Server) RegisterStaticFiles(staticPath string, prefixPath string) {
	if staticPath == "" {
		s.logger.Warn("静态文件路径为空，跳过静态文件服务配置")
		return
	}

	// 注册静态文件目录
	s.app.Static(prefixPath, staticPath, fiber.Static{
		Compress:      true,           // 启用压缩
		ByteRange:     true,           // 启用字节范围请求
		Browse:        false,          // 禁止目录浏览
		Index:         "index.html",   // 默认索引文件
		CacheDuration: 24 * 3600 * 10, // 设置缓存时间（例如10天）
	})

	// 处理SPA路由 - 将所有不匹配的请求重定向到index.html
	// 注意：这应该在API路由配置之后添加
	s.app.Get("*", func(c *fiber.Ctx) error {
		// 检查请求的路径是否以API路径开头，如果是API请求则跳过
		path := c.Path()
		if len(path) >= 4 && path[0:4] == "/api" {
			return c.Next()
		}

		// 静态资源文件检查（不重定向常见的静态资源请求）
		if len(path) > 0 {
			ext := getFileExtension(path)
			if ext == ".js" || ext == ".css" || ext == ".png" || ext == ".jpg" ||
				ext == ".jpeg" || ext == ".gif" || ext == ".svg" || ext == ".ico" ||
				ext == ".woff" || ext == ".woff2" || ext == ".ttf" || ext == ".eot" {
				return c.Next()
			}
		}

		// 重定向到index.html以支持SPA路由
		return c.SendFile(staticPath + "/index.html")
	})

	s.logger.Info("已注册静态文件服务", zap.String("路径", staticPath), zap.String("前缀", prefixPath))
}

// 辅助函数：获取文件扩展名
func getFileExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

// RegisterAuthManagerAPI 注册AuthManager API路由
func (s *Server) RegisterAuthManagerAPI(authManager *authmanager.AuthManager) {
	if authManager == nil {
		s.logger.Warn("AuthManager为空，跳过API注册")
		return
	}

	api := authmanager.NewAPI(authManager)

	s.baseAuthHandler = api.AuthMiddleware
	s.adminRoleHandler = api.AdminRoleMiddleware

	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册AuthManager API路由")
}

// RegisterEtcdAPI 注册ETCD API路由
func (s *Server) RegisterEtcdAPI(etcdServer *etcd.EtcdServer, etcdClient *etcd.EtcdClient) {
	if etcdClient == nil {
		s.logger.Warn("EtcdClient为空，跳过API注册")
		return
	}

	api := etcd.NewAPI(etcdServer, etcdClient, s.logger)
	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册ETCD API路由")
}

// RegisterCacheServerAPI 注册缓存服务器API路由
func (s *Server) RegisterCacheServerAPI(cacheServer *server.Server) {
	if cacheServer == nil {
		s.logger.Warn("缓存服务器为空，跳过API注册")
		return
	}

	api := server.NewAPI(cacheServer, s.logger)
	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册缓存服务器API路由")
}

// RegisterConfigAPI 注册配置中心API路由
func (s *Server) RegisterConfigAPI(configSer *config2.Service) {
	if configSer == nil {
		s.logger.Warn("配置中心服务为空，跳过API注册")
		return
	}

	api := config2.NewAPI(configSer, s.logger)
	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册配置中心API路由")
}

// RegisterDistributedAPI 注册分布式组件API路由
func (s *Server) RegisterDistributedAPI(manager discovery.DiscoveryService, election election.ElectionService) {
	if manager == nil {
		s.logger.Warn("分布式组件管理器为空，跳过API注册")
		return
	}

	api := distributed.NewAPI(manager, election, s.logger)
	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册分布式组件API路由")
}

// RegisterMonitorAPI 注册监控API路由
func (s *Server) RegisterMonitorAPI(monitorService *monitoring.Monitor) {
	if monitorService == nil {
		s.logger.Warn("监控服务为空，跳过API注册")
		return
	}

	api := monitorapi.NewAPI(monitorService.GetStorage(), monitorService.GetAlertManager(), monitorService.GetNotifierManager(), s.logger)
	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册监控API路由")
}

// RegisterCertManagerAPI 注册证书管理器API路由
func (s *Server) RegisterCertManagerAPI(certManager *certmanager.CertManager) {
	if certManager == nil {
		s.logger.Warn("证书管理器为空，跳过API注册")
		return
	}

	api := certmanager.NewAPI(certManager, s.logger)
	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册证书管理器API路由")
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

package fiber

import (
	"context"
	"fmt"

	config2 "github.com/xsxdot/aio/pkg/config"
	"github.com/xsxdot/aio/pkg/monitoring"
	monitorapi "github.com/xsxdot/aio/pkg/monitoring/api"
	"github.com/xsxdot/aio/pkg/registry"
	"github.com/xsxdot/aio/pkg/server"
	"github.com/xsxdot/aio/pkg/server/credential"
	"github.com/xsxdot/aio/pkg/server/nginx"
	"github.com/xsxdot/aio/pkg/server/service"
	"github.com/xsxdot/aio/pkg/server/systemd"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/xsxdot/aio/internal/authmanager"
	"github.com/xsxdot/aio/internal/certmanager"
	"github.com/xsxdot/aio/pkg/common"
	"go.uber.org/zap"
)

// Server 代表Fiber Web服务器
type Server struct {
	app              *fiber.App
	logger           *zap.Logger
	shutdown         chan struct{}
	router           fiber.Router
	baseAuthHandler  func(c *fiber.Ctx) error
	adminRoleHandler func(c *fiber.Ctx) error
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
		logger:   common.GetLogger().GetZapLogger("FiberServer"),
		shutdown: make(chan struct{}),
	}
}

func (s *Server) GetRouter() (fiber.Router, func(c *fiber.Ctx) error, func(c *fiber.Ctx) error) {
	return s.router, s.baseAuthHandler, s.adminRoleHandler
}

// Start 启动Fiber服务器
func (s *Server) Start(port int) error {
	s.logger.Info("启动Fiber API服务器")
	go func() {
		err := s.app.Listen(fmt.Sprintf(":%d", port))
		if err != nil {
			panic(err)
		}
	}()
	return nil
}

// Stop 停止Fiber服务器
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("关闭Fiber API服务器")
	err := s.app.Shutdown()
	if err != nil {
		return err
	}
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
func (s *Server) RegisterDistributedAPI(manager registry.Registry) {
	if manager == nil {
		s.logger.Warn("分布式组件管理器为空，跳过API注册")
		return
	}

	api := registry.NewAPI(manager, s.logger)
	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册分布式组件API路由")
}

// RegisterMonitorAPI 注册监控API路由
func (s *Server) RegisterMonitorAPI(port int, monitorService *monitoring.Monitor) {
	if monitorService == nil {
		s.logger.Warn("监控服务为空，跳过API注册")
		return
	}

	api := monitorapi.NewAPI(port, monitorService.GetStorage(), monitorService.GetGrpcStorage(), monitorService.GetAlertManager(), monitorService.GetNotifierManager(), s.logger)
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

// RegisterServerAPI 注册服务器API路由
func (s *Server) RegisterServerAPI(serverService server.Service) {
	if serverService == nil {
		s.logger.Warn("服务器服务为空，跳过API注册")
		return
	}

	api := service.NewAPI(serverService, s.logger)
	api.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册服务器API路由")
	credentialApi := credential.NewAPI(serverService.GetCredentialService(), s.logger)
	credentialApi.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册凭证API路由")
	systemdApi := systemd.NewAPIHandler(serverService.GetSystemdManager())
	systemdApi.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册systemd API路由")
	nginxApi := nginx.NewAPIHandler(serverService.GetNginxManager())
	nginxApi.RegisterRoutes(s.router, s.baseAuthHandler, s.adminRoleHandler)
	s.logger.Info("已注册nginx API路由")
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

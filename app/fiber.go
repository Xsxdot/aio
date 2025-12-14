package app

import (
	"strings"
	"time"
	"xiaozhizhang/pkg/core/start"

	"github.com/gofiber/fiber/v2"
	"github.com/xsxdot/aio/pkg/common"
	"go.uber.org/zap"
)

func GetApp() *fiber.App {
	app := start.GetApp()

	RegisterStaticFiles(app, "./web", "/")

	return app
}

// func RegisterRoutes(router *fiber.App) {
// 	api := router.Group("/api", fiber_handle.NewApiTracer(fiber_handle.TracerConfig{
// 		Tracer:  tracer.NewSimpleTracer(),
// 		AppName: base.Configures.Config.AppName,
// 	}), logger.NewApiLogger(logger.Config{Logger: base.Logger}))
// 	admin := router.Group("/admin", fiber_handle.NewApiTracer(fiber_handle.TracerConfig{
// 		Tracer:  tracer.NewSimpleTracer(),
// 		AppName: base.Configures.Config.AppName,
// 	}), logger.NewAdminLogger(logger.AdminConfig{Logger: base.Logger}))
// 	third := router.Group("/third", fiber_handle.NewApiTracer(fiber_handle.TracerConfig{
// 		Tracer:  tracer.NewSimpleTracer(),
// 		AppName: base.Configures.Config.AppName,
// 	}), logger.NewAdminLogger(logger.AdminConfig{Logger: base.Logger}))
// 	// 配置中心需要先初始化，因为其他模块可能依赖它
// 	configAppF(api, admin, third)
// 	publicAppF(api, admin, third)
// 	fileApp(api, admin, third)
// 	productAppF(api, admin, third)
// 	// 订单模块需要在product和public之后初始化
// 	orderAppF(api, admin, third)
// 	// 统计模块
// 	statisticsAppF(api, admin, third)
// }

// func publicAppF(api, admin, third fiber.Router) {
// 	app := publicApp.NewPublicApp(base.DB, ConfigApp)
// 	PublicApp = app

// 	adminCtrl := publicController.NewAdministratorController(app, base.AdminAuth)
// 	adminCtrl.RegisterRoutes(admin)

// 	// 首页控制器
// 	indexCtrl := publicController.NewIndexController(app, base.AdminAuth, base.DB)
// 	indexCtrl.RegisterRoutes(admin, api)

// 	// 用户控制器
// 	userCtrl := publicController.NewUserController(app)
// 	userCtrl.RegisterRoutes(api, admin)

// 	// 支付控制器
// 	paymentCtrl := publicController.NewPaymentController(app)
// 	paymentCtrl.RegisterRoutes(api, admin, third)

// 	// 用户收货地址控制器
// 	addressCtrl := publicController.NewUserAddressController(app)
// 	addressCtrl.RegisterRoutes(api)
// }

// func fileApp(api, admin, third fiber.Router) {
// 	managerApp := file.NewFileManagerApp()
// 	FileApp = managerApp
// 	base.FileApp = managerApp // 设置全局FileApp
// 	fileCtrl := fileController.NewFileController(managerApp)

// 	fileCtrl.RegisterRoutes(api, admin, third)
// }

// func productAppF(api, admin, third fiber.Router) {
// 	app := productApp.NewProductApp(base.DB)
// 	ProductApp = app

// 	// 启动购物车清空消费者（处理订单创建后的购物车清空消息）
// 	ctx := context.Background()
// 	app.CartAppService.StartCartClearConsumer(ctx)
// 	base.Logger.Info("购物车清空消费者已启动")

// 	// 注册商品模块路由（管理后台）
// 	productController.RegisterProductRoutes(admin, app)

// 	// 注册用户端商品路由（移动端，不需要登录）
// 	productController.RegisterUserProductRoutes(api, app)

// 	// 注册购物车路由（用户端,需要用户认证）
// 	productController.RegisterCartRoutes(api, app)

// 	// 注册库存管理路由（管理后台）
// 	productController.RegisterStockRoutes(admin, app)

// 	// 注册库存定时任务
// 	if base.Scheduler != nil {
// 		app.StockApp.RegisterStockSchedulers(base.Scheduler)
// 	}
// }

// RegisterStaticFiles 配置静态文件服务，用于提供Vue打包的前端页面
func RegisterStaticFiles(app *fiber.App, staticPath string, prefixPath string) {
	if staticPath == "" {
		return
	}

	// 注册静态文件目录
	app.Static(prefixPath, staticPath, fiber.Static{
		Compress:      true,             // 启用压缩
		ByteRange:     true,             // 启用字节范围请求
		Browse:        false,            // 禁止目录浏览
		Index:         "index.html",     // 默认索引文件
		CacheDuration: 10 * time.Minute, // 设置缓存时间（例如10天）
	})

	// 处理SPA路由 - 将所有不匹配的请求重定向到index.html
	// 注意：这应该在API路由配置之后添加
	app.Get("*", func(c *fiber.Ctx) error {
		// 检查请求的路径是否以API路径开头，如果是API请求则跳过
		path := c.Path()
		if strings.HasPrefix(path, "/api") {
			return c.Next()
		}
		if strings.HasPrefix(path, "/admin") {
			return c.Next()
		}
		if strings.HasPrefix(path, "/third") || strings.HasPrefix(path, "/internal") || strings.HasPrefix(path, "/gateway") {
			return c.Next()
		}
		if strings.HasPrefix(path, "/health") {
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

	common.GetLogger().Info("已注册静态文件服务", zap.String("路径", staticPath), zap.String("前缀", prefixPath))
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

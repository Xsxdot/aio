package router

import (
	"xiaozhizhang/app"
	"xiaozhizhang/system/config"
	"xiaozhizhang/system/registry"
	"xiaozhizhang/system/server"
	"xiaozhizhang/system/shorturl"
	"xiaozhizhang/system/ssl"
	"xiaozhizhang/system/user"

	"github.com/gofiber/fiber/v2"
)

// Register 负责集中注册所有 HTTP 路由。
// 按规范：
//   - 只依赖 app.App（业务编排入口）和 fiber.App（HTTP Server）。
//   - 不直接依赖任何 DAO / Service / system/internal 包。
//   - 不包含业务逻辑，只做分组与路由绑定。
func Register(a *app.App, f *fiber.App) {
	// 示例：公共 API 分组
	api := f.Group("/api")

	// 示例演示：简单健康检查路由
	api.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"msg": "ok"})
	})

	// 后台管理路由分组
	admin := f.Group("/admin")

	// 注册用户组件路由（管理员登录、管理员管理、客户端凭证管理）
	user.RegisterRoutes(a.UserModule, api, admin)

	// 注册配置中心路由（通过 config 组件暴露的统一入口）
	config.RegisterRoutes(a.ConfigModule, api, admin)

	// 注册注册中心路由
	registry.RegisterRoutes(a.RegistryModule, api, admin)

	// 注册 SSL 证书组件路由
	ssl.RegisterRoutes(a.SslModule, api, admin)

	// 注册 Server 管理组件路由
	server.RegisterRoutes(a.ServerModule, api, admin)

	// 注册短网址组件路由
	shorturl.RegisterRoutes(a.ShortURLModule, api, admin)
}

package ssl

import (
	controller "github.com/xsxdot/aio/system/ssl/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册 SSL 证书组件的所有 HTTP 路由
// 此函数在 ssl 包内，可以访问 Module 的私有字段 internalApp
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.App）
	sslController := controller.NewSslController(m.internalApp)
	sslController.RegisterRoutes(admin)
}

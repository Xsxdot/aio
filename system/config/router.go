package config

import (
	controller "xiaozhizhang/system/config/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册配置中心的所有 HTTP 路由
// 此函数在 config 包内，可以访问 Module 的私有字段 internalApp
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.App）
	adminController := controller.NewConfigController(m.internalApp)
	adminController.RegisterRoutes(admin)

}

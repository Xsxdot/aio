package executor

import (
	controller "github.com/xsxdot/aio/system/executor/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册任务执行器的所有 HTTP 路由
// 此函数在 executor 包内，可以访问 Module 的私有字段 internalApp
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.App）
	adminController := controller.NewExecutorAdminController(m.internalApp)
	adminController.RegisterRoutes(admin)
}

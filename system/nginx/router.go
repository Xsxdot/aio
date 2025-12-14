package nginx

import (
	controller "xiaozhizhang/system/nginx/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册 Nginx 管理组件的所有 HTTP 路由
// 此函数在 nginx 包内，可以访问 Module 的私有字段 internalApp
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.App）
	configController := controller.NewConfigController(m.internalApp)
	configController.RegisterRoutes(admin)

	// 如果未来有对外 API，可在这里添加
	_ = api // 暂未使用
}

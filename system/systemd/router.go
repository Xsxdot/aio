package systemd

import (
	controller "xiaozhizhang/system/systemd/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册 Systemd 管理组件的所有 HTTP 路由
// 此函数在 systemd 包内，可以访问 Module 的私有字段 internalApp
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.App）
	serviceController := controller.NewSystemdServiceController(m.internalApp)
	serviceController.RegisterRoutes(admin)

	// 如果未来有对外 API，可在这里添加
	_ = api // 暂未使用
}


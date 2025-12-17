package nginx

import (
	controller "xiaozhizhang/system/nginx/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutesAgentified 注册 Nginx Agent化组件的所有 HTTP 路由
func RegisterRoutesAgentified(m *ModuleAgentified, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.AppAgentified）
	configController := controller.NewConfigControllerAgentified(m.internalApp)
	configController.RegisterRoutes(admin)

	// 如果未来有对外 API，可在这里添加
	_ = api // 暂未使用
}


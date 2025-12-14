package application

import (
	controller "xiaozhizhang/system/application/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册 Application 组件的所有 HTTP 路由
// 此函数在 application 包内，可以访问 Module 的私有字段 internalApp
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.App）
	appController := controller.NewApplicationController(m.internalApp)
	appController.RegisterRoutes(admin)

	// 对外 API（如需要公开查询接口）
	_ = api // 暂未使用
}


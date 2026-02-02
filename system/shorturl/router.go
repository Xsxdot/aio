package shorturl

import (
	controller "xiaozhizhang/system/shorturl/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册短网址组件的所有 HTTP 路由
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.App）
	adminController := controller.NewShortURLAdminController(m.internalApp)
	adminController.RegisterRoutes(admin)

	// 对外接口（短链访问与跳转、成功上报）
	apiController := controller.NewShortURLAPIController(m.internalApp)
	apiController.RegisterRoutes(api)
}


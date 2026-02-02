package registry

import (
	controller "github.com/xsxdot/aio/system/registry/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册注册中心组件的所有 HTTP 路由
// 此函数在 registry 包内，可以访问 Module 的私有字段 internalApp
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（依赖 internal/app.App）
	adminController := controller.NewRegistryAdminController(m.internalApp)
	adminController.RegisterRoutes(admin)

	// 对外接口（依赖 api/client）
	apiController := controller.NewRegistryAPIController(m.Client)
	apiController.RegisterRoutes(api)
}

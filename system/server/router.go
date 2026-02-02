package server

import (
	controller "github.com/xsxdot/aio/system/server/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册 server 组件的所有 HTTP 路由
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 后台管理接口（CRUD + 状态查询）
	serverController := controller.NewServerController(m.internalApp)
	serverController.RegisterRoutes(admin)

	// 上报接口（对外 API，ClientAuth 鉴权）
	reportController := controller.NewReportController(m.internalApp)
	reportController.RegisterRoutes(api)
}



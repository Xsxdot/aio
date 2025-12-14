package user

import (
	controller "xiaozhizhang/system/user/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册用户组件路由
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	// 创建 HTTP Controllers
	adminController := controller.NewAdminController(m.internalApp)
	clientController := controller.NewClientCredentialController(m.internalApp)

	// 注册管理员路由（包含登录接口）
	adminController.RegisterRoutes(admin)

	// 注册客户端凭证路由
	clientController.RegisterRoutes(admin)
}




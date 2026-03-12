package workflow

import (
	controller "github.com/xsxdot/aio/system/workflow/external/http"

	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes 注册工作流组件的所有 HTTP 路由
func RegisterRoutes(m *Module, api, admin fiber.Router) {
	adminCtrl := controller.NewWorkflowAdminController(m.internalApp)

	adminCtrl.RegisterRoutes(admin)
}

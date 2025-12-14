package start

import (
	"fmt"
	"xiaozhizhang/pkg/core/fiber_handle"
	"xiaozhizhang/pkg/core/util"

	"github.com/gofiber/fiber/v2"
	recover2 "github.com/gofiber/fiber/v2/middleware/recover"
)

func GetApp() *fiber.App {
	app := fiber.New(
		fiber.Config{
			BodyLimit:    10 * 1024 * 1024,
			ErrorHandler: fiber_handle.ErrHandler,
		})
	app.Use(fiber_handle.Cors())
	app.Use(recover2.New(recover2.Config{
		Next:             nil,
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			util.SendOpsMessage(util.Context(c), fmt.Sprintf("url：%s崩溃了。%+v", c.Path(), e))
			fmt.Println(e)
		},
	}))
	//app.Use(pprof.New())
	app.Use(fiber_handle.HealthCheck(fiber_handle.HealthCheckConfig{Path: "/health"}))
	return app
}

func UseMonitor(client fiber_handle.MonitorClient) fiber.Handler {
	return fiber_handle.NewAPIMonitorWithFilters(fiber_handle.MonitorConfig{
		Client: client,
	}, fiber_handle.SkipMethods("OPTIONS"), fiber_handle.OnlyPathStartWith("/api", "/admin", "/third", "/internal", "/gateway"))
}

package fiber_handle

import (
	"github.com/gofiber/fiber/v2"
	errorc "github.com/xsxdot/aio/pkg/core/err"
)

func InternalHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		if err != nil {
			cError := errorc.ParseError(err)
			return c.Status(cError.Code).JSON(fiber.Map{"message": cError.Error()})
		}
		return nil
	}
}

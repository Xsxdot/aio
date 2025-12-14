package result

import (
	"github.com/gofiber/fiber/v2"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/util"
)

func OK(c *fiber.Ctx, v interface{}) error {
	return c.Status(200).JSON(fiber.Map{"status": 200, "data": v})
}

func BadRequestNormal(c *fiber.Ctx, message string, err error) error {
	return errorc.New(message, err).WithTraceID(util.Context(c))
}

func BadRequest(c *fiber.Ctx, err error) error {
	return err
}

func Once(c *fiber.Ctx, v interface{}, err error) error {
	if err == nil {
		return OK(c, v)
	} else {
		return BadRequest(c, err)
	}
}

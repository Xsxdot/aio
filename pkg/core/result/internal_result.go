package result

import (
	"github.com/gofiber/fiber/v2"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/util"
)

func InternalOK(c *fiber.Ctx, v interface{}) error {
	return c.Status(200).JSON(v)
}

func InternalBadRequestNormal(c *fiber.Ctx, message string, err error) error {
	return errorc.New(message, err).WithTraceID(util.Context(c))
}

func InternalBadRequest(c *fiber.Ctx, err error) error {
	return err
}

func InternalOnce(c *fiber.Ctx, v interface{}, err error) error {
	if err == nil {
		return InternalOK(c, v)
	} else {
		return InternalBadRequest(c, err)
	}
}

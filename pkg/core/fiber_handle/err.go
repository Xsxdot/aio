package fiber_handle

import (
	"errors"
	errorc "xiaozhizhang/pkg/core/err"

	"github.com/gofiber/fiber/v2"
)

func ErrHandler(ctx *fiber.Ctx, err error) error {

	var e *fiber.Error
	if errors.As(err, &e) {
		return ctx.Status(e.Code).SendString(e.Message)
	}

	cError := errorc.ParseError(err)

	return ctx.Status(200).JSON(fiber.Map{"status": cError.Code, "message": cError.Msg, "errData": cError})
}

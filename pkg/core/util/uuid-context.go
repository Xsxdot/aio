package util

import (
	"context"
	"github.com/gofiber/fiber/v2"
	uuid "github.com/satori/go.uuid"
	"github.com/xsxdot/aio/pkg/core/consts"
)

func Context(c *fiber.Ctx) context.Context {

	ctx := c.UserContext()
	if ctx.Value(consts.TraceKey) == nil {
		return context.WithValue(context.Background(), consts.TraceKey, uuid.NewV4().String())
	}
	return ctx
}

package fiber_handle

import (
	"context"
	"strings"
	"xiaozhizhang/pkg/core/consts"
	"xiaozhizhang/pkg/core/tracer"

	"github.com/gofiber/fiber/v2"
)

type TracerConfig struct {
	Tracer  tracer.Tracer
	AppName string
}

func NewApiTracer(config TracerConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		url := strings.SplitN(c.OriginalURL(), "?", 2)[0]

		ctx := c.UserContext()

		ctx, traceID, finish := config.Tracer.StartTrace(ctx, url[1:])
		defer finish()

		ctx = context.WithValue(ctx, consts.TraceKey, traceID)
		c.SetUserContext(ctx)
		c.Locals(consts.TraceKey, traceID)
		return c.Next()
	}
}

func NewInternalTracer(config TracerConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		url := strings.SplitN(c.OriginalURL(), "?", 2)[0]
		ctx := c.UserContext()

		parentTrace := c.Get(consts.TraceHeaderName)
		traceID := ""
		finish := func() {}
		if len(parentTrace) == 0 {
			ctx, traceID, finish = config.Tracer.StartTrace(ctx, url[1:])
		} else {
			var err error
			ctx, traceID, finish, err = config.Tracer.StartTraceWithParent(ctx, url[1:], parentTrace)
			if err != nil {
				// 如果解析父追踪上下文失败，创建新的追踪
				ctx, traceID, finish = config.Tracer.StartTrace(ctx, url[1:])
			}
		}
		defer finish()

		c.SetUserContext(ctx)
		c.Locals(consts.TraceKey, traceID)
		return c.Next()
	}
}

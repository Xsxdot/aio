package logger

import (
	"fmt"
	"time"

	errorc "xiaozhizhang/pkg/core/err"

	"github.com/gofiber/fiber/v2"
)

type AdminConfig struct {
	Logger *Log
}

// NewAdminLogger creates a new middleware handler
func NewAdminLogger(config AdminConfig) fiber.Handler {
	// Set variables
	var (
		start time.Time
		log   = config.Logger.WithField("EntryName", "API")
	)

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		start = time.Now()
		// Handle request, store err for logging
		err = c.Next()

		cLog := log.WithField("status", c.Response().StatusCode()).
			WithField("latency", time.Now().Sub(start).Round(time.Millisecond)).
			WithField("method", c.Method()).
			WithField("path", c.OriginalURL()).
			WithField("user_id", c.Locals("user_id")).
			WithField("operator", c.Locals("operator")).
			WithField("TraceId", c.Locals("traceId")).
			WithField("req", string(c.Request().Body()))

		if c.Method() != fiber.MethodGet {
			cLog = cLog.WithField("resp", string(c.Response().Body()))
		}

		if err != nil {
			errc := errorc.ParseError(err)
			errc.ToLog(log.WithTrace(c.UserContext()).GetLogger())
			cLog = cLog.WithField("Err", errc.RootCause())
			fmt.Println(errc.Error())
		}

		cLog.Debug("请求处理完毕")

		return err
	}
}

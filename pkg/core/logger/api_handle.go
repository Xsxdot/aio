package logger

import (
	"fmt"
	"strings"
	errorc "xiaozhizhang/pkg/core/err"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	Logger *Log
}

// NewApiLogger creates a new middleware handler
func NewApiLogger(config Config) fiber.Handler {
	// Set variables
	var (
		start, stop time.Time
		log         = config.Logger.WithField("EntryName", "API")
	)

	// Return new handler
	return func(c *fiber.Ctx) (err error) {
		url := c.OriginalURL()
		url = strings.SplitN(url, "?", 2)[0]

		start = time.Now()

		// Handle request, store err for logging
		err = c.Next()

		stop = time.Now()

		log.WithField("status", c.Response().StatusCode()).
			WithField("latency", stop.Sub(start).Round(time.Millisecond)).
			WithField("method", c.Method()).
			WithField("path", url).
			WithField("TraceId", c.Locals("traceId")).
			WithField("userId", c.Locals("user_id")).
			Debug("请求处理完毕")

		if err != nil {
			errc := errorc.ParseError(err)
			errc.ToLog(log.WithTrace(c.UserContext()).GetLogger())
			log = log.WithField("Err", errc.RootCause())
			fmt.Println(errc.Error())
		}

		return err
	}
}

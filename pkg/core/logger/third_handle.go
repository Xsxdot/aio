package logger

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

type ThirdConfig struct {
	Logger *logrus.Entry
}

// NewThirdLogger creates a new middleware handler
func NewThirdLogger(config ThirdConfig) fiber.Handler {
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
		_ = c.Next()

		stop = time.Now()

		duration := stop.Sub(start).Round(time.Millisecond)

		log.WithField("status", c.Response().StatusCode()).
			WithField("latency", duration).
			WithField("method", c.Method()).
			WithField("path", url).
			WithField("req", c.Request().String()).
			WithField("resp", c.Response().String()).
			Debug("请求处理完毕")

		return nil
	}
}

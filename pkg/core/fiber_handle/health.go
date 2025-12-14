package fiber_handle

import "github.com/gofiber/fiber/v2"

type HealthCheckConfig struct {
	Path string
}

func HealthCheck(config HealthCheckConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		url := c.OriginalURL()
		if url == config.Path {
			return c.Status(200).SendString("")
		}
		return c.Next()
	}
}

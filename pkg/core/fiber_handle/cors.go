package fiber_handle

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func Cors() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "*",
		AllowHeaders: "*",
		//AllowCredentials: true,
		ExposeHeaders: "Authorization,Link,X-Total-Count",
		MaxAge:        1800,
	})
}

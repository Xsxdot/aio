package app

import (
	"strings"
	"time"

	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/start"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func GetApp() *fiber.App {
	app := start.GetApp()

	RegisterStaticFiles(app, "./web", "/")

	return app
}

// RegisterStaticFiles 配置静态文件服务，用于提供Vue打包的前端页面
func RegisterStaticFiles(app *fiber.App, staticPath string, prefixPath string) {
	if staticPath == "" {
		return
	}

	// 注册静态文件目录
	app.Static(prefixPath, staticPath, fiber.Static{
		Compress:      true,             // 启用压缩
		ByteRange:     true,             // 启用字节范围请求
		Browse:        false,            // 禁止目录浏览
		Index:         "index.html",     // 默认索引文件
		CacheDuration: 10 * time.Minute, // 设置缓存时间（例如10天）
	})

	// 处理SPA路由 - 将所有不匹配的请求重定向到index.html
	// 注意：这应该在API路由配置之后添加
	app.Get("*", func(c *fiber.Ctx) error {
		// 检查请求的路径是否以API路径开头，如果是API请求则跳过
		path := c.Path()
		if strings.HasPrefix(path, "/api") {
			return c.Next()
		}
		if strings.HasPrefix(path, "/admin") {
			return c.Next()
		}
		if strings.HasPrefix(path, "/third") || strings.HasPrefix(path, "/internal") || strings.HasPrefix(path, "/gateway") {
			return c.Next()
		}
		if strings.HasPrefix(path, "/health") {
			return c.Next()
		}

		// 静态资源文件检查（不重定向常见的静态资源请求）
		if len(path) > 0 {
			ext := getFileExtension(path)
			if ext == ".js" || ext == ".css" || ext == ".png" || ext == ".jpg" ||
				ext == ".jpeg" || ext == ".gif" || ext == ".svg" || ext == ".ico" ||
				ext == ".woff" || ext == ".woff2" || ext == ".ttf" || ext == ".eot" {
				return c.Next()
			}
		}

		// 重定向到index.html以支持SPA路由
		return c.SendFile(staticPath + "/index.html")
	})

	logger.GetLogger().Info("已注册静态文件服务", zap.String("路径", staticPath), zap.String("前缀", prefixPath))
}

// 辅助函数：获取文件扩展名
func getFileExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

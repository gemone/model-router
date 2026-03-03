package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/gemone/model-router/internal/config"
	"github.com/gofiber/fiber/v2"
)

// AdminAuth admin 认证中间件
func AdminAuth() fiber.Handler {
	// 在生产环境强制要求 AdminToken
	cfg := config.Get()
	env := os.Getenv("ENV")
	env = os.Getenv("GO_ENV")
	if env == "" {
		env = os.Getenv("APP_ENV")
	}

	// 检查生产环境是否配置了 AdminToken
	if (env == "production" || env == "prod") && cfg.AdminToken == "" {
		// 在生产环境，如果没有配置 AdminToken，应该拒绝请求
		// 注意：这个检查应该在应用启动时进行，但这里作为双重检查
		ErrorLog("SECURITY: AdminToken not configured in production environment!")
	}

	return func(c *fiber.Ctx) error {
		// 如果没有配置 AdminToken，跳过认证并记录警告
		if cfg.AdminToken == "" {
			// 在生产环境拒绝访问
			if env == "production" || env == "prod" {
				return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{
					"error":   "service_unavailable",
					"message": "Admin authentication not configured. Please set ADMIN_TOKEN environment variable.",
				})
			}
			// 开发环境记录警告
			WarnLog("AdminToken not configured - skipping authentication (OK for development)")
			return c.Next()
		}

		token := c.Get("Authorization")
		if token == "" {
			token = c.Query("token")
		}

		// 移除 "Bearer " 前缀
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// 使用常量时间比较防止时序攻击
		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AdminToken)) != 1 {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error":   "unauthorized",
				"message": "invalid or missing admin token",
			})
		}

		return c.Next()
	}
}

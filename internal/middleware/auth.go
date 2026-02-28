package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gemone/model-router/internal/config"
	"github.com/gofiber/fiber/v2"
)

// AdminAuth admin 认证中间件
func AdminAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		cfg := config.Get()

		// 如果没有配置 AdminToken，跳过认证
		if cfg.AdminToken == "" {
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

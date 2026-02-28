package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gemone/model-router/internal/config"
	"github.com/gin-gonic/gin"
)

// AdminAuth admin 认证中间件
func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()

		// 如果没有配置 AdminToken，跳过认证
		if cfg.AdminToken == "" {
			c.Next()
			return
		}

		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}

		// 移除 "Bearer " 前缀
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// 使用常量时间比较防止时序攻击
		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AdminToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
				"message": "invalid or missing admin token",
			})
			return
		}

		c.Next()
	}
}

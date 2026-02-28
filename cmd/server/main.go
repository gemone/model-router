package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/handler"
	"github.com/gemone/model-router/internal/service"
	"github.com/gemone/model-router/internal/utils"
	"github.com/gemone/model-router/internal/web"
	"github.com/gin-gonic/gin"

	// Import adapters to register them
	_ "github.com/gemone/model-router/internal/adapter/anthropic"
	_ "github.com/gemone/model-router/internal/adapter/deepseek"
	_ "github.com/gemone/model-router/internal/adapter/ollama"
	_ "github.com/gemone/model-router/internal/adapter/openai"
	_ "github.com/gemone/model-router/internal/adapter/openai_compatible"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化加密
	utils.InitEncryptionKey(cfg.JWTSecret)

	// 初始化数据库
	if err := database.Init(cfg.DBPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// 初始化服务
	service.GetStatsCollector()
	service.GetProfileManager()

	// 设置 Gin 模式
	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS
	if cfg.EnableCORS {
		r.Use(corsMiddleware())
	}

	// 日志
	r.Use(requestLogger())

	// Web UI - 放在 API 路由之前
	if fs := web.FS(); fs != nil {
		// 嵌入模式：服务静态文件
		// 使用 NoRoute 来处理前端路由，避免与 API 路由冲突
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			// API 请求返回 404
			if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			// 静态文件服务
			http.FileServer(fs).ServeHTTP(c.Writer, c.Request)
		})
	}

	// Admin API
	adminHandler := handler.NewAdminHandler()
	admin := r.Group("/api/admin")
	adminHandler.RegisterRoutes(admin)

	// API 路由（包括 OpenAI 兼容接口）
	apiHandler := handler.NewAPIHandler()
	apiHandler.RegisterRoutes(r)

	// 启动服务
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("Model Router starting on %s", addr)
	log.Printf("API Endpoint: http://%s/api/{profile}/v1/chat/completions", addr)
	log.Printf("Admin UI: http://%s/", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// CORS 中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// 请求日志中间件
func requestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s\n",
			param.TimeStamp.Format("2006-01-02 15:04:05"),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
		)
	})
}

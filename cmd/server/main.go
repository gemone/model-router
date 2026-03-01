package main

import (
	"fmt"
	"log"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/handler"
	"github.com/gemone/model-router/internal/service"
	"github.com/gemone/model-router/internal/utils"
	"github.com/gemone/model-router/internal/web"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	// Import adapters to register them
	_ "github.com/gemone/model-router/internal/adapter"
	_ "github.com/gemone/model-router/internal/adapter/anthropic"
	_ "github.com/gemone/model-router/internal/adapter/deepseek"
	_ "github.com/gemone/model-router/internal/adapter/ollama"
	_ "github.com/gemone/model-router/internal/adapter/openai_compatible"
)

func main() {
	// Load .env file (optional - ignore errors if file doesn't exist)
	_ = godotenv.Load()

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

	// 创建 Fiber 应用
	app := fiber.New(fiber.Config{
		AppName:      "Model Router",
		ErrorHandler: customErrorHandler,
	})

	// 全局中间件
	app.Use(recover.New())
	app.Use(logger.New())

	// CORS
	if cfg.EnableCORS {
		app.Use(cors.New())
	}

	// Web UI - 静态文件服务
	if fs := web.FS(); fs != nil {
		app.Use(func(c *fiber.Ctx) error {
			path := c.Path()
			// API 请求继续处理，不服务静态文件
			if len(path) >= 5 && (path[:5] == "/api/" || path[:4] == "/v1/") {
				return c.Next()
			}
			// 其他请求尝试服务静态文件
			return c.SendFile(c.Path())
		})
	}

	// Admin API
	adminHandler := handler.NewAdminHandler()
	compressionAdminHandler := handler.NewCompressionAdminHandler()
	admin := app.Group("/api/admin")
	adminHandler.RegisterRoutes(admin)
	compressionAdminHandler.RegisterRoutes(admin)

	// API 路由（包括 OpenAI 兼容接口）
	apiHandler := handler.NewAPIHandler()
	apiHandler.RegisterRoutes(app)

	// 启动服务
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("Model Router starting on %s", addr)
	log.Printf("API Endpoint: http://%s/api/{profile}/v1/chat/completions", addr)
	log.Printf("Admin UI: http://%s/", addr)

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// customErrorHandler 自定义错误处理
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{
		"error": err.Error(),
	})
}

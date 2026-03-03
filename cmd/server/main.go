package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/handler"
	"github.com/gemone/model-router/internal/middleware"
	"github.com/gemone/model-router/internal/service"
	"github.com/gemone/model-router/internal/template"
	"github.com/gemone/model-router/internal/utils"
	"github.com/gemone/model-router/internal/web"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
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

	// 初始化日志级别（在加载配置后立即调用）
	middleware.InitLogLevel()

	// 初始化加密
	if err := utils.InitEncryptionKey(cfg.EncryptionKey); err != nil {
		log.Printf("ERROR: Failed to initialize encryption: %v", err)
		if cfg.EncryptionKey == "" {
			log.Printf("\nENCRYPTION_KEY environment variable is required for secure operation.")
			log.Printf("\nGenerate a secure key with one of these methods:")
			log.Printf("  openssl rand -base64 32")
			log.Printf("  OR")
			log.Printf("  ENCRYPTION_KEY=$(openssl rand -base64 32)")
			log.Printf("\nThen set it in your environment or .env file:")
			log.Printf("  ENCRYPTION_KEY=<your-generated-key>")
		}
		os.Exit(1)
	}

	// 警告：如果没有设置 AdminToken，打印安全警告
	if cfg.AdminToken == "" {
		log.Printf("\n[WARNING] ADMIN_TOKEN is not set!")
		log.Printf("[WARNING] Admin endpoints will be accessible without authentication!")
		log.Printf("[WARNING] This is a security risk and should NOT be used in production.")
		log.Printf("[WARNING] Set ADMIN_TOKEN environment variable to secure admin endpoints.")
		log.Printf("[WARNING] Example: ADMIN_TOKEN=$(openssl rand -base64 32)\n")
	}

	// 初始化数据库
	if err := database.Init(cfg.DBPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// 初始化服务
	service.GetStatsCollector()
	service.GetProfileManager()

	// 初始化默认模板
	templateService := template.NewService(database.GetDB())
	if err := templateService.InitDefaultTemplates(); err != nil {
		log.Printf("Warning: failed to initialize default templates: %v", err)
	} else {
		log.Println("Default templates initialized successfully")
	}

	// 创建 Fiber 应用
	app := fiber.New(fiber.Config{
		AppName:      "Model Router",
		ErrorHandler: customErrorHandler,
	})

	// 全局中间件
	app.Use(recover.New())
	app.Use(middleware.Logger())

	// Security headers middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Only set HSTS if HTTPS is enabled
		if cfg.EnableHTTPS {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		return c.Next()
	})

	// CORS with proper origin restrictions
	if cfg.EnableCORS {
		// Parse allowed origins from config and trim whitespace
		origins := strings.Split(cfg.AllowedOrigins, ",")
		trimmedOrigins := make([]string, 0, len(origins))
		for _, origin := range origins {
			trimmed := strings.TrimSpace(origin)
			if trimmed != "" {
				trimmedOrigins = append(trimmedOrigins, trimmed)
			}
		}

		app.Use(cors.New(cors.Config{
			AllowOrigins:     strings.Join(trimmedOrigins, ","),
			AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
			AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}

	// Web UI - 静态文件服务 (必须在 API 路由之前)
	wfs, err := web.FS()
	if err != nil {
		log.Printf("Warning: failed to load embedded web UI: %v", err)
		log.Printf("Web UI will not be available, but API endpoints will work normally")
	} else if wfs != nil {
		app.Use(func(c *fiber.Ctx) error {
			path := c.Path()
			// API 请求继续处理，不服务静态文件
			if len(path) >= 5 && (path[:5] == "/api/" || path[:4] == "/v1/") {
				return c.Next()
			}
			// 根路径返回 index.html
			indexPath := "index.html"
			if path == "/" {
				path = indexPath
			} else {
				// 移除前导斜杠
				path = path[1:]
			}
			// 从嵌入的文件系统读取
			file, err := wfs.Open(path)
			if err != nil {
				// 文件不存在，返回 index.html (SPA 路由支持)
				file, err = wfs.Open(indexPath)
				if err != nil {
					// 最后的fallback，让下一个处理器处理
					return c.Next()
				}
			}
			defer file.Close()
			// 读取文件内容
			content, err := io.ReadAll(file)
			if err != nil {
				return c.SendStatus(500)
			}
			// 设置正确的 Content-Type
			if len(path) >= 5 && path[len(path)-5:] == ".html" {
				c.Set("Content-Type", "text/html")
			} else if len(path) >= 3 && path[len(path)-3:] == ".js" {
				c.Set("Content-Type", "application/javascript")
			} else if len(path) >= 4 && path[len(path)-4:] == ".css" {
				c.Set("Content-Type", "text/css")
			}
			return c.Send(content)
		})
	}

	// Admin API
	adminHandler := handler.NewAdminHandler()
	compressionAdminHandler := handler.NewCompressionAdminHandler()
	// Add rate limiting to admin endpoints to prevent brute force attacks
	admin := app.Group("/api/admin",
		middleware.AdminAuth(),
		limiter.New(limiter.Config{
			Max:        100,
			Expiration: 1 * 60 * 1000, // 1 minute in milliseconds
			KeyGenerator: func(c *fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(429).JSON(fiber.Map{
					"error": "rate limit exceeded",
				})
			},
		}),
	)
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

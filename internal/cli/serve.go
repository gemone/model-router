package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gemone/model-router/internal/browser"
	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/handler"
	"github.com/gemone/model-router/internal/middleware"
	"github.com/gemone/model-router/internal/service"
	"github.com/gemone/model-router/internal/template"
	"github.com/gemone/model-router/internal/utils"
	"github.com/gemone/model-router/internal/web"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	// Import adapters to register them
	_ "github.com/gemone/model-router/internal/adapter"
	_ "github.com/gemone/model-router/internal/adapter/anthropic"
	_ "github.com/gemone/model-router/internal/adapter/deepseek"
	_ "github.com/gemone/model-router/internal/adapter/ollama"
	_ "github.com/gemone/model-router/internal/adapter/openai_compatible"
)

var openUI bool

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	Long: `Start the model-router API server.

The server provides:
- OpenAI-compatible API endpoints
- Admin UI for managing providers, models, and profiles
- REST API for configuration

Examples:
  # Start server with default settings
  model-router serve

  # Start with custom port
  model-router serve --port 9000

  # Start and open web UI
  model-router serve --open-ui

  # Use custom config file
  model-router serve --config /path/to/config.json`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Server flags
	serveCmd.Flags().IntP("port", "p", 0, "Server port (env: MODEL_ROUTER_PORT, default: 8080)")
	serveCmd.Flags().String("host", "", "Server host (env: MODEL_ROUTER_HOST, default: 0.0.0.0)")
	serveCmd.Flags().String("db-path", "", "Database path (env: MODEL_ROUTER_DB_PATH)")
	serveCmd.Flags().BoolVar(&openUI, "open-ui", false, "Open web UI in browser on start")
	serveCmd.Flags().String("admin-token", "", "Admin authentication token (env: MODEL_ROUTER_ADMIN_TOKEN)")
	serveCmd.Flags().String("jwt-secret", "", "JWT signing secret (env: MODEL_ROUTER_JWT_SECRET)")
	serveCmd.Flags().String("encryption-key", "", "Data encryption key (env: MODEL_ROUTER_ENCRYPTION_KEY)")
	serveCmd.Flags().String("log-level", "", "Log level: debug/info/warn/error (env: MODEL_ROUTER_LOG_LEVEL)")
	serveCmd.Flags().Int("read-timeout", 0, "Read timeout in seconds (env: MODEL_ROUTER_READ_TIMEOUT)")
	serveCmd.Flags().Int("write-timeout", 0, "Write timeout in seconds (env: MODEL_ROUTER_WRITE_TIMEOUT)")

	// Bind flags to viper
	_ = viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("server.host", serveCmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("database.path", serveCmd.Flags().Lookup("db-path"))
	_ = viper.BindPFlag("security.admin_token", serveCmd.Flags().Lookup("admin-token"))
	_ = viper.BindPFlag("security.jwt_secret", serveCmd.Flags().Lookup("jwt-secret"))
	_ = viper.BindPFlag("security.encryption_key", serveCmd.Flags().Lookup("encryption-key"))
	_ = viper.BindPFlag("logging.level", serveCmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("server.read_timeout", serveCmd.Flags().Lookup("read-timeout"))
	_ = viper.BindPFlag("server.write_timeout", serveCmd.Flags().Lookup("write-timeout"))
}

func runServe(cmd *cobra.Command, args []string) error {
	// Get config (already initialized by PersistentPreRun)
	cfg := config.GetConfig()

	// Initialize logging
	middleware.InitLogLevel()

	// Initialize encryption (optional - skip if not configured)
	encryptionKey := cfg.Security.EncryptionKey
	if encryptionKey != "" {
		if err := utils.InitEncryptionKey(encryptionKey); err != nil {
			return fmt.Errorf("failed to initialize encryption: %w", err)
		}
		log.Println("Encryption enabled - API keys will be encrypted")
	}
	// No encryption key = API keys stored as plaintext (silent)

	// Admin token check
	if cfg.Security.AdminToken == "" {
		log.Println("WARNING: ADMIN_TOKEN is not set - admin endpoints will be accessible without authentication!")
		log.Println("Set ADMIN_TOKEN environment variable to secure admin endpoints")
	} else {
		log.Println("Admin authentication enabled")
	}

	// Initialize database
	dbPath := cfg.GetEffectiveDBPath()
	if err := config.EnsureDataDir(); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := database.Init(dbPath); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.Close()

	// Initialize services
	statsCollector := service.GetStatsCollector()
	service.GetProfileManager()

	// Initialize default templates
	templateService := template.NewService(database.GetDB())
	if err := templateService.InitDefaultTemplates(); err != nil {
		log.Printf("Warning: failed to initialize default templates: %v", err)
	} else {
		log.Println("Default templates initialized successfully")
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "Model Router",
		ErrorHandler: customErrorHandler,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(middleware.Logger())

	// Security headers middleware
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if cfg.CORS.EnableHTTPS {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		return c.Next()
	})

	// CORS
	if cfg.CORS.Enabled {
		origins := cfg.CORS.AllowedOrigins
		app.Use(cors.New(cors.Config{
			AllowOrigins:     origins,
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}

	// Web UI - Static file serving
	wfs, err := web.FS()
	if err != nil {
		log.Printf("Warning: failed to load embedded web UI: %v", err)
		log.Printf("Web UI will not be available, but API endpoints will work normally")
	} else if wfs != nil {
		app.Use(func(c fiber.Ctx) error {
			path := c.Path()
			if len(path) >= 5 && (path[:5] == "/api/" || path[:4] == "/v1/") {
				return c.Next()
			}
			indexPath := "index.html"
			if path == "/" {
				path = indexPath
			} else {
				path = path[1:]
			}
			file, err := wfs.Open(path)
			if err != nil {
				file, err = wfs.Open(indexPath)
				if err != nil {
					return c.Next()
				}
			}
			defer file.Close()
			// Set content type and caching headers based on file extension
			switch {
			case len(path) >= 5 && path[len(path)-5:] == ".html":
				c.Set("Content-Type", "text/html")
				c.Set("Cache-Control", "no-cache")
			case len(path) >= 3 && path[len(path)-3:] == ".js":
				c.Set("Content-Type", "application/javascript")
				c.Set("Cache-Control", "public, max-age=31536000") // 1 year
			case len(path) >= 4 && path[len(path)-4:] == ".css":
				c.Set("Content-Type", "text/css")
				c.Set("Cache-Control", "public, max-age=31536000") // 1 year
			default:
				c.Set("Cache-Control", "public, max-age=86400") // 1 day
			}
			// Stream file directly to response instead of loading into memory
			return c.SendStream(file)
		})
	}

	// Admin API - Authentication endpoints (public, stricter rate limiting)
	adminHandler := handler.NewAdminHandler()

	// Separate rate limiter for auth endpoints (login, logout, auth/status)
	// More restrictive to prevent brute force attacks: 10 attempts per minute
	authLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * 60 * 1000,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP() + ":auth"
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"error":   "rate_limit_exceeded",
				"message": "Too many authentication attempts. Please try again later.",
			})
		},
	})

	// Register auth endpoints with stricter rate limiting
	authGroup := app.Group("/api/admin", authLimiter)
	authGroup.Post("/login", adminHandler.Login)
	authGroup.Post("/logout", adminHandler.Logout)
	authGroup.Get("/auth/status", adminHandler.GetAuthStatus)

	// Protected admin endpoints (require authentication, standard rate limiting)
	compressionAdminHandler := handler.NewCompressionAdminHandler()
	
	// Standard rate limiter for most admin endpoints: 300 req/min
	standardLimiter := limiter.New(limiter.Config{
		Max:        300,
		Expiration: 1 * 60 * 1000,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "rate limit exceeded"})
		},
	})
	
	// Relaxed rate limiter for test endpoints: 600 req/min
	testLimiter := limiter.New(limiter.Config{
		Max:        600,
		Expiration: 1 * 60 * 1000,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "rate limit exceeded"})
		},
	})
	
	// Main admin group with standard limiting (for all other endpoints)
	admin := app.Group("/api/admin", middleware.AdminAuth(), standardLimiter)

	// Register all admin routes
	adminHandler.RegisterRoutes(admin)
	compressionAdminHandler.RegisterRoutes(admin)
	
	// Test endpoint with relaxed rate limiting (re-register to override)
	app.Post("/api/admin/test", middleware.AdminAuth(), testLimiter, adminHandler.TestModel)

	// Register rule admin routes
	ruleAdminHandler := handler.NewRuleAdminHandler()
	ruleAdminHandler.RegisterRoutes(app)

	// API routes - Use Enhanced API Handler for full format support
	enhancedAPIHandler := handler.NewEnhancedAPIHandler()
	enhancedAPIHandler.RegisterEnhancedRoutes(app)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Model Router starting on %s", addr)
	log.Printf("API Endpoint: http://%s/api/{profile}/v1/chat/completions", addr)
	log.Printf("Admin UI: http://%s/", addr)

	// Open UI if requested
	if openUI || cfg.UI.AutoOpen {
		uiURL := fmt.Sprintf("http://%s/", addr)
		go func() {
			time.Sleep(500 * time.Millisecond) // Wait for server to start
			if err := browser.OpenURL(uiURL); err != nil {
				log.Printf("Failed to open browser: %v", err)
			}
		}()
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		
		// Stop background services
		statsCollector.Stop()
		
		if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Start listening
	if err := app.Listen(addr); err != nil {
		if err.Error() != "Server closed" {
			return fmt.Errorf("failed to start server: %w", err)
		}
	}

	return nil
}

// customErrorHandler handles errors globally
func customErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}

// RunServer is the main entry point for starting the server
// This is kept for backward compatibility with existing code
func RunServer(ctx context.Context) error {
	return runServe(nil, nil)
}

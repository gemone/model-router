package config

import (
	"os"
	"strconv"
)

// Config 应用配置
type Config struct {
	// Server
	Port         int
	Host         string
	ReadTimeout  int
	WriteTimeout int

	// Database
	DBPath string

	// Security
	AdminToken     string
	JWTSecret      string
	EncryptionKey  string
	EnableCORS     bool
	AllowedOrigins string // Comma-separated list of allowed CORS origins
	EnableHTTPS    bool   // Whether to enable HSTS headers

	// Features
	EnableStats    bool
	EnableFallback bool
	MaxRetries     int

	// Logging
	LogLevel      string
	LogBufferSize int // Maximum number of log entries to keep in memory
}

var cfg *Config

// Load 加载配置
func Load() *Config {
	if cfg != nil {
		return cfg
	}

	cfg = &Config{
		Port:           getEnvInt("PORT", 8080),
		Host:           getEnv("HOST", "0.0.0.0"),
		ReadTimeout:    getEnvInt("READ_TIMEOUT", 30),
		WriteTimeout:   getEnvInt("WRITE_TIMEOUT", 60),
		DBPath:         getEnv("DB_PATH", ""),
		AdminToken:     getEnv("ADMIN_TOKEN", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		EncryptionKey:  getEnv("ENCRYPTION_KEY", ""),
		EnableCORS:     getEnvBool("ENABLE_CORS", true),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173"),
		EnableHTTPS:    getEnvBool("ENABLE_HTTPS", false),
		EnableStats:    getEnvBool("ENABLE_STATS", true),
		EnableFallback: getEnvBool("ENABLE_FALLBACK", true),
		MaxRetries:     getEnvInt("MAX_RETRIES", 3),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		LogBufferSize:  clamp(getEnvInt("LOG_BUFFER_SIZE", 1000), 100, 10000),
	}

	return cfg
}

// Get 获取配置
func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// clamp returns a value clamped between min and max (inclusive)
func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

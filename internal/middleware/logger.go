package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/config"
	"github.com/gofiber/fiber/v3"
)

// LogLevel 日志级别
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	currentLogLevel LogLevel = LevelInfo
	logLevelMutex   sync.RWMutex

	// 内存日志缓冲区（用于前端实时查看）
	logBuffer    []string
	logBufferMux sync.RWMutex
	maxLogBuffer = 1000 // 最大保留 1000 条日志
)

// logLevelMap 日志级别映射
var logLevelMap = map[string]LogLevel{
	"debug": LevelDebug,
	"info":  LevelInfo,
	"warn":  LevelWarn,
	"error": LevelError,
}

// InitLogLevel 初始化日志级别和缓冲区大小
func InitLogLevel() {
	cfg := config.Get()
	SetLogLevel(cfg.GetLogLevel())

	// Update log buffer size from config
	if cfg.GetLogBufferSize() > 0 {
		logBufferMux.Lock()
		maxLogBuffer = cfg.GetLogBufferSize()
		logBufferMux.Unlock()
	}
}

// SetLogLevel 设置日志级别
func SetLogLevel(level string) {
	logLevelMutex.Lock()
	defer logLevelMutex.Unlock()

	if lvl, ok := logLevelMap[strings.ToLower(level)]; ok {
		currentLogLevel = lvl
	} else {
		currentLogLevel = LevelInfo
	}
}

// GetLogLevel 获取当前日志级别
func GetLogLevel() LogLevel {
	logLevelMutex.RLock()
	defer logLevelMutex.RUnlock()
	return currentLogLevel
}

// GetLogLevelString 获取当前日志级别字符串
func GetLogLevelString() string {
	logLevelMutex.RLock()
	defer logLevelMutex.RUnlock()

	switch currentLogLevel {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

// shouldLog 检查是否应该记录该级别
func shouldLog(level LogLevel) bool {
	logLevelMutex.RLock()
	defer logLevelMutex.RUnlock()
	return level >= currentLogLevel
}

// addToBuffer 添加日志到缓冲区
func addToBuffer(log string) {
	logBufferMux.Lock()
	defer logBufferMux.Unlock()

	logBuffer = append(logBuffer, log)
	if len(logBuffer) > maxLogBuffer {
		logBuffer = logBuffer[len(logBuffer)-maxLogBuffer:]
	}

	// 同时添加到结构化存储
	AddRawLog(log)
}

// GetRecentLogs 获取最近的日志
func GetRecentLogs(limit int) []string {
	logBufferMux.RLock()
	defer logBufferMux.RUnlock()

	if limit <= 0 || limit > len(logBuffer) {
		limit = len(logBuffer)
	}

	result := make([]string, limit)
	start := len(logBuffer) - limit
	for i := 0; i < limit; i++ {
		result[i] = logBuffer[start+i]
	}
	return result
}

// ClearBuffer 清空日志缓冲区
func ClearBuffer() {
	logBufferMux.Lock()
	defer logBufferMux.Unlock()
	logBuffer = []string{}
}

// Logger 日志中间件
func Logger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// 获取请求体（如果是 debug 级别）
		var requestBody []byte
		if shouldLog(LevelDebug) && c.Method() != "GET" && c.Method() != "HEAD" {
			requestBody = c.Body()
			// 重新设置 body，因为 fiber 读取后 body 会被消费
			c.Request().SetBody(requestBody)
		}

		// 继续处理请求
		err := c.Next()

		// 记录日志
		duration := time.Since(start)
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		ip := c.IP()
		userAgent := c.Get("User-Agent")

		// 根据状态码选择日志级别
		var level LogLevel
		switch {
		case status >= 500:
			level = LevelError
		case status >= 400:
			level = LevelWarn
		default:
			level = LevelInfo
		}

		// 构建基础日志消息
		msg := fmt.Sprintf("[%s] %s %s %d %s - %s",
			time.Now().Format("2006-01-02 15:04:05"),
			method,
			path,
			status,
			duration,
			ip,
		)

		// 根据级别输出日志
		if shouldLog(level) {
			var logLine string
			switch level {
			case LevelError:
				logLine = fmt.Sprintf("[ERROR] %s", msg)
				fmt.Printf("%s\n", logLine)
			case LevelWarn:
				logLine = fmt.Sprintf("[WARN]  %s", msg)
				fmt.Printf("%s\n", logLine)
			default:
				logLine = fmt.Sprintf("[INFO]  %s", msg)
				fmt.Printf("%s\n", logLine)
			}

			// 添加到缓冲区
			addToBuffer(logLine)

			// Debug 模式下打印详细信息
			if shouldLog(LevelDebug) {
				debugInfo := getDebugInfo(c, requestBody, userAgent, duration)
				fmt.Print(debugInfo)
				addToBuffer(debugInfo)
			}
		}

		return err
	}
}

// getDebugInfo 获取调试信息字符串
func getDebugInfo(c fiber.Ctx, requestBody []byte, userAgent string, duration time.Duration) string {
	var sb strings.Builder

	// 打印请求头
	sb.WriteString("[DEBUG] Request Headers:\n")
	c.Request().Header.VisitAll(func(key, value []byte) {
		// 隐藏敏感信息
		k := string(key)
		if shouldRedactHeader(k) {
			sb.WriteString(fmt.Sprintf("  %s: ***hidden***\n", k))
		} else {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, string(value)))
		}
	})

	// 打印请求体
	if len(requestBody) > 0 {
		sb.WriteString(fmt.Sprintf("[DEBUG] Request Body (%d bytes):\n", len(requestBody)))
		// 限制请求体大小，避免日志过大
		if len(requestBody) > 10000 {
			sanitized := sanitizeJSON(requestBody[:10000])
			sb.WriteString(fmt.Sprintf("  %s... (truncated)\n", sanitized))
		} else {
			sb.WriteString(sanitizeJSON(requestBody))
		}
	}

	// 打印响应体（如果是 JSON）
	contentType := string(c.Response().Header.ContentType())
	if strings.Contains(contentType, "application/json") {
		responseBody := c.Response().Body()
		if len(responseBody) > 0 {
			sb.WriteString(fmt.Sprintf("[DEBUG] Response Body (%d bytes):\n", len(responseBody)))
			// 限制响应体大小，避免日志过大
			if len(responseBody) > 10000 {
				sanitized := sanitizeJSON(responseBody[:10000])
				sb.WriteString(fmt.Sprintf("  %s... (truncated)\n", sanitized))
			} else {
				sb.WriteString(sanitizeJSON(responseBody))
			}
		}
	}

	// 打印 User-Agent
	if userAgent != "" {
		sb.WriteString(fmt.Sprintf("[DEBUG] User-Agent: %s\n", userAgent))
	}

	sb.WriteString(fmt.Sprintf("[DEBUG] Duration: %v\n", duration))
	sb.WriteString(strings.Repeat("-", 80))
	sb.WriteString("\n")

	return sb.String()
}

// prettyFormatJSON 格式化 JSON 并返回字符串
func prettyFormatJSON(data []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "  ", "  "); err == nil {
		// 限制输出大小
		output := buf.String()
		if len(output) > 5000 {
			return fmt.Sprintf("  %s... (truncated)\n", output[:5000])
		}
		return fmt.Sprintf("  %s\n", output)
	}
	// 不是有效的 JSON，直接打印
	if len(data) > 5000 {
		return fmt.Sprintf("  %s... (truncated)\n", string(data[:5000]))
	}
	return fmt.Sprintf("  %s\n", string(data))
}

// DebugLog 打印 debug 日志
func DebugLog(format string, args ...interface{}) {
	if shouldLog(LevelDebug) {
		log := fmt.Sprintf("[DEBUG] "+format, args...)
		fmt.Println(log)
		addToBuffer(log)
	}
}

// InfoLog 打印 info 日志
func InfoLog(format string, args ...interface{}) {
	if shouldLog(LevelInfo) {
		log := fmt.Sprintf("[INFO]  "+format, args...)
		fmt.Println(log)
		addToBuffer(log)
	}
}

// WarnLog 打印 warn 日志
func WarnLog(format string, args ...interface{}) {
	if shouldLog(LevelWarn) {
		log := fmt.Sprintf("[WARN]  "+format, args...)
		fmt.Println(log)
		addToBuffer(log)
	}
}

// ErrorLog 打印 error 日志
func ErrorLog(format string, args ...interface{}) {
	if shouldLog(LevelError) {
		log := fmt.Sprintf("[ERROR] "+format, args...)
		fmt.Println(log)
		addToBuffer(log)
	}
}

// LogRequest 记录特定请求的详细信息（用于外部调用）
func LogRequest(ctx fiber.Ctx, label string, extra map[string]interface{}) {
	if !shouldLog(LevelDebug) {
		return
	}

	fmt.Printf("[DEBUG] === %s ===\n", label)
	fmt.Printf("  Method: %s\n", ctx.Method())
	fmt.Printf("  Path: %s\n", ctx.Path())

	if extra != nil {
		for k, v := range extra {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}
}

// LogAdapterRequest 记录适配器请求（用于外部适配器调用）
func LogAdapterRequest(provider, model, endpoint string, requestBody interface{}) {
	if !shouldLog(LevelDebug) {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[DEBUG] Adapter Request [%s/%s] -> %s\n", provider, model, endpoint))
	if requestBody != nil {
		if data, err := json.MarshalIndent(requestBody, "  ", "  "); err == nil {
			sb.WriteString(fmt.Sprintf("  Request: %s\n", string(data)))
		}
	}
	log := sb.String()
	fmt.Print(log)
	addToBuffer(log)
}

// LogAdapterResponse 记录适配器响应
func LogAdapterResponse(provider, model string, statusCode int, responseBody interface{}, duration time.Duration) {
	if !shouldLog(LevelDebug) {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[DEBUG] Adapter Response [%s/%s] <- %d (%v)\n", provider, model, statusCode, duration))
	if responseBody != nil {
		if data, err := json.MarshalIndent(responseBody, "  ", "  "); err == nil {
			// 限制输出大小
			if len(data) > 5000 {
				sb.WriteString(fmt.Sprintf("  Response: %s... (truncated)\n", string(data[:5000])))
			} else {
				sb.WriteString(fmt.Sprintf("  Response: %s\n", string(data)))
			}
		}
	}
	log := sb.String()
	fmt.Print(log)
	addToBuffer(log)
}

// LogWithLevel 使用指定级别记录日志
func LogWithLevel(level LogLevel, format string, args ...interface{}) {
	if !shouldLog(level) {
		return
	}

	prefix := "[INFO]  "
	switch level {
	case LevelDebug:
		prefix = "[DEBUG] "
	case LevelWarn:
		prefix = "[WARN]  "
	case LevelError:
		prefix = "[ERROR] "
	}

	fmt.Printf(prefix+format+"\n", args...)
}

// shouldRedactHeader 检查请求头是否应该被隐藏
func shouldRedactHeader(key string) bool {
	k := strings.ToLower(key)
	sensitiveHeaders := []string{
		"authorization",
		"token",
		"api-key",
		"apikey",
		"x-api-key",
		"openai-api-key",
		"anthropic-api-key",
		"bearer",
		"secret",
		"password",
		"jwt",
		"session",
		"cookie",
	}
	for _, sensitive := range sensitiveHeaders {
		if strings.Contains(k, sensitive) {
			return true
		}
	}
	return false
}

// sanitizeJSON 清理 JSON 中的敏感数据
func sanitizeJSON(data []byte) string {
	// Try to parse as JSON
	var jsonObj map[string]interface{}
	if err := json.Unmarshal(data, &jsonObj); err != nil {
		// Not valid JSON, return as-is (with length limit)
		return prettyFormatJSON(data)
	}

	// Recursively sanitize the JSON object
	sanitizeObject(jsonObj)

	// Marshal back to JSON
	result, err := json.MarshalIndent(jsonObj, "  ", "  ")
	if err != nil {
		return prettyFormatJSON(data)
	}

	// Limit output size
	output := string(result)
	if len(output) > 5000 {
		return fmt.Sprintf("  %s... (truncated)\n", output[:5000])
	}
	return fmt.Sprintf("  %s\n", output)
}

// sanitizeObject 递归清理对象中的敏感字段
func sanitizeObject(obj map[string]interface{}) {
	sensitiveKeys := []string{
		"api_key", "apikey", "api-key",
		"openai_api_key", "anthropic_api_key", "azure_api_key",
		"authorization", "bearer",
		"token", "jwt", "secret",
		"password", "pass", "passwd",
		"credentials", "credential",
		"private_key", "privatekey", "private-key",
		"access_key", "accesskey", "access-key",
		"session_token", "session_token",
		"refresh_token", "refreshtoken",
		"csrf_token", "csrftoken",
		"auth_token", "authtoken",
	}

	for key, value := range obj {
		keyLower := strings.ToLower(key)
		isSensitive := false
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(keyLower, sensitive) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			obj[key] = "***REDACTED***"
		} else if nestedObj, ok := value.(map[string]interface{}); ok {
			sanitizeObject(nestedObj)
		} else if nestedArray, ok := value.([]interface{}); ok {
			for _, item := range nestedArray {
				if itemObj, ok := item.(map[string]interface{}); ok {
					sanitizeObject(itemObj)
				}
			}
		}
	}
}

// IsDebugEnabled 检查是否启用了 debug 级别
func IsDebugEnabled() bool {
	return shouldLog(LevelDebug)
}

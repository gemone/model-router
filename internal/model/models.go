package model

import (
	"time"
)

// Profile 定义路由配置文件
type Profile struct {
	ID          string      `json:"id" gorm:"primaryKey"`
	Name        string      `json:"name" gorm:"index"`       // 显示名称
	Path        string      `json:"path" gorm:"uniqueIndex"` // URI 路径，如 "default", "claudecode"
	Description string      `json:"description"`
	Enabled     bool        `json:"enabled" gorm:"default:true"`
	Priority    int         `json:"priority" gorm:"default:0"`          // 优先级
	Models      []Model     `json:"models" gorm:"foreignKey:ProfileID"` // 关联的模型
	Rules       []RouteRule `json:"rules" gorm:"foreignKey:ProfileID"`  // 路由规则
	Settings    string      `json:"settings" gorm:"type:text"`          // JSON 格式的额外设置
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ProviderType 定义供应商类型
type ProviderType string

const (
	ProviderOpenAI           ProviderType = "openai"
	ProviderClaude           ProviderType = "claude"
	ProviderAzure            ProviderType = "azure"
	ProviderDeepSeek         ProviderType = "deepseek"
	ProviderOllama           ProviderType = "ollama"
	ProviderOpenAICompatible ProviderType = "openai-compatible"
)

// Provider 定义模型供应商配置
type Provider struct {
	ID        string       `json:"id" gorm:"primaryKey"`
	Name      string       `json:"name" gorm:"index"`
	Type      ProviderType `json:"type"`
	BaseURL   string       `json:"base_url"`
	APIKey    string       `json:"api_key" gorm:"-"` // 不存储到数据库，加密存储
	APIKeyEnc string       `json:"-" gorm:"column:api_key"`
	Models    []Model      `json:"models" gorm:"foreignKey:ProviderID"`
	Enabled   bool         `json:"enabled" gorm:"default:true"`
	Priority  int          `json:"priority" gorm:"default:0"`   // 优先级，数值越高优先级越高
	Weight    int          `json:"weight" gorm:"default:100"`   // 负载均衡权重
	RateLimit int          `json:"rate_limit" gorm:"default:0"` // 每分钟请求限制，0表示无限制
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Model 定义模型配置
type Model struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	ProfileID      string    `json:"profile_id" gorm:"index"` // 所属 Profile
	ProviderID     string    `json:"provider_id" gorm:"index"`
	Name           string    `json:"name" gorm:"index"` // 对外暴露的模型名称
	OriginalName   string    `json:"original_name"`     // 供应商原始模型名称
	Enabled        bool      `json:"enabled" gorm:"default:true"`
	SupportsFunc   bool      `json:"supports_func" gorm:"default:false"`   // 是否支持函数调用
	SupportsVision bool      `json:"supports_vision" gorm:"default:false"` // 是否支持视觉
	ContextWindow  int       `json:"context_window" gorm:"default:4096"`   // 上下文窗口大小
	MaxTokens      int       `json:"max_tokens" gorm:"default:4096"`       // 最大输出token
	InputPrice     float64   `json:"input_price" gorm:"default:0"`         // 输入价格（每1K token）
	OutputPrice    float64   `json:"output_price" gorm:"default:0"`        // 输出价格（每1K token）
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RouteStrategy 定义路由策略
type RouteStrategy string

const (
	RouteStrategyPriority RouteStrategy = "priority" // 按优先级路由
	RouteStrategyWeighted RouteStrategy = "weighted" // 加权轮询
	RouteStrategyFallback RouteStrategy = "fallback" // 故障转移
	RouteStrategyAuto     RouteStrategy = "auto"     // 自动选择（根据延迟和可用性）
)

// RouteRule 定义路由规则
type RouteRule struct {
	ID              string        `json:"id" gorm:"primaryKey"`
	ProfileID       string        `json:"profile_id" gorm:"index"` // 所属 Profile
	Name            string        `json:"name"`
	ModelPattern    string        `json:"model_pattern" gorm:"index"`           // 模型名称匹配模式，支持通配符
	TargetModels    []string      `json:"target_models" gorm:"serializer:json"` // 目标模型列表
	Strategy        RouteStrategy `json:"strategy" gorm:"default:'priority'"`
	FallbackEnabled bool          `json:"fallback_enabled" gorm:"default:true"`
	FallbackModels  []string      `json:"fallback_models" gorm:"serializer:json"` // 后备模型列表
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// RequestLog 记录API请求日志
type RequestLog struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	RequestID        string    `json:"request_id" gorm:"index"`
	Model            string    `json:"model" gorm:"index"`
	ProviderID       string    `json:"provider_id" gorm:"index"`
	Status           string    `json:"status" gorm:"index"` // success, error, timeout
	Latency          int64     `json:"latency"`             // 延迟（毫秒）
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	ErrorMessage     string    `json:"error_message"`
	ClientIP         string    `json:"client_ip"`
	CreatedAt        time.Time `json:"created_at" gorm:"index"`
}

// APIKey 定义API密钥
type APIKey struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	Name          string    `json:"name"`
	Key           string    `json:"key" gorm:"uniqueIndex"`
	Enabled       bool      `json:"enabled" gorm:"default:true"`
	RateLimit     int       `json:"rate_limit" gorm:"default:0"`
	AllowedModels []string  `json:"allowed_models" gorm:"serializer:json"` // 空数组表示允许所有模型
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Stats 定义统计数据
type Stats struct {
	ID           uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Date         string    `json:"date" gorm:"index"` // YYYY-MM-DD
	Hour         int       `json:"hour" gorm:"index"` // 0-23
	ProviderID   string    `json:"provider_id" gorm:"index"`
	Model        string    `json:"model" gorm:"index"`
	RequestCount int64     `json:"request_count"`
	SuccessCount int64     `json:"success_count"`
	ErrorCount   int64     `json:"error_count"`
	TotalTokens  int64     `json:"total_tokens"`
	AvgLatency   float64   `json:"avg_latency"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Setting 定义系统设置
type Setting struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}

// TestResult 定义模型测试结果
type TestResult struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	ProviderID string    `json:"provider_id"`
	Model      string    `json:"model"`
	Success    bool      `json:"success"`
	Latency    int64     `json:"latency"`
	Error      string    `json:"error"`
	CreatedAt  time.Time `json:"created_at"`
}

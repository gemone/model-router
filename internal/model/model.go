package model

import (
	"time"
)

// Model 定义模型配置
type Model struct {
	ID             string    `json:"id" gorm:"primaryKey"`
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
	SkipCompression bool     `json:"skip_compression" gorm:"default:false"` // 跳过压缩（如 1M+ 模型）

	// 场景路由配置 (scene 策略)
	Scene                string `json:"scene"` // 场景标签: default/background/think/longContext
	LongContextThreshold int    `json:"long_context_threshold"` // 触发长上下文的 token 阈值

	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Scene常量
const (
	SceneDefault      = "default"       // 默认场景
	SceneBackground  = "background"   // 后台任务
	SceneThink       = "think"        // 推理任务
	SceneLongContext = "longContext"  // 长上下文
)

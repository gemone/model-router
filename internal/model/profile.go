package model

import (
	"time"
)

// Profile 定义路由配置文件（接入点）
// Profile 作为 API 接入点，可以直接绑定模型或通过路由访问模型
type Profile struct {
	ID          string      `json:"id" gorm:"primaryKey"`
	Name        string      `json:"name" gorm:"index"`       // 显示名称
	Path        string      `json:"path" gorm:"uniqueIndex"` // URI 路径，如 "default", "claudecode"
	Description string      `json:"description"`
	Enabled     bool        `json:"enabled" gorm:"default:true"`
	Settings    string      `json:"settings" gorm:"type:text"` // JSON 格式的额外设置

	// 认证配置
	APIToken string `json:"api_token,omitempty" gorm:"size:255"` // Profile 专属的 API Token（可选）
	// APITokenEnc 是加密存储的 API Token，不在 API 响应中返回
	APITokenEnc string `json:"api_token_enc,omitempty" gorm:"size:512"`

	// 模型关联 - 直接绑定模型（不经过路由，直接访问）
	ModelIDs []string `json:"model_ids" gorm:"serializer:json"` // 关联的模型ID列表

	// 路由关联 - 通过路由访问模型（使用路由的权重和优先级策略）
	RouteIDs []string `json:"route_ids" gorm:"serializer:json"` // 关联的路由ID列表

	// Fallback configuration
	FallbackModels []string `json:"fallback_models" gorm:"serializer:json"` // 故障转移模型列表（按顺序）

	// 上下文压缩配置
	EnableCompression     bool   `json:"enable_compression"`     // 启用压缩
	CompressionStrategy   string `json:"compression_strategy"`   // rolling/summary/hybrid
	CompressionLevel      string `json:"compression_level"`      // session/threshold - session: 每次会话都压缩, threshold: 达到阈值才压缩
	CompressionThreshold  int    `json:"compression_threshold"`  // 触发压缩的 token 阈值（仅 threshold 模式）
	MaxContextWindow      int    `json:"max_context_window"`     // 最大上下文

	// 多模型组合配置
	EnableMultiModel bool   `json:"enable_multi_model"`  // 启用多模型
	MultiModelConfig string `json:"multi_model_config"` // JSON 配置

	// Compression model group configuration
	DefaultCompressionGroup string                  `json:"default_compression_group,omitempty" gorm:"size:255"`
	CompressionGroups        []CompressionModelGroup `json:"compression_groups,omitempty" gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CompressionLevel 常量
const (
	CompressionLevelSession   = "session"   // 每次会话都执行压缩
	CompressionLevelThreshold = "threshold" // 达到 token 阈值才压缩
)

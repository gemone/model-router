package model

import "time"

// Route 定义路由策略（后端模型管理）
// Route 作为后端模型的管理者，可以配置多个模型的权重和优先级
// 只有从路由入口的请求才使用权重和优先级进行负载均衡
type Route struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"index"`        // 路由名称（唯一标识）
	Description string    `json:"description"`              // 描述
	Enabled     bool      `json:"enabled" gorm:"default:true"`

	// 路由策略
	Strategy    RouteStrategy `json:"strategy" gorm:"default:'auto'"` // 路由策略

	// 内容类型过滤
	ContentType ContentType `json:"content_type" gorm:"default:'all'"` // 内容类型: text/image/all

	// 模型配置（存储为JSON，包含权重和优先级）
	ModelConfig string `json:"model_config" gorm:"type:text"` // JSON格式的模型配置

	// 健康检查配置
	HealthThreshold float64 `json:"health_threshold" gorm:"default:70"` // 健康阈值（0-100）
	FallbackPolicy  string  `json:"fallback_policy" gorm:"default:'next_model'"` // 降级策略: next_model/same_model/none

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ContentType 内容类型
type ContentType string

const (
	ContentTypeText  ContentType = "text"  // 仅文本
	ContentTypeImage ContentType = "image" // 图像/多模态
	ContentTypeAll   ContentType = "all"   // 所有类型（默认）
)

// RouteStrategy 路由策略
type RouteStrategy string

const (
	RouteStrategyAuto     RouteStrategy = "auto"     // 自动选择（基于健康状态和优先级）
	RouteStrategyPriority RouteStrategy = "priority" // 优先级模式（只按优先级）
	RouteStrategyWeighted RouteStrategy = "weighted" // 加权轮询（按权重分配）
	RouteStrategyRandom   RouteStrategy = "random"   // 随机选择
)

// RouteModelConfig 路由模型配置（用于JSON序列化）
type RouteModelConfig struct {
	Models []RouteModelEntry `json:"models"` // 模型列表，每个包含权重和优先级
}

// RouteModelEntry 路由中的模型配置（带权重和优先级）
type RouteModelEntry struct {
	ModelID   string  `json:"model_id"`   // 模型ID（对应 models 表的 id）
	Weight    int     `json:"weight"`    // 权重（用于加权轮询），默认 100
	Priority  int     `json:"priority"`  // 优先级（数值越高越优先），默认 0
	Enabled   bool    `json:"enabled"`   // 是否启用
}

// RouteWithModels Route with resolved model details
type RouteWithModels struct {
	Route
	Models []ModelWithWeight `json:"models"` // 展开后的模型详情
}

// ModelWithWeight Model with weight and priority from route
type ModelWithWeight struct {
	Model
	Weight   int    `json:"weight"`   // 在路由中的权重
	Priority int    `json:"priority"`  // 在路由中的优先级
	Enabled  bool   `json:"enabled"`   // 在路由中是否启用
}

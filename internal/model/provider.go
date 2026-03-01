package model

import (
	"time"
)

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
	ID           string       `json:"id" gorm:"primaryKey"`
	Name         string       `json:"name" gorm:"index"`
	Type         ProviderType `json:"type"`
	BaseURL      string       `json:"base_url"`
	APIKey       string       `json:"api_key" gorm:"-"` // 不存储到数据库，加密存储
	APIKeyEnc    string       `json:"-" gorm:"column:api_key"`
	DeploymentID string       `json:"deployment_id"` // Azure deployment name
	APIVersion   string       `json:"api_version"`   // Azure API 版本
	Models       []Model      `json:"models" gorm:"foreignKey:ProviderID"` // TODO: Remove in Phase 4 when refactoring service layer
	Enabled      bool         `json:"enabled" gorm:"default:true"`
	Priority     int          `json:"priority" gorm:"default:0"`   // 优先级，数值越高优先级越高
	Weight       int          `json:"weight" gorm:"default:100"`   // 负载均衡权重
	RateLimit    int          `json:"rate_limit" gorm:"default:0"` // 每分钟请求限制，0表示无限制
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

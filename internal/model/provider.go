package model

import (
	"time"
)

// ProviderType 定义供应商类型
type ProviderType string

const (
	ProviderOpenAI           ProviderType = "openai"
	ProviderClaude           ProviderType = "claude"
	ProviderAnthropic        ProviderType = "anthropic" // 别名，与 ProviderClaude 相同
	ProviderAzure            ProviderType = "azure"
	ProviderDeepSeek         ProviderType = "deepseek"
	ProviderOllama           ProviderType = "ollama"
	ProviderOpenAICompatible ProviderType = "openai_compatible"
)

// Provider 定义模型供应商配置
// Provider 作为纯粹的供应商配置，不包含负载均衡相关字段
type Provider struct {
	ID           string       `json:"id" gorm:"primaryKey"`
	Name         string       `json:"name" gorm:"index"`
	Type         ProviderType `json:"type"`
	BaseURL      string       `json:"base_url"`
	ChatPath     string       `json:"chat_path" gorm:"default:/v1/chat/completions"` // 自定义聊天完成路径，如 /chat/completions
	APIKey       string       `json:"api_key" gorm:"-"` // 不存储到数据库，加密存储
	APIKeyEnc    string       `json:"-" gorm:"column:api_key"`
	DeploymentID string       `json:"deployment_id"` // Azure deployment name
	APIVersion   string       `json:"api_version"`   // Azure API 版本
	Models       []Model      `json:"models" gorm:"foreignKey:ProviderID"` // Deprecated: Will be removed in v4 when refactoring service layer
	Enabled      bool         `json:"enabled" gorm:"default:true"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

package model

import (
	"slices"
	"time"
)

// models.go - Remaining database models that are not in the new structure
// This file contains types that haven't been moved to specialized files

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
	AllowedProfiles []string `json:"allowed_profiles" gorm:"serializer:json"` // 允许访问的 Profiles，空数组表示所有
	ExpiredAt     *time.Time `json:"expired_at"` // 过期时间
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CanAccess 检查 API Key 是否有权限访问指定 Profile
func (a *APIKey) CanAccess(profile string) bool {
	if !a.Enabled {
		return false
	}
	// 检查是否过期
	if a.ExpiredAt != nil && time.Now().After(*a.ExpiredAt) {
		return false
	}
	// 空数组表示允许所有
	if len(a.AllowedProfiles) == 0 {
		return true
	}
	// 检查是否在允许列表中
	return slices.Contains(a.AllowedProfiles, profile)
}

// Session 定义长期会话上下文
type Session struct {
	ID                string     `json:"id" gorm:"primaryKey"`
	APIKeyID         string     `json:"api_key_id" gorm:"index"`
	ProfileID        string     `json:"profile_id" gorm:"index"`
	SessionKey       string     `json:"session_key" gorm:"uniqueIndex"` // 用户会话标识

	// 上下文管理
	ContextWindow     int        `json:"context_window"`      // 最大上下文窗口
	CompressedTokens  int        `json:"compressed_tokens"`  // 压缩后 token 数

	// 摘要状态
	LastSummaryAt    *time.Time `json:"last_summary_at"`
	SummaryVersion   int        `json:"summary_version"`

	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// SessionMessage 定义会话消息
type SessionMessage struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	SessionID       string    `json:"session_id" gorm:"index"`
	Role            string    `json:"role"` // user/assistant/system
	Content         string    `json:"content"`
	Tokens          int       `json:"tokens"`

	// 压缩信息
	IsCompressed      bool      `json:"is_compressed"`
	CompressionLevel int       `json:"compression_level"` // 0=原始, 1=轻度, 2=中度, 3=摘要

	CreatedAt        time.Time `json:"created_at"`
}

// CompressionLevel 常量
const (
	CompressionNone        = 0  // 不压缩
	CompressionSliding    = 1  // 滑动窗口
	CompressionSummary    = 2  // 摘要提取
	CompressionAggressive = 3  // 激进压缩
)

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

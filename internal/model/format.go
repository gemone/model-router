package model

import (
	"time"
)

// OpenAI 标准API格式

// ChatCompletionRequest 聊天完成请求
type ChatCompletionRequest struct {
	Model            string         `json:"model"`
	Messages         []Message      `json:"messages"`
	Temperature      *float32       `json:"temperature,omitempty"`
	TopP             *float32       `json:"top_p,omitempty"`
	N                *int           `json:"n,omitempty"`
	Stream           bool           `json:"stream,omitempty"`
	Stop             interface{}    `json:"stop,omitempty"` // string or []string
	MaxTokens        int            `json:"max_tokens,omitempty"`
	PresencePenalty  float32        `json:"presence_penalty,omitempty"`
	FrequencyPenalty float32        `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int `json:"logit_bias,omitempty"`
	User             string         `json:"user,omitempty"`
	Tools            []Tool         `json:"tools,omitempty"`
	ToolChoice       interface{}    `json:"tool_choice,omitempty"` // "none", "auto", or {type: "function", function: {name: "xxx"}}
	ResponseFormat   interface{}    `json:"response_format,omitempty"`

	// Compression override - allows API client to specify compression group
	CompressionModelGroup *string `json:"compression_model_group,omitempty"`
}

// Message 聊天消息
type Message struct {
	Role            string      `json:"role"` // system, user, assistant, tool
	Content         interface{} `json:"content"` // string or []ContentPart
	ReasoningContent string      `json:"reasoning_content,omitempty"` // GLM reasoning model response
	Name            string      `json:"name,omitempty"`
	ToolCalls       []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID      string      `json:"tool_call_id,omitempty"`
}

// GetActualContent returns the actual content, checking reasoning_content for GLM models
func (m *Message) GetActualContent() string {
	// First check if Content is a string and non-empty
	if content, ok := m.Content.(string); ok && content != "" {
		return content
	}
	// Fall back to reasoning_content for GLM models
	if m.ReasoningContent != "" {
		return m.ReasoningContent
	}
	// Return empty string if Content is not a string
	if s, ok := m.Content.(string); ok {
		return s
	}
	return ""
}

// ContentPart 多模态内容部分
type ContentPart struct {
	Type     string   `json:"type"` // text, image_url
	Text     string   `json:"text,omitempty"`
	ImageURL ImageURL `json:"image_url,omitempty"`
}

// ImageURL 图片URL
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // auto, low, high
}

// Tool 工具定义
type Tool struct {
	Type     string   `json:"type"` // function
	Function Function `json:"function"`
}

// Function 函数定义
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters"`  // JSON Schema object
	Arguments   string      `json:"arguments,omitempty"` // Arguments string for streaming tool calls
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string   `json:"id"`
	Index    int      `json:"index,omitempty"` // Index for streaming tool call identification
	Type     string   `json:"type"`            // function
	Function Function `json:"function"`
}

// ChatCompletionResponse 聊天完成响应
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
	Error   *APIError              `json:"error,omitempty"`

	// Compression metadata - information about which models were used for compression
	// Underscore prefix avoids conflicts with OpenAI API spec
	Compression *CompressionMetadata `json:"_compression,omitempty"`
}

// ChatCompletionChoice 完成选项
type ChatCompletionChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// ChatCompletionStreamResponse 流式响应
type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
}

// ChatCompletionStreamChoice 流式响应选项
type ChatCompletionStreamChoice struct {
	Index        int      `json:"index"`
	Delta        Delta    `json:"delta"`
	FinishReason *string  `json:"finish_reason,omitempty"`
}

// Delta 流式增量数据
type Delta struct {
	Role            string      `json:"role,omitempty"`
	Content         string      `json:"content,omitempty"`
	ReasoningContent string      `json:"reasoning_content,omitempty"` // GLM reasoning model streaming
	ToolCalls       []ToolCall  `json:"tool_calls,omitempty"`
}

// GetActualContent returns the actual content from delta
func (d *Delta) GetActualContent() string {
	if d.Content != "" {
		return d.Content
	}
	if d.ReasoningContent != "" {
		return d.ReasoningContent
	}
	return ""
}

// Usage Token使用情况
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// APIError API错误
type APIError struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param,omitempty"`
	Code    string  `json:"code"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// ListModelsResponse 模型列表响应
type ListModelsResponse struct {
	Object string       `json:"object"`
	Data   []ModelInfo  `json:"data"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	ID         string `json:"id"`
	Object     string `json:"object"`
	Created    int64  `json:"created"`
	OwnedBy    string `json:"owned_by"`
}

// EmbeddingRequest 嵌入请求
type EmbeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"` // string or []string
	User  string      `json:"user,omitempty"`
}

// EmbeddingResponse 嵌入响应
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []Embedding     `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

// Embedding 嵌入向量
type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// ModelListRequest 模型列表请求
type ModelListRequest struct {
	After string `json:"after,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// CompressionMetadata tracks compression operation details for API response
type CompressionMetadata struct {
	GroupUsed        string  `json:"group_used,omitempty"`
	ModelSelected    string  `json:"model_selected,omitempty"`
	ProviderID       string  `json:"provider_id,omitempty"`
	FallbackUsed     bool    `json:"fallback_used"`
	CompressionRatio float64 `json:"compression_ratio"`
	TokensBefore     int     `json:"tokens_before,omitempty"`
	TokensAfter      int     `json:"tokens_after,omitempty"`
}

// CompressionModelGroup defines a named group of models for compression tasks
type CompressionModelGroup struct {
	ID        string           `json:"id" gorm:"primaryKey;size:255"`
	ProfileID string           `json:"profile_id" gorm:"index:idx_compression_group_profile;size:255"`
	Name      string           `json:"name" gorm:"index:idx_compression_group_name;size:255"`
	Models    []ModelReference `json:"models" gorm:"serializer:json"`
	Priority  int              `json:"priority" gorm:"default:1"`
	Enabled   bool             `json:"enabled" gorm:"default:true"`

	// Configuration
	HealthThreshold float64 `json:"health_threshold" gorm:"default:70.0"`
	FallbackPolicy  string  `json:"fallback_policy" gorm:"default:'same_model';size:50"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

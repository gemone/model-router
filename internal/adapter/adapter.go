package adapter

import (
	"context"
	"net/http"

	"github.com/gemone/model-router/internal/model"
)

// Adapter 定义模型供应商适配器接口
type Adapter interface {
	// Name 返回适配器名称
	Name() string

	// Type 返回适配器类型
	Type() model.ProviderType

	// Init 初始化适配器
	Init(config *model.Provider) error

	// ChatCompletion 执行聊天完成请求
	ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error)

	// ChatCompletions 执行聊天完成请求（与ChatCompletion相同，用于接口兼容）
	ChatCompletions(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)

	// ChatCompletionStream 执行流式聊天完成请求
	ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error)

	// Embeddings 执行嵌入请求
	Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error)

	// ListModels 列出可用模型
	ListModels(ctx context.Context) (*model.ListModelsResponse, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) (bool, error)

	// GetRequestHeaders 获取请求头
	GetRequestHeaders() map[string]string

	// ConvertRequest 将OpenAI格式请求转换为供应商特定格式
	ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error)

	// ConvertResponse 将供应商特定响应转换为OpenAI格式
	ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error)

	// ConvertStreamResponse 将供应商特定流式响应转换为OpenAI格式
	ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error)

	// DoRequest 执行HTTP请求
	DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error)
}

// Type aliases for compatibility with existing code
type (
	ChatCompletionRequest       = model.ChatCompletionRequest
	ChatCompletionResponse      = model.ChatCompletionResponse
	ChatCompletionStreamResponse = model.ChatCompletionStreamResponse
	EmbeddingRequest            = model.EmbeddingRequest
	EmbeddingResponse           = model.EmbeddingResponse
	ListModelsResponse          = model.ListModelsResponse
)

// AdapterFactory 适配器工厂
type AdapterFactory struct {
	adapters map[model.ProviderType]func() Adapter
}

var factory = &AdapterFactory{
	adapters: make(map[model.ProviderType]func() Adapter),
}

// Register 注册适配器
func Register(providerType model.ProviderType, factoryFunc func() Adapter) {
	factory.adapters[providerType] = factoryFunc
}

// Create 创建适配器实例
func Create(providerType model.ProviderType) Adapter {
	if factoryFunc, ok := factory.adapters[providerType]; ok {
		return factoryFunc()
	}
	return nil
}

// GetSupportedTypes 获取支持的供应商类型
func GetSupportedTypes() []model.ProviderType {
	types := make([]model.ProviderType, 0, len(factory.adapters))
	for t := range factory.adapters {
		types = append(types, t)
	}
	return types
}

// HealthStatus represents the health check result
type HealthStatus struct {
	Healthy   bool   `json:"healthy"`
	Message   string `json:"message,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

// ProviderError wraps provider-specific errors with context
type ProviderError struct {
	ProviderType model.ProviderType `json:"provider_type"`
	Message      string             `json:"message"`
	StatusCode   int                `json:"status_code,omitempty"`
	Err          error              `json:"-"`
}

// Error returns the error message
func (e *ProviderError) Error() string {
	if e.Err != nil {
		return string(e.ProviderType) + ": " + e.Message + ": " + e.Err.Error()
	}
	return string(e.ProviderType) + ": " + e.Message
}

// Unwrap returns the underlying error
func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new provider error
func NewProviderError(providerType model.ProviderType, message string, err error) *ProviderError {
	return &ProviderError{
		ProviderType: providerType,
		Message:      message,
		Err:          err,
	}
}

// NewProviderErrorWithStatus creates a new provider error with HTTP status code
func NewProviderErrorWithStatus(providerType model.ProviderType, message string, statusCode int, err error) *ProviderError {
	return &ProviderError{
		ProviderType: providerType,
		Message:      message,
		StatusCode:   statusCode,
		Err:          err,
	}
}

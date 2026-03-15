package openai_compatible

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

// upstreamRequest 定义了发送给上游 API 的请求结构
// 排除了 model-router 内部使用的字段
type upstreamRequest struct {
	Model            string         `json:"model"`
	Messages         []model.Message `json:"messages"`
	Temperature      *float32       `json:"temperature,omitempty"`
	TopP             *float32       `json:"top_p,omitempty"`
	N                *int           `json:"n,omitempty"`
	Stream           bool           `json:"stream,omitempty"`
	Stop             interface{}    `json:"stop,omitempty"`
	MaxTokens        int            `json:"max_tokens,omitempty"`
	PresencePenalty  float32        `json:"presence_penalty,omitempty"`
	FrequencyPenalty float32        `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int `json:"logit_bias,omitempty"`
	User             string         `json:"user,omitempty"`
	Tools            []model.Tool   `json:"tools,omitempty"`
	ToolChoice       interface{}    `json:"tool_choice,omitempty"`
	ResponseFormat   interface{}    `json:"response_format,omitempty"`
}

// toUpstreamRequest 将内部请求转换为上游请求，清理内部字段
func toUpstreamRequest(req *model.ChatCompletionRequest) *upstreamRequest {
	return &upstreamRequest{
		Model:            req.Model,
		Messages:         req.Messages,
		Temperature:      req.Temperature,
		TopP:             req.TopP,
		N:                req.N,
		Stream:           req.Stream,
		Stop:             req.Stop,
		MaxTokens:        req.MaxTokens,
		PresencePenalty:  req.PresencePenalty,
		FrequencyPenalty: req.FrequencyPenalty,
		LogitBias:        req.LogitBias,
		User:             req.User,
		Tools:            req.Tools,
		ToolChoice:       req.ToolChoice,
		ResponseFormat:   req.ResponseFormat,
	}
}

func init() {
	// Register with underscore format (current standard)
	adapter.Register(model.ProviderOpenAICompatible, NewOpenAICompatibleAdapter)
	// Also register with hyphen format for backward compatibility
	adapter.Register("openai-compatible", NewOpenAICompatibleAdapter)
}

// OpenAICompatibleAdapter 通用 OpenAI 兼容适配器
// 支持任何 OpenAI API 兼容的服务，包括自定义模型
type OpenAICompatibleAdapter struct {
	adapter.BaseAdapter
	customHeaders map[string]string
}

// NewOpenAICompatibleAdapter 创建新的 OpenAI 兼容适配器
func NewOpenAICompatibleAdapter() adapter.Adapter {
	return &OpenAICompatibleAdapter{
		customHeaders: make(map[string]string),
	}
}

// Name 返回适配器名称
func (o *OpenAICompatibleAdapter) Name() string {
	return "OpenAI Compatible"
}

// Type 返回适配器类型
func (o *OpenAICompatibleAdapter) Type() model.ProviderType {
	return model.ProviderOpenAICompatible
}

// Init 初始化适配器（支持自定义 headers）
func (o *OpenAICompatibleAdapter) Init(provider *model.Provider) error {
	if err := o.BaseAdapter.Init(provider); err != nil {
		return err
	}

	// 解析额外 headers（如果有）
	// 格式: Header1:Value1|Header2:Value2
	if len(provider.Models) > 0 {
		_ = provider.Models[0].OriginalName // 实际使用时从配置读取
	}

	return nil
}

// GetRequestHeaders 获取请求头
func (o *OpenAICompatibleAdapter) GetRequestHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + o.getAPIKey(),
	}

	// 合并自定义 headers
	for k, v := range o.customHeaders {
		headers[k] = v
	}

	return headers
}

// SetCustomHeader 设置自定义 header
func (o *OpenAICompatibleAdapter) SetCustomHeader(key, value string) {
	o.customHeaders[key] = value
}

// ChatCompletion 执行聊天完成请求
func (o *OpenAICompatibleAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	// 使用自定义路径，默认为 /v1/chat/completions
	chatPath := o.GetProvider().ChatPath
	if chatPath == "" {
		chatPath = "/v1/chat/completions"
	}
	// 转换为上游请求，清理内部字段
	upstreamReq := toUpstreamRequest(req)

	resp, err := o.DoRequest(ctx, "POST", chatPath, upstreamReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp model.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("API error: %s", errResp.Error.Message)
	}

	var response model.ChatCompletionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// ChatCompletionStream 执行流式聊天完成请求
func (o *OpenAICompatibleAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	// 转换为上游请求，清理内部字段（Stream 设置为 true）
	upstreamReq := toUpstreamRequest(req)
	upstreamReq.Stream = true

	// 使用自定义路径，默认为 /v1/chat/completions
	chatPath := o.GetProvider().ChatPath
	if chatPath == "" {
		chatPath = "/v1/chat/completions"
	}
	resp, err := o.DoRequest(ctx, "POST", chatPath, upstreamReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	ch := make(chan *model.ChatCompletionStreamResponse)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var streamResp model.ChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			select {
			case ch <- &streamResp:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// ListModels 列出可用模型
func (o *OpenAICompatibleAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	resp, err := o.DoRequest(ctx, "GET", "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 如果 API 不支持，返回空列表
		return &model.ListModelsResponse{
			Object: "list",
			Data:   []model.ModelInfo{},
		}, nil
	}

	var response model.ListModelsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// HealthCheck 健康检查
func (o *OpenAICompatibleAdapter) HealthCheck(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := o.ListModels(ctx)
	return err == nil, err
}

// Embeddings 执行嵌入请求
func (o *OpenAICompatibleAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	resp, err := o.DoRequest(ctx, "POST", "/v1/embeddings", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response model.EmbeddingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// ConvertRequest 请求转换（OpenAI 兼容格式无需转换）
func (o *OpenAICompatibleAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}

// ConvertResponse 响应转换（OpenAI 兼容格式无需转换）
func (o *OpenAICompatibleAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	var response model.ChatCompletionResponse
	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ConvertStreamResponse 流式响应转换（OpenAI 兼容格式无需转换）
func (o *OpenAICompatibleAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	var response model.ChatCompletionStreamResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// getAPIKey 获取 API Key
func (o *OpenAICompatibleAdapter) getAPIKey() string {
	headers := o.BaseAdapter.GetRequestHeaders()
	if auth := headers["Authorization"]; auth != "" && len(auth) > 7 {
		return auth[7:] // 去掉 "Bearer " 前缀
	}
	return ""
}

// CustomModelConfig 自定义模型配置
type CustomModelConfig struct {
	ModelName      string            `json:"model_name"`      // 对外暴露的模型名
	OriginalName   string            `json:"original_name"`   // 供应商原始模型名
	ContextWindow  int               `json:"context_window"`  // 上下文窗口
	MaxTokens      int               `json:"max_tokens"`      // 最大输出 token
	SupportsFunc   bool              `json:"supports_func"`   // 是否支持函数调用
	SupportsVision bool              `json:"supports_vision"` // 是否支持视觉
	CustomHeaders  map[string]string `json:"custom_headers"`  // 自定义 headers
}

// ValidateModelConfig 验证模型配置
func ValidateModelConfig(config *CustomModelConfig) error {
	if config.ModelName == "" {
		return fmt.Errorf("model name is required")
	}
	if config.OriginalName == "" {
		config.OriginalName = config.ModelName
	}
	if config.ContextWindow <= 0 {
		config.ContextWindow = 4096
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = 4096
	}
	return nil
}

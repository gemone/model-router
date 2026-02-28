package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/utils"
)

// BaseAdapter 基础适配器实现
type BaseAdapter struct {
	provider   *model.Provider
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// Init 初始化基础适配器
func (b *BaseAdapter) Init(provider *model.Provider) error {
	b.provider = provider
	b.baseURL = provider.BaseURL

	// 解密API密钥
	if provider.APIKeyEnc != "" {
		decryptedKey, err := utils.Decrypt(provider.APIKeyEnc)
		if err != nil {
			return fmt.Errorf("failed to decrypt API key: %w", err)
		}
		b.apiKey = decryptedKey
	} else {
		b.apiKey = provider.APIKey
	}

	b.httpClient = &http.Client{
		Timeout: 120 * time.Second,
	}

	return nil
}

// GetRequestHeaders 获取默认请求头
func (b *BaseAdapter) GetRequestHeaders() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + b.apiKey,
	}
}

// DoRequest 执行HTTP请求
func (b *BaseAdapter) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := b.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range b.GetRequestHeaders() {
		req.Header.Set(key, value)
	}

	return b.httpClient.Do(req)
}

// ChatCompletion 默认实现（子类需要覆盖）
func (b *BaseAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ChatCompletionStream 默认实现（子类需要覆盖）
func (b *BaseAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// Embeddings 默认实现（子类需要覆盖）
func (b *BaseAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ListModels 默认实现（子类需要覆盖）
func (b *BaseAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// HealthCheck 默认实现（子类需要覆盖）
func (b *BaseAdapter) HealthCheck(ctx context.Context) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

// ConvertRequest 默认实现（子类需要覆盖）
func (b *BaseAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}

// ConvertResponse 默认实现（子类需要覆盖）
func (b *BaseAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// ConvertStreamResponse 默认实现（子类需要覆盖）
func (b *BaseAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetBaseURL 获取BaseURL
func (b *BaseAdapter) GetBaseURL() string {
	return b.baseURL
}

// GetHTTPClient 获取HTTP客户端
func (b *BaseAdapter) GetHTTPClient() *http.Client {
	return b.httpClient
}

// GetAPIKey 获取API Key
func (b *BaseAdapter) GetAPIKey() string {
	return b.apiKey
}

// DecryptAPIKey 解密API Key
func (b *BaseAdapter) DecryptAPIKey(provider *model.Provider) (string, error) {
	if provider.APIKeyEnc != "" {
		decryptedKey, err := utils.Decrypt(provider.APIKeyEnc)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt API key: %w", err)
		}
		return decryptedKey, nil
	}
	return provider.APIKey, nil
}

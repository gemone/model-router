package azure

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

func init() {
	adapter.Register(model.ProviderAzure, NewAzureAdapter)
}

// AzureAdapter Azure OpenAI适配器
type AzureAdapter struct {
	adapter.BaseAdapter
	deploymentName string
	apiVersion     string
}

// NewAzureAdapter 创建新的Azure适配器
func NewAzureAdapter() adapter.Adapter {
	return &AzureAdapter{}
}

// Name 返回适配器名称
func (a *AzureAdapter) Name() string {
	return "Azure OpenAI"
}

// Type 返回适配器类型
func (a *AzureAdapter) Type() model.ProviderType {
	return model.ProviderAzure
}

// Init 初始化Azure适配器
func (a *AzureAdapter) Init(provider *model.Provider) error {
	if err := a.BaseAdapter.Init(provider); err != nil {
		return err
	}

	// Azure使用API Key作为header
	apiKey, err := a.BaseAdapter.DecryptAPIKey(provider)
	if err != nil {
		return err
	}
	_ = apiKey // 存储在BaseAdapter中

	// 从BaseURL提取deployment name
	// BaseURL格式: https://{resource}.openai.azure.com/openai/deployments/{deployment}
	// 或者用户可以设置自定义的BaseURL
	a.deploymentName = provider.DeploymentID

	// API版本默认为2024-02-15-preview
	a.apiVersion = "2024-02-15-preview"
	if provider.APIVersion != "" {
		a.apiVersion = provider.APIVersion
	}

	return nil
}

// GetRequestHeaders 获取Azure请求头
func (a *AzureAdapter) GetRequestHeaders() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"api-key":       a.BaseAdapter.GetAPIKey(),
		"Accept":        "application/json",
	}
}

// BuildURL 构建Azure API URL
func (a *AzureAdapter) BuildURL(path string) string {
	// Azure URL格式: {baseURL}/openai/deployments/{deployment}/{path}?api-version={version}
	baseURL := strings.TrimRight(a.BaseAdapter.GetBaseURL(), "/")

	// 如果baseURL已经包含完整路径，直接使用
	if strings.Contains(baseURL, "/deployments/") {
		return fmt.Sprintf("%s%s?api-version=%s", baseURL, path, a.apiVersion)
	}

	// 否则构建标准Azure URL
	return fmt.Sprintf("%s/openai/deployments/%s%s?api-version=%s", baseURL, a.deploymentName, path, a.apiVersion)
}

// ChatCompletion 执行聊天完成请求
func (a *AzureAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	url := a.BuildURL("/chat/completions")

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range a.GetRequestHeaders() {
		httpReq.Header.Set(key, value)
	}

	resp, err := a.BaseAdapter.GetHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
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
func (a *AzureAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	req.Stream = true

	url := a.BuildURL("/chat/completions")

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range a.GetRequestHeaders() {
		httpReq.Header.Set(key, value)
	}
	// Azure流式响应需要text/event-stream
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.BaseAdapter.GetHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
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

// Embeddings 执行嵌入请求
func (a *AzureAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	url := a.BuildURL("/embeddings")

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range a.GetRequestHeaders() {
		httpReq.Header.Set(key, value)
	}

	resp, err := a.BaseAdapter.GetHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
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

// ListModels 列出可用模型
func (a *AzureAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	// Azure的模型列表需要通过API获取
	// 这里返回一个基于配置的模拟响应
	url := fmt.Sprintf("%s/openai/models?api-version=%s",
		strings.TrimRight(a.BaseAdapter.GetBaseURL(), "/"),
		a.apiVersion)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range a.GetRequestHeaders() {
		httpReq.Header.Set(key, value)
	}

	resp, err := a.BaseAdapter.GetHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response model.ListModelsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// HealthCheck 健康检查
func (a *AzureAdapter) HealthCheck(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := a.ListModels(ctx)
	return err == nil, err
}

// ConvertRequest 请求转换（Azure格式无需转换）
func (a *AzureAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}

// ConvertResponse 响应转换（Azure格式无需转换）
func (a *AzureAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	var response model.ChatCompletionResponse
	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ConvertStreamResponse 流式响应转换（Azure格式无需转换）
func (a *AzureAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	var response model.ChatCompletionStreamResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

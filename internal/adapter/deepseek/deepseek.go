package deepseek

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
	adapter.Register(model.ProviderDeepSeek, NewDeepSeekAdapter)
}

// DeepSeekAdapter DeepSeek 适配器
type DeepSeekAdapter struct {
	adapter.BaseAdapter
}

// NewDeepSeekAdapter 创建新的 DeepSeek 适配器
func NewDeepSeekAdapter() adapter.Adapter {
	return &DeepSeekAdapter{}
}

// Name 返回适配器名称
func (d *DeepSeekAdapter) Name() string {
	return "DeepSeek"
}

// Type 返回适配器类型
func (d *DeepSeekAdapter) Type() model.ProviderType {
	return model.ProviderDeepSeek
}

// ChatCompletion 执行聊天完成请求
func (d *DeepSeekAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	resp, err := d.DoRequest(ctx, "POST", "/chat/completions", req)
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
func (d *DeepSeekAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	req.Stream = true

	resp, err := d.DoRequest(ctx, "POST", "/chat/completions", req)
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
func (d *DeepSeekAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	models := []model.ModelInfo{
		{ID: "deepseek-chat", Object: "model", Created: time.Now().Unix(), OwnedBy: "deepseek"},
		{ID: "deepseek-reasoner", Object: "model", Created: time.Now().Unix(), OwnedBy: "deepseek"},
		{ID: "deepseek-coder", Object: "model", Created: time.Now().Unix(), OwnedBy: "deepseek"},
	}

	return &model.ListModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// HealthCheck 健康检查
func (d *DeepSeekAdapter) HealthCheck(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := d.ListModels(ctx)
	return err == nil, err
}

// Embeddings 执行嵌入请求
func (d *DeepSeekAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	resp, err := d.DoRequest(ctx, "POST", "/embeddings", req)
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

// ConvertRequest 请求转换（DeepSeek 兼容 OpenAI 格式）
func (d *DeepSeekAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}

// ConvertResponse 响应转换（DeepSeek 兼容 OpenAI 格式）
func (d *DeepSeekAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	var response model.ChatCompletionResponse
	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ConvertStreamResponse 流式响应转换（DeepSeek 兼容 OpenAI 格式）
func (d *DeepSeekAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	var response model.ChatCompletionStreamResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

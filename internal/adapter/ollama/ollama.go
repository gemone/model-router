package ollama

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
	"github.com/gemone/model-router/internal/middleware"
	"github.com/gemone/model-router/internal/model"
)

func init() {
	adapter.Register(model.ProviderOllama, NewOllamaAdapter)
}

// OllamaAdapter Ollama 本地模型适配器
type OllamaAdapter struct {
	adapter.BaseAdapter
}

// NewOllamaAdapter 创建新的 Ollama 适配器
func NewOllamaAdapter() adapter.Adapter {
	return &OllamaAdapter{}
}

// Name 返回适配器名称
func (o *OllamaAdapter) Name() string {
	return "Ollama"
}

// Type 返回适配器类型
func (o *OllamaAdapter) Type() model.ProviderType {
	return model.ProviderOllama
}

// GetRequestHeaders 获取请求头
func (o *OllamaAdapter) GetRequestHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

// ChatCompletion 执行聊天完成请求
func (o *OllamaAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	start := time.Now()
	providerName := "Ollama"
	if o.GetProvider() != nil && o.GetProvider().Name != "" {
		providerName = o.GetProvider().Name
	}

	ollamaReq := o.convertRequest(req)

	// Debug log request
	middleware.LogAdapterRequest(providerName, req.Model, o.GetBaseURL()+"/api/chat", ollamaReq)

	resp, err := o.DoRequest(ctx, "POST", "/api/chat", ollamaReq)
	if err != nil {
		middleware.LogAdapterResponse(providerName, req.Model, 0, map[string]string{"error": err.Error()}, time.Since(start))
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		middleware.LogAdapterResponse(providerName, req.Model, resp.StatusCode, map[string]string{"error": err.Error()}, time.Since(start))
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		middleware.LogAdapterResponse(providerName, req.Model, resp.StatusCode, map[string]string{"error": string(body)}, time.Since(start))
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		middleware.LogAdapterResponse(providerName, req.Model, resp.StatusCode, map[string]string{"error": err.Error()}, time.Since(start))
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	result := o.convertResponse(&ollamaResp, req.Model)
	middleware.LogAdapterResponse(providerName, req.Model, resp.StatusCode, result, time.Since(start))

	return result, nil
}

// ChatCompletionStream 执行流式聊天完成请求
func (o *OllamaAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	ollamaReq := o.convertRequest(req)
	ollamaReq.Stream = true

	resp, err := o.DoRequest(ctx, "POST", "/api/chat", ollamaReq)
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
		var fullContent string

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var streamResp ollamaResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				continue
			}

			if streamResp.Message != nil {
				fullContent += streamResp.Message.Content
			}

			// Ollama 流式响应需要累积内容
			openAIResp := &model.ChatCompletionStreamResponse{
				ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []model.ChatCompletionStreamChoice{
					{
						Index: 0,
						Delta: model.Delta{
							Content: streamResp.Message.Content,
						},
					},
				},
			}

			if streamResp.Done {
				finishReason := "stop"
				openAIResp.Choices[0].FinishReason = &finishReason
			}

			select {
			case ch <- openAIResp:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// ListModels 列出可用模型（本地 Ollama 模型）
func (o *OllamaAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	resp, err := o.DoRequest(ctx, "GET", "/api/tags", nil)
	if err != nil {
		// 如果无法连接，返回常见模型列表
		return &model.ListModelsResponse{
			Object: "list",
			Data: []model.ModelInfo{
				{ID: "llama2", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "llama3", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "llama3.1", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "mistral", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "codellama", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "qwen", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "qwen2", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "gemma", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "gemma2", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
				{ID: "phi3", Object: "model", Created: time.Now().Unix(), OwnedBy: "ollama"},
			},
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	models := make([]model.ModelInfo, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, model.ModelInfo{
			ID:      m.Name,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "ollama",
		})
	}

	return &model.ListModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// HealthCheck 健康检查
func (o *OllamaAdapter) HealthCheck(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := o.ListModels(ctx)
	return err == nil, err
}

// Embeddings 执行嵌入请求
func (o *OllamaAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	var inputs []string
	switch v := req.Input.(type) {
	case string:
		inputs = []string{v}
	case []string:
		inputs = v
	default:
		return nil, fmt.Errorf("unsupported input type")
	}

	ollamaReq := ollamaEmbedRequest{
		Model: req.Model,
	}

	embeddings := make([]model.Embedding, 0, len(inputs))
	for i, input := range inputs {
		ollamaReq.Prompt = input

		resp, err := o.DoRequest(ctx, "POST", "/api/embeddings", ollamaReq)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		var result ollamaEmbedResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		embeddings = append(embeddings, model.Embedding{
			Object:    "embedding",
			Embedding: result.Embedding,
			Index:     i,
		})
	}

	return &model.EmbeddingResponse{
		Object: "list",
		Data:   embeddings,
		Model:  req.Model,
	}, nil
}

// ConvertRequest 将 OpenAI 请求转换为 Ollama 格式
func (o *OllamaAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return o.convertRequest(req), nil
}

// ConvertResponse 将 Ollama 响应转换为 OpenAI 格式
func (o *OllamaAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(resp, &ollamaResp); err != nil {
		return nil, err
	}
	return o.convertResponse(&ollamaResp, ""), nil
}

// ConvertStreamResponse 将 Ollama 流式响应转换为 OpenAI 格式
func (o *OllamaAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(data, &ollamaResp); err != nil {
		return nil, err
	}

	return &model.ChatCompletionStreamResponse{
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Choices: []model.ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: model.Delta{
					Content: ollamaResp.Message.Content,
				},
			},
		},
	}, nil
}

// Ollama 请求/响应结构
type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float32 `json:"temperature,omitempty"`
	TopP        float32 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Model           string         `json:"model"`
	CreatedAt       string         `json:"created_at,omitempty"`
	Message         *ollamaMessage `json:"message,omitempty"`
	Done            bool           `json:"done"`
	PromptEvalCount int            `json:"prompt_eval_count,omitempty"`
	EvalCount       int            `json:"eval_count,omitempty"`
}

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (o *OllamaAdapter) convertRequest(req *model.ChatCompletionRequest) *ollamaRequest {
	ollamaReq := &ollamaRequest{
		Model:    req.Model,
		Messages: make([]ollamaMessage, 0, len(req.Messages)),
		Stream:   false,
	}

	if req.Temperature != nil || req.TopP != nil || req.MaxTokens > 0 {
		ollamaReq.Options = &ollamaOptions{}
		if req.Temperature != nil {
			ollamaReq.Options.Temperature = *req.Temperature
		}
		if req.TopP != nil {
			ollamaReq.Options.TopP = *req.TopP
		}
		if req.MaxTokens > 0 {
			ollamaReq.Options.NumPredict = req.MaxTokens
		}
	}

	for _, msg := range req.Messages {
		role := msg.Role
		if role == "system" {
			// Ollama 将 system 作为第一条 user 消息处理，或使用特定格式
			// 这里我们保留 system 角色
		}

		content := ""
		switch v := msg.Content.(type) {
		case string:
			content = v
		case []interface{}:
			// 多模态内容处理（Ollama 支持图片）
			parts := []string{}
			for _, part := range v {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
			content = strings.Join(parts, "")
		}

		ollamaReq.Messages = append(ollamaReq.Messages, ollamaMessage{
			Role:    role,
			Content: content,
		})
	}

	return ollamaReq
}

func (o *OllamaAdapter) convertResponse(resp *ollamaResponse, modelName string) *model.ChatCompletionResponse {
	content := ""
	if resp.Message != nil {
		content = resp.Message.Content
	}

	finishReason := "stop"
	if !resp.Done {
		finishReason = ""
	}

	return &model.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.ChatCompletionChoice{
			{
				Index: 0,
				Message: model.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: model.Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
	}
}

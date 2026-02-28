package anthropic

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
	adapter.Register(model.ProviderClaude, NewClaudeAdapter)
}

// ClaudeAdapter Claude适配器
type ClaudeAdapter struct {
	adapter.BaseAdapter
}

// NewClaudeAdapter 创建新的Claude适配器
func NewClaudeAdapter() adapter.Adapter {
	return &ClaudeAdapter{}
}

// Name 返回适配器名称
func (c *ClaudeAdapter) Name() string {
	return "Claude"
}

// Type 返回适配器类型
func (c *ClaudeAdapter) Type() model.ProviderType {
	return model.ProviderClaude
}

// GetRequestHeaders 获取请求头
func (c *ClaudeAdapter) GetRequestHeaders() map[string]string {
	return map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         c.BaseAdapter.GetRequestHeaders()["Authorization"][7:], // 去掉 "Bearer " 前缀
		"anthropic-version": "2023-06-01",
	}
}

// ChatCompletion 执行聊天完成请求
func (c *ClaudeAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	claudeReq := c.convertRequest(req)

	resp, err := c.DoRequest(ctx, "POST", "/v1/messages", claudeReq)
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

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return c.convertResponse(&claudeResp, req.Model), nil
}

// ChatCompletionStream 执行流式聊天完成请求
func (c *ClaudeAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	claudeReq := c.convertRequest(req)
	claudeReq.Stream = true

	resp, err := c.DoRequest(ctx, "POST", "/v1/messages", claudeReq)
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

			var streamResp claudeStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			openAIResp := c.convertStreamResponse(&streamResp, req.Model)
			if openAIResp != nil {
				select {
				case ch <- openAIResp:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// ListModels 列出可用模型
func (c *ClaudeAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	// Claude没有官方的模型列表API，返回常用模型
	models := []model.ModelInfo{
		{ID: "claude-3-5-sonnet-20241022", Object: "model", Created: time.Now().Unix(), OwnedBy: "anthropic"},
		{ID: "claude-3-5-sonnet-20240620", Object: "model", Created: time.Now().Unix(), OwnedBy: "anthropic"},
		{ID: "claude-3-opus-20240229", Object: "model", Created: time.Now().Unix(), OwnedBy: "anthropic"},
		{ID: "claude-3-sonnet-20240229", Object: "model", Created: time.Now().Unix(), OwnedBy: "anthropic"},
		{ID: "claude-3-haiku-20240307", Object: "model", Created: time.Now().Unix(), OwnedBy: "anthropic"},
		{ID: "claude-3-7-sonnet-20250219", Object: "model", Created: time.Now().Unix(), OwnedBy: "anthropic"},
	}

	return &model.ListModelsResponse{
		Object: "list",
		Data:   models,
	}, nil
}

// HealthCheck 健康检查
func (c *ClaudeAdapter) HealthCheck(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := c.ListModels(ctx)
	return err == nil, err
}

// Claude 请求/响应结构
type claudeRequest struct {
	Model       string          `json:"model"`
	Messages    []claudeMessage `json:"messages"`
	System      string          `json:"system,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float32         `json:"temperature,omitempty"`
	TopP        float32         `json:"top_p,omitempty"`
	TopK        int             `json:"top_k,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       []claudeTool    `json:"tools,omitempty"`
	ToolChoice  interface{}     `json:"tool_choice,omitempty"`
}

type claudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []contentBlock
}

type contentBlock struct {
	Type  string          `json:"type"` // text, image, tool_use, tool_result
	Text  string          `json:"text,omitempty"`
	URL   string          `json:"url,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type claudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type claudeResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []contentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        claudeUsage    `json:"usage"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeStreamResponse struct {
	Type         string          `json:"type"`
	Message      *claudeResponse `json:"message,omitempty"`
	Index        int             `json:"index,omitempty"`
	ContentBlock *contentBlock   `json:"content_block,omitempty"`
	Delta        *delta          `json:"delta,omitempty"`
	Usage        *claudeUsage    `json:"usage,omitempty"`
}

type delta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

func (c *ClaudeAdapter) convertRequest(req *model.ChatCompletionRequest) *claudeRequest {
	claudeReq := &claudeRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: 0.7,
		Stream:      req.Stream,
	}

	if req.Temperature != nil {
		claudeReq.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		claudeReq.TopP = *req.TopP
	}

	// 处理消息
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if content, ok := msg.Content.(string); ok {
				claudeReq.System = content
			}
			continue
		}

		claudeMsg := claudeMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// 转换工具调用
		if len(msg.ToolCalls) > 0 {
			blocks := []contentBlock{}
			if content, ok := msg.Content.(string); ok && content != "" {
				blocks = append(blocks, contentBlock{Type: "text", Text: content})
			}
			for _, tc := range msg.ToolCalls {
				input, _ := json.Marshal(tc.Function.Parameters)
				blocks = append(blocks, contentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			claudeMsg.Content = blocks
		}

		claudeReq.Messages = append(claudeReq.Messages, claudeMsg)
	}

	// 转换工具
	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			schema, _ := json.Marshal(tool.Function.Parameters)
			claudeReq.Tools = append(claudeReq.Tools, claudeTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: schema,
			})
		}
	}

	return claudeReq
}

func (c *ClaudeAdapter) convertResponse(resp *claudeResponse, modelName string) *model.ChatCompletionResponse {
	var content string
	var toolCalls []model.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, model.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: model.Function{
					Name:       block.Name,
					Parameters: block.Input,
				},
			})
		}
	}

	finishReason := "stop"
	if resp.StopReason == "max_tokens" {
		finishReason = "length"
	} else if resp.StopReason == "tool_use" {
		finishReason = "tool_calls"
	}

	return &model.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", resp.ID),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelName,
		Choices: []model.ChatCompletionChoice{
			{
				Index: 0,
				Message: model.Message{
					Role:      "assistant",
					Content:   content,
					ToolCalls: toolCalls,
				},
				FinishReason: finishReason,
			},
		},
		Usage: model.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

func (c *ClaudeAdapter) convertStreamResponse(resp *claudeStreamResponse, modelName string) *model.ChatCompletionStreamResponse {
	if resp.Type == "message_start" {
		return &model.ChatCompletionStreamResponse{
			ID:      fmt.Sprintf("chatcmpl-%s", resp.Message.ID),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   modelName,
			Choices: []model.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: model.Delta{
						Role: "assistant",
					},
				},
			},
		}
	}

	if resp.Type == "content_block_delta" && resp.Delta != nil {
		return &model.ChatCompletionStreamResponse{
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   modelName,
			Choices: []model.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: model.Delta{
						Content: resp.Delta.Text,
					},
				},
			},
		}
	}

	if resp.Type == "message_delta" && resp.Delta != nil && resp.Delta.StopReason != "" {
		finishReason := "stop"
		if resp.Delta.StopReason == "max_tokens" {
			finishReason = "length"
		}
		return &model.ChatCompletionStreamResponse{
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   modelName,
			Choices: []model.ChatCompletionStreamChoice{
				{
					Index:        0,
					Delta:        model.Delta{},
					FinishReason: &finishReason,
				},
			},
		}
	}

	return nil
}

// ConvertRequest 将OpenAI请求转换为Claude格式
func (c *ClaudeAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return c.convertRequest(req), nil
}

// ConvertResponse 将Claude响应转换为OpenAI格式
func (c *ClaudeAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	var claudeResp claudeResponse
	if err := json.Unmarshal(resp, &claudeResp); err != nil {
		return nil, err
	}
	return c.convertResponse(&claudeResp, ""), nil
}

// ConvertStreamResponse 将Claude流式响应转换为OpenAI格式
func (c *ClaudeAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	var streamResp claudeStreamResponse
	if err := json.Unmarshal(data, &streamResp); err != nil {
		return nil, err
	}
	return c.convertStreamResponse(&streamResp, ""), nil
}

// Embeddings Claude不支持Embeddings
func (c *ClaudeAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	return nil, fmt.Errorf("Claude does not support embeddings")
}

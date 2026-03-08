package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/model"
)

// OllamaMessage Ollama 消息格式
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Images  []string `json:"images,omitempty"` // base64 编码的图片
}

// OllamaChatRequest Ollama Chat API 请求
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Format   string          `json:"format,omitempty"` // json 格式输出
	Options  OllamaOptions   `json:"options,omitempty"`
	Stream   bool            `json:"stream,omitempty"`
	KeepAlive string         `json:"keep_alive,omitempty"` // 模型在内存中保持的时间
}

// OllamaGenerateRequest Ollama Generate API 请求
type OllamaGenerateRequest struct {
	Model   string        `json:"model"`
	Prompt  string        `json:"prompt"`
	Suffix  string        `json:"suffix,omitempty"`
	Images  []string      `json:"images,omitempty"`
	Format  string        `json:"format,omitempty"`
	Options OllamaOptions `json:"options,omitempty"`
	System  string        `json:"system,omitempty"`
	Template string       `json:"template,omitempty"`
	Context []int         `json:"context,omitempty"`
	Stream  bool          `json:"stream,omitempty"`
	Raw     bool          `json:"raw,omitempty"`
	KeepAlive string      `json:"keep_alive,omitempty"`
}

// OllamaOptions Ollama 生成选项
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // 最大生成 token 数
	Seed        int     `json:"seed,omitempty"`
	RepeatPenalty float64 `json:"repeat_penalty,omitempty"`
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64 `json:"presence_penalty,omitempty"`
	NumCtx      int     `json:"num_ctx,omitempty"`     // 上下文窗口大小
	NumThread   int     `json:"num_thread,omitempty"`  // 线程数
}

// OllamaChatResponse Ollama Chat API 响应
type OllamaChatResponse struct {
	Model     string         `json:"model"`
	CreatedAt string         `json:"created_at"`
	Message   OllamaMessage  `json:"message"`
	Done      bool           `json:"done"`
	
	// 统计信息（仅在 Done=true 时有效）
	TotalDuration    int64 `json:"total_duration,omitempty"`    // 总耗时（纳秒）
	LoadDuration     int64 `json:"load_duration,omitempty"`     // 模型加载耗时
	PromptEvalCount  int   `json:"prompt_eval_count,omitempty"` // 输入 token 数
	PromptEvalDuration int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount        int   `json:"eval_count,omitempty"`        // 输出 token 数
	EvalDuration     int64 `json:"eval_duration,omitempty"`     // 生成耗时
}

// OllamaGenerateResponse Ollama Generate API 响应
type OllamaGenerateResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	Context   []int  `json:"context,omitempty"`
	
	// 统计信息
	TotalDuration    int64 `json:"total_duration,omitempty"`
	LoadDuration     int64 `json:"load_duration,omitempty"`
	PromptEvalCount  int   `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount        int   `json:"eval_count,omitempty"`
	EvalDuration     int64 `json:"eval_duration,omitempty"`
}

// OllamaModelInfo Ollama 模型信息
type OllamaModelInfo struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	Details    struct {
		Format           string   `json:"format"`
		Family           string   `json:"family"`
		Families         []string `json:"families"`
		ParameterSize    string   `json:"parameter_size"`
		QuantizationLevel string  `json:"quantization_level"`
	} `json:"details"`
}

// OllamaListResponse Ollama List Models 响应
type OllamaListResponse struct {
	Models []OllamaModelInfo `json:"models"`
}

// OllamaEmbeddingRequest Ollama Embedding 请求
type OllamaEmbeddingRequest struct {
	Model string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingResponse Ollama Embedding 响应
type OllamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// OllamaErrorResponse Ollama 错误响应
type OllamaErrorResponse struct {
	Error string `json:"error"`
}

// ==================== 转换函数 ====================

// ConvertOpenAIToOllamaChat 将 OpenAI 请求转换为 Ollama Chat 请求
func ConvertOpenAIToOllamaChat(req *model.ChatCompletionRequest) *OllamaChatRequest {
	ollamaReq := &OllamaChatRequest{
		Model:    req.Model,
		Messages: make([]OllamaMessage, 0, len(req.Messages)),
		Stream:   req.Stream,
	}

	// 转换选项
	if req.Temperature != nil {
		ollamaReq.Options.Temperature = float64(*req.Temperature)
	}
	if req.TopP != nil {
		ollamaReq.Options.TopP = float64(*req.TopP)
	}
	if req.MaxTokens > 0 {
		ollamaReq.Options.NumPredict = req.MaxTokens
	}


	// 转换消息
	for _, msg := range req.Messages {
		ollamaMsg := OllamaMessage{
			Role: msg.Role,
		}

		// 处理内容
		switch content := msg.Content.(type) {
		case string:
			ollamaMsg.Content = content
		case []interface{}:
			// 多模态内容处理
			ollamaMsg = convertMultimodalContent(content, msg.Role)
		}

		ollamaReq.Messages = append(ollamaReq.Messages, ollamaMsg)
	}

	return ollamaReq
}

// ConvertOpenAIToOllamaGenerate 将 OpenAI 请求转换为 Ollama Generate 请求
func ConvertOpenAIToOllamaGenerate(req *model.ChatCompletionRequest) *OllamaGenerateRequest {
	ollamaReq := &OllamaGenerateRequest{
		Model:  req.Model,
		Stream: req.Stream,
	}

	// 转换选项
	if req.Temperature != nil {
		ollamaReq.Options.Temperature = float64(*req.Temperature)
	}
	if req.TopP != nil {
		ollamaReq.Options.TopP = float64(*req.TopP)
	}
	if req.MaxTokens > 0 {
		ollamaReq.Options.NumPredict = req.MaxTokens
	}

	// 构建 prompt（将消息列表合并为单个 prompt）
	prompt, system := buildOllamaPrompt(req.Messages)
	ollamaReq.Prompt = prompt
	if system != "" {
		ollamaReq.System = system
	}

	return ollamaReq
}

// ConvertOllamaChatToOpenAI 将 Ollama Chat 响应转换为 OpenAI 格式
func ConvertOllamaChatToOpenAI(resp *OllamaChatResponse, modelName string) *model.ChatCompletionResponse {
	finishReason := "stop"
	if !resp.Done {
		finishReason = ""
	}

	created := time.Now().Unix()
	if resp.CreatedAt != "" {
		// 尝试解析时间
		if t, err := time.Parse(time.RFC3339, resp.CreatedAt); err == nil {
			created = t.Unix()
		}
	}

	return &model.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: created,
		Model:   modelName,
		Choices: []model.ChatCompletionChoice{
			{
				Index: 0,
				Message: model.Message{
					Role:    resp.Message.Role,
					Content: resp.Message.Content,
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

// ConvertOllamaGenerateToOpenAI 将 Ollama Generate 响应转换为 OpenAI 格式
func ConvertOllamaGenerateToOpenAI(resp *OllamaGenerateResponse, modelName string) *model.ChatCompletionResponse {
	finishReason := "stop"
	if !resp.Done {
		finishReason = ""
	}

	created := time.Now().Unix()
	if resp.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, resp.CreatedAt); err == nil {
			created = t.Unix()
		}
	}

	return &model.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: created,
		Model:   modelName,
		Choices: []model.ChatCompletionChoice{
			{
				Index: 0,
				Message: model.Message{
					Role:    "assistant",
					Content: resp.Response,
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

// ConvertOllamaListToOpenAI 将 Ollama 模型列表转换为 OpenAI 格式
func ConvertOllamaListToOpenAI(resp *OllamaListResponse) *model.ListModelsResponse {
	models := make([]model.ModelInfo, 0, len(resp.Models))

	for _, m := range resp.Models {
		created := time.Now().Unix()
		if m.ModifiedAt != "" {
			if t, err := time.Parse(time.RFC3339, m.ModifiedAt); err == nil {
				created = t.Unix()
			}
		}

		models = append(models, model.ModelInfo{
			ID:      m.Name,
			Object:  "model",
			Created: created,
			OwnedBy: "ollama",
		})
	}

	return &model.ListModelsResponse{
		Object: "list",
		Data:   models,
	}
}

// ConvertOllamaEmbeddingToOpenAI 将 Ollama Embedding 响应转换为 OpenAI 格式
func ConvertOllamaEmbeddingToOpenAI(resp *OllamaEmbeddingResponse, modelName string, index int) *model.EmbeddingResponse {
	return &model.EmbeddingResponse{
		Object: "list",
		Data: []model.Embedding{
			{
				Object:    "embedding",
				Embedding: resp.Embedding,
				Index:     index,
			},
		},
		Model: modelName,
		Usage: model.Usage{
			TotalTokens: 0, // Ollama 不返回 token 使用
		},
	}
}

// ==================== 辅助函数 ====================

// convertMultimodalContent 转换多模态内容
func convertMultimodalContent(content []interface{}, role string) OllamaMessage {
	msg := OllamaMessage{
		Role:   role,
		Images: make([]string, 0),
	}

	var textParts []string

	for _, part := range content {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}

		partType, _ := partMap["type"].(string)

		switch partType {
		case "text":
			if text, ok := partMap["text"].(string); ok {
				textParts = append(textParts, text)
			}
		case "image_url":
			if imageURL, ok := partMap["image_url"].(map[string]interface{}); ok {
				if url, ok := imageURL["url"].(string); ok {
					// 提取 base64 图片数据
					if data, err := extractBase64FromDataURL(url); err == nil {
						msg.Images = append(msg.Images, data)
					}
				}
			}
		}
	}

	msg.Content = ""
	for i, text := range textParts {
		if i > 0 {
			msg.Content += " "
		}
		msg.Content += text
	}

	return msg
}

// extractBase64FromDataURL 从 data URL 提取 base64 数据
func extractBase64FromDataURL(dataURL string) (string, error) {
	const prefix = "data:image/"
	if !strings.HasPrefix(dataURL, prefix) {
		return "", fmt.Errorf("not a data URL")
	}

	// 找到 base64, 后的内容
	idx := strings.Index(dataURL, "base64,")
	if idx == -1 {
		return "", fmt.Errorf("invalid data URL format")
	}

	return dataURL[idx+7:], nil // 跳过 "base64,"
}

// buildOllamaPrompt 构建 Ollama prompt
func buildOllamaPrompt(messages []model.Message) (prompt, system string) {
	var prompts []string

	for _, msg := range messages {
		content := ""
		switch c := msg.Content.(type) {
		case string:
			content = c
		case []interface{}:
			// 提取文本内容
			for _, part := range c {
				if partMap, ok := part.(map[string]interface{}); ok {
					if partType, ok := partMap["type"].(string); ok && partType == "text" {
						if text, ok := partMap["text"].(string); ok {
							content += text + " "
						}
					}
				}
			}
		}

		switch msg.Role {
		case "system":
			system = content
		case "user":
			prompts = append(prompts, fmt.Sprintf("User: %s", content))
		case "assistant":
			prompts = append(prompts, fmt.Sprintf("Assistant: %s", content))
		}
	}

	// 添加 Assistant 前缀以引导生成
	prompts = append(prompts, "Assistant:")

	return strings.Join(prompts, "\n"), system
}

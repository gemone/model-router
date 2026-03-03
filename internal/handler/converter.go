package handler

import (
	"strings"

	"github.com/gemone/model-router/internal/model"
)

// APIFormat defines the API format type
type APIFormat string

const (
	APIFormatOpenAI    APIFormat = "openai"
	APIFormatAnthropic APIFormat = "anthropic"
	APIFormatClaude    APIFormat = "claude" // Alias for Anthropic
)

// AnthropicMessage represents a message in Anthropic's format
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicRequest represents a request in Anthropic's format
type AnthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []AnthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

// AnthropicContent represents content in Anthropic's response format
type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicUsage represents token usage in Anthropic's format
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicResponse represents a response in Anthropic's format
type AnthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []AnthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason,omitempty"`
	Usage      AnthropicUsage     `json:"usage"`
}

// ConvertAnthropicToOpenAI converts an Anthropic format request to OpenAI format
func ConvertAnthropicToOpenAI(req *AnthropicRequest) *model.ChatCompletionRequest {
	messages := make([]model.Message, 0, len(req.Messages)+1)

	// Add system message if present
	if req.System != "" {
		messages = append(messages, model.Message{
			Role:    "system",
			Content: req.System,
		})
	}

	// Add conversation messages
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, model.Message{
			Role:    role,
			Content: m.Content,
		})
	}

	return &model.ChatCompletionRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}
}

// ConvertOpenAIToAnthropic converts an OpenAI format response to Anthropic format
func ConvertOpenAIToAnthropic(resp *model.ChatCompletionResponse) *AnthropicResponse {
	if len(resp.Choices) == 0 {
		return &AnthropicResponse{
			ID:   resp.ID,
			Type: "message",
			Role: "assistant",
		}
	}

	choice := resp.Choices[0]
	content := ""
	if c, ok := choice.Message.Content.(string); ok {
		content = c
	}

	stopReason := "end_turn"
	switch choice.FinishReason {
	case "length":
		stopReason = "max_tokens"
	case "stop":
		stopReason = "end_turn"
	}

	return &AnthropicResponse{
		ID:   resp.ID,
		Type: "message",
		Role: "assistant",
		Content: []AnthropicContent{
			{Type: "text", Text: content},
		},
		Model:      resp.Model,
		StopReason: stopReason,
		Usage: AnthropicUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}

// GetAPIFormatFromPath extracts the API format from the request path
// Returns "openai" for /api/openai/... or default paths
// Returns "anthropic" for /api/claude/... or /api/anthropic/...
func GetAPIFormatFromPath(path string) APIFormat {
	// Check for Anthropic/Claude format
	segments := strings.Split(path, "/")
	for _, seg := range segments {
		if seg == "claude" || seg == "anthropic" {
			return APIFormatAnthropic
		}
	}
	// Default to OpenAI format
	return APIFormatOpenAI
}

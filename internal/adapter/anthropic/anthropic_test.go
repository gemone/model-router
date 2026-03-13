package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/gemone/model-router/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestConvertContentToClaudeFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "string content",
			input:    "Hello, Claude!",
			expected: "Hello, Claude!",
		},
		{
			name: "ContentPart array with text",
			input: []model.ContentPart{
				{Type: "text", Text: "Hello"},
				{Type: "text", Text: " World"},
			},
			expected: []contentBlock{
				{Type: "text", Text: "Hello"},
				{Type: "text", Text: " World"},
			},
		},
		{
			name: "ContentPart array with image",
			input: []model.ContentPart{
				{Type: "text", Text: "Look at this image:"},
				{Type: "image_url", ImageURL: model.ImageURL{URL: "https://example.com/image.png"}},
			},
			expected: []contentBlock{
				{Type: "text", Text: "Look at this image:"},
				{Type: "image", URL: "https://example.com/image.png"},
			},
		},
		{
			name:     "empty interface array",
			input:    []interface{}{},
			expected: []interface{}{},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertContentToClaudeFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToolChoiceToClaudeFormat(t *testing.T) {
	tests := []struct {
		name       string
		toolChoice interface{}
		expected   interface{}
	}{
		{
			name:       "none",
			toolChoice: "none",
			expected:   "none",
		},
		{
			name:       "auto",
			toolChoice: "auto",
			expected:   "auto",
		},
		{
			name:       "required",
			toolChoice: "required",
			expected:   "any",
		},
		{
			name:       "unknown string",
			toolChoice: "unknown",
			expected:   "auto",
		},
		{
			name: "specific function",
			toolChoice: map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name": "get_weather",
				},
			},
			expected: map[string]interface{}{
				"type": "tool",
				"name": "get_weather",
			},
		},
		{
			name:       "nil",
			toolChoice: nil,
			expected:   "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToolChoiceToClaudeFormat(tt.toolChoice)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClaudeAdapter_convertRequest(t *testing.T) {
	adapter := &ClaudeAdapter{}

	t.Run("basic request", func(t *testing.T) {
		temp := float32(0.8)
		topP := float32(0.9)
		req := &model.ChatCompletionRequest{
			Model:       "claude-3-opus",
			Messages:    []model.Message{
				{Role: "user", Content: "Hello!"},
			},
			Temperature: &temp,
			TopP:        &topP,
			MaxTokens:   1024,
			Stream:      false,
		}

		claudeReq := adapter.convertRequest(req)

		assert.Equal(t, "claude-3-opus", claudeReq.Model)
		assert.Equal(t, float32(0.8), claudeReq.Temperature)
		assert.Equal(t, float32(0.9), claudeReq.TopP)
		assert.Equal(t, 1024, claudeReq.MaxTokens)
		assert.False(t, claudeReq.Stream)
		assert.Len(t, claudeReq.Messages, 1)
	})

	t.Run("request with system message", func(t *testing.T) {
		req := &model.ChatCompletionRequest{
			Model: "claude-3-opus",
			Messages: []model.Message{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "Hello!"},
			},
		}

		claudeReq := adapter.convertRequest(req)

		assert.Equal(t, "You are a helpful assistant.", claudeReq.System)
		assert.Len(t, claudeReq.Messages, 1) // system message is extracted, only user message remains
	})

	t.Run("request with tools", func(t *testing.T) {
		req := &model.ChatCompletionRequest{
			Model: "claude-3-opus",
			Messages: []model.Message{
				{Role: "user", Content: "What's the weather?"},
			},
			Tools: []model.Tool{
				{
					Type: "function",
					Function: model.Function{
						Name:        "get_weather",
						Description: "Get weather information",
						Parameters: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
			ToolChoice: "auto",
		}

		claudeReq := adapter.convertRequest(req)

		assert.Len(t, claudeReq.Tools, 1)
		assert.Equal(t, "get_weather", claudeReq.Tools[0].Name)
		assert.Equal(t, "auto", claudeReq.ToolChoice)
	})

	t.Run("request with multimodal content", func(t *testing.T) {
		req := &model.ChatCompletionRequest{
			Model: "claude-3-opus",
			Messages: []model.Message{
				{
					Role: "user",
					Content: []model.ContentPart{
						{Type: "text", Text: "Describe this image:"},
						{Type: "image_url", ImageURL: model.ImageURL{URL: "https://example.com/image.png"}},
					},
				},
			},
		}

		claudeReq := adapter.convertRequest(req)

		assert.Len(t, claudeReq.Messages, 1)
		blocks, ok := claudeReq.Messages[0].Content.([]contentBlock)
		assert.True(t, ok)
		assert.Len(t, blocks, 2)
		assert.Equal(t, "text", blocks[0].Type)
		assert.Equal(t, "Describe this image:", blocks[0].Text)
		assert.Equal(t, "image", blocks[1].Type)
		assert.Equal(t, "https://example.com/image.png", blocks[1].URL)
	})
}

func TestClaudeAdapter_convertResponse(t *testing.T) {
	adapter := &ClaudeAdapter{}

	t.Run("basic response", func(t *testing.T) {
		resp := &claudeResponse{
			ID:         "msg_123",
			Type:       "message",
			Role:       "assistant",
			Content:    []contentBlock{
				{Type: "text", Text: "Hello! How can I help you?"},
			},
			Model:      "claude-3-opus",
			StopReason: "end_turn",
			Usage: claudeUsage{
				InputTokens:  10,
				OutputTokens: 20,
			},
		}

		result := adapter.convertResponse(resp, "claude-3-opus")

		assert.Equal(t, "chat.completion", result.Object)
		assert.Equal(t, "claude-3-opus", result.Model)
		assert.Len(t, result.Choices, 1)
		assert.Equal(t, "Hello! How can I help you?", result.Choices[0].Message.Content)
		assert.Equal(t, "stop", result.Choices[0].FinishReason)
		assert.Equal(t, 10, result.Usage.PromptTokens)
		assert.Equal(t, 20, result.Usage.CompletionTokens)
		assert.Equal(t, 30, result.Usage.TotalTokens)
	})

	t.Run("response with tool use", func(t *testing.T) {
		inputJSON := json.RawMessage(`{"location":"New York"}`)
		resp := &claudeResponse{
			ID:         "msg_456",
			Type:       "message",
			Role:       "assistant",
			Content:    []contentBlock{
				{Type: "tool_use", ID: "tool_1", Name: "get_weather", Input: inputJSON},
			},
			Model:      "claude-3-opus",
			StopReason: "tool_use",
			Usage: claudeUsage{
				InputTokens:  15,
				OutputTokens: 25,
			},
		}

		result := adapter.convertResponse(resp, "claude-3-opus")

		assert.Len(t, result.Choices, 1)
		assert.Equal(t, "tool_calls", result.Choices[0].FinishReason)
		assert.Len(t, result.Choices[0].Message.ToolCalls, 1)
		assert.Equal(t, "tool_1", result.Choices[0].Message.ToolCalls[0].ID)
		assert.Equal(t, "get_weather", result.Choices[0].Message.ToolCalls[0].Function.Name)
	})

	t.Run("max_tokens stop reason", func(t *testing.T) {
		resp := &claudeResponse{
			ID:         "msg_789",
			Type:       "message",
			Role:       "assistant",
			Content:    []contentBlock{
				{Type: "text", Text: "This is a partial response..."},
			},
			Model:      "claude-3-opus",
			StopReason: "max_tokens",
			Usage:      claudeUsage{InputTokens: 10, OutputTokens: 100},
		}

		result := adapter.convertResponse(resp, "claude-3-opus")

		assert.Equal(t, "length", result.Choices[0].FinishReason)
	})
}

func TestClaudeAdapter_convertStreamResponse(t *testing.T) {
	adapter := &ClaudeAdapter{}

	t.Run("message_start", func(t *testing.T) {
		resp := &claudeStreamResponse{
			Type: "message_start",
			Message: &claudeResponse{
				ID: "msg_stream_1",
			},
		}

		result := adapter.convertStreamResponse(resp, "claude-3-opus")

		assert.NotNil(t, result)
		assert.Equal(t, "chat.completion.chunk", result.Object)
		assert.Equal(t, "assistant", result.Choices[0].Delta.Role)
	})

	t.Run("content_block_delta", func(t *testing.T) {
		resp := &claudeStreamResponse{
			Type: "content_block_delta",
			Delta: &delta{
				Type: "text",
				Text: "Hello",
			},
		}

		result := adapter.convertStreamResponse(resp, "claude-3-opus")

		assert.NotNil(t, result)
		assert.Equal(t, "Hello", result.Choices[0].Delta.Content)
	})

	t.Run("message_delta with stop", func(t *testing.T) {
		finishReason := "stop"
		resp := &claudeStreamResponse{
			Type: "message_delta",
			Delta: &delta{
				StopReason: "end_turn",
			},
		}

		result := adapter.convertStreamResponse(resp, "claude-3-opus")

		assert.NotNil(t, result)
		assert.Equal(t, &finishReason, result.Choices[0].FinishReason)
	})

	t.Run("message_delta with max_tokens", func(t *testing.T) {
		finishReason := "length"
		resp := &claudeStreamResponse{
			Type: "message_delta",
			Delta: &delta{
				StopReason: "max_tokens",
			},
		}

		result := adapter.convertStreamResponse(resp, "claude-3-opus")

		assert.NotNil(t, result)
		assert.Equal(t, &finishReason, result.Choices[0].FinishReason)
	})

	t.Run("message_delta with tool_use", func(t *testing.T) {
		finishReason := "tool_calls"
		resp := &claudeStreamResponse{
			Type: "message_delta",
			Delta: &delta{
				StopReason: "tool_use",
			},
		}

		result := adapter.convertStreamResponse(resp, "claude-3-opus")

		assert.NotNil(t, result)
		assert.Equal(t, &finishReason, result.Choices[0].FinishReason)
	})

	t.Run("message_stop", func(t *testing.T) {
		resp := &claudeStreamResponse{
			Type: "message_stop",
		}

		result := adapter.convertStreamResponse(resp, "claude-3-opus")

		assert.Nil(t, result)
	})

	t.Run("unknown type", func(t *testing.T) {
		resp := &claudeStreamResponse{
			Type: "unknown_type",
		}

		result := adapter.convertStreamResponse(resp, "claude-3-opus")

		assert.Nil(t, result)
	})
}

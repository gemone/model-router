package handler

import (
	"encoding/json"
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

// AnthropicContentPart represents a part of content in Anthropic's format
type AnthropicContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicMessage represents a message in Anthropic's format
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []AnthropicContentPart
}

// GetContentString extracts text content from message
// For messages with content blocks, extracts text from text blocks
func (m *AnthropicMessage) GetContentString() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []interface{}:
		// Content is an array of parts
		var result string
		for _, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
				if blockType, ok := partMap["type"].(string); ok {
					switch blockType {
					case "text":
						if text, ok := partMap["text"].(string); ok {
							result += text
						}
					case "tool_result":
						// For tool_result, extract content as string representation
						if content, ok := partMap["content"]; ok {
							switch c := content.(type) {
							case string:
								result += c
							case []interface{}:
								for _, item := range c {
									if itemMap, ok := item.(map[string]interface{}); ok && itemMap["type"] == "text" {
										if text, ok := itemMap["text"].(string); ok {
											result += text
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return result
	default:
		return ""
	}
}

// AnthropicTool represents a tool definition in Anthropic's format
type AnthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"input_schema"`
}

// AnthropicRequest represents a request in Anthropic's format
type AnthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []AnthropicMessage `json:"messages"`
	System    interface{}        `json:"system,omitempty"` // Can be string or []AnthropicContentPart
	MaxTokens int                `json:"max_tokens,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
	Tools     []AnthropicTool    `json:"tools,omitempty"`
}

// AnthropicContent represents content in Anthropic's format (for both request and response)
type AnthropicContent struct {
	Type       string          `json:"type"`                  // "text", "tool_use", "tool_result", "image"
	Text       string          `json:"text,omitempty"`        // for text type
	ID         string          `json:"id,omitempty"`          // for tool_use (tool call id)
	Name       string          `json:"name,omitempty"`        // for tool_use (tool name)
	Input      json.RawMessage `json:"input,omitempty"`       // for tool_use (tool arguments)
	ToolUseID  string          `json:"tool_use_id,omitempty"` // for tool_result
	Content    interface{}     `json:"content,omitempty"`     // for tool_result (can be string or array)
	IsError    bool            `json:"is_error,omitempty"`    // for tool_result
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

	// Add system message if present (can be string or array)
	if req.System != nil {
		systemContent := ""
		switch v := req.System.(type) {
		case string:
			systemContent = v
		case []interface{}:
			// System is an array of content parts
			for _, part := range v {
				if partMap, ok := part.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						systemContent += text
					}
				}
			}
		}
		if systemContent != "" {
			messages = append(messages, model.Message{
				Role:    "system",
				Content: systemContent,
			})
		}
	}

	// Add conversation messages
	for _, m := range req.Messages {
		role := m.Role
		
		// Handle content blocks (for tool_use and tool_result)
		if contentBlocks, ok := m.Content.([]interface{}); ok {
			// Check for tool_result blocks in user messages
			if role == "user" {
				for _, block := range contentBlocks {
					if blockMap, ok := block.(map[string]interface{}); ok {
						blockType, _ := blockMap["type"].(string)
						
						switch blockType {
						case "tool_result":
							// Convert tool_result to OpenAI tool role message
							toolUseID, _ := blockMap["tool_use_id"].(string)
							content := extractToolResultContent(blockMap["content"])

							messages = append(messages, model.Message{
								Role:       "tool",
								Content:    content,
								ToolCallID: toolUseID,
							})
						case "text":
							// Regular text content
							text, _ := blockMap["text"].(string)
							messages = append(messages, model.Message{
								Role:    "user",
								Content: text,
							})
						}
					}
				}
			} else {
				// For assistant messages, extract text and tool_calls
				var textContent string
				var toolCalls []model.ToolCall
				
				for _, block := range contentBlocks {
					if blockMap, ok := block.(map[string]interface{}); ok {
						blockType, _ := blockMap["type"].(string)
						
						switch blockType {
						case "text":
							if text, ok := blockMap["text"].(string); ok {
								textContent += text
							}
						case "tool_use":
							// Extract tool call
							toolID, _ := blockMap["id"].(string)
							toolName, _ := blockMap["name"].(string)
							inputJSON, _ := json.Marshal(blockMap["input"])
							
							toolCalls = append(toolCalls, model.ToolCall{
								ID:   toolID,
								Type: "function",
								Function: model.Function{
									Name:      toolName,
									Arguments: string(inputJSON),
								},
							})
						}
					}
				}
				
				msg := model.Message{
					Role:      role,
					Content:   textContent,
					ToolCalls: toolCalls,
				}
				messages = append(messages, msg)
			}
		} else {
			// Simple string content
			messages = append(messages, model.Message{
				Role:    role,
				Content: m.GetContentString(),
			})
		}
	}

	// Convert tools to OpenAI format
	var tools []model.Tool
	for _, t := range req.Tools {
		tools = append(tools, model.Tool{
			Type: "function",
			Function: model.Function{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	return &model.ChatCompletionRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
		Tools:     tools,
	}
}

// extractToolResultContent extracts string content from tool_result content field
func extractToolResultContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var result string
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["type"] == "text" {
					if text, ok := itemMap["text"].(string); ok {
						result += text
					}
				}
			}
		}
		return result
	default:
		return ""
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
	
	// Build content blocks
	var contentBlocks []AnthropicContent
	
	// Add text content if present
	if c, ok := choice.Message.Content.(string); ok && c != "" {
		contentBlocks = append(contentBlocks, AnthropicContent{
			Type: "text",
			Text: c,
		})
	}
	
	// Add tool_use content blocks if present
	for _, toolCall := range choice.Message.ToolCalls {
		// Parse arguments JSON string to RawMessage
		var input json.RawMessage
		if toolCall.Function.Arguments != "" {
			input = json.RawMessage(toolCall.Function.Arguments)
		} else {
			input = json.RawMessage("{}")
		}
		
		contentBlocks = append(contentBlocks, AnthropicContent{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Input: input,
		})
	}
	
	// If no content blocks, add empty text
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, AnthropicContent{
			Type: "text",
			Text: "",
		})
	}

	stopReason := "end_turn"
	switch choice.FinishReason {
	case "length":
		stopReason = "max_tokens"
	case "stop":
		stopReason = "end_turn"
	case "tool_calls":
		stopReason = "tool_use"
	}

	return &AnthropicResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
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

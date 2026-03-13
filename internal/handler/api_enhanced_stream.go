package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
)

// handleClaudeStreaming handles Claude/Anthropic format streaming responses
func (h *EnhancedAPIHandler) handleClaudeStreaming(
	c fiber.Ctx,
	routeResult *service.RouteResult,
	req *model.ChatCompletionRequest,
) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	stream, err := routeResult.Adapter.ChatCompletionStream(c.Context(), req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	requestID := uuid.New().String()
	modelName := req.Model
	if routeResult.Model != nil {
		modelName = routeResult.Model.Name
	}

	c.Status(http.StatusOK).RequestCtx().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		// 1. message_start
		messageStart := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":              requestID,
				"type":            "message",
				"role":            "assistant",
				"model":           modelName,
				"content":         []map[string]interface{}{},
				"stop_reason":     nil,
				"stop_sequence":   nil,
				"usage":           map[string]interface{}{"input_tokens": 0, "output_tokens": 0},
			},
		}
		data, _ := json.Marshal(messageStart)
		fmt.Fprintf(w, "event: message_start\ndata: %s\n\n", string(data))
		w.Flush()

		// Send ping to keep connection alive (like Anthropic does)
		fmt.Fprintf(w, "event: ping\ndata: %s\n\n", `{"type": "ping"}`)
		w.Flush()

		// Track state
		var (
			contentBlockIndex  = 0
			currentTextContent strings.Builder
			toolCallStates     = make(map[int]*toolCallState) // index -> state
			hasSentTextBlock   = false
			textBlockClosed    = false
			totalOutputTokens  = 0
			hasSentStopReason  = false
			lastToolIndex      = 0
		)

		for resp := range stream {
			if len(resp.Choices) == 0 {
				continue
			}

			choice := resp.Choices[0]
			delta := choice.Delta

			// Get finish reason
			finishReason := ""
			if choice.FinishReason != nil {
				finishReason = *choice.FinishReason
			}

			// Get actual content (Content or ReasoningContent for GLM)
			content := delta.Content
			if content == "" {
				content = delta.ReasoningContent
			}

			// Handle text content
			if content != "" && !textBlockClosed {
				if !hasSentTextBlock {
					// Send content_block_start for text
					contentBlockStart := map[string]interface{}{
						"type":  "content_block_start",
						"index": contentBlockIndex,
						"content_block": map[string]interface{}{
							"type": "text",
							"text": "",
						},
					}
					data, _ := json.Marshal(contentBlockStart)
					fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", string(data))
					w.Flush()
					hasSentTextBlock = true
				}
				currentTextContent.WriteString(content)
				claudeEvent := map[string]interface{}{
					"type":  "content_block_delta",
					"index": contentBlockIndex,
					"delta": map[string]interface{}{
						"type": "text_delta",
						"text": content,
					},
				}
				data, _ := json.Marshal(claudeEvent)
				fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", string(data))
				w.Flush()
			}

			// Handle tool calls
			if len(delta.ToolCalls) > 0 {
				// First tool call - close text block
				if hasSentTextBlock && !textBlockClosed {
					textBlockClosed = true
					contentBlockStop := map[string]interface{}{
						"type":  "content_block_stop",
						"index": contentBlockIndex,
					}
					data, _ := json.Marshal(contentBlockStop)
					fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", string(data))
					w.Flush()
					contentBlockIndex++
				}

				for _, tc := range delta.ToolCalls {
					toolIndex := tc.Index

					// Check if this is a new tool call
					if _, exists := toolCallStates[toolIndex]; !exists {
						// New tool call
						toolCallStates[toolIndex] = &toolCallState{
							id:   tc.ID,
							name: tc.Function.Name,
						}
						lastToolIndex = contentBlockIndex + toolIndex

						// Send content_block_start for tool_use
						contentBlockStart := map[string]interface{}{
							"type":  "content_block_start",
							"index": lastToolIndex,
							"content_block": map[string]interface{}{
								"type":  "tool_use",
								"id":    tc.ID,
								"name":  tc.Function.Name,
								"input": map[string]interface{}{},
							},
						}
						data, _ := json.Marshal(contentBlockStart)
						fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n", string(data))
						w.Flush()
					}

					// Send input_json_delta for arguments
					if tc.Function.Arguments != "" {
						toolCallStates[toolIndex].inputBuilder.WriteString(tc.Function.Arguments)
						claudeEvent := map[string]interface{}{
							"type":  "content_block_delta",
							"index": lastToolIndex,
							"delta": map[string]interface{}{
								"type":         "input_json_delta",
								"partial_json": tc.Function.Arguments,
							},
						}
						data, _ := json.Marshal(claudeEvent)
						fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n", string(data))
						w.Flush()
					}
				}
			}

			// Update output tokens
			totalOutputTokens += len(content) / 4
			for _, tc := range delta.ToolCalls {
				totalOutputTokens += len(tc.Function.Arguments) / 4
			}

			// Handle finish reason
			if finishReason != "" && !hasSentStopReason {
				hasSentStopReason = true

				// Close text block if still open
				if hasSentTextBlock && !textBlockClosed {
					textBlockClosed = true
					contentBlockStop := map[string]interface{}{
						"type":  "content_block_stop",
						"index": 0,
					}
					data, _ := json.Marshal(contentBlockStop)
					fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", string(data))
					w.Flush()
				}

				// Close all tool call blocks
				for i := range toolCallStates {
					idx := contentBlockIndex + i
					if i == 0 && !hasSentTextBlock {
						idx = 0
					}
					contentBlockStop := map[string]interface{}{
						"type":  "content_block_stop",
						"index": idx,
					}
					data, _ := json.Marshal(contentBlockStop)
					fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", string(data))
					w.Flush()
				}

				// Map finish reason
				stopReason := "end_turn"
				if finishReason == "length" {
					stopReason = "max_tokens"
				} else if finishReason == "tool_calls" {
					stopReason = "tool_use"
				}

				// Send message_delta
				messageDelta := map[string]interface{}{
					"type": "message_delta",
					"delta": map[string]interface{}{
						"stop_reason":   stopReason,
						"stop_sequence": nil,
					},
					"usage": map[string]interface{}{
						"output_tokens": totalOutputTokens,
					},
				}
				data, _ := json.Marshal(messageDelta)
				fmt.Fprintf(w, "event: message_delta\ndata: %s\n\n", string(data))
				w.Flush()

				// Send message_stop
				fmt.Fprint(w, "event: message_stop\ndata: {\"type\": \"message_stop\"}\n\n")
				w.Flush()

				// Send final [DONE] marker
				fmt.Fprint(w, "data: [DONE]\n\n")
				w.Flush()
				return
			}
		}

		// End of stream - close any open blocks
		if !hasSentStopReason {
			// Close text block if still open
			if hasSentTextBlock && !textBlockClosed {
				contentBlockStop := map[string]interface{}{
					"type":  "content_block_stop",
					"index": 0,
				}
				data, _ := json.Marshal(contentBlockStop)
				fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", string(data))
				w.Flush()
			}

			// Close all tool call blocks
			for i := range toolCallStates {
				idx := contentBlockIndex + i
				if i == 0 && !hasSentTextBlock {
					idx = 0
				}
				contentBlockStop := map[string]interface{}{
					"type":  "content_block_stop",
					"index": idx,
				}
				data, _ := json.Marshal(contentBlockStop)
				fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n", string(data))
				w.Flush()
			}

			// Send message_delta
			stopReason := "end_turn"
			if len(toolCallStates) > 0 {
				stopReason = "tool_use"
			}
			messageDelta := map[string]interface{}{
				"type": "message_delta",
				"delta": map[string]interface{}{
					"stop_reason":   stopReason,
					"stop_sequence": nil,
				},
				"usage": map[string]interface{}{
					"output_tokens": totalOutputTokens,
				},
			}
			data, _ := json.Marshal(messageDelta)
			fmt.Fprintf(w, "event: message_delta\ndata: %s\n\n", string(data))
			w.Flush()

			// Send message_stop
			fmt.Fprint(w, "event: message_stop\ndata: {\"type\": \"message_stop\"}\n\n")
			w.Flush()

			// Send final [DONE] marker
			fmt.Fprint(w, "data: [DONE]\n\n")
			w.Flush()
		}
	}))

	return nil
}

// toolCallState tracks the state of a streaming tool call
type toolCallState struct {
	id          string
	name        string
	inputBuilder strings.Builder
}

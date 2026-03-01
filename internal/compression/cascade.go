// Package compression implements cascade compression with expert model prompt optimization.
package compression

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

// CascadeCompression implements cascade compression with expert model prompt optimization.
// Strategy: Use strong model (e.g., GPT-4) to optimize context, then feed to weak model (e.g., GPT-3.5)
type CascadeCompression struct {
	expertAdapter   adapter.Adapter // Strong model for optimization
	workerAdapter   adapter.Adapter // Regular model for execution
	expertModel      string          // Model name for expert optimization
	workerModel      string          // Model name for regular execution
	maxOptimizeTokens int            // Maximum tokens for optimization output
}

// CascadeCompressionConfig configures the cascade compression behavior
type CascadeCompressionConfig struct {
	ExpertAdapter    adapter.Adapter // Strong model (GPT-4, Claude, etc.)
	WorkerAdapter    adapter.Adapter // Regular model (GPT-3.5, etc.)
	ExpertModel      string          // Expert model name
	WorkerModel      string          // Worker model name
	MaxOptimizeTokens int            // Max tokens for optimization output
}

// NewCascadeCompression creates a new cascade compression instance
func NewCascadeCompression(config *CascadeCompressionConfig) *CascadeCompression {
	if config == nil {
		return nil
	}

	return &CascadeCompression{
		expertAdapter:   config.ExpertAdapter,
		workerAdapter:   config.WorkerAdapter,
		expertModel:      config.ExpertModel,
		workerModel:      config.WorkerModel,
		maxOptimizeTokens: config.MaxOptimizeTokens,
	}
}

// CascadeResult represents the result of cascade compression
type CascadeResult struct {
	OriginalMessages   []model.Message  // Original conversation
	OptimizedContext   string          // Optimized context by expert model
	OptimizedPrompt     string          // Optimized prompt for worker model
	OriginalTokens      int             // Original token count
	OptimizedTokens     int             // Optimized token count
	CompressionRatio    float64         // Compression ratio
	QualityScore        float64         // Estimated quality improvement
}

// OptimizeWithContext uses expert model to optimize the conversation context
func (c *CascadeCompression) OptimizeWithContext(ctx context.Context, messages []model.Message) (*CascadeResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to optimize")
	}

	// Calculate original tokens
	originalTokens := c.estimateMessagesTokens(messages)

	// Build optimization prompt for expert model
	optimizationPrompt := c.buildOptimizationPrompt(messages)

	// Call expert model to optimize context
	expertRequest := &model.ChatCompletionRequest{
		Model: c.expertModel,
		Messages: []model.Message{
			{
				Role:    "system",
				Content: "You are an expert prompt engineer specializing in optimizing LLM conversations for better instruction following and output quality. Analyze the conversation and extract key information, reformulate instructions clearly, and organize context logically.",
			},
			{
				Role:    "user",
				Content: optimizationPrompt,
			},
		},
		MaxTokens:     c.maxOptimizeTokens,
		Temperature:   func() *float32 { t := float32(0.3); return &t }(),
	}

	expertResponse, err := c.expertAdapter.ChatCompletion(ctx, expertRequest)
	if err != nil {
		return nil, fmt.Errorf("expert model optimization failed: %w", err)
	}

	if len(expertResponse.Choices) == 0 {
		return nil, fmt.Errorf("expert model returned no response")
	}

	contentStr, ok := expertResponse.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("expert model response is not a string")
	}

	optimizedContext := strings.TrimSpace(contentStr)

	// Build optimized prompt for worker model
	optimizedPrompt := c.buildWorkerPrompt(optimizedContext, messages)

	result := &CascadeResult{
		OriginalMessages: messages,
		OptimizedContext:  optimizedContext,
		OptimizedPrompt:    optimizedPrompt,
		OriginalTokens:     originalTokens,
		OptimizedTokens:    c.estimateTokens(optimizedPrompt),
		CompressionRatio:   float64(c.estimateTokens(optimizedPrompt)) / float64(originalTokens),
		QualityScore:       c.calculateQualityScore(messages, optimizedContext),
	}

	return result, nil
}

// buildOptimizationPrompt creates the optimization prompt for the expert model
func (c *CascadeCompression) buildOptimizationPrompt(messages []model.Message) string {
	var sb strings.Builder

	sb.WriteString("# Conversation Optimization Task\n\n")
	sb.WriteString("Please analyze the following conversation and produce an optimized version that:\n")
	sb.WriteString("1. Preserves all critical information, decisions, and action items\n")
	sb.WriteString("2. Improves instruction clarity and structure\n")
	sb.WriteString("3. Eliminates redundancy while maintaining context\n")
	sb.WriteString("4. Organizes information logically for better LLM understanding\n\n")
	sb.WriteString("## Original Conversation\n\n")

	// Include conversation (may be truncated if too long)
	includedTokens := 0
	maxInputTokens := 100000 // Cap input for expert model

	for _, msg := range messages {
		msgTokens := c.estimateMessageTokens(&msg)
		if includedTokens+msgTokens > maxInputTokens && includedTokens > 0 {
			sb.WriteString("\n[... Earlier conversation truncated for optimization ...]\n")
			break
		}

		content := c.contentToString(msg.Content)
		if len(content) > 2000 {
			content = content[:1970] + "...\n[truncated]"
		}

		sb.WriteString(fmt.Sprintf("**[%s]**: %s\n\n", msg.Role, content))
		includedTokens += msgTokens
	}

	sb.WriteString("\n## Optimization Output Format\n\n")
	sb.WriteString("Please provide the optimized context in the following structure:\n\n")
	sb.WriteString("### Optimized Context\n")
	sb.WriteString("[Provide a clear, well-structured summary with sections for:]\n")
	sb.WriteString("- Core task/objective\n")
	sb.WriteString("- Key decisions made\n")
	sb.WriteString("- Action items and requirements\n")
	sb.WriteString("- Important constraints or preferences\n")
	sb.WriteString("- Relevant context from earlier discussion\n\n")

	return sb.String()
}

// buildWorkerPrompt builds the optimized prompt for the worker model
func (c *CascadeCompression) buildWorkerPrompt(optimizedContext string, originalMessages []model.Message) string {
	var sb strings.Builder

	// Add the optimized context as a system message
	sb.WriteString(fmt.Sprintf("=== Optimized Context ===\n%s\n\n", optimizedContext))

	// Get the most recent user message (the actual request)
	lastUserMessage := c.getLastUserMessage(originalMessages)
	if lastUserMessage != "" {
		sb.WriteString(fmt.Sprintf("=== Current Request ===\n%s\n", lastUserMessage))
	}

	sb.WriteString("\n=== Instructions ===\n")
	sb.WriteString("Based on the optimized context above, please provide a helpful and accurate response. ")
	sb.WriteString("The context contains all relevant information from the conversation history. ")

	return sb.String()
}

// getLastUserMessage extracts the last user message from the conversation
func (c *CascadeCompression) getLastUserMessage(messages []model.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return c.contentToString(messages[i].Content)
		}
	}
	return ""
}

// calculateQualityScore estimates the quality improvement score (0-1)
func (c *CascadeCompression) calculateQualityScore(originalMessages []model.Message, optimizedContext string) float64 {
	// Simple heuristic-based scoring
	score := 0.5 // Base score

	// Check for key elements in optimized context
	optimizedLower := strings.ToLower(optimizedContext)

	// Presence of structured sections
	if strings.Contains(optimizedLower, "task") || strings.Contains(optimizedLower, "objective") {
		score += 0.1
	}
	if strings.Contains(optimizedLower, "decision") || strings.Contains(optimizedLower, "agreed") {
		score += 0.1
	}
	if strings.Contains(optimizedLower, "action") || strings.Contains(optimizedLower, "require") {
		score += 0.1
	}
	if strings.Contains(optimizedLower, "context") || strings.Contains(optimizedLower, "background") {
		score += 0.1
	}

	// Check length optimization
	if len(optimizedContext) > 100 && len(optimizedContext) < 5000 {
		score += 0.1
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// EstimateTokensForPrompt estimates tokens for a specific prompt
func (c *CascadeCompression) EstimateTokensForPrompt(prompt string) int {
	return len(prompt)/4 + 10
}

// EstimateTokensForMessages estimates tokens for messages
func (c *CascadeCompression) estimateMessagesTokens(messages []model.Message) int {
	total := 0
	for i := range messages {
		total += c.estimateMessageTokens(&messages[i])
	}
	return total
}

// estimateMessageTokens estimates tokens for a single message
func (c *CascadeCompression) estimateMessageTokens(msg *model.Message) int {
	content := c.contentToString(msg.Content)
	return len(content)/4 + 10
}

// estimateTokens estimates tokens for a string
func (c *CascadeCompression) estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

// contentToString converts message content to string
func (c *CascadeCompression) contentToString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []model.ContentPart:
		var result string
		for _, part := range v {
			if part.Type == "text" {
				result += part.Text + " "
			}
		}
		return result
	default:
		return fmt.Sprintf("%v", content)
	}
}

// GetOptimizationMetrics returns metrics about the optimization process
func (c *CascadeCompression) GetOptimizationMetrics(originalMessages []model.Message, optimizedContext string) map[string]interface{} {
	return map[string]interface{}{
		"original_message_count":  len(originalMessages),
		"original_tokens":         c.estimateMessagesTokens(originalMessages),
		"optimized_tokens":        c.estimateTokens(optimizedContext),
		"compression_ratio":       float64(c.estimateTokens(optimizedContext)) / float64(c.estimateMessagesTokens(originalMessages)),
		"quality_score":           c.calculateQualityScore(originalMessages, optimizedContext),
		"expert_model":            c.expertModel,
		"worker_model":            c.workerModel,
	}
}

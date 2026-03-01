package ensemble

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

// Synthesizer handles Level 4 large model synthesis (5x 20k -> 100k)
type Synthesizer struct {
	adapter       adapter.Adapter
	synthesisModel string          // Model for synthesis (large model)
	maxTokens     int             // Maximum output tokens (default: 100k)
}

// SynthesizerConfig configures the synthesizer behavior
type SynthesizerConfig struct {
	Adapter       adapter.Adapter
	SynthesisModel string         // Large model for synthesis
	MaxTokens     int            // Maximum output tokens
}

// NewSynthesizer creates a new synthesizer for combining compressed results
func NewSynthesizer(config *SynthesizerConfig) *Synthesizer {
	if config == nil {
		config = &SynthesizerConfig{}
	}

	maxTokens := config.MaxTokens
	if maxTokens == 0 {
		maxTokens = 100000 // 100k tokens output
	}

	synthesisModel := config.SynthesisModel
	if synthesisModel == "" {
		synthesisModel = "gpt-4-turbo" // Default large model
	}

	return &Synthesizer{
		adapter:       config.Adapter,
		synthesisModel: synthesisModel,
		maxTokens:     maxTokens,
	}
}

// SynthesisResult represents the result of synthesis
type SynthesisResult struct {
	Messages        []model.Message      // Synthesized messages
	Summary         string               // Overall summary if generated
	TotalTokens     int                  // Total tokens in result
	InputTokens     int                  // Total input tokens from chunks
	ReductionRatio  float64              // Compression ratio achieved
	Duration        time.Duration        // Time taken for synthesis
	Metrics         map[string]interface{} // Level 4 metrics
}

// Synthesize combines multiple compressed chunk results into a unified result
func (s *Synthesizer) Synthesize(ctx context.Context, chunkResults []ChunkResult) (*SynthesisResult, error) {
	if len(chunkResults) == 0 {
		return nil, fmt.Errorf("no chunk results to synthesize")
	}

	startTime := time.Now()

	// Collect all compressed messages
	var allMessages []model.Message
	totalInputTokens := 0
	successCount := 0

	for _, result := range chunkResults {
		if result.Error != nil || result.Compressed == nil {
			continue
		}
		// Extract messages from compressed result
		allMessages = append(allMessages, result.Compressed.Messages...)
		totalInputTokens += result.Compressed.CompressedTokens
		successCount++
	}

	if len(allMessages) == 0 {
		return &SynthesisResult{
			Messages:    []model.Message{},
			TotalTokens: 0,
			InputTokens: totalInputTokens,
			Duration:    time.Since(startTime),
			Metrics:     s.createMetrics(0, totalInputTokens, 0, time.Since(startTime)),
		}, nil
	}

	// Estimate current token count
	currentTokens := s.estimateMessagesTokens(allMessages)

	// If within budget, return as-is
	if currentTokens <= s.maxTokens {
		return &SynthesisResult{
			Messages:        allMessages,
			TotalTokens:     currentTokens,
			InputTokens:     totalInputTokens,
			ReductionRatio:  float64(currentTokens) / float64(totalInputTokens),
			Duration:        time.Since(startTime),
			Metrics:         s.createMetrics(successCount, totalInputTokens, currentTokens, time.Since(startTime)),
		}, nil
	}

	// Level 4: Large model synthesis to merge and compress further
	synthesized, err := s.performSynthesis(ctx, allMessages, totalInputTokens)
	if err != nil {
		// Fallback: truncate to fit budget
		return s.fallbackSynthesis(allMessages, totalInputTokens, startTime)
	}

	synthesized.Duration = time.Since(startTime)
	synthesized.Metrics = s.createMetrics(successCount, totalInputTokens, synthesized.TotalTokens, synthesized.Duration)

	return synthesized, nil
}

// performSynthesis uses a large model to intelligently merge compressed chunks
func (s *Synthesizer) performSynthesis(ctx context.Context, messages []model.Message, inputTokens int) (*SynthesisResult, error) {
	if s.adapter == nil {
		return nil, fmt.Errorf("adapter required for synthesis")
	}

	// Build synthesis prompt
	prompt := s.buildSynthesisPrompt(messages)

	// Create synthesis request
	synthesisRequest := &adapter.ChatCompletionRequest{
		Model: s.synthesisModel,
		Messages: []model.Message{
			{
				Role:    "system",
				Content: "You are an expert at synthesizing compressed conversation summaries into coherent, unified context. Preserve all critical information, decisions, and action items while eliminating redundancy.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:     s.maxTokens,
		Temperature:   func() *float32 { t := float32(0.3); return &t }(),
	}

	// Call the large model for synthesis
	response, err := s.adapter.ChatCompletions(ctx, synthesisRequest)
	if err != nil {
		return nil, fmt.Errorf("synthesis request failed: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no synthesis generated")
	}

	contentStr, ok := response.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("synthesis content is not a string")
	}

	synthesis := strings.TrimSpace(contentStr)
	if synthesis == "" {
		return nil, fmt.Errorf("empty synthesis generated")
	}

	// Parse synthesis into structured messages
	synthesizedMessages := s.parseSynthesis(synthesis)

	return &SynthesisResult{
		Messages:        synthesizedMessages,
		Summary:         synthesis,
		TotalTokens:     s.estimateTokens(synthesis),
		InputTokens:     inputTokens,
		ReductionRatio:  float64(s.estimateTokens(synthesis)) / float64(inputTokens),
	}, nil
}

// buildSynthesisPrompt creates a prompt for synthesizing compressed chunks
func (s *Synthesizer) buildSynthesisPrompt(messages []model.Message) string {
	var sb strings.Builder

	sb.WriteString("Synthesize the following compressed conversation chunks into a unified, coherent summary.\n\n")
	sb.WriteString("Requirements:\n")
	sb.WriteString("- Maximum output: 100,000 tokens\n")
	sb.WriteString("- Preserve all critical information, decisions, and action items\n")
	sb.WriteString("- Eliminate redundancy across chunks\n")
	sb.WriteString("- Maintain chronological flow where relevant\n")
	sb.WriteString("- Organize by topic/theme where appropriate\n\n")
	sb.WriteString("Compressed chunks:\n\n")

	// Add messages (may include summaries from compression)
	includedTokens := 0
	maxInputTokens := 150000 // Cap input to synthesis model

	for i, msg := range messages {
		msgTokens := s.estimateMessageTokens(&msg)
		if includedTokens+msgTokens > maxInputTokens && i > 0 {
			sb.WriteString("\n[... additional context truncated ...]\n")
			break
		}

		content := s.contentToString(msg.Content)
		if len(content) > 5000 {
			content = content[:4970] + "...\n[truncated]"
		}

		sb.WriteString(fmt.Sprintf("[Chunk %d - %s]:\n%s\n\n", i+1, msg.Role, content))
		includedTokens += msgTokens
	}

	sb.WriteString("\nProvide a comprehensive, unified synthesis.")

	return sb.String()
}

// parseSynthesis parses the synthesis text into structured messages
func (s *Synthesizer) parseSynthesis(synthesis string) []model.Message {
	// For simplicity, treat the entire synthesis as a system context message
	// In a more sophisticated implementation, we could parse into sections
	return []model.Message{
		{
			Role:    "system",
			Content: fmt.Sprintf("[Synthesized Context from Parallel Processing]\n\n%s", synthesis),
		},
	}
}

// fallbackSynthesis provides truncation-based fallback when synthesis fails
func (s *Synthesizer) fallbackSynthesis(messages []model.Message, inputTokens int, startTime time.Time) (*SynthesisResult, error) {
	// Keep most recent messages that fit within budget
	var result []model.Message
	tokenCount := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := s.estimateMessageTokens(&messages[i])
		if tokenCount+msgTokens > s.maxTokens && len(result) > 0 {
			break
		}
		result = append([]model.Message{messages[i]}, result...)
		tokenCount += msgTokens
	}

	return &SynthesisResult{
		Messages:        result,
		TotalTokens:     tokenCount,
		InputTokens:     inputTokens,
		ReductionRatio:  float64(tokenCount) / float64(inputTokens),
		Duration:        time.Since(startTime),
		Metrics:         s.createMetrics(len(messages), inputTokens, tokenCount, time.Since(startTime)),
	}, nil
}

// estimateMessagesTokens estimates total tokens for messages
func (s *Synthesizer) estimateMessagesTokens(messages []model.Message) int {
	total := 0
	for i := range messages {
		total += s.estimateMessageTokens(&messages[i])
	}
	return total
}

// estimateMessageTokens estimates tokens for a single message
func (s *Synthesizer) estimateMessageTokens(msg *model.Message) int {
	content := s.contentToString(msg.Content)
	return len(content)/4 + 10
}

// estimateTokens estimates tokens for a string
func (s *Synthesizer) estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

// contentToString converts message content to string
func (s *Synthesizer) contentToString(content interface{}) string {
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

// createMetrics creates Level 4 metrics
func (s *Synthesizer) createMetrics(inputChunks int, inputTokens int, outputTokens int, duration time.Duration) map[string]interface{} {
	metrics := map[string]interface{}{
		"level4_input_chunks":      inputChunks,
		"level4_input_tokens":      inputTokens,
		"level4_output_tokens":     outputTokens,
		"level4_duration_ms":       duration.Milliseconds(),
	}

	if inputTokens > 0 {
		metrics["level4_reduction_ratio"] = float64(outputTokens) / float64(inputTokens)
		metrics["level4_tokens_saved"] = inputTokens - outputTokens
	}

	return metrics
}

// GetMetrics returns synthesizer configuration metrics
func (s *Synthesizer) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"synthesis_model": s.synthesisModel,
		"max_tokens":      s.maxTokens,
		"has_adapter":     s.adapter != nil,
	}
}

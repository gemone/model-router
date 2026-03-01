package compression

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

const (
	// RawTokensTarget is the target number of recent tokens to keep uncompressed
	RawTokensTarget = 40000
	// SummaryRatio is the compression ratio for older content (10:1)
	SummaryRatio = 10
	// TokenBudgetRatio is the percentage of context window allocated for token budget
	TokenBudgetRatio = 0.2
	// DefaultSummaryMaxTokens is the default maximum tokens for a summary
	DefaultSummaryMaxTokens = 20000
	// SummaryModel is the default model to use for summarization (small, fast model)
	SummaryModel = "gpt-4o-mini"
)

// SlidingWindowCompression implements sliding window with summary compression
type SlidingWindowCompression struct {
	adapter      adapter.Adapter
	summaryModel string
}

// CompressedContext represents the result of compression
type CompressedContext struct {
	Messages       []model.Message `json:"messages"`
	Summary        string          `json:"summary,omitempty"`
	OriginalTokens int             `json:"original_tokens"`
	CompressedTokens int           `json:"compressed_tokens"`
	CompressionRatio float64       `json:"compression_ratio"`
}

// NewSlidingWindowCompression creates a new sliding window compression instance
func NewSlidingWindowCompression(adapter adapter.Adapter) *SlidingWindowCompression {
	return &SlidingWindowCompression{
		adapter:      adapter,
		summaryModel: SummaryModel,
	}
}

// SetSummaryModel sets a custom model for summarization
func (s *SlidingWindowCompression) SetSummaryModel(modelName string) {
	s.summaryModel = modelName
}

// Compress compresses the session context using sliding window with summary compression
// Keeps last 40k tokens raw, summarizes older content (200k -> 20k, 10:1 ratio)
func (s *SlidingWindowCompression) Compress(ctx context.Context, session *model.Session, messages []model.Message, maxTokens int) (*CompressedContext, error) {
	if session == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}
	if len(messages) == 0 {
		return &CompressedContext{
			Messages:       messages,
			OriginalTokens: 0,
			CompressedTokens: 0,
			CompressionRatio: 1.0,
		}, nil
	}

	// Calculate total tokens in messages
	totalTokens := s.estimateMessagesTokens(messages)
	originalTokens := totalTokens

	// If tokens are within budget, no compression needed
	if totalTokens <= maxTokens {
		return &CompressedContext{
			Messages:         messages,
			OriginalTokens:   originalTokens,
			CompressedTokens: totalTokens,
			CompressionRatio: 1.0,
		}, nil
	}

	// Split messages into recent (last 40k tokens) and older
	recentMessages, olderMessages := s.splitByRecentTokens(messages, RawTokensTarget)

	// If no older messages, return recent as-is
	if len(olderMessages) == 0 {
		return &CompressedContext{
			Messages:         recentMessages,
			OriginalTokens:   originalTokens,
			CompressedTokens: s.estimateMessagesTokens(recentMessages),
			CompressionRatio: float64(s.estimateMessagesTokens(recentMessages)) / float64(originalTokens),
		}, nil
	}

	// Summarize older messages
	summary, err := s.summarizeMessages(ctx, olderMessages)
	if err != nil {
		// If summarization fails, truncate older messages to fit
		return s.fallbackCompression(messages, maxTokens, originalTokens), nil
	}

	// Build compressed context
	result := &CompressedContext{
		Summary:          summary,
		Messages:         recentMessages,
		OriginalTokens:   originalTokens,
		CompressedTokens: s.estimateTokens(summary) + s.estimateMessagesTokens(recentMessages),
	}
	result.CompressionRatio = float64(result.CompressedTokens) / float64(result.OriginalTokens)

	return result, nil
}

// splitByRecentTokens splits messages into recent (within token budget) and older
func (s *SlidingWindowCompression) splitByRecentTokens(messages []model.Message, recentTokenBudget int) ([]model.Message, []model.Message) {
	if len(messages) == 0 {
		return nil, nil
	}

	// Iterate from newest to oldest, collecting recent messages
	var recent []model.Message
	tokenCount := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := s.estimateMessageTokens(&messages[i])
		if tokenCount+msgTokens > recentTokenBudget && len(recent) > 0 {
			break
		}
		recent = append([]model.Message{messages[i]}, recent...)
		tokenCount += msgTokens
	}

	// Older messages are everything before the recent ones
	older := make([]model.Message, 0)
	if len(recent) < len(messages) {
		older = messages[:len(messages)-len(recent)]
	}

	return recent, older
}

// summarizeMessages creates a summary of older messages using the summarization model
func (s *SlidingWindowCompression) summarizeMessages(ctx context.Context, messages []model.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Build summarization prompt
	prompt := s.buildSummaryPrompt(messages)

	// Create summary request
	summaryRequest := &adapter.ChatCompletionRequest{
		Model: s.summaryModel,
		Messages: []model.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant that summarizes conversations concisely while preserving important context, decisions, and action items.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens: DefaultSummaryMaxTokens,
		Temperature: func() *float32 { t := float32(0.3); return &t }(),
	}

	// Call the model to generate summary
	response, err := s.adapter.ChatCompletion(ctx, summaryRequest)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	contentStr, ok := response.Choices[0].Message.Content.(string)
	if !ok {
		return "", fmt.Errorf("summary content is not a string")
	}
	summary := strings.TrimSpace(contentStr)
	if summary == "" {
		return "", fmt.Errorf("empty summary generated")
	}

	return summary, nil
}

// buildSummaryPrompt creates a prompt for summarizing messages
func (s *SlidingWindowCompression) buildSummaryPrompt(messages []model.Message) string {
	var sb strings.Builder

	sb.WriteString("Summarize the following conversation history concisely (maximum 2000 tokens).\n\n")
	sb.WriteString("Focus on:\n")
	sb.WriteString("- Key topics discussed\n")
	sb.WriteString("- Important decisions made\n")
	sb.WriteString("- Action items or tasks\n")
	sb.WriteString("- Context relevant to ongoing conversation\n\n")
	sb.WriteString("Conversation:\n\n")

	// Include messages (truncate if too long)
	includedTokens := 0
	maxInputTokens := 200000 // 200k tokens max for summarization input

	for _, msg := range messages {
		msgTokens := s.estimateMessageTokens(&msg)
		if includedTokens+msgTokens > maxInputTokens {
			sb.WriteString("\n[... earlier conversation truncated ...]\n")
			break
		}

		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.Role, s.truncateContent(msg.Content, 1000)))
		includedTokens += msgTokens
	}

	sb.WriteString("\nProvide a concise summary that preserves essential context.")

	return sb.String()
}

// fallbackCompression provides simple truncation when summarization fails
func (s *SlidingWindowCompression) fallbackCompression(messages []model.Message, maxTokens int, originalTokens int) *CompressedContext {
	// Keep most recent messages that fit within budget
	var result []model.Message
	tokenCount := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := s.estimateMessageTokens(&messages[i])
		if tokenCount+msgTokens > maxTokens && len(result) > 0 {
			break
		}
		result = append([]model.Message{messages[i]}, result...)
		tokenCount += msgTokens
	}

	return &CompressedContext{
		Messages:         result,
		OriginalTokens:   originalTokens,
		CompressedTokens: tokenCount,
		CompressionRatio: float64(tokenCount) / float64(originalTokens),
	}
}

// estimateMessagesTokens estimates total tokens for all messages
func (s *SlidingWindowCompression) estimateMessagesTokens(messages []model.Message) int {
	total := 0
	for i := range messages {
		total += s.estimateMessageTokens(&messages[i])
	}
	return total
}

// estimateMessageTokens estimates tokens for a single message
func (s *SlidingWindowCompression) estimateMessageTokens(msg *model.Message) int {
	// Rough estimation: ~4 chars per token for English text
	// Add overhead for role and other metadata
	content := s.contentToString(msg.Content)
	return len(content)/4 + 10 // 10 tokens overhead per message
}

// estimateTokens estimates tokens for a string
func (s *SlidingWindowCompression) estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

// contentToString converts message content to string
func (s *SlidingWindowCompression) contentToString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []model.ContentPart:
		var sb strings.Builder
		for _, part := range v {
			if part.Type == "text" {
				sb.WriteString(part.Text)
				sb.WriteString(" ")
			}
		}
		return sb.String()
	default:
		return fmt.Sprintf("%v", content)
	}
}

// truncateContent truncates content to maximum characters
func (s *SlidingWindowCompression) truncateContent(content interface{}, maxChars int) string {
	str := s.contentToString(content)
	if len(str) <= maxChars {
		return str
	}
	return str[:maxChars] + "..."
}

// GetTokenBudget calculates the token budget based on context window
func (s *SlidingWindowCompression) GetTokenBudget(contextWindow int) int {
	budget := int(float64(contextWindow) * TokenBudgetRatio)
	if budget < RawTokensTarget {
		return RawTokensTarget
	}
	return budget
}

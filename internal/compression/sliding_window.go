package compression

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/middleware"
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
	// MaxSummaryInputTokens is the maximum tokens for summarization input
	MaxSummaryInputTokens = 200000
)

// SlidingWindowCompression implements sliding window with summary compression
type SlidingWindowCompression struct {
	adapter          adapter.Adapter
	summaryModel     string
	templateRenderer TemplateRenderer // 模板渲染器
	profileID        string           // Profile ID 用于获取自定义模板
}

// CompressedContext represents the result of compression
type CompressedContext struct {
	Messages         []model.Message `json:"messages"`
	Summary          string          `json:"summary,omitempty"`
	OriginalTokens   int             `json:"original_tokens"`
	CompressedTokens int             `json:"compressed_tokens"`
	CompressionRatio float64         `json:"compression_ratio"`
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

// SetTemplateRenderer 设置模板渲染器
func (s *SlidingWindowCompression) SetTemplateRenderer(renderer TemplateRenderer) {
	s.templateRenderer = renderer
}

// SetProfileID 设置 Profile ID
func (s *SlidingWindowCompression) SetProfileID(profileID string) {
	s.profileID = profileID
}

// Compress compresses the session context using sliding window with summary compression
// Keeps last 40k tokens raw, summarizes older content (200k -> 20k, 10:1 ratio)
func (s *SlidingWindowCompression) Compress(ctx context.Context, session *model.Session, messages []model.Message, maxTokens int) (*CompressedContext, error) {
	if session == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}
	if len(messages) == 0 {
		return &CompressedContext{
			Messages:         messages,
			OriginalTokens:   0,
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

	// Get system prompt
	systemPrompt := s.getSummarySystemPrompt()

	// Create summary request
	summaryRequest := &model.ChatCompletionRequest{
		Model: s.summaryModel,
		Messages: []model.Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   DefaultSummaryMaxTokens,
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

// getSummarySystemPrompt 获取摘要系统提示词
func (s *SlidingWindowCompression) getSummarySystemPrompt() string {
	defaultPrompt := "You are a helpful assistant that summarizes conversations concisely while preserving important context, decisions, and action items."

	if s.templateRenderer == nil {
		return defaultPrompt
	}

	// 尝试从模板渲染
	rendered, err := s.templateRenderer.Render(model.TemplateSummarySystem, s.profileID, nil)
	if err == nil && rendered != "" {
		return rendered
	}
	// Log template render error for monitoring
	if err != nil {
		middleware.WarnLog("Summary system prompt template render failed for profile %s: %v", s.profileID, err)
	}

	return defaultPrompt
}

// SummaryMessageInfo 用于模板渲染的摘要消息信息
type SummaryMessageInfo struct {
	Role    string
	Content string
}

// SummaryPromptData 摘要提示词模板数据
type SummaryPromptData struct {
	MaxOutputTokens int
	Messages        []SummaryMessageInfo
	Truncated       bool
}

// buildSummaryPrompt creates a prompt for summarizing messages
func (s *SlidingWindowCompression) buildSummaryPrompt(messages []model.Message) string {
	// 如果使用模板渲染器，尝试使用模板
	if s.templateRenderer != nil {
		data := s.buildSummaryPromptData(messages)
		rendered, err := s.templateRenderer.Render(model.TemplateSummaryUserPrompt, s.profileID, map[string]interface{}{
			"MaxOutputTokens": data.MaxOutputTokens,
			"Messages":        data.Messages,
			"Truncated":       data.Truncated,
		})
		if err == nil {
			return rendered
		}
		// Log template render error for monitoring
		middleware.WarnLog("Summary user prompt template render failed for profile %s: %v", s.profileID, err)
	}

	return s.buildDefaultSummaryPrompt(messages)
}

// buildSummaryPromptData 构建摘要提示词数据
func (s *SlidingWindowCompression) buildSummaryPromptData(messages []model.Message) *SummaryPromptData {
	data := &SummaryPromptData{
		MaxOutputTokens: 2000,
		Messages:        make([]SummaryMessageInfo, 0),
		Truncated:       false,
	}

	// Include messages (truncate if too long)
	includedTokens := 0
	maxInputTokens := MaxSummaryInputTokens // Max tokens for summarization input

	for _, msg := range messages {
		msgTokens := s.estimateMessageTokens(&msg)
		if includedTokens+msgTokens > maxInputTokens {
			data.Truncated = true
			break
		}

		data.Messages = append(data.Messages, SummaryMessageInfo{
			Role:    msg.Role,
			Content: s.truncateContent(msg.Content, 1000),
		})
		includedTokens += msgTokens
	}

	return data
}

// buildDefaultSummaryPrompt 构建默认摘要提示词
func (s *SlidingWindowCompression) buildDefaultSummaryPrompt(messages []model.Message) string {
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
	maxInputTokens := MaxSummaryInputTokens // Max tokens for summarization input

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
	return estimateTokensForMessages(messages)
}

// estimateMessageTokens estimates tokens for a single message
func (s *SlidingWindowCompression) estimateMessageTokens(msg *model.Message) int {
	return estimateTokensForMessage(msg)
}

// estimateTokens estimates tokens for a string
func (s *SlidingWindowCompression) estimateTokens(text string) int {
	return estimateTokensForText(text)
}

// truncateContent truncates content to maximum characters
func (s *SlidingWindowCompression) truncateContent(content interface{}, maxChars int) string {
	return truncateContent(content, maxChars)
}

// GetTokenBudget calculates the token budget based on context window
func (s *SlidingWindowCompression) GetTokenBudget(contextWindow int) int {
	budget := int(float64(contextWindow) * TokenBudgetRatio)
	if budget < RawTokensTarget {
		return RawTokensTarget
	}
	return budget
}

package router

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/tokenizer"
)

// EstimateMessagesTokens estimates the total token count for a list of messages
func EstimateMessagesTokens(messages []model.Message) int {
	return tokenizer.CountTokensForMessages(messages)
}

// EstimateMessageTokens estimates the token count for a single message
func EstimateMessageTokens(msg *model.Message) int {
	return tokenizer.CountTokensForMessage(msg)
}

// ContentToString converts message content to string
func ContentToString(content interface{}) string {
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

// DetectLanguage detects the primary language of messages using simple CJK heuristic
func DetectLanguage(messages []model.Message) string {
	cjkCount := 0
	latinCount := 0
	for _, msg := range messages {
		content := ContentToString(msg.Content)
		for _, r := range content {
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
				cjkCount++
			} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				latinCount++
			}
		}
	}
	if cjkCount > latinCount {
		return "zh" // Simplified CJK detection
	}
	return "en"
}

// CalculateComplexityScore calculates a complexity score for messages
func CalculateComplexityScore(messages []model.Message) float64 {
	score := 0.0

	// Conversation length: 0.1 per message
	score += float64(len(messages)) * 0.1

	// Technical terms: 0.3 per match
	technicalTerms := []string{"algorithm", "optimize", "architecture", "implement", "debug", "refactor", "asynchronous", "concurrent", "database", "api", "interface", "abstract"}
	score += float64(countTerms(messages, technicalTerms)) * 0.3

	// Code blocks: 2.0 per ``` block
	score += float64(countCodeBlocks(messages)) * 2.0

	// Average message depth: /100
	score += averageMessageLength(messages) / 100.0

	// Multi-step: 1.5
	if hasMultiStepRequest(messages) {
		score += 1.5
	}

	return score
}

// countTerms counts occurrences of technical terms in messages
func countTerms(messages []model.Message, terms []string) int {
	count := 0
	content := strings.ToLower(ContentToStringFromMessages(messages))
	for _, term := range terms {
		if strings.Contains(content, term) {
			count++
		}
	}
	return count
}

// countCodeBlocks counts the number of code blocks (```) in messages
func countCodeBlocks(messages []model.Message) int {
	count := 0
	for _, msg := range messages {
		content := ContentToString(msg.Content)
		count += strings.Count(content, "```")
	}
	return count / 2 // Each code block has opening and closing ```
}

// averageMessageLength calculates the average message length in characters
func averageMessageLength(messages []model.Message) float64 {
	if len(messages) == 0 {
		return 0
	}
	total := 0
	for _, msg := range messages {
		total += len(ContentToString(msg.Content))
	}
	return float64(total) / float64(len(messages))
}

// hasMultiStepRequest checks if the conversation contains multi-step requests
func hasMultiStepRequest(messages []model.Message) bool {
	multiStepKeywords := []string{"then", "after that", "next", "finally", "first", "second", "third", "step", "followed by"}
	content := strings.ToLower(ContentToStringFromMessages(messages))
	for _, keyword := range multiStepKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}
	return false
}

// ContentToStringFromMessages converts all messages to a single string
func ContentToStringFromMessages(messages []model.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(ContentToString(msg.Content))
		sb.WriteString(" ")
	}
	return sb.String()
}

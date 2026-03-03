package compression

import (
	"fmt"
	"strings"

	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/tokenizer"
)

// contentToString converts message content to string
// This is a shared utility function used across multiple compression implementations
func contentToString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []model.ContentPart:
		var result strings.Builder
		for _, part := range v {
			if part.Type == "text" {
				result.WriteString(part.Text)
				result.WriteString(" ")
			}
		}
		return result.String()
	default:
		return fmt.Sprintf("%v", content)
	}
}

// truncateContent truncates content to maximum characters with ellipsis
func truncateContent(content interface{}, maxChars int) string {
	str := contentToString(content)
	if len(str) <= maxChars {
		return str
	}
	return str[:maxChars] + "..."
}

// estimateTokensForText estimates token count for text content
// Now uses actual tokenizer for accurate counting
func estimateTokensForText(text string) int {
	return tokenizer.CountTokens(text)
}

// estimateTokensForMessage estimates tokens for a single message
func estimateTokensForMessage(msg *model.Message) int {
	return tokenizer.CountTokensForMessage(msg)
}

// estimateTokensForMessages estimates total tokens for multiple messages
func estimateTokensForMessages(messages []model.Message) int {
	return tokenizer.CountTokensForMessages(messages)
}

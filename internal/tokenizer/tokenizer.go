package tokenizer

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/gemone/model-router/internal/model"
	"github.com/pkoukk/tiktoken-go"
)

// defaultEncoding is the default encoding to use (cl100k_base for GPT-4/GPT-3.5)
const defaultEncoding = "cl100k_base"

const (
	// MaxInputSize is the maximum input size in bytes to prevent DoS
	MaxInputSize = 10 * 1024 * 1024 // 10MB
	// MaxMessagesPerRequest is the maximum number of messages allowed
	MaxMessagesPerRequest = 10000
	// TokenCountTimeout is the maximum time to spend counting tokens before giving up
	TokenCountTimeout = 100 * time.Millisecond
)

// Counter provides token counting functionality
// Note: tiktoken.Encode is safe for concurrent reads, no mutex needed
type Counter struct {
	tiktoken    *tiktoken.Tiktoken
	useFallback bool // true if using fallback estimation
}

var (
	defaultCounter *Counter
	initErr        error
)

func init() {
	// Initialize tokenizer during package init to fail fast on startup
	// This prevents runtime panics and allows for proper error handling
	defaultCounter = &Counter{}
	tkt, err := tiktoken.GetEncoding(defaultEncoding)
	if err != nil {
		log.Printf("[WARNING] Failed to initialize tiktoken encoding %q: %v", defaultEncoding, err)
		log.Printf("[WARNING] Using fallback token estimation (4 chars/token)")
		defaultCounter.useFallback = true
		initErr = fmt.Errorf("tiktoken initialization failed, using fallback: %w", err)
		return
	}
	defaultCounter.tiktoken = tkt
	defaultCounter.useFallback = false
	log.Printf("[INFO] Tiktoken encoding %q initialized successfully", defaultEncoding)
}

// GetDefaultCounter returns the default singleton counter instance
// Returns a counter that either uses real tiktoken or fallback estimation
// The initialization happens in init() to fail fast on startup
func GetDefaultCounter() *Counter {
	return defaultCounter
}

// InitError returns any error that occurred during initialization
// Returns nil if initialization was successful
func InitError() error {
	return initErr
}

// NewCounter creates a new counter with the specified encoding
func NewCounter(encodingName string) (*Counter, error) {
	tkt, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return nil, err
	}
	return &Counter{tiktoken: tkt}, nil
}

// CountTokens returns the exact token count for the given text
// Note: tiktoken.Encode is safe for concurrent reads, no mutex needed
func (c *Counter) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// Check input size to prevent DoS attacks
	// For very large inputs, return fast estimate without processing
	if len(text) > MaxInputSize {
		// Use a simple heuristic: ~4 chars per token for most text
		// This avoids expensive processing while still providing reasonable estimates
		return len(text) / 4
	}

	// Use fallback estimation if tiktoken is not available
	if c.useFallback || c.tiktoken == nil {
		return len(text) / 4
	}

	tokens := c.tiktoken.Encode(text, nil, nil)
	return len(tokens)
}

// sanitizeTokenInput removes control characters and invalid Unicode from input
// to prevent log injection and other security issues
func sanitizeTokenInput(s string) string {
	if s == "" {
		return s
	}
	var result strings.Builder
	for _, r := range s {
		// Skip null bytes and control characters (except newline and tab)
		if r == 0 || (unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r') {
			continue
		}
		// Skip invalid Unicode characters
		if r > 0x10FFFF {
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// CountTokensForMessage returns the token count for a single message
// Includes overhead for message format based on ChatML format
// Format: <|im_start|>role\ncontent<|im_end|>\n
func (c *Counter) CountTokensForMessage(msg *model.Message) int {
	content := ContentToString(msg.Content)

	// Count actual message format tokens
	// Base format: <|im_start|>role\n
	var formatBuilder strings.Builder
	formatBuilder.WriteString("<|im_start|>")
	// Sanitize role to prevent injection
	formatBuilder.WriteString(sanitizeTokenInput(msg.Role))

	// Add name field if present
	if msg.Name != "" {
		formatBuilder.WriteString(" name=")
		// Sanitize name to prevent injection
		formatBuilder.WriteString(sanitizeTokenInput(msg.Name))
	}

	formatBuilder.WriteString("\n")

	// Count format tokens
	formatTokens := c.CountTokens(formatBuilder.String())

	// Count content tokens
	contentTokens := c.CountTokens(content)

	// Add <|im_end|>\n suffix
	suffixTokens := c.CountTokens("<|im_end|>\n")

	return formatTokens + contentTokens + suffixTokens
}

// CountTokensForMessages returns the total token count for multiple messages
// Validates message count to prevent DoS attacks
func (c *Counter) CountTokensForMessages(messages []model.Message) int {
	// Validate message count to prevent DoS
	if len(messages) > MaxMessagesPerRequest {
		// Log the fact of truncation without exposing exact count
		log.Printf("[WARNING] Message count exceeds maximum, truncating to %d", MaxMessagesPerRequest)
		messages = messages[:MaxMessagesPerRequest]
	}

	total := 0
	for i := range messages {
		total += c.CountTokensForMessage(&messages[i])
	}
	// Add overhead for the entire messages array (typically 3 tokens for reply priming)
	return total + 3
}

// ContentToString converts message content to string
func ContentToString(content interface{}) string {
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
		return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(
			strings.ReplaceAll(fmt.Sprintf("%v", content), "[", ""),
			"]", ""), " ", ""))
	}
}

// EstimateTokensForText is a fallback estimation method
//
// Deprecated: Since v1.1.0 - Use CountTokens for accurate counting.
// These estimation functions will be removed in v2.0.0.
// Migration: Replace EstimateTokensForText(text) with CountTokens(text)
func EstimateTokensForText(text string) int {
	return GetDefaultCounter().CountTokens(text)
}

// EstimateTokensForMessage is a fallback estimation method
//
// Deprecated: Since v1.1.0 - Use CountTokensForMessage for accurate counting.
// These estimation functions will be removed in v2.0.0.
// Migration: Replace EstimateTokensForMessage(msg) with CountTokensForMessage(msg)
func EstimateTokensForMessage(msg *model.Message) int {
	return GetDefaultCounter().CountTokensForMessage(msg)
}

// EstimateTokensForMessages is a fallback estimation method
//
// Deprecated: Since v1.1.0 - Use CountTokensForMessages for accurate counting.
// These estimation functions will be removed in v2.0.0.
// Migration: Replace EstimateTokensForMessages(messages) with CountTokensForMessages(messages)
func EstimateTokensForMessages(messages []model.Message) int {
	return GetDefaultCounter().CountTokensForMessages(messages)
}

// Global convenience functions using the default counter

// CountTokens returns the token count for text using the default counter
func CountTokens(text string) int {
	return GetDefaultCounter().CountTokens(text)
}

// CountTokensForMessage returns the token count for a message using the default counter
func CountTokensForMessage(msg *model.Message) int {
	return GetDefaultCounter().CountTokensForMessage(msg)
}

// CountTokensForMessages returns the token count for messages using the default counter
func CountTokensForMessages(messages []model.Message) int {
	return GetDefaultCounter().CountTokensForMessages(messages)
}

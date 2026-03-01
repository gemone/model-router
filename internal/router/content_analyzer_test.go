package router

import (
	"testing"

	"github.com/gemone/model-router/internal/model"
)

func TestContentAnalyzer(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Hello, how are you?"},
		{Role: "assistant", Content: "I'm doing well, thank you!"},
	}

	t.Run("EstimateMessagesTokens returns positive value", func(t *testing.T) {
		tokens := EstimateMessagesTokens(messages)
		if tokens <= 0 {
			t.Error("expected positive token count")
		}
	})

	t.Run("DetectLanguage detects language", func(t *testing.T) {
		lang := DetectLanguage(messages)
		if lang != "en" && lang != "zh" {
			t.Errorf("expected 'en' or 'zh', got %s", lang)
		}
	})

	t.Run("CalculateComplexityScore returns positive score", func(t *testing.T) {
		score := CalculateComplexityScore(messages)
		if score < 0 {
			t.Errorf("expected non-negative score, got %f", score)
		}
	})

	t.Run("ContentToString converts message content", func(t *testing.T) {
		msg := &model.Message{
			Role:    "user",
			Content: "test content",
		}
		result := ContentToString(msg.Content)
		if result != "test content" {
			t.Errorf("expected 'test content', got '%s'", result)
		}
	})
}

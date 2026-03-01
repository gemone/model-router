// Package compression implements compression strategies
package compression

import (
	"context"
	"fmt"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

// SlidingWindowStrategy implements Strategy interface for sliding window compression
type SlidingWindowStrategy struct {
	compressor *SlidingWindowCompression
	adapter    adapter.Adapter
}

// NewSlidingWindowStrategy creates a new sliding window strategy
func NewSlidingWindowStrategy(adapter adapter.Adapter) *SlidingWindowStrategy {
	return &SlidingWindowStrategy{
		compressor: NewSlidingWindowCompression(adapter),
		adapter:    adapter,
	}
}

// Name returns the strategy name
func (s *SlidingWindowStrategy) Name() string {
	return "sliding_window"
}

// Compress implements Strategy interface with new signature
func (s *SlidingWindowStrategy) Compress(ctx context.Context, messages []model.Message, maxTokens int, getAdapter AdapterProvider) ([]model.Message, int, error) {
	session := &model.Session{
		ID:            "compression-session",
		ContextWindow: maxTokens * 2, // Assume 2x context window for compression
	}

	result, err := s.compressor.Compress(ctx, session, messages, maxTokens)
	if err != nil {
		return nil, 0, fmt.Errorf("sliding window compression failed: %w", err)
	}

	// Build final messages
	var finalMessages []model.Message

	// Add summary first if it exists
	if result.Summary != "" {
		finalMessages = append(finalMessages, model.Message{
			Role:    "system",
			Content: "[Summary of earlier conversation: " + result.Summary + "]",
		})
	}

	// Add recent messages
	finalMessages = append(finalMessages, result.Messages...)

	return finalMessages, result.CompressedTokens, nil
}

// Package compression implements cascade compression strategy.
package compression

import (
	"context"
	"fmt"
	"strings"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

// CascadeStrategy implements Strategy interface for cascade compression with expert model optimization
type CascadeStrategy struct {
	cascade *CascadeCompression
}

// NewCascadeStrategy creates a new cascade strategy
func NewCascadeStrategy(expertAdapter, workerAdapter adapter.Adapter, expertModel, workerModel string) *CascadeStrategy {
	cascade := NewCascadeCompression(&CascadeCompressionConfig{
		ExpertAdapter:    expertAdapter,
		WorkerAdapter:    workerAdapter,
		ExpertModel:      expertModel,
		WorkerModel:      workerModel,
		MaxOptimizeTokens: 5000, // Default max tokens for optimization
	})

	return &CascadeStrategy{
		cascade: cascade,
	}
}

// Name returns the strategy name
func (s *CascadeStrategy) Name() string {
	return "cascade_expert_optimization"
}

// Compress implements Strategy interface
func (s *CascadeStrategy) Compress(messages []model.Message, maxTokens int) ([]model.Message, int, error) {
	ctx := context.Background()

	// Run cascade optimization
	result, err := s.cascade.OptimizeWithContext(ctx, messages)
	if err != nil {
		return nil, 0, fmt.Errorf("cascade optimization failed: %w", err)
	}

	// Build final messages with optimized context
	finalMessages := []model.Message{
		{
			Role:    "system",
			Content: result.OptimizedPrompt,
		},
	}

	// Add the most recent messages to maintain continuity
	if len(messages) >= 2 {
		// Keep last 2 messages for context continuity
		recentMessages := messages[len(messages)-2:]
		for _, msg := range recentMessages {
			if msg.Role == "user" {
				finalMessages = append(finalMessages, msg)
				break
			}
		}
	}

	return finalMessages, result.OptimizedTokens, nil
}

// ExpertOptimizedCompression provides a high-level API for expert-optimized compression
type ExpertOptimizedCompression struct {
	cascade *CascadeCompression
}

// NewExpertOptimizedCompression creates a new expert-optimized compression
func NewExpertOptimizedCompression(expertAdapter, workerAdapter adapter.Adapter) *ExpertOptimizedCompression {
	return &ExpertOptimizedCompression{
		cascade: NewCascadeCompression(&CascadeCompressionConfig{
			ExpertAdapter:    expertAdapter,
			WorkerAdapter:    workerAdapter,
			ExpertModel:      "gpt-4-turbo",  // Default expert model
			WorkerModel:      "gpt-3.5-turbo", // Default worker model
			MaxOptimizeTokens: 5000,
		}),
	}
}

// CompressWithExpertOptimization performs expert model optimization before compression
func (e *ExpertOptimizedCompression) CompressWithExpertOptimization(
	ctx context.Context,
	messages []model.Message,
	maxTokens int,
) (*CascadeResult, error) {
	return e.cascade.OptimizeWithContext(ctx, messages)
}

// GetOptimizedPromptForWorker returns the optimized prompt for the worker model
func (e *ExpertOptimizedCompression) GetOptimizedPromptForWorker(
	ctx context.Context,
	messages []model.Message,
) (string, error) {
	result, err := e.cascade.OptimizeWithContext(ctx, messages)
	if err != nil {
		return "", err
	}
	return result.OptimizedPrompt, nil
}

// QualityMetrics represents quality metrics for the compression
type QualityMetrics struct {
	InstructionFollowingScore float64 // Estimated instruction following improvement
	ContextPreservationScore   float64 // How well context is preserved
	CompressionEfficiency       float64 // Tokens saved ratio
	QualityScore                float64 // Overall quality score
}

// CalculateQualityMetrics calculates quality metrics for compression
func CalculateQualityMetrics(originalMessages []model.Message, optimizedResult *CascadeResult) *QualityMetrics {
	return &QualityMetrics{
		InstructionFollowingScore: optimizedResult.QualityScore * 1.2, // Expert optimization improves instruction following
		ContextPreservationScore:   calculateContextPreservation(originalMessages, optimizedResult.OptimizedContext),
		CompressionEfficiency:       1.0 - optimizedResult.CompressionRatio,
		QualityScore:                optimizedResult.QualityScore,
	}
}

// calculateContextPreservation calculates how well the context is preserved
func calculateContextPreservation(messages []model.Message, optimizedContext string) float64 {
	// Check for key elements from original messages
	optimizedLower := strings.ToLower(optimizedContext)
	found := 0
	total := 0

	keyElements := []string{"task", "objective", "decision", "agreed", "action", "require"}

	// Count how many key elements are mentioned in original messages
	for _, msg := range messages {
		contentLower := strings.ToLower(contentToString(msg.Content))
		for _, element := range keyElements {
			if strings.Contains(contentLower, element) {
				total++
			}
		}
	}

	// Count how many are preserved in optimized context
	for _, element := range keyElements {
		if strings.Contains(optimizedLower, element) {
			found++
		}
	}

	if total == 0 {
		return 1.0
	}

	// Calculate ratio, but cap at 1.0 since found counts unique elements
	// while total counts all occurrences
	ratio := float64(found) / float64(total)
	if ratio > 1.0 {
		ratio = 1.0
	}
	return ratio
}

// Helper function
func contentToString(content interface{}) string {
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

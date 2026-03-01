package compression

import (
	"context"
	"fmt"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/session"
)

// AdapterProvider provides an adapter for compression operations
// Allows strategies to dynamically request adapters based on compression group
type AdapterProvider func(ctx context.Context) (adapter.Adapter, error)

// Strategy defines a compression strategy interface
type Strategy interface {
	// Name returns the strategy name
	Name() string

	// Compress applies compression to messages
	// NEW SIGNATURE: Accepts context and adapter provider for dynamic adapter selection
	Compress(ctx context.Context, messages []model.Message, maxTokens int, getAdapter AdapterProvider) ([]model.Message, int, error)
}

// LegacyStrategy wraps old-style strategies for backward compatibility
type LegacyStrategy struct {
	inner     interface{} // Can be old Strategy or any compatible type
	adapter   adapter.Adapter
}

func NewLegacyStrategy(inner interface{}, adapter adapter.Adapter) Strategy {
	return &LegacyStrategy{
		inner:   inner,
		adapter: adapter,
	}
}

func (l *LegacyStrategy) Name() string {
	// Try to get Name from inner interface
	if n, ok := l.inner.(interface{ Name() string }); ok {
		return n.Name()
	}
	return "legacy"
}

func (l *LegacyStrategy) Compress(ctx context.Context, messages []model.Message, maxTokens int, _ AdapterProvider) ([]model.Message, int, error) {
	// Try to call old-style Compress method
	if c, ok := l.inner.(interface{ Compress([]model.Message, int) ([]model.Message, int, error) }); ok {
		return c.Compress(messages, maxTokens)
	}
	return nil, 0, fmt.Errorf("legacy strategy does not implement Compress method")
}

// CompressionPipeline orchestrates multiple compression strategies
type CompressionPipeline struct {
	strategies      map[string]Strategy
	sessionManager  *session.Manager
}

// NewPipeline creates a new compression pipeline
func NewPipeline() *CompressionPipeline {
	return &CompressionPipeline{
		strategies:     make(map[string]Strategy),
		sessionManager: session.NewManager(),
	}
}

// NewPipelineWithManager creates a new compression pipeline with a custom session manager
func NewPipelineWithManager(sm *session.Manager) *CompressionPipeline {
	return &CompressionPipeline{
		strategies:     make(map[string]Strategy),
		sessionManager: sm,
	}
}

// Register registers a compression strategy
func (p *CompressionPipeline) Register(strategy Strategy) {
	p.strategies[strategy.Name()] = strategy
}

// StrategyConfig configures which strategies to use and their token budgets
type StrategyConfig struct {
	Name       string // Strategy name
	MaxTokens  int    // Token budget for this strategy
	Weight     int    // Priority weight (higher = applied first)
}

// CompressionResult contains the compressed result and metadata
type CompressionResult struct {
	Messages    []model.Message        // Compressed messages
	TotalTokens int                    // Total tokens after compression
	Stats       []StrategyStat         // Per-strategy statistics
	Metadata    map[string]interface{} // Additional metadata
}

// StrategyStat tracks per-strategy statistics
type StrategyStat struct {
	Name          string // Strategy name
	InputTokens   int    // Tokens before compression
	OutputTokens  int    // Tokens after compression
	Reduction     int    // Tokens reduced
	ReductionRate float64 // Percentage reduced
}

// Compress applies compression strategies in sequence
func (p *CompressionPipeline) Compress(ctx context.Context, session *model.Session, maxTokens int, configs []StrategyConfig, getAdapter AdapterProvider) (*CompressionResult, error) {
	result := &CompressionResult{
		Metadata: make(map[string]interface{}),
		Stats:    make([]StrategyStat, 0, len(configs)),
	}

	// Load session messages from database
	messages := []model.Message{}
	if session != nil && session.ID != "" {
		loadedMsgs, err := p.sessionManager.LoadSessionMessages(session.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to load session messages: %w", err)
		}
		messages = loadedMsgs
		result.Metadata["session_id"] = session.ID
		result.Metadata["messages_loaded"] = len(messages)
	} else {
		result.Metadata["messages_loaded"] = 0
		result.Metadata["reason"] = "no session provided"
	}

	currentTokens := 0

	// Sort configs by weight (highest first)
	sortedConfigs := make([]StrategyConfig, len(configs))
	copy(sortedConfigs, configs)
	for i := 0; i < len(sortedConfigs); i++ {
		for j := i + 1; j < len(sortedConfigs); j++ {
			if sortedConfigs[j].Weight > sortedConfigs[i].Weight {
				sortedConfigs[i], sortedConfigs[j] = sortedConfigs[j], sortedConfigs[i]
			}
		}
	}

	// Apply each strategy in sequence
	for _, config := range sortedConfigs {
		// Check if we've already met the target
		if currentTokens <= maxTokens {
			break
		}

		strategy, exists := p.strategies[config.Name]
		if !exists {
			return nil, fmt.Errorf("strategy not registered: %s", config.Name)
		}

		// Calculate remaining budget
		remainingBudget := config.MaxTokens
		if remainingBudget == 0 || remainingBudget > maxTokens-currentTokens {
			remainingBudget = maxTokens - currentTokens
		}

		if remainingBudget <= 0 {
			break
		}

		// Apply strategy - NEW: Pass context and adapter provider
		inputTokens := currentTokens
		compressedMsgs, outputTokens, err := strategy.Compress(ctx, messages, remainingBudget, getAdapter)
		if err != nil {
			return nil, fmt.Errorf("strategy %s failed: %w", strategy.Name(), err)
		}

		// Update state
		messages = compressedMsgs
		currentTokens = outputTokens

		// Record statistics
		stat := StrategyStat{
			Name:          strategy.Name(),
			InputTokens:   inputTokens,
			OutputTokens:  outputTokens,
			Reduction:     inputTokens - outputTokens,
			ReductionRate: float64(inputTokens-outputTokens) / float64(inputTokens) * 100,
		}
		result.Stats = append(result.Stats, stat)
	}

	// Final result
	result.Messages = messages
	result.TotalTokens = currentTokens
	result.Metadata["strategies_applied"] = len(result.Stats)
	result.Metadata["target_tokens"] = maxTokens
	result.Metadata["within_limit"] = currentTokens <= maxTokens

	return result, nil
}

// GetStrategy returns a registered strategy by name
func (p *CompressionPipeline) GetStrategy(name string) (Strategy, bool) {
	strategy, exists := p.strategies[name]
	return strategy, exists
}

// ListStrategies returns all registered strategy names
func (p *CompressionPipeline) ListStrategies() []string {
	names := make([]string, 0, len(p.strategies))
	for name := range p.strategies {
		names = append(names, name)
	}
	return names
}

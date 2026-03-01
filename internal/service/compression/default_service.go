package compression

import (
	"context"
	"fmt"

	"github.com/gemone/model-router/internal/adapter"
	compressionpkg "github.com/gemone/model-router/internal/compression"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/repository"
)

// DefaultService implements the compression Service interface
type DefaultService struct {
	profileID          string
	adapters           map[string]adapter.Adapter
	compressionPipeline *compressionpkg.CompressionPipeline
	selector           *repository.CompressionGroupSelector
	config             *CompressionConfig
}

// NewDefaultService creates a new default compression service
func NewDefaultService(profileID string, adapters map[string]adapter.Adapter, selector *repository.CompressionGroupSelector, config *CompressionConfig) *DefaultService {
	if config == nil {
		config = DefaultConfig()
	}

	return &DefaultService{
		profileID:           profileID,
		adapters:            adapters,
		compressionPipeline: compressionpkg.NewPipeline(),
		selector:            selector,
		config:              config,
	}
}

// Init initializes the compression service with strategies
func (s *DefaultService) Init() error {
	if !s.config.EnableCompression {
		return nil
	}

	// Get first available adapter for compression initialization
	var adapterForCompression adapter.Adapter
	for _, adp := range s.adapters {
		adapterForCompression = adp
		break
	}

	if adapterForCompression == nil {
		return fmt.Errorf("no adapter available for compression initialization")
	}

	// Register compression strategies based on config
	switch s.config.DefaultStrategy {
	case "sliding_window", "hybrid", "":
		// Sliding window is the primary strategy
		strategy := compressionpkg.NewLegacyStrategy(
			compressionpkg.NewSlidingWindowStrategy(adapterForCompression),
			adapterForCompression,
		)
		s.compressionPipeline.Register(strategy)
	default:
		return fmt.Errorf("unsupported compression strategy: %s", s.config.DefaultStrategy)
	}

	return nil
}

// ShouldCompress determines if compression should be applied
func (s *DefaultService) ShouldCompress(profile *model.Profile, mdl *model.Model, messageCount int) bool {
	// Check if compression is enabled at profile level
	if !profile.EnableCompression || !s.config.EnableCompression {
		return false
	}

	// Check if model has specific compression settings
	// (This can be extended based on model-level configuration)

	// For threshold-based compression, check message count
	if profile.CompressionLevel == model.CompressionLevelThreshold {
		// This is a simplified check - in real implementation, you'd check actual token count
		return messageCount > 0 // Will be determined by actual token counting
	}

	// For session-based compression, always compress if enabled
	if profile.CompressionLevel == model.CompressionLevelSession {
		return true
	}

	// Default: don't compress
	return false
}

// Compress applies compression to messages using the configured strategy
func (s *DefaultService) Compress(ctx context.Context, profile *model.Profile, session *model.Session, maxTokens int, compressionGroup *string) ([]model.Message, *model.CompressionMetadata, error) {
	if !s.config.EnableCompression || s.compressionPipeline == nil || !profile.EnableCompression {
		// Return empty messages with empty metadata when compression is disabled
		return []model.Message{}, &model.CompressionMetadata{}, nil
	}

	// Determine compression group name
	groupName := s.getCompressionGroupName(profile, compressionGroup)

	// Create getAdapter function with fallback logic
	getAdapter := func(ctx context.Context) (adapter.Adapter, error) {
		if groupName == "" {
			// Legacy mode: return first available adapter
			for _, adp := range s.adapters {
				return adp, nil
			}
			return nil, fmt.Errorf("no adapter available for compression")
		}
		// Group mode: use compression selector with fallback
		if s.selector != nil {
			adp, _, _, err := s.selector.SelectAdapter(ctx, groupName)
			if err == nil {
				return adp, nil
			}
			// Fall through to legacy adapter on error
		}
		// Fallback: return first available adapter
		for _, adp := range s.adapters {
			return adp, nil
		}
		return nil, fmt.Errorf("no adapter available for compression")
	}

	// Build strategy configs from profile settings
	configs := []compressionpkg.StrategyConfig{
		{
			Name:      profile.CompressionStrategy,
			MaxTokens: maxTokens,
			Weight:    100,
		},
	}

	// Call compression pipeline
	result, err := s.compressionPipeline.Compress(ctx, session, maxTokens, configs, getAdapter)
	if err != nil {
		return nil, nil, err
	}

	// Populate CompressionMetadata
	metadata := &model.CompressionMetadata{
		GroupUsed:    groupName,
		FallbackUsed: groupName != "" && s.selector == nil,
		TokensAfter:  result.TotalTokens,
	}

	// Get tokens before from first strategy stat if available
	if len(result.Stats) > 0 {
		metadata.TokensBefore = result.Stats[0].InputTokens
	}

	// Calculate compression ratio
	if metadata.TokensBefore > 0 {
		metadata.CompressionRatio = float64(metadata.TokensAfter) / float64(metadata.TokensBefore)
	}

	return result.Messages, metadata, nil
}

// getCompressionGroupName determines which compression group to use
// Priority: API override > profile default > empty (legacy mode)
func (s *DefaultService) getCompressionGroupName(profile *model.Profile, apiGroup *string) string {
	if apiGroup != nil && *apiGroup != "" {
		return *apiGroup
	}
	return profile.DefaultCompressionGroup
}

// Close closes the compression service and releases resources
func (s *DefaultService) Close() error {
	if s.selector != nil {
		return s.selector.Close()
	}
	return nil
}

// GetPipeline returns the underlying compression pipeline
func (s *DefaultService) GetPipeline() *compressionpkg.CompressionPipeline {
	return s.compressionPipeline
}

// GetSelector returns the compression group selector
func (s *DefaultService) GetSelector() *repository.CompressionGroupSelector {
	return s.selector
}

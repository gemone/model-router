package compression

import (
	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/repository"
)

// Factory creates compression service instances
type Factory struct {
	config *CompressionConfig
}

// NewFactory creates a new compression service factory
func NewFactory(config *CompressionConfig) *Factory {
	if config == nil {
		config = DefaultConfig()
	}
	return &Factory{config: config}
}

// CreateService creates a compression service for a profile
func (f *Factory) CreateService(profile *model.Profile, adapters map[string]adapter.Adapter, selector *repository.CompressionGroupSelector) Service {
	// Create profile-specific config by merging with defaults
	config := &CompressionConfig{
		EnableCompression:      profile.EnableCompression && f.config.EnableCompression,
		DefaultStrategy:        profile.CompressionStrategy,
		DefaultThreshold:       profile.CompressionThreshold,
		DefaultMaxContextWindow: profile.MaxContextWindow,
	}

	// Use defaults if profile values are not set
	if config.DefaultStrategy == "" {
		config.DefaultStrategy = f.config.DefaultStrategy
	}
	if config.DefaultThreshold == 0 {
		config.DefaultThreshold = f.config.DefaultThreshold
	}
	if config.DefaultMaxContextWindow == 0 {
		config.DefaultMaxContextWindow = f.config.DefaultMaxContextWindow
	}

	svc := NewDefaultService(profile.ID, adapters, selector, config)
	if err := svc.Init(); err != nil {
		// Log error but don't fail - service will still work with basic functionality
		// In production, you might want to log this properly
	}

	return svc
}

// CreateServiceWithoutSelector creates a compression service without a selector (for backward compatibility)
func (f *Factory) CreateServiceWithoutSelector(profile *model.Profile, adapters map[string]adapter.Adapter) Service {
	return f.CreateService(profile, adapters, nil)
}

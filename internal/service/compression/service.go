package compression

import (
	"context"

	"github.com/gemone/model-router/internal/model"
)

// Service defines the compression service interface
type Service interface {
	// ShouldCompress determines if compression should be applied based on profile, model, and message count
	ShouldCompress(profile *model.Profile, mdl *model.Model, messageCount int) bool

	// Compress applies compression to messages using the configured strategy
	// Returns compressed messages, metadata about the compression operation, and any error
	Compress(ctx context.Context, profile *model.Profile, session *model.Session, maxTokens int, compressionGroup *string) ([]model.Message, *model.CompressionMetadata, error)
}

// CompressionConfig holds configuration for compression service
type CompressionConfig struct {
	// EnableCompression enables compression globally
	EnableCompression bool

	// DefaultStrategy is the default compression strategy (sliding_window, hybrid, etc.)
	DefaultStrategy string

	// DefaultThreshold is the default token threshold for compression
	DefaultThreshold int

	// DefaultMaxContextWindow is the default maximum context window size
	DefaultMaxContextWindow int
}

// DefaultConfig returns the default compression configuration
func DefaultConfig() *CompressionConfig {
	return &CompressionConfig{
		EnableCompression:       true,
		DefaultStrategy:         "sliding_window",
		DefaultThreshold:        8000,
		DefaultMaxContextWindow: 16000,
	}
}

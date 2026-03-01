package ensemble

import (
	"time"

	"github.com/gemone/model-router/internal/compression"
	"github.com/gemone/model-router/internal/model"
)

// Chunk represents a portion of messages for parallel processing
type Chunk struct {
	ID         int              // Chunk identifier
	Messages   []model.Message  // Messages in this chunk
	TokenCount int              // Estimated token count
	StartIndex int              // Start index in original messages (optional)
	EndIndex   int              // End index in original messages (optional)
}

// ChunkResult represents the result of processing a single chunk
type ChunkResult struct {
	ChunkID          int                              // Original chunk ID
	Compressed       *compression.CompressedContext   // Compressed content
	Error            error                            // Processing error if any
	Duration         time.Duration                    // Time taken to process
}

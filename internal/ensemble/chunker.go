package ensemble

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/compression"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/tokenizer"
)

// Chunker handles Level 1 parallel chunking and Level 2 compression
type Chunker struct {
	chunkSize     int           // Target tokens per chunk (default: 200k)
	numChunks     int           // Number of parallel chunks (default: 5)
	compressor    *compression.SlidingWindowCompression
}

// ChunkerConfig configures the chunker behavior
type ChunkerConfig struct {
	ChunkSize      int                           // Target tokens per chunk
	NumChunks      int                           // Number of parallel chunks
	Compressor     *compression.SlidingWindowCompression
}

// NewChunker creates a new chunker for parallel chunk processing
func NewChunker(config *ChunkerConfig) *Chunker {
	if config == nil {
		config = &ChunkerConfig{}
	}

	// Set defaults
	chunkSize := config.ChunkSize
	if chunkSize == 0 {
		chunkSize = 200000 // 200k tokens per chunk
	}

	numChunks := config.NumChunks
	if numChunks == 0 {
		numChunks = 5 // 5 parallel chunks
	}

	return &Chunker{
		chunkSize:  chunkSize,
		numChunks:  numChunks,
		compressor: config.Compressor,
	}
}

// ProcessChunks splits messages into chunks and processes them in parallel
func (c *Chunker) ProcessChunks(ctx context.Context, messages []model.Message) ([]Chunk, []ChunkResult, error) {
	if len(messages) == 0 {
		return nil, nil, fmt.Errorf("no messages to process")
	}

	startTime := time.Now()

	// Level 1: Split into parallel chunks
	chunks := c.splitIntoChunks(messages)

	// Process chunks in parallel using Level 2: Small model compression
	results := c.processChunksParallel(ctx, chunks)

	// Note: Timing metrics can be added here for monitoring in production
	duration := time.Since(startTime)
	if duration > 5*time.Second {
		// Log slow chunking operations
		fmt.Printf("[WARN] Chunking took %v for %d messages\n", duration, len(messages))
	}

	return chunks, results, nil
}

// splitIntoChunks divides messages into roughly equal token chunks
func (c *Chunker) splitIntoChunks(messages []model.Message) []Chunk {
	if len(messages) == 0 {
		return []Chunk{}
	}

	// Estimate total tokens
	totalTokens := c.estimateTotalTokens(messages)

	// Calculate optimal chunk size
	targetChunkTokens := totalTokens / c.numChunks
	if targetChunkTokens < c.chunkSize {
		targetChunkTokens = c.chunkSize
	}

	// Split messages into chunks
	chunks := make([]Chunk, 0, c.numChunks)
	currentChunk := Chunk{
		ID: 0,
	}
	currentTokens := 0

	for _, msg := range messages {
		msgTokens := c.estimateMessageTokens(&msg)

		// Check if adding this message would exceed chunk size
		if currentTokens+msgTokens > targetChunkTokens && len(currentChunk.Messages) > 0 {
			// Finalize current chunk
			currentChunk.TokenCount = currentTokens
			chunks = append(chunks, currentChunk)

			// Start new chunk
			currentChunk = Chunk{
				ID: len(chunks),
			}
			currentTokens = 0
		}

		// Add message to current chunk
		currentChunk.Messages = append(currentChunk.Messages, msg)
		currentTokens += msgTokens
	}

	// Add final chunk
	if len(currentChunk.Messages) > 0 {
		currentChunk.TokenCount = currentTokens
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

// processChunksParallel processes chunks in parallel using Level 2 compression
func (c *Chunker) processChunksParallel(ctx context.Context, chunks []Chunk) []ChunkResult {
	results := make([]ChunkResult, len(chunks))
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(chunkIdx int, ch Chunk) {
			defer wg.Done()

			// Level 2: Small model compression (200k -> 20k)
			result := ChunkResult{
				ChunkID: chunkIdx,
			}

			// Apply compression if compressor is available
			if c.compressor != nil {
				compressed, err := c.compressor.Compress(ctx, &model.Session{}, ch.Messages, 20000)
				if err != nil {
					result.Error = fmt.Errorf("chunk %d compression failed: %w", chunkIdx, err)
				} else {
					result.Compressed = compressed
				}
			} else {
				// No compression - create pass-through compressed context
				result.Compressed = &compression.CompressedContext{
					Messages:         ch.Messages,
					OriginalTokens:   ch.TokenCount,
					CompressedTokens: ch.TokenCount,
					CompressionRatio: 1.0,
				}
			}

			results[chunkIdx] = result
		}(i, chunk)
	}

	wg.Wait()
	return results
}

// estimateTotalTokens estimates total tokens across all messages
func (c *Chunker) estimateTotalTokens(messages []model.Message) int {
	return tokenizer.CountTokensForMessages(messages)
}

// estimateMessageTokens estimates tokens for a single message
func (c *Chunker) estimateMessageTokens(msg *model.Message) int {
	return tokenizer.CountTokensForMessage(msg)
}

// GetMetrics returns chunker performance metrics
func (c *Chunker) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"chunk_size":    c.chunkSize,
		"num_chunks":    c.numChunks,
		"has_compressor": c.compressor != nil,
	}
}

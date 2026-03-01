// Package ensemble provides hierarchical ensemble orchestration for 1M context processing.
package ensemble

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/compression"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/vector"
)

// Orchestrator coordinates the 5-level hierarchical compression pipeline:
// Level 1: Parallel chunking (5x 200k)
// Level 2: Small model compression (200k -> 20k)
// Level 3: Vector embedding & loss detection
// Level 4: Large model synthesis (5x20k -> 100k)
// Level 5: Vector-based loss recovery
type Orchestrator struct {
	dispatcher    *Dispatcher
	synthesizer   *Synthesizer
	lossRecovery  *LossRecovery
	compressor    *compression.SlidingWindowCompression // For Level 2 compression
	metrics       map[string]interface{}
	metricsMutex  sync.RWMutex
}

// OrchestratorConfig configures the orchestrator behavior
type OrchestratorConfig struct {
	// Adapter for model calls
	Adapter adapter.Adapter

	// Vector store for embeddings
	VectorStore vector.Store

	// Embedding function for loss detection/recovery
	EmbeddingFunc func(ctx context.Context, text string) ([]float32, error)

	// Model configurations
	SmallCompressionModel string  // Small model for compression (default: gpt-4o-mini)
	LargeSynthesisModel   string  // Large model for synthesis (default: gpt-4-turbo)

	// Pipeline configuration
	ChunkSize             int     // Target tokens per chunk (default: 200k)
	NumChunks             int     // Number of parallel chunks (default: 5)
	MaxOutputTokens       int     // Maximum output tokens (default: 100k)
	LossThreshold         float32 // Similarity threshold for loss detection (default: 0.85)
}

// NewOrchestrator creates a new ensemble orchestrator
func NewOrchestrator(config *OrchestratorConfig) *Orchestrator {
	if config == nil {
		config = &OrchestratorConfig{}
	}

	// Create compression engine for Level 2
	compressor := compression.NewSlidingWindowCompression(config.Adapter)
	if config.SmallCompressionModel != "" {
		compressor.SetSummaryModel(config.SmallCompressionModel)
	}

	// Create dispatcher (Level 1 & 2)
	dispatcher := NewDispatcher()

	// Create synthesizer (Level 4)
	synthesizer := NewSynthesizer(&SynthesizerConfig{
		Adapter:       config.Adapter,
		SynthesisModel: config.LargeSynthesisModel,
		MaxTokens:     config.MaxOutputTokens,
	})

	// Create loss recovery (Level 3 & 5)
	lossRecovery := NewLossRecovery(&LossRecoveryConfig{
		VectorStore:   config.VectorStore,
		EmbeddingFunc: config.EmbeddingFunc,
		Threshold:     config.LossThreshold,
	})

	return &Orchestrator{
		dispatcher:   dispatcher,
		synthesizer:  synthesizer,
		lossRecovery: lossRecovery,
		compressor:   compressor,
		metrics:      make(map[string]interface{}),
	}
}

// Process1MContext processes a large context through the 5-level hierarchical pipeline
func (o *Orchestrator) Process1MContext(ctx context.Context, messages []model.Message) (*ProcessResult, error) {
	if len(messages) == 0 {
		return &ProcessResult{
			Success: true,
			Messages: []model.Message{},
			Metrics:  o.getMetricsSnapshot(),
		}, nil
	}

	startTime := time.Now()
	result := &ProcessResult{
		Metrics: make(map[string]interface{}),
	}

	// Calculate initial token count
	initialTokens := o.estimateMessagesTokens(messages)
	result.Metrics["initial_tokens"] = initialTokens

	// ===== LEVEL 1: Parallel Chunking =====
	level1Start := time.Now()
	contextChunks := o.dispatcher.ChunkContext(messages)
	result.Metrics["level1_duration_ms"] = time.Since(level1Start).Milliseconds()
	result.Metrics["level1_num_chunks"] = len(contextChunks)

	// Convert ContextChunk to Chunk format for processing
	chunks := make([]Chunk, len(contextChunks))
	for i, cc := range contextChunks {
		chunks[i] = Chunk{
			ID:         cc.Index,
			Messages:   cc.Messages,
			TokenCount: len(cc.Messages) * 100, // Rough estimation
		}
	}

	// ===== LEVEL 2: Small Model Compression =====
	// Compress each chunk using the compression engine
	level2Tokens := 0
	successCount := 0
	chunkResults := make([]ChunkResult, len(chunks))
	for i, chunk := range chunks {
		compressed, err := o.compressor.Compress(ctx, &model.Session{}, chunk.Messages, 20000)
		if err != nil {
			chunkResults[i] = ChunkResult{
				ChunkID:    i,
				Compressed: nil,
				Error:      err,
			}
			continue
		}
		chunkResults[i] = ChunkResult{
			ChunkID:    i,
			Compressed: compressed,
			Error:      nil,
		}
		level2Tokens += compressed.CompressedTokens
		successCount++
	}
	result.Metrics["level2_output_tokens"] = level2Tokens
	result.Metrics["level2_success_chunks"] = successCount
	if initialTokens > 0 {
		result.Metrics["level2_compression_ratio"] = float64(level2Tokens) / float64(initialTokens)
	}

	// ===== LEVEL 3: Vector Embedding & Loss Detection =====
	level3Start := time.Now()
	lossInfo, err := o.lossRecovery.DetectLoss(ctx, chunks, chunkResults)
	if err != nil {
		// Log error but continue - loss detection is non-critical
		result.Metrics["level3_error"] = err.Error()
	}
	result.Metrics["level3_duration_ms"] = time.Since(level3Start).Milliseconds()
	result.Metrics["level3_loss_detected"] = len(lossInfo)

	// ===== LEVEL 4: Large Model Synthesis =====
	level4Start := time.Now()
	synthesis, err := o.synthesizer.Synthesize(ctx, chunkResults)
	if err != nil {
		return nil, fmt.Errorf("level 4 synthesis failed: %w", err)
	}
	result.Metrics["level4_duration_ms"] = time.Since(level4Start).Milliseconds()
	result.Metrics["level4_output_tokens"] = synthesis.TotalTokens

	// ===== LEVEL 5: Vector-Based Loss Recovery =====
	level5Start := time.Now()
	recovery, err := o.lossRecovery.RecoverLoss(ctx, lossInfo, synthesis)
	if err != nil {
		// Log error but continue - recovery is non-critical
		result.Metrics["level5_error"] = err.Error()
	}
	result.Metrics["level5_duration_ms"] = time.Since(level5Start).Milliseconds()
	if recovery != nil {
		result.Metrics["level5_recovered"] = recovery.Recovered
		result.Metrics["level5_chunks_recovered"] = len(recovery.RecoveredChunks)
	}

	// Build final result
	result.Messages = synthesis.Messages
	result.Success = true
	result.TotalDuration = time.Since(startTime)
	result.Metrics["total_duration_ms"] = result.TotalDuration.Milliseconds()
	result.Metrics["final_tokens"] = synthesis.TotalTokens
	result.Metrics["overall_compression_ratio"] = float64(synthesis.TotalTokens) / float64(initialTokens)
	result.Metrics["tokens_saved"] = initialTokens - synthesis.TotalTokens

	// Update orchestrator metrics
	o.updateMetrics(result.Metrics)

	return result, nil
}

// ProcessResult represents the result of processing through the 5-level pipeline
type ProcessResult struct {
	Success       bool                   // Overall success status
	Messages      []model.Message        // Final processed messages
	TotalDuration time.Duration          // Total processing time
	Metrics       map[string]interface{} // Detailed metrics per level
}

// GetMetrics returns current orchestrator metrics
func (o *Orchestrator) GetMetrics() map[string]interface{} {
	o.metricsMutex.RLock()
	defer o.metricsMutex.RUnlock()

	// Return a copy to avoid concurrent modification
	metrics := make(map[string]interface{})
	for k, v := range o.metrics {
		metrics[k] = v
	}

	// Add component metrics
	metrics["dispatcher"] = map[string]interface{}{
		"max_workers": o.dispatcher.maxWorkers,
		"chunk_size":  o.dispatcher.chunkSize,
	}
	metrics["synthesizer"] = o.synthesizer.GetMetrics()
	metrics["loss_recovery"] = o.lossRecovery.GetMetrics()

	return metrics
}

// ResetMetrics resets all orchestrator metrics
func (o *Orchestrator) ResetMetrics() {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()
	o.metrics = make(map[string]interface{})
}

// updateMetrics updates the orchestrator metrics with new data
func (o *Orchestrator) updateMetrics(newMetrics map[string]interface{}) {
	o.metricsMutex.Lock()
	defer o.metricsMutex.Unlock()

	// Initialize counters if not present
	if _, exists := o.metrics["total_processed"]; !exists {
		o.metrics["total_processed"] = 0
	}
	if _, exists := o.metrics["total_tokens_saved"]; !exists {
		o.metrics["total_tokens_saved"] = 0
	}
	if _, exists := o.metrics["total_duration_ms"]; !exists {
		o.metrics["total_duration_ms"] = int64(0)
	}

	// Update counters
	o.metrics["total_processed"] = o.metrics["total_processed"].(int) + 1
	if tokensSaved, ok := newMetrics["tokens_saved"].(int); ok {
		o.metrics["total_tokens_saved"] = o.metrics["total_tokens_saved"].(int) + tokensSaved
	}
	if duration, ok := newMetrics["total_duration_ms"].(int64); ok {
		o.metrics["total_duration_ms"] = o.metrics["total_duration_ms"].(int64) + duration
	}

	// Store latest run metrics
	o.metrics["last_run"] = newMetrics
}

// getMetricsSnapshot returns a snapshot of current metrics
func (o *Orchestrator) getMetricsSnapshot() map[string]interface{} {
	o.metricsMutex.RLock()
	defer o.metricsMutex.RUnlock()

	snapshot := make(map[string]interface{})
	for k, v := range o.metrics {
		snapshot[k] = v
	}
	return snapshot
}

// estimateMessagesTokens estimates total tokens for messages
func (o *Orchestrator) estimateMessagesTokens(messages []model.Message) int {
	total := 0
	for i := range messages {
		total += o.estimateMessageTokens(&messages[i])
	}
	return total
}

// estimateMessageTokens estimates tokens for a single message
func (o *Orchestrator) estimateMessageTokens(msg *model.Message) int {
	content := o.contentToString(msg.Content)
	return len(content)/4 + 10 // 10 tokens overhead per message
}

// contentToString converts message content to string
func (o *Orchestrator) contentToString(content interface{}) string {
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

// GetPipelineStatus returns the current status of all pipeline components
func (o *Orchestrator) GetPipelineStatus() map[string]interface{} {
	// Get compression model name safely
	compressionModel := "disabled"
	if o.compressor != nil {
		// Try to get model info if available
		compressionModel = "enabled"
	}

	return map[string]interface{}{
		"level1_chunking": map[string]interface{}{
			"enabled":     true,
			"description": "Parallel chunking (5x 200k)",
		},
		"level2_compression": map[string]interface{}{
			"enabled":     o.compressor != nil,
			"description": "Small model compression (200k -> 20k)",
			"model":       compressionModel,
		},
		"level3_loss_detection": map[string]interface{}{
			"enabled":     o.lossRecovery.embeddingFunc != nil,
			"description": "Vector embedding & loss detection",
		},
		"level4_synthesis": map[string]interface{}{
			"enabled":     o.synthesizer.adapter != nil,
			"description": "Large model synthesis (5x20k -> 100k)",
			"model":       o.synthesizer.synthesisModel,
		},
		"level5_loss_recovery": map[string]interface{}{
			"enabled":     o.lossRecovery.vectorStore != nil && o.lossRecovery.embeddingFunc != nil,
			"description": "Vector-based loss recovery",
		},
	}
}

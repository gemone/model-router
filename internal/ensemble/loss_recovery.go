package ensemble

import (
	"context"
	"fmt"
	"time"

	"github.com/gemone/model-router/internal/compression"
	"github.com/gemone/model-router/internal/vector"
)

// LossRecovery handles Level 3 vector embedding & loss detection and Level 5 vector-based loss recovery
type LossRecovery struct {
	vectorStore   vector.Store
	embeddingFunc func(ctx context.Context, text string) ([]float32, error) // Function to generate embeddings
	threshold     float32 // Similarity threshold for loss detection (default: 0.85)
}

// LossRecoveryConfig configures the loss recovery behavior
type LossRecoveryConfig struct {
	VectorStore   vector.Store
	EmbeddingFunc func(ctx context.Context, text string) ([]float32, error)
	Threshold     float32 // Similarity threshold (0-1)
}

// NewLossRecovery creates a new loss recovery system
func NewLossRecovery(config *LossRecoveryConfig) *LossRecovery {
	if config == nil {
		config = &LossRecoveryConfig{}
	}

	threshold := config.Threshold
	if threshold == 0 {
		threshold = 0.85 // Default 85% similarity threshold
	}

	return &LossRecovery{
		vectorStore:   config.VectorStore,
		embeddingFunc: config.EmbeddingFunc,
		threshold:     threshold,
	}
}

// LossInfo represents information about detected loss
type LossInfo struct {
	ChunkID       int     // Source chunk ID
	Similarity    float32 // Similarity score (0-1)
	OriginalText  string  // Original text that was lost
	RecoveredText string  // Recovered text if available
	MissingInfo   []string // List of missing information detected
}

// RecoveryResult represents the result of loss recovery
type RecoveryResult struct {
	Recovered       bool                   // Whether recovery was performed
	LossDetected    []LossInfo             // Detected losses
	RecoveredChunks map[int]string         // Recovered content by chunk ID
	Metrics         map[string]interface{} // Level 3 & 5 metrics
	Duration        time.Duration          // Time taken for recovery
}

// DetectLoss (Level 3) performs vector embedding and loss detection on compressed chunks
func (lr *LossRecovery) DetectLoss(ctx context.Context, chunks []Chunk, chunkResults []ChunkResult) ([]LossInfo, error) {
	if lr.embeddingFunc == nil {
		// No embedding function - skip loss detection
		return []LossInfo{}, nil
	}

	startTime := time.Now()
	lossInfo := make([]LossInfo, 0)

	// For each chunk, compare original with compressed to detect loss
	for i, chunk := range chunks {
		if i >= len(chunkResults) {
			continue
		}

		result := chunkResults[i]
		if result.Error != nil || result.Compressed == nil {
			continue
		}

		// Generate embedding for original chunk
		originalText := lr.chunkToText(chunk)
		originalEmbedding, err := lr.embeddingFunc(ctx, originalText)
		if err != nil {
			// Log error but continue
			continue
		}

		// Generate embedding for compressed chunk
		compressedText := lr.compressedToText(result.Compressed)
		compressedEmbedding, err := lr.embeddingFunc(ctx, compressedText)
		if err != nil {
			continue
		}

		// Calculate similarity
		similarity := lr.cosineSimilarity(originalEmbedding, compressedEmbedding)

		// Store original embedding for potential recovery
		if lr.vectorStore != nil {
			_ = lr.vectorStore.Store(ctx, fmt.Sprintf("chunk_%d_original", i), originalEmbedding, map[string]interface{}{
				"chunk_id":      i,
				"type":          "original",
				"text":          originalText,
				"token_count":   chunk.TokenCount,
				"created_at":    time.Now().UTC().Format(time.RFC3339),
			})
		}

		// If similarity below threshold, mark as potential loss
		if similarity < lr.threshold {
			missingInfo := lr.identifyMissingInfo(chunk, result.Compressed)

			lossInfo = append(lossInfo, LossInfo{
				ChunkID:      i,
				Similarity:   similarity,
				OriginalText: originalText,
				MissingInfo:  missingInfo,
			})
		}
	}

	// Note: Timing metrics can be added here for monitoring in production
	duration := time.Since(startTime)
	if duration > 3*time.Second {
		// Log slow loss detection operations
		fmt.Printf("[WARN] Loss detection took %v for %d chunks\n", duration, len(chunks))
	}

	return lossInfo, nil
}

// RecoverLoss (Level 5) performs vector-based loss recovery
func (lr *LossRecovery) RecoverLoss(ctx context.Context, lossInfo []LossInfo, synthesized *SynthesisResult) (*RecoveryResult, error) {
	if lr.vectorStore == nil || lr.embeddingFunc == nil {
		return &RecoveryResult{
			Recovered: false,
			Metrics:   map[string]interface{}{"level5_enabled": false},
		}, nil
	}

	startTime := time.Now()
	result := &RecoveryResult{
		RecoveredChunks: make(map[int]string),
		Metrics:         make(map[string]interface{}),
	}

	// Generate embedding for synthesized result
	synthesizedText := lr.synthesisToText(synthesized)
	synthesizedEmbedding, err := lr.embeddingFunc(ctx, synthesizedText)
	if err != nil {
		return result, fmt.Errorf("failed to generate synthesis embedding: %w", err)
	}

	// Store synthesis embedding
	_ = lr.vectorStore.Store(ctx, "synthesis_result", synthesizedEmbedding, map[string]interface{}{
		"type":       "synthesis",
		"text":       synthesizedText,
		"tokens":     synthesized.TotalTokens,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})

	// For each detected loss, try to recover missing information
	recoveredCount := 0
	for _, loss := range lossInfo {
		// Search vector store for similar contexts that might contain missing info
		searchResults, err := lr.vectorStore.Search(ctx, synthesizedEmbedding, 5)
		if err != nil {
			continue
		}

		// Check if any search results contain the missing information
		for _, searchResult := range searchResults {
			if searchResult.Score < lr.threshold {
				continue
			}

			// Extract text from metadata
			if text, ok := searchResult.Metadata["text"].(string); ok {
				// Check if this contains missing information
				if lr.containsMissingInfo(text, loss.MissingInfo) {
					result.RecoveredChunks[loss.ChunkID] = text
					recoveredCount++
					break
				}
			}
		}
	}

	result.Recovered = recoveredCount > 0
	result.LossDetected = lossInfo
	result.Duration = time.Since(startTime)

	// Track Level 5 metrics
	result.Metrics["level5_loss_detected"] = len(lossInfo)
	result.Metrics["level5_chunks_recovered"] = recoveredCount
	result.Metrics["level5_duration_ms"] = result.Duration.Milliseconds()
	if len(lossInfo) > 0 {
		result.Metrics["level5_recovery_rate"] = float64(recoveredCount) / float64(len(lossInfo))
	}

	return result, nil
}

// chunkToText converts a chunk to text string for embedding
func (lr *LossRecovery) chunkToText(chunk Chunk) string {
	var result string
	for _, msg := range chunk.Messages {
		result += fmt.Sprintf("[%s]: %s\n", msg.Role, lr.contentToString(msg.Content))
	}
	return result
}

// compressedToText converts compressed chunk to text string for embedding
func (lr *LossRecovery) compressedToText(compressed *compression.CompressedContext) string {
	var result string
	if compressed.Summary != "" {
		result += "Summary: " + compressed.Summary + "\n"
	}
	for _, msg := range compressed.Messages {
		result += fmt.Sprintf("[%s]: %s\n", msg.Role, lr.contentToString(msg.Content))
	}
	return result
}

// synthesisToText converts synthesis result to text string for embedding
func (lr *LossRecovery) synthesisToText(synthesis *SynthesisResult) string {
	var result string
	if synthesis.Summary != "" {
		result += synthesis.Summary + "\n"
	}
	for _, msg := range synthesis.Messages {
		result += fmt.Sprintf("[%s]: %s\n", msg.Role, lr.contentToString(msg.Content))
	}
	return result
}

// identifyMissingInfo identifies what information might be missing from compression
func (lr *LossRecovery) identifyMissingInfo(chunk Chunk, compressed *compression.CompressedContext) []string {
	// Simplified implementation: check for key phrases in original not in compressed
	originalText := lr.chunkToText(chunk)
	compressedText := lr.compressedToText(compressed)

	// Look for key phrases that might be missing
	missing := make([]string, 0)
	keyPhrases := []string{"action item", "decision", "agreed", "todo", "must", "should", "will"}

	for _, phrase := range keyPhrases {
		// Check if phrase exists in original but not in compressed
		if containsPhrase(originalText, phrase) && !containsPhrase(compressedText, phrase) {
			missing = append(missing, phrase)
		}
	}

	return missing
}

// containsMissingInfo checks if text contains the missing information
func (lr *LossRecovery) containsMissingInfo(text string, missingInfo []string) bool {
	for _, info := range missingInfo {
		if containsPhrase(text, info) {
			return true
		}
	}
	return false
}

// cosineSimilarity calculates cosine similarity between two vectors
func (lr *LossRecovery) cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct float32
	var normA float32
	var normB float32

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt32(normA) * sqrt32(normB))
}

// sqrt32 calculates square root for float32
func sqrt32(x float32) float32 {
	return float32(sqrt(float64(x)))
}

// sqrt is a simple square root implementation
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// containsPhrase checks if text contains a phrase (case-insensitive)
func containsPhrase(text, phrase string) bool {
	return len(text) >= len(phrase) && contains(text, phrase)
}

// contains is a simple contains implementation
func contains(text, substr string) bool {
	return len(text) >= len(substr) && findIndex(text, substr) >= 0
}

// findIndex finds the index of substr in text
func findIndex(text, substr string) int {
	for i := 0; i <= len(text)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(text[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// toLower converts a byte to lowercase
func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// contentToString converts message content to string
func (lr *LossRecovery) contentToString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case interface{}:
		return fmt.Sprintf("%v", v)
	default:
		return ""
	}
}

// GetMetrics returns loss recovery configuration metrics
func (lr *LossRecovery) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"has_vector_store":     lr.vectorStore != nil,
		"has_embedding_func":   lr.embeddingFunc != nil,
		"similarity_threshold": lr.threshold,
	}
}

// Package vector provides vector storage and similarity search capabilities.
package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Vector represents a vector with metadata.
type Vector struct {
	ID       string
	Vector   []float32
	Metadata map[string]interface{}
}

// SearchResult represents a similarity search result.
type SearchResult struct {
	ID       string
	Score    float32
	Metadata map[string]interface{}
}

// HNSWConfig contains HNSW index parameters.
type HNSWConfig struct {
	M               int     // Number of bidirectional links for each node (default: 16)
	EFConstruction  int     // Size of dynamic candidate list for construction (default: 200)
	EF              int     // Size of dynamic candidate list for search (default: 100 for search, can be higher for better recall)
	Metric          string  // Distance metric: "cosine", "euclidean", "dot" (default: "cosine")
}

// DefaultHNSWConfig returns default HNSW configuration.
func DefaultHNSWConfig() *HNSWConfig {
	return &HNSWConfig{
		M:              16,
		EFConstruction: 200,
		EF:             100,
		Metric:         "cosine",
	}
}

// Store defines the interface for vector storage operations.
type Store interface {
	// Store stores a vector with associated metadata under the given ID.
	Store(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error

	// Search finds the most similar vectors to the query vector.
	// Returns up to limit results sorted by similarity score (highest first).
	Search(ctx context.Context, query []float32, limit int) ([]SearchResult, error)

	// Delete removes a vector from storage.
	Delete(ctx context.Context, id string) error

	// Close closes the store and releases resources.
	Close() error
}

// InMemoryStore provides an in-memory implementation of Store for testing and fallback.
type InMemoryStore struct {
	mu    sync.RWMutex
	data  map[string]*Vector
	hnsw  *HNSWIndex
	config *HNSWConfig
}

// NewInMemoryStore creates a new in-memory vector store.
func NewInMemoryStore(config *HNSWConfig) *InMemoryStore {
	if config == nil {
		config = DefaultHNSWConfig()
	}
	return &InMemoryStore{
		data:   make(map[string]*Vector),
		hnsw:   NewHNSWIndex(config),
		config: config,
	}
}

// Store stores a vector with metadata.
func (s *InMemoryStore) Store(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	if len(vector) == 0 {
		return fmt.Errorf("vector cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Copy metadata to avoid external modifications
	metaCopy := make(map[string]interface{})
	for k, v := range metadata {
		metaCopy[k] = v
	}

	vecCopy := make([]float32, len(vector))
	copy(vecCopy, vector)

	v := &Vector{
		ID:       id,
		Vector:   vecCopy,
		Metadata: metaCopy,
	}

	// Check if updating existing vector
	_, exists := s.data[id]
	s.data[id] = v

	// Add to HNSW index
	if exists {
		// For simplicity in this implementation, we rebuild index on update
		// In production, you'd implement proper HNSW update/delete
		s.rebuildIndex()
	} else {
		s.hnsw.Add(id, vecCopy)
	}

	return nil
}

// Search finds similar vectors using HNSW index.
func (s *InMemoryStore) Search(ctx context.Context, query []float32, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		return nil, nil
	}
	if len(query) == 0 {
		return nil, fmt.Errorf("query vector cannot be empty")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.data) == 0 {
		return []SearchResult{}, nil
	}

	// Use HNSW for efficient search
	neighbors := s.hnsw.Search(query, limit)

	results := make([]SearchResult, 0, len(neighbors))
	for _, neighbor := range neighbors {
		if vec, ok := s.data[neighbor.ID]; ok {
			results = append(results, SearchResult{
				ID:       vec.ID,
				Score:    neighbor.Distance,
				Metadata: vec.Metadata,
			})
		}
	}

	return results, nil
}

// Delete removes a vector from storage.
func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("vector with id %s not found", id)
	}

	delete(s.data, id)
	s.rebuildIndex()

	return nil
}

// Close closes the store.
func (s *InMemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]*Vector)
	s.hnsw = NewHNSWIndex(s.config)
	return nil
}

// rebuildIndex rebuilds the HNSW index from current data.
func (s *InMemoryStore) rebuildIndex() {
	s.hnsw = NewHNSWIndex(s.config)
	for _, vec := range s.data {
		s.hnsw.Add(vec.ID, vec.Vector)
	}
}

// GetStats returns statistics about the store.
func (s *InMemoryStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"count":     len(s.data),
		"dimension": s.getDimension(),
		"config": map[string]interface{}{
			"m":                s.config.M,
			"ef_construction":  s.config.EFConstruction,
			"ef":               s.config.EF,
			"metric":           s.config.Metric,
		},
	}
}

// getDimension returns the dimension of vectors in the store.
func (s *InMemoryStore) getDimension() int {
	for _, vec := range s.data {
		return len(vec.Vector)
	}
	return 0
}

// QdrantVectorStore is a placeholder for future Qdrant integration.
// It wraps the in-memory store for now.
type QdrantVectorStore struct {
	inner *InMemoryStore
	// Future fields:
	// client *qdrant.Client
	// collectionName string
	// config *QdrantConfig
}

// QdrantConfig contains Qdrant client configuration.
type QdrantConfig struct {
	Endpoint        string
	APIKey          string
	CollectionName  string
	VectorSize      int
	HNSWConfig      *HNSWConfig
	Timeout         time.Duration
}

// NewQdrantVectorStore creates a new Qdrant-backed vector store.
// Currently uses in-memory fallback. Full Qdrant integration pending.
func NewQdrantVectorStore(config *QdrantConfig) *QdrantVectorStore {
	hnswConfig := config.HNSWConfig
	if hnswConfig == nil {
		hnswConfig = DefaultHNSWConfig()
	}

	return &QdrantVectorStore{
		inner: NewInMemoryStore(hnswConfig),
	}
}

// Store stores a vector (delegates to in-memory store).
func (q *QdrantVectorStore) Store(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error {
	return q.inner.Store(ctx, id, vector, metadata)
}

// Search finds similar vectors (delegates to in-memory store).
func (q *QdrantVectorStore) Search(ctx context.Context, query []float32, limit int) ([]SearchResult, error) {
	return q.inner.Search(ctx, query, limit)
}

// Delete removes a vector (delegates to in-memory store).
func (q *QdrantVectorStore) Delete(ctx context.Context, id string) error {
	return q.inner.Delete(ctx, id)
}

// Close closes the store.
func (q *QdrantVectorStore) Close() error {
	return q.inner.Close()
}

// GetStats returns statistics.
func (q *QdrantVectorStore) GetStats() map[string]interface{} {
	stats := q.inner.GetStats()
	stats["backend"] = "memory" // Will change to "qdrant" when integrated
	return stats
}

// StoreCompressedContext stores a compressed context embedding for deduplication.
func StoreCompressedContext(store Store, id string, embedding []float32, contextText string, tokens int) error {
	metadata := map[string]interface{}{
		"type":        "compressed_context",
		"text":        contextText,
		"tokens":      tokens,
		"stored_at":   time.Now().UTC().Format(time.RFC3339),
	}
	return store.Store(context.Background(), id, embedding, metadata)
}

// FindSimilarContexts searches for similar contexts using cosine similarity.
// Useful for deduplication and context retrieval.
func FindSimilarContexts(store Store, queryEmbedding []float32, threshold float32, limit int) ([]SearchResult, error) {
	results, err := store.Search(context.Background(), queryEmbedding, limit)
	if err != nil {
		return nil, err
	}

	// Filter by threshold (cosine similarity is 0-1, higher is better)
	filtered := make([]SearchResult, 0)
	for _, r := range results {
		if r.Score >= threshold {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// MarshalJSON implements JSON serialization for Vector.
func (v *Vector) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID       string                 `json:"id"`
		Vector   []float32              `json:"vector"`
		Metadata map[string]interface{} `json:"metadata"`
	}{
		ID:       v.ID,
		Vector:   v.Vector,
		Metadata: v.Metadata,
	})
}

// UnmarshalJSON implements JSON deserialization for Vector.
func (v *Vector) UnmarshalJSON(data []byte) error {
	var aux struct {
		ID       string                 `json:"id"`
		Vector   []float32              `json:"vector"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	v.ID = aux.ID
	v.Vector = aux.Vector
	v.Metadata = aux.Metadata
	return nil
}

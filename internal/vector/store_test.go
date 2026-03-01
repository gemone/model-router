package vector

import (
	"context"
	"testing"
)

func TestInMemoryStore_Store(t *testing.T) {
	store := NewInMemoryStore(DefaultHNSWConfig())
	defer store.Close()

	ctx := context.Background()

	// Test storing a vector
	vector := []float32{0.1, 0.2, 0.3, 0.4}
	metadata := map[string]interface{}{
		"type":  "test",
		"value": "example",
	}

	err := store.Store(ctx, "test-id", vector, metadata)
	if err != nil {
		t.Fatalf("failed to store vector: %v", err)
	}

	// Verify stats
	stats := store.GetStats()
	if count, ok := stats["count"].(int); !ok || count != 1 {
		t.Errorf("expected count 1, got %v", stats["count"])
	}
}

func TestInMemoryStore_Search(t *testing.T) {
	store := NewInMemoryStore(DefaultHNSWConfig())
	defer store.Close()

	ctx := context.Background()

	// Store test vectors
	vectors := []struct {
		id     string
		vector []float32
		meta   map[string]interface{}
	}{
		{"doc1", []float32{1.0, 0.0, 0.0, 0.0}, map[string]interface{}{"type": "a"}},
		{"doc2", []float32{0.0, 1.0, 0.0, 0.0}, map[string]interface{}{"type": "b"}},
		{"doc3", []float32{1.0, 0.1, 0.0, 0.0}, map[string]interface{}{"type": "a"}}, // Similar to doc1
	}

	for _, v := range vectors {
		if err := store.Store(ctx, v.id, v.vector, v.meta); err != nil {
			t.Fatalf("failed to store %s: %v", v.id, err)
		}
	}

	// Search for similar vectors
	query := []float32{1.0, 0.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 2)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// First result should be doc1 (exact match)
	if results[0].ID != "doc1" {
		t.Errorf("expected first result to be doc1, got %s", results[0].ID)
	}

	// Score should be high (close to 1.0 for similar vectors)
	if results[0].Score < 0.9 {
		t.Errorf("expected high similarity score, got %f", results[0].Score)
	}
}

func TestInMemoryStore_Delete(t *testing.T) {
	store := NewInMemoryStore(DefaultHNSWConfig())
	defer store.Close()

	ctx := context.Background()

	// Store a vector
	vector := []float32{0.1, 0.2, 0.3, 0.4}
	err := store.Store(ctx, "delete-me", vector, map[string]interface{}{})
	if err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	// Delete it
	err = store.Delete(ctx, "delete-me")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify it's gone
	results, err := store.Search(ctx, vector, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results after deletion, got %d", len(results))
	}
}

func TestInMemoryStore_DeleteNotFound(t *testing.T) {
	store := NewInMemoryStore(DefaultHNSWConfig())
	defer store.Close()

	ctx := context.Background()

	err := store.Delete(ctx, "non-existent")
	if err == nil {
		t.Error("expected error when deleting non-existent vector")
	}
}

func TestInMemoryStore_Update(t *testing.T) {
	store := NewInMemoryStore(DefaultHNSWConfig())
	defer store.Close()

	ctx := context.Background()

	// Store initial vector
	err := store.Store(ctx, "update-test", []float32{1.0, 0.0, 0.0}, map[string]interface{}{"v": 1})
	if err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	// Update with new vector
	err = store.Store(ctx, "update-test", []float32{0.0, 1.0, 0.0}, map[string]interface{}{"v": 2})
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	// Search should find updated vector
	query := []float32{0.0, 1.0, 0.0}
	results, err := store.Search(ctx, query, 1)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Score < 0.9 {
		t.Errorf("expected high similarity after update, got %f", results[0].Score)
	}
}

func TestHNSWConfig_Default(t *testing.T) {
	config := DefaultHNSWConfig()

	if config.M != 16 {
		t.Errorf("expected M=16, got %d", config.M)
	}
	if config.EFConstruction != 200 {
		t.Errorf("expected EFConstruction=200, got %d", config.EFConstruction)
	}
	if config.EF != 100 {
		t.Errorf("expected EF=100, got %d", config.EF)
	}
	if config.Metric != "cosine" {
		t.Errorf("expected metric=cosine, got %s", config.Metric)
	}
}

func TestQdrantVectorStore_Create(t *testing.T) {
	config := &QdrantConfig{
		Endpoint:       "http://localhost:6333",
		CollectionName: "test",
		VectorSize:     384,
		HNSWConfig:     DefaultHNSWConfig(),
	}

	store := NewQdrantVectorStore(config)
	defer store.Close()

	ctx := context.Background()

	// Should work with in-memory fallback
	err := store.Store(ctx, "qdrant-test", []float32{0.1, 0.2, 0.3}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	stats := store.GetStats()
	if backend, ok := stats["backend"].(string); !ok || backend != "memory" {
		t.Errorf("expected backend=memory, got %v", stats["backend"])
	}
}

func TestFindSimilarContexts(t *testing.T) {
	store := NewInMemoryStore(DefaultHNSWConfig())
	defer store.Close()

	// Store compressed contexts
	ctx1 := []float32{0.9, 0.1, 0.0}
	ctx2 := []float32{0.1, 0.9, 0.0}
	ctx3 := []float32{0.85, 0.15, 0.0} // Similar to ctx1

	context.Background()

	_ = StoreCompressedContext(store, "ctx1", ctx1, "context about topic A", 10)
	_ = StoreCompressedContext(store, "ctx2", ctx2, "context about topic B", 10)
	_ = StoreCompressedContext(store, "ctx3", ctx3, "context also about topic A", 10)

	// Find similar contexts
	results, err := FindSimilarContexts(store, []float32{1.0, 0.0, 0.0}, 0.8, 2)
	if err != nil {
		t.Fatalf("failed to find similar contexts: %v", err)
	}

	// Should find ctx1 and ctx3
	if len(results) == 0 {
		t.Error("expected to find similar contexts")
	}

	// All results should have high similarity
	for _, r := range results {
		if r.Score < 0.8 {
			t.Errorf("result %s has low similarity: %f", r.ID, r.Score)
		}
	}
}

func TestCosineDistance(t *testing.T) {
	tests := []struct {
		a        []float32
		b        []float32
		expected float32
	}{
		// Identical vectors
		{[]float32{1.0, 0.0, 0.0}, []float32{1.0, 0.0, 0.0}, 0.0},
		// Orthogonal vectors
		{[]float32{1.0, 0.0, 0.0}, []float32{0.0, 1.0, 0.0}, 1.0},
		// Opposite vectors
		{[]float32{1.0, 0.0, 0.0}, []float32{-1.0, 0.0, 0.0}, 2.0},
	}

	for _, tt := range tests {
		result := cosineDistance(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("cosineDistance(%v, %v) = %f, want %f", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestHNSWIndex_Add(t *testing.T) {
	index := NewHNSWIndex(DefaultHNSWConfig())

	// Add vectors
	index.Add("a", []float32{1.0, 0.0, 0.0})
	index.Add("b", []float32{0.0, 1.0, 0.0})
	index.Add("c", []float32{1.0, 0.1, 0.0})

	if index.Size() != 3 {
		t.Errorf("expected size 3, got %d", index.Size())
	}
}

func TestHNSWIndex_Search(t *testing.T) {
	index := NewHNSWIndex(DefaultHNSWConfig())

	// Add vectors
	index.Add("a", []float32{1.0, 0.0, 0.0})
	index.Add("b", []float32{0.0, 1.0, 0.0})
	index.Add("c", []float32{0.9, 0.1, 0.0})

	// Search for nearest neighbor
	results := index.Search([]float32{1.0, 0.0, 0.0}, 2)

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	// First result should be "a" (exact match)
	if results[0].ID != "a" {
		t.Errorf("expected first result to be 'a', got '%s'", results[0].ID)
	}

	// Distance field contains similarity score (1 - cosine_distance)
	// For exact match, similarity should be close to 1.0
	if results[0].Distance < 0.95 {
		t.Errorf("expected high similarity, got %f", results[0].Distance)
	}
}

func TestStoreInterface(t *testing.T) {
	// Verify InMemoryStore satisfies the Store interface
	var _ Store = (*InMemoryStore)(nil)

	// Verify QdrantVectorStore satisfies the Store interface
	var _ Store = (*QdrantVectorStore)(nil)
}

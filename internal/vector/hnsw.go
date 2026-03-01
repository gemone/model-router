// Package vector provides HNSW (Hierarchical Navigable Small World) index implementation.
package vector

import (
	"math"
	"sort"
)

// Neighbor represents a node neighbor with distance.
type Neighbor struct {
	ID       string
	Distance float32
}

// HNSWIndex implements a basic HNSW graph for approximate nearest neighbor search.
// This is a simplified implementation suitable for in-memory testing.
type HNSWIndex struct {
	config      *HNSWConfig
	graph       map[string]map[string]struct{} // adjacency list
	connections map[string][]Neighbor          // stored neighbors
	vectors     map[string][]float32           // stored vectors
	entryPoint  string                         // entry point for search
	maxLevel    int                            // number of levels in hierarchy
}

// NewHNSWIndex creates a new HNSW index.
func NewHNSWIndex(config *HNSWConfig) *HNSWIndex {
	return &HNSWIndex{
		config:      config,
		graph:       make(map[string]map[string]struct{}),
		connections: make(map[string][]Neighbor),
		vectors:     make(map[string][]float32),
		maxLevel:    1, // Simplified: single level for now
	}
}

// Add adds a vector to the index.
func (h *HNSWIndex) Add(id string, vector []float32) {
	h.vectors[id] = vector
	h.graph[id] = make(map[string]struct{})

	// If this is the first point, make it entry point
	if h.entryPoint == "" {
		h.entryPoint = id
		return
	}

	// Connect to existing nodes
	// In a full HNSW implementation, this would use multiple layers
	// and beam search for construction
	h.connectToNeighbors(id)
}

// connectToNeighbors connects a new node to its nearest neighbors.
func (h *HNSWIndex) connectToNeighbors(id string) {
	vector := h.vectors[id]

	// Find M nearest neighbors (excluding self)
	candidates := make([]Neighbor, 0, len(h.vectors)-1)

	for otherID, otherVector := range h.vectors {
		if otherID == id {
			continue
		}

		dist := h.distance(vector, otherVector)
		candidates = append(candidates, Neighbor{
			ID:       otherID,
			Distance: dist,
		})
	}

	// Sort by distance and keep top M
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Distance < candidates[j].Distance
	})

	maxNeighbors := h.config.M
	if len(candidates) < maxNeighbors {
		maxNeighbors = len(candidates)
	}

	// Add bidirectional connections
	for i := 0; i < maxNeighbors; i++ {
		neighborID := candidates[i].ID
		h.graph[id][neighborID] = struct{}{}
		h.graph[neighborID][id] = struct{}{}

		// Store neighbor info
		h.connections[id] = append(h.connections[id], Neighbor{
			ID:       neighborID,
			Distance: candidates[i].Distance,
		})
	}
}

// Search finds the k nearest neighbors to the query vector.
func (h *HNSWIndex) Search(query []float32, k int) []Neighbor {
	if len(h.vectors) == 0 {
		return []Neighbor{}
	}

	if h.entryPoint == "" {
		return []Neighbor{}
	}

	// Use greedy search starting from entry point
	visited := make(map[string]struct{})
	candidates := make([]Neighbor, 0)

	// Start from entry point
	current := h.entryPoint
	currentDist := h.distance(query, h.vectors[current])
	visited[current] = struct{}{}
	candidates = append(candidates, Neighbor{ID: current, Distance: currentDist})

	// Greedy expansion
	changed := true
	for changed && len(candidates) < len(h.vectors) {
		changed = false
		newCandidates := make([]Neighbor, 0)

		for _, candidate := range candidates {
			// Explore neighbors
			for neighborID := range h.graph[candidate.ID] {
				if _, seen := visited[neighborID]; seen {
					continue
				}
				visited[neighborID] = struct{}{}

				neighborDist := h.distance(query, h.vectors[neighborID])
				newCandidates = append(newCandidates, Neighbor{
					ID:       neighborID,
					Distance: neighborDist,
				})
				changed = true
			}
		}

		// Merge and keep best candidates
		candidates = append(candidates, newCandidates...)
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Distance < candidates[j].Distance
		})

		// Limit candidates for efficiency
		ef := h.config.EF
		if len(candidates) > ef {
			candidates = candidates[:ef]
		}
	}

	// Return top k results (for cosine similarity, convert distance to similarity)
	results := make([]Neighbor, 0, k)
	for i := 0; i < k && i < len(candidates); i++ {
		// Convert distance to similarity (1 - distance for normalized vectors)
		similarity := 1.0 - candidates[i].Distance
		results = append(results, Neighbor{
			ID:       candidates[i].ID,
			Distance: similarity,
		})
	}

	return results
}

// distance computes the distance between two vectors based on the configured metric.
func (h *HNSWIndex) distance(a, b []float32) float32 {
	if len(a) != len(b) {
		return math.MaxFloat32
	}

	switch h.config.Metric {
	case "euclidean":
		return euclideanDistance(a, b)
	case "dot":
		return -dotProduct(a, b) // Negate because we want to minimize distance
	case "cosine":
		fallthrough
	default:
		return cosineDistance(a, b)
	}
}

// cosineDistance computes cosine distance (1 - cosine similarity).
// Assumes vectors are not normalized.
func cosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32

	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 1.0 // Maximum distance for zero vectors
	}

	similarity := dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
	return 1.0 - similarity
}

// euclideanDistance computes L2 distance.
func euclideanDistance(a, b []float32) float32 {
	var sum float32
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return float32(math.Sqrt(float64(sum)))
}

// dotProduct computes dot product of two vectors.
func dotProduct(a, b []float32) float32 {
	var sum float32
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}
	return sum
}

// Size returns the number of vectors in the index.
func (h *HNSWIndex) Size() int {
	return len(h.vectors)
}

// Clear removes all vectors from the index.
func (h *HNSWIndex) Clear() {
	h.graph = make(map[string]map[string]struct{})
	h.connections = make(map[string][]Neighbor)
	h.vectors = make(map[string][]float32)
	h.entryPoint = ""
}

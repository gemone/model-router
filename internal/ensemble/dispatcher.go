package ensemble

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

const (
	maxParallelCalls = 5
	chunkSizeTokens  = 200_000
)

// Dispatcher manages parallel model dispatching with worker pool
type Dispatcher struct {
	mu         sync.RWMutex
	stats      map[string]*ModelStats
	maxWorkers int
	chunkSize  int
}

// ModelStats tracks per-model performance metrics
type ModelStats struct {
	TotalCalls    int64
	SuccessCalls  int64
	ErrorCalls    int64
	TotalLatency  int64 // in milliseconds
	LastError     string
	LastUpdatedAt time.Time
}

// DispatchRequest represents a parallel dispatch request
type DispatchRequest struct {
	// Original request to dispatch
	Request *model.ChatCompletionRequest
	// Models to dispatch to (must implement Adapter)
	Adapters []adapter.Adapter
	// Context for cancellation
	Context context.Context
}

// DispatchResult represents result from a single model
type DispatchResult struct {
	// Model identifier
	Model string
	// Provider type
	Provider model.ProviderType
	// Response from the model
	Response *model.ChatCompletionResponse
	// Error if the call failed
	Error error
	// Latency in milliseconds
	Latency int64
	// Success status
	Success bool
}

// DispatchResponse contains results from all parallel calls
type DispatchResponse struct {
	// Results from each model
	Results []DispatchResult
	// Total number of calls made
	TotalCalls int
	// Number of successful calls
	SuccessCount int
	// Number of failed calls
	ErrorCount int
	// Overall latency in milliseconds
	TotalLatency int64
	// Timestamp when dispatch started
	StartedAt time.Time
	// Timestamp when dispatch completed
	CompletedAt time.Time
}

// ContextChunk represents a chunk of context for parallel processing
type ContextChunk struct {
	Index    int
	Messages []model.Message
	Offset   int
}

// NewDispatcher creates a new parallel dispatcher
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		stats:      make(map[string]*ModelStats),
		maxWorkers: maxParallelCalls,
		chunkSize:  chunkSizeTokens,
	}
}

// Dispatch sends the request to multiple models in parallel
func (d *Dispatcher) Dispatch(req *DispatchRequest) (*DispatchResponse, error) {
	if req == nil || req.Request == nil {
		return nil, fmt.Errorf("nil request")
	}

	if len(req.Adapters) == 0 {
		return nil, fmt.Errorf("no adapters provided")
	}

	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}

	startTime := time.Now()

	// Limit number of parallel calls
	adapters := req.Adapters
	if len(adapters) > d.maxWorkers {
		adapters = adapters[:d.maxWorkers]
	}

	results := make([]DispatchResult, len(adapters))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Dispatch to each adapter in parallel
	for i, adp := range adapters {
		wg.Add(1)
		go func(index int, a adapter.Adapter) {
			defer wg.Done()

			result := d.dispatchToAdapter(ctx, req.Request, a)

			mu.Lock()
			results[index] = result
			d.updateStats(a, result)
			mu.Unlock()

		}(i, adp)
	}

	// Wait for all calls to complete or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All calls completed
	case <-ctx.Done():
		// Context cancelled, some calls may still be running
	}

	// Compile response
	resp := &DispatchResponse{
		Results:     results,
		TotalCalls:  len(results),
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}

	for _, r := range results {
		if r.Success {
			resp.SuccessCount++
		} else {
			resp.ErrorCount++
		}
		resp.TotalLatency += r.Latency
	}

	return resp, nil
}

// dispatchToAdapter sends a request to a single adapter
func (d *Dispatcher) dispatchToAdapter(ctx context.Context, req *model.ChatCompletionRequest, adp adapter.Adapter) DispatchResult {
	startTime := time.Now()
	modelID := string(adp.Type())

	result := DispatchResult{
		Model:    modelID,
		Provider: adp.Type(),
		Success:  false,
	}

	// Clone request to avoid mutation
	clonedReq := d.cloneRequest(req)

	// Execute the call
	resp, err := adp.ChatCompletion(ctx, clonedReq)
	latency := time.Since(startTime).Milliseconds()

	result.Latency = latency

	if err != nil {
		result.Error = fmt.Errorf("adapter %s: %w", modelID, err)
		return result
	}

	result.Response = resp
	result.Success = true

	return result
}

// ChunkContext splits the context into chunks for parallel processing
// For MVP: simple message-based chunking
func (d *Dispatcher) ChunkContext(messages []model.Message) []ContextChunk {
	if len(messages) == 0 {
		return []ContextChunk{}
	}

	// Simple strategy: split by messages
	// Each chunk gets approximately equal number of messages
	totalMessages := len(messages)
	chunkCount := (totalMessages + d.chunkSize - 1) / d.chunkSize
	if chunkCount == 0 {
		chunkCount = 1
	}

	chunks := make([]ContextChunk, chunkCount)
	messagesPerChunk := (totalMessages + chunkCount - 1) / chunkCount

	for i := 0; i < chunkCount; i++ {
		start := i * messagesPerChunk
		end := start + messagesPerChunk
		if end > totalMessages {
			end = totalMessages
		}

		chunks[i] = ContextChunk{
			Index:    i,
			Messages: messages[start:end],
			Offset:   start,
		}
	}

	return chunks
}

// GetStats returns statistics for a specific model
func (d *Dispatcher) GetStats(model string) (*ModelStats, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats, ok := d.stats[model]
	if !ok {
		return nil, false
	}

	// Return a copy to avoid race conditions
	return &ModelStats{
		TotalCalls:    stats.TotalCalls,
		SuccessCalls:  stats.SuccessCalls,
		ErrorCalls:    stats.ErrorCalls,
		TotalLatency:  stats.TotalLatency,
		LastError:     stats.LastError,
		LastUpdatedAt: stats.LastUpdatedAt,
	}, true
}

// GetAllStats returns statistics for all models
func (d *Dispatcher) GetAllStats() map[string]*ModelStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]*ModelStats, len(d.stats))
	for k, v := range d.stats {
		result[k] = &ModelStats{
			TotalCalls:    v.TotalCalls,
			SuccessCalls:  v.SuccessCalls,
			ErrorCalls:    v.ErrorCalls,
			TotalLatency:  v.TotalLatency,
			LastError:     v.LastError,
			LastUpdatedAt: v.LastUpdatedAt,
		}
	}
	return result
}

// GetSuccessRate returns the success rate for a model
func (d *Dispatcher) GetSuccessRate(model string) float64 {
	stats, ok := d.GetStats(model)
	if !ok || stats.TotalCalls == 0 {
		return 0.0
	}
	return float64(stats.SuccessCalls) / float64(stats.TotalCalls)
}

// GetAverageLatency returns the average latency for a model
func (d *Dispatcher) GetAverageLatency(model string) int64 {
	stats, ok := d.GetStats(model)
	if !ok || stats.SuccessCalls == 0 {
		return 0
	}
	return stats.TotalLatency / stats.SuccessCalls
}

// ResetStats resets statistics for a model
func (d *Dispatcher) ResetStats(model string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.stats, model)
}

// ResetAllStats resets all statistics
func (d *Dispatcher) ResetAllStats() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stats = make(map[string]*ModelStats)
}

// updateStats updates statistics after a call
func (d *Dispatcher) updateStats(adp adapter.Adapter, result DispatchResult) {
	d.mu.Lock()
	defer d.mu.Unlock()

	modelID := string(adp.Type())

	stats, ok := d.stats[modelID]
	if !ok {
		stats = &ModelStats{
			LastUpdatedAt: time.Now(),
		}
		d.stats[modelID] = stats
	}

	stats.TotalCalls++
	stats.TotalLatency += result.Latency
	stats.LastUpdatedAt = time.Now()

	if result.Success {
		stats.SuccessCalls++
		stats.LastError = ""
	} else {
		stats.ErrorCalls++
		if result.Error != nil {
			stats.LastError = result.Error.Error()
		}
	}
}

// cloneRequest creates a shallow copy of the request to avoid mutation
func (d *Dispatcher) cloneRequest(req *model.ChatCompletionRequest) *model.ChatCompletionRequest {
	if req == nil {
		return nil
	}

	// Shallow copy is sufficient for most fields
	cloned := *req

	// Deep copy messages to avoid concurrent modification
	if req.Messages != nil {
		cloned.Messages = make([]model.Message, len(req.Messages))
		copy(cloned.Messages, req.Messages)
	}

	return &cloned
}

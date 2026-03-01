package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/compression"
	"github.com/gemone/model-router/internal/ensemble"
	"github.com/gemone/model-router/internal/model"
)

// Aggregator defines the interface for aggregating responses from multiple backends
type Aggregator interface {
	Aggregate(ctx context.Context, req interface{}, backends []BackendModel) (interface{}, error)
}

// BackendModel represents a backend model with its adapter and configuration
type BackendModel struct {
	Adapter   adapter.Adapter
	Model     *model.Model
	TimeoutMs int64
}

// FirstAggregator wraps ensemble.Dispatcher to return the first successful response
type FirstAggregator struct {
	dispatcher *ensemble.Dispatcher
}

// NewFirstAggregator creates a new FirstAggregator
func NewFirstAggregator(dispatcher *ensemble.Dispatcher) *FirstAggregator {
	return &FirstAggregator{
		dispatcher: dispatcher,
	}
}

// Aggregate sends the request to multiple backends and returns the first successful response
func (a *FirstAggregator) Aggregate(ctx context.Context, req interface{}, backends []BackendModel) (interface{}, error) {
	chatReq, ok := req.(*model.ChatCompletionRequest)
	if !ok {
		return nil, fmt.Errorf("FirstAggregator: expected ChatCompletionRequest, got %T", req)
	}

	if len(backends) == 0 {
		return nil, fmt.Errorf("FirstAggregator: no backends provided")
	}

	// Create dispatch request
	adapters := make([]adapter.Adapter, len(backends))
	for i, b := range backends {
		adapters[i] = b.Adapter
	}

	dispatchReq := &ensemble.DispatchRequest{
		Request:  chatReq,
		Adapters: adapters,
		Context:  ctx,
	}

	// Dispatch to all backends in parallel
	resp, err := a.dispatcher.Dispatch(dispatchReq)
	if err != nil {
		return nil, fmt.Errorf("FirstAggregator: dispatch failed: %w", err)
	}

	// Return first successful response
	for _, result := range resp.Results {
		if result.Success && result.Response != nil {
			return result.Response, nil
		}
	}

	// All backends failed
	return nil, fmt.Errorf("FirstAggregator: all %d backends failed", len(resp.Results))
}

// SynthesisAggregator wraps ensemble.Synthesizer to synthesize multiple responses
type SynthesisAggregator struct {
	synthesizer  *ensemble.Synthesizer
	judgeAdapter adapter.Adapter
	minResponses int // Minimum number of responses needed for synthesis
}

// NewSynthesisAggregator creates a new SynthesisAggregator
func NewSynthesisAggregator(synthesizer *ensemble.Synthesizer, judgeAdapter adapter.Adapter) *SynthesisAggregator {
	return &SynthesisAggregator{
		synthesizer:  synthesizer,
		judgeAdapter: judgeAdapter,
		minResponses: 2, // Default: need at least 2 responses
	}
}

// SetMinResponses sets the minimum number of responses required
func (a *SynthesisAggregator) SetMinResponses(min int) {
	a.minResponses = min
}

// Aggregate sends the request to multiple backends and synthesizes the responses
func (a *SynthesisAggregator) Aggregate(ctx context.Context, req interface{}, backends []BackendModel) (interface{}, error) {
	chatReq, ok := req.(*model.ChatCompletionRequest)
	if !ok {
		return nil, fmt.Errorf("SynthesisAggregator: expected ChatCompletionRequest, got %T", req)
	}

	if len(backends) == 0 {
		return nil, fmt.Errorf("SynthesisAggregator: no backends provided")
	}

	// Adjust min responses if needed
	minResponses := a.minResponses
	if minResponses > len(backends) {
		minResponses = len(backends)
	}

	// Collect responses with timeout handling
	type responseResult struct {
		Response *model.ChatCompletionResponse
		Error    error
	}

	results := make([]responseResult, len(backends))
	var wg sync.WaitGroup

	for i, backend := range backends {
		wg.Add(1)
		go func(index int, b BackendModel) {
			defer wg.Done()

			// Create context with timeout
			timeout := time.Duration(b.TimeoutMs) * time.Millisecond
			if timeout == 0 {
				timeout = 30 * time.Second // Default timeout
			}
			backendCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Call the backend
			resp, err := b.Adapter.ChatCompletions(backendCtx, chatReq)
			results[index] = responseResult{Response: resp, Error: err}
		}(i, backend)
	}

	// Wait for all requests to complete or timeout
	wg.Wait()

	// Count successful responses and build chunk results
	successCount := 0
	chunkResults := make([]ensemble.ChunkResult, 0, len(backends))

	for i, r := range results {
		if r.Error == nil && r.Response != nil {
			successCount++
			// Convert response to ChunkResult format expected by synthesizer
			contentStr := ""
			if len(r.Response.Choices) > 0 {
				if s, ok := r.Response.Choices[0].Message.Content.(string); ok {
					contentStr = s
				}
			}

			chunkResults = append(chunkResults, ensemble.ChunkResult{
				ChunkID: i,
				Compressed: &compression.CompressedContext{
					Messages: []model.Message{
						{
							Role:    "assistant",
							Content: contentStr,
						},
					},
					OriginalTokens:   estimateTokens(r.Response),
					CompressedTokens: estimateTokens(r.Response),
				},
				Error: nil,
			})
		}
	}

	// Check if we have enough responses
	if successCount < minResponses {
		// Try to return the first successful response as fallback
		for _, r := range results {
			if r.Error == nil && r.Response != nil {
				return r.Response, nil
			}
		}
		return nil, fmt.Errorf("SynthesisAggregator: only %d successful responses, need at least %d", successCount, minResponses)
	}

	// Synthesize the responses using the synthesizer
	synthesis, err := a.synthesizer.Synthesize(ctx, chunkResults)
	if err != nil {
		// Fallback: return first successful response
		for _, r := range results {
			if r.Error == nil && r.Response != nil {
				return r.Response, nil
			}
		}
		return nil, fmt.Errorf("SynthesisAggregator: synthesis failed: %w", err)
	}

	// Convert synthesis result to response
	if len(synthesis.Messages) > 0 {
		// Create response from synthesized messages
		response := &model.ChatCompletionResponse{
			ID:      fmt.Sprintf("synth-%d", time.Now().Unix()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   chatReq.Model,
			Choices: []model.ChatCompletionChoice{
				{
					Index: 0,
					Message: model.Message{
						Role:    "assistant",
						Content: synthesis.Messages[0].Content,
					},
					FinishReason: "stop",
				},
			},
			Usage: model.Usage{
				PromptTokens:     estimateTokensInRequest(chatReq),
				CompletionTokens: synthesis.TotalTokens,
				TotalTokens:      estimateTokensInRequest(chatReq) + synthesis.TotalTokens,
			},
		}
		return response, nil
	}

	// Fallback: return first successful response
	for _, r := range results {
		if r.Error == nil && r.Response != nil {
			return r.Response, nil
		}
	}
	return nil, fmt.Errorf("SynthesisAggregator: no synthesized messages")
}

// AverageAggregator averages embedding vectors from multiple backends
type AverageAggregator struct {
	minResponses int // Minimum number of responses needed
}

// NewAverageAggregator creates a new AverageAggregator
func NewAverageAggregator() *AverageAggregator {
	return &AverageAggregator{
		minResponses: 1, // Default: need at least 1 response
	}
}

// SetMinResponses sets the minimum number of responses required
func (a *AverageAggregator) SetMinResponses(min int) {
	a.minResponses = min
}

// Aggregate implements the Aggregator interface (not used for embeddings)
func (a *AverageAggregator) Aggregate(ctx context.Context, req interface{}, backends []BackendModel) (interface{}, error) {
	return nil, fmt.Errorf("AverageAggregator: use AggregateEmbeddings for embedding requests")
}

// AggregateEmbeddings sends the request to multiple backends and averages the embedding vectors
func (a *AverageAggregator) AggregateEmbeddings(ctx context.Context, req *model.EmbeddingRequest, backends []BackendModel) (*model.EmbeddingResponse, error) {
	if len(backends) == 0 {
		return nil, fmt.Errorf("AverageAggregator: no backends provided")
	}

	// Adjust min responses if needed
	minResponses := a.minResponses
	if minResponses > len(backends) {
		minResponses = len(backends)
	}

	// Collect responses with timeout handling
	type embeddingResult struct {
		Response *model.EmbeddingResponse
		Error    error
	}

	results := make([]embeddingResult, len(backends))
	var wg sync.WaitGroup

	for i, backend := range backends {
		wg.Add(1)
		go func(index int, b BackendModel) {
			defer wg.Done()

			// Create context with timeout
			timeout := time.Duration(b.TimeoutMs) * time.Millisecond
			if timeout == 0 {
				timeout = 30 * time.Second // Default timeout
			}
			backendCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Call the backend
			resp, err := b.Adapter.Embeddings(backendCtx, req)
			results[index] = embeddingResult{Response: resp, Error: err}
		}(i, backend)
	}

	// Wait for all requests to complete or timeout
	wg.Wait()

	// Collect successful responses
	var successfulResponses []*model.EmbeddingResponse
	for _, r := range results {
		if r.Error == nil && r.Response != nil && len(r.Response.Data) > 0 {
			successfulResponses = append(successfulResponses, r.Response)
		}
	}

	// Check if we have enough responses
	if len(successfulResponses) < minResponses {
		if len(successfulResponses) == 0 {
			return nil, fmt.Errorf("AverageAggregator: no successful responses from %d backends", len(backends))
		}
		// Return the first successful response as fallback
		return successfulResponses[0], nil
	}

	// Average the embeddings
	// Assume all responses have the same number of embeddings with the same dimensions
	if len(successfulResponses[0].Data) == 0 {
		return nil, fmt.Errorf("AverageAggregator: no embedding data in response")
	}

	// Determine dimensions from first response
	dim := len(successfulResponses[0].Data[0].Embedding)
	numEmbeddings := len(successfulResponses[0].Data)

	// Initialize averaged embeddings
	averagedEmbeddings := make([]model.Embedding, numEmbeddings)
	for i := 0; i < numEmbeddings; i++ {
		averagedEmbeddings[i] = model.Embedding{
			Object:    "embedding",
			Embedding: make([]float32, dim),
			Index:     i,
		}
	}

	// Sum all embeddings
	for _, resp := range successfulResponses {
		for i := 0; i < numEmbeddings; i++ {
			for j := 0; j < dim; j++ {
				averagedEmbeddings[i].Embedding[j] += resp.Data[i].Embedding[j]
			}
		}
	}

	// Divide by number of responses to get average
	for i := 0; i < numEmbeddings; i++ {
		for j := 0; j < dim; j++ {
			averagedEmbeddings[i].Embedding[j] /= float32(len(successfulResponses))
		}
	}

	// Create response
	response := &model.EmbeddingResponse{
		Object: "list",
		Data:   averagedEmbeddings,
		Model:  req.Model,
		Usage: model.Usage{
			PromptTokens:     successfulResponses[0].Usage.PromptTokens,
			CompletionTokens: 0,
			TotalTokens:      successfulResponses[0].Usage.TotalTokens,
		},
	}

	return response, nil
}

// Helper functions

// estimateTokens estimates the number of tokens in a response
func estimateTokens(resp interface{}) int {
	if r, ok := resp.(*model.ChatCompletionResponse); ok {
		return r.Usage.TotalTokens
	}
	return 0
}

// estimateTokensInRequest estimates the number of tokens in a request
func estimateTokensInRequest(req *model.ChatCompletionRequest) int {
	total := 0
	for _, msg := range req.Messages {
		content := ""
		switch v := msg.Content.(type) {
		case string:
			content = v
		case []model.ContentPart:
			for _, part := range v {
				if part.Type == "text" {
					content += part.Text + " "
				}
			}
		}
		total += len(content)/4 + 10 // Rough estimate
	}
	return total
}

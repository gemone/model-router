package ensemble

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/compression"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/vector"
)

// mockEnsembleAdapter creates a mock adapter for testing
func newMockEnsembleAdapter() *mockEnsembleAdapter {
	return &mockEnsembleAdapter{}
}

type mockEnsembleAdapter struct{}

func (m *mockEnsembleAdapter) Name() string                          { return "mock-ensemble" }
func (m *mockEnsembleAdapter) Type() model.ProviderType             { return model.ProviderOpenAI }
func (m *mockEnsembleAdapter) Init(config *model.Provider) error    { return nil }
func (m *mockEnsembleAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return &model.ChatCompletionResponse{
		ID:      "test-" + time.Now().Format("20060102150405"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []model.ChatCompletionChoice{
			{
				Index: 0,
				Message: model.Message{
					Role:    "assistant",
					Content: "This is a summary of the conversation covering key topics discussed.",
				},
				FinishReason: "stop",
			},
		},
		Usage: model.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}
func (m *mockEnsembleAdapter) ChatCompletions(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return m.ChatCompletion(ctx, req)
}
func (m *mockEnsembleAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	return nil, nil
}
func (m *mockEnsembleAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	return nil, nil
}
func (m *mockEnsembleAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	return nil, nil
}
func (m *mockEnsembleAdapter) HealthCheck(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *mockEnsembleAdapter) GetRequestHeaders() map[string]string {
	return nil
}
func (m *mockEnsembleAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}
func (m *mockEnsembleAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	return nil, nil
}
func (m *mockEnsembleAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	return nil, nil
}
func (m *mockEnsembleAdapter) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
}

// TestDispatcher tests the parallel dispatcher
func TestDispatcher(t *testing.T) {
	ctx := context.Background()
	dispatcher := NewDispatcher()

	// Create mock adapters
	mock1 := newMockEnsembleAdapter()
	mock2 := newMockEnsembleAdapter()

	// Create test request
	req := &model.ChatCompletionRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hello, this is a test message."},
		},
	}

	// Create dispatch request with wrapped adapters
	dispatchReq := &DispatchRequest{
		Request:  req,
		Adapters: []adapter.Adapter{mock1, mock2},
		Context:  ctx,
	}

	// Execute dispatch
	resp, err := dispatcher.Dispatch(dispatchReq)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	// Verify results
	expectedCalls := 2
	if resp.TotalCalls != expectedCalls {
		t.Errorf("Expected %d total calls, got %d", expectedCalls, resp.TotalCalls)
	}

	if len(resp.Results) != expectedCalls {
		t.Errorf("Expected %d results, got %d", expectedCalls, len(resp.Results))
	}

	// Check success rate
	successCount := 0
	for _, result := range resp.Results {
		if result.Success {
			successCount++
		}
	}

	if successCount != expectedCalls {
		t.Errorf("Expected %d successful calls, got %d", expectedCalls, successCount)
	}

	t.Logf("Dispatch completed: %d/%d successful in %dms",
		resp.SuccessCount, resp.TotalCalls, resp.TotalLatency)
}

// TestChunker tests the context chunker
func TestChunker(t *testing.T) {
	ctx := context.Background()
	mock := newMockEnsembleAdapter()
	compressor := compression.NewSlidingWindowCompression(mock)

	chunker := NewChunker(&ChunkerConfig{
		ChunkSize:  100, // Small chunk size for testing
		NumChunks: 3,
		Compressor: compressor,
	})

	// Create test messages (simulate a long conversation)
	messages := make([]model.Message, 50)
	for i := range messages {
		if i%2 == 0 {
			messages[i] = model.Message{
				Role:    "user",
				Content: "This is user message number " + string(rune('A'+i%26)) + " with some content to test chunking.",
			}
		} else {
			messages[i] = model.Message{
				Role:    "assistant",
				Content: "This is assistant response number " + string(rune('A'+i%26)) + " providing helpful information.",
			}
		}
	}

	// Process chunks
	chunks, results, err := chunker.ProcessChunks(ctx, messages)
	if err != nil {
		t.Fatalf("ProcessChunks failed: %v", err)
	}

	// Verify chunking
	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	if len(chunks) != len(results) {
		t.Errorf("Chunk count (%d) != results count (%d)", len(chunks), len(results))
	}

	// Verify results
	successCount := 0
	for i, result := range results {
		if result.Error == nil && result.Compressed != nil {
			successCount++
		} else if result.Error != nil {
			t.Logf("Chunk %d error: %v", i, result.Error)
		}
	}

	t.Logf("Chunker: %d chunks created, %d/%d processed successfully",
		len(chunks), successCount, len(results))
}

// TestSynthesizer tests the synthesis of compressed chunks
func TestSynthesizer(t *testing.T) {
	ctx := context.Background()
	mock := newMockEnsembleAdapter()

	synthesizer := NewSynthesizer(&SynthesizerConfig{
		Adapter:   mock,
		MaxTokens: 1000,
	})

	// Create test chunk results
	chunkResults := []ChunkResult{
		{
			ChunkID: 0,
			Compressed: &compression.CompressedContext{
				Messages: []model.Message{
					{Role: "system", Content: "Summary of part 1: User asked about testing frameworks."},
				},
				CompressedTokens: 50,
			},
		},
		{
			ChunkID: 1,
			Compressed: &compression.CompressedContext{
				Messages: []model.Message{
					{Role: "system", Content: "Summary of part 2: Discussion covered unit testing practices."},
				},
				CompressedTokens: 50,
			},
		},
		{
			ChunkID: 2,
			Compressed: &compression.CompressedContext{
				Messages: []model.Message{
					{Role: "system", Content: "Summary of part 3: Integration testing was also mentioned."},
				},
				CompressedTokens: 50,
			},
		},
	}

	// Run synthesis
	result, err := synthesizer.Synthesize(ctx, chunkResults)
	if err != nil {
		t.Fatalf("Synthesize failed: %v", err)
	}

	// Verify synthesis
	if result.Messages == nil {
		t.Error("Expected synthesized messages")
	}

	if result.TotalTokens <= 0 {
		t.Errorf("Expected positive token count, got %d", result.TotalTokens)
	}

	t.Logf("Synthesis: %d input tokens -> %d output tokens (%.2f%% reduction)",
		result.InputTokens, result.TotalTokens, result.ReductionRatio*100)
}

// TestLossRecovery tests the loss detection and recovery
func TestLossRecovery(t *testing.T) {
	ctx := context.Background()

	// Create in-memory vector store for testing
	vectorStore := vector.NewInMemoryStore(nil)

	// Mock embedding function (returns simple hash-based vectors)
	embeddingFunc := func(ctx context.Context, text string) ([]float32, error) {
		vec := make([]float32, 128)
		for i, c := range text {
			if i < len(vec) {
				vec[i] = float32(c) / 255.0
			}
		}
		return vec, nil
	}

	lossRecovery := NewLossRecovery(&LossRecoveryConfig{
		VectorStore:   vectorStore,
		EmbeddingFunc: embeddingFunc,
		Threshold:     0.85,
	})

	// Create test chunks and results
	chunks := []Chunk{
		{
			ID: 0,
			Messages: []model.Message{
				{Role: "user", Content: "Important decision: Use TDD for development."},
				{Role: "assistant", Content: "Agreed, we will implement test-driven development."},
			},
		},
	}

	chunkResults := []ChunkResult{
		{
			ChunkID: 0,
			Compressed: &compression.CompressedContext{
				Messages: []model.Message{
					{Role: "user", Content: "Brief summary..."},
				},
			},
		},
	}

	// Test loss detection
	lossInfo, err := lossRecovery.DetectLoss(ctx, chunks, chunkResults)
	if err != nil {
		t.Fatalf("DetectLoss failed: %v", err)
	}

	t.Logf("Loss detection: found %d potential losses", len(lossInfo))

	// Test recovery
	synthesis := &SynthesisResult{
		Messages: []model.Message{
			{Role: "system", Content: "Synthesized context about testing approaches."},
		},
		TotalTokens: 100,
	}

	recovery, err := lossRecovery.RecoverLoss(ctx, lossInfo, synthesis)
	if err != nil {
		t.Fatalf("RecoverLoss failed: %v", err)
	}

	if recovery.Recovered {
		t.Logf("Recovery: recovered %d chunks", len(recovery.RecoveredChunks))
	} else {
		t.Log("Recovery: no chunks needed recovery (or no vector store match)")
	}

	t.Logf("Loss recovery metrics: %+v", recovery.Metrics)
}

// TestDispatcherStats tests statistics tracking
func TestDispatcherStats(t *testing.T) {
	dispatcher := NewDispatcher()

	// Get stats for non-existent model
	stats, exists := dispatcher.GetStats("nonexistent")
	if exists {
		t.Error("Expected stats to not exist for non-existent model")
	}
	if stats != nil {
		t.Error("Expected nil stats for non-existent model")
	}

	// Verify GetAllStats returns empty map initially
	allStats := dispatcher.GetAllStats()
	if len(allStats) != 0 {
		t.Errorf("Expected empty stats map, got %d entries", len(allStats))
	}
}

// Helper functions

type adapterInterface interface {
	ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error)
	Type() model.ProviderType
}

func wrapAdapters(adapters []adapterInterface) []wrappedAdapter {
	wrapped := make([]wrappedAdapter, len(adapters))
	for i, a := range adapters {
		wrapped[i] = wrappedAdapter{adapter: a}
	}
	return wrapped
}

type wrappedAdapter struct {
	adapter adapterInterface
}

func (w wrappedAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return w.adapter.ChatCompletion(ctx, req)
}

func (w wrappedAdapter) ChatCompletions(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return w.adapter.ChatCompletion(ctx, req)
}

func (w wrappedAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	return nil, nil
}

func (w wrappedAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	return nil, nil
}

func (w wrappedAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	return nil, nil
}

func (w wrappedAdapter) HealthCheck(ctx context.Context) (bool, error) {
	return true, nil
}

func (w wrappedAdapter) Name() string { return "wrapped" }

func (w wrappedAdapter) Type() model.ProviderType { return w.adapter.Type() }

func (w wrappedAdapter) Init(config *model.Provider) error { return nil }

func (w wrappedAdapter) GetRequestHeaders() map[string]string { return nil }

func (w wrappedAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) { return req, nil }

func (w wrappedAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) { return nil, nil }

func (w wrappedAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) { return nil, nil }

func (w wrappedAdapter) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
}

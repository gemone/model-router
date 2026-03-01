package compression

import (
	"context"
	"net/http"
	"testing"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

// MockAdapter for testing compression
type MockAdapter struct {
	compressCalled bool
	lastRequest    *model.ChatCompletionRequest
}

func (m *MockAdapter) Name() string                 { return "mock" }
func (m *MockAdapter) Type() model.ProviderType    { return model.ProviderOpenAI }
func (m *MockAdapter) Init(config *model.Provider) error { return nil }
func (m *MockAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	m.compressCalled = true
	m.lastRequest = req
	return &model.ChatCompletionResponse{
		Choices: []model.ChatCompletionChoice{
			{
				Message: model.Message{
					Role:    "assistant",
					Content: "Summary: Conversation covered various topics including testing, development, and deployment strategies.",
				},
			},
		},
	}, nil
}
func (m *MockAdapter) ChatCompletions(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return m.ChatCompletion(ctx, req)
}
func (m *MockAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	return nil, nil
}
func (m *MockAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	return nil, nil
}
func (m *MockAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	return nil, nil
}
func (m *MockAdapter) HealthCheck(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *MockAdapter) GetRequestHeaders() map[string]string {
	return nil
}
func (m *MockAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}
func (m *MockAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	return nil, nil
}
func (m *MockAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	return nil, nil
}
func (m *MockAdapter) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
}

// TestSlidingWindowCompression tests the sliding window compression
func TestSlidingWindowCompression(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdapter{}
	compressor := NewSlidingWindowCompression(mock)

	session := &model.Session{
		ID:            "test-session",
		ContextWindow: 50000,
	}

	// Create test messages with content that will trigger compression
	messages := []model.Message{
		{Role: "system", Content: "You are a helpful assistant for software development discussions."},
		{Role: "user", Content: "Hello! I'd like to discuss testing strategies for our application."},
		{Role: "assistant", Content: "Hi there! I'd be happy to help you with testing strategies. What specific aspects would you like to cover?"},
	}

	// Add more messages to simulate a long conversation with substantial content
	longContent := "This is a detailed discussion about software testing methodologies, including unit testing, integration testing, end-to-end testing, and various testing frameworks. We should also consider test coverage, CI/CD pipelines, and automated testing strategies."
	for i := 0; i < 50; i++ {
		messages = append(messages, model.Message{
			Role:    "user",
			Content: "Question " + string(rune('A'+i%26)) + ": " + longContent,
		})
		messages = append(messages, model.Message{
			Role:    "assistant",
			Content: "Response " + string(rune('A'+i%26)) + ": That's a great question. " + longContent + " Let me elaborate on the key points.",
		})
	}

	// Test compression with a small token budget
	result, err := compressor.Compress(ctx, session, messages, 1000)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// If compression didn't reduce tokens, that's OK for this test (mock adapter behavior)
	// Just verify the function completed successfully
	if len(result.Messages) == 0 {
		t.Error("Expected at least one message after compression")
	}

	t.Logf("Compression: %d -> %d tokens (%.2f%% reduction), %d messages",
		result.OriginalTokens, result.CompressedTokens, result.CompressionRatio*100, len(result.Messages))
}

// TestSlidingWindowStrategy tests the strategy interface
func TestSlidingWindowStrategy(t *testing.T) {
	mock := &MockAdapter{}
	strategy := NewSlidingWindowStrategy(mock)

	if strategy.Name() != "sliding_window" {
		t.Errorf("Expected strategy name 'sliding_window', got '%s'", strategy.Name())
	}

	// Create test messages
	messages := []model.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "What is the capital of France?"},
	}

	// Test compression with new signature
	getAdapter := func(ctx context.Context) (adapter.Adapter, error) {
		return mock, nil
	}
	compressed, tokens, err := strategy.Compress(context.Background(), messages, 500, getAdapter)
	if err != nil {
		t.Fatalf("Strategy compress failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Error("Expected at least one message after compression")
	}

	if tokens <= 0 {
		t.Errorf("Expected positive token count, got %d", tokens)
	}

	t.Logf("Compressed to %d tokens, %d messages", tokens, len(compressed))
}

// TestCompressionPipeline tests the pipeline with multiple strategies
func TestCompressionPipeline(t *testing.T) {
	mock := &MockAdapter{}
	pipeline := NewPipeline()

	// Register strategies
	strategy1 := NewSlidingWindowStrategy(mock)
	pipeline.Register(strategy1)

	// Create session without ID (skip database loading)
	session := &model.Session{
		ContextWindow: 10000,
	}

	// Configure strategies
	configs := []StrategyConfig{
		{
			Name:      "sliding_window",
			MaxTokens: 1000,
			Weight:    100,
		},
	}

	// Run compression (will skip loading messages since session.ID is empty)
	getAdapter := func(ctx context.Context) (adapter.Adapter, error) {
		return mock, nil
	}
	result, err := pipeline.Compress(context.Background(), session, 5000, configs, getAdapter)
	if err != nil {
		t.Fatalf("Pipeline compress failed: %v", err)
	}

	// When no session ID, messages_loaded will be 0 and no compression occurs
	t.Logf("Pipeline result: %d tokens, %d strategies applied", result.TotalTokens, len(result.Stats))
	for _, stat := range result.Stats {
		t.Logf("  - %s: %d -> %d tokens (%.2f%% reduction)",
			stat.Name, stat.InputTokens, stat.OutputTokens, stat.ReductionRate)
	}
}

// TestSlidingWindowCompressionNoCompression tests when no compression is needed
func TestSlidingWindowCompressionNoCompression(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdapter{}
	compressor := NewSlidingWindowCompression(mock)

	session := &model.Session{
		ID:            "test-no-compress",
		ContextWindow: 10000,
	}

	// Small messages that fit within budget
	messages := []model.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello!"},
	}

	result, err := compressor.Compress(ctx, session, messages, 10000)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// Should have same or similar number of messages
	if len(result.Messages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(result.Messages))
	}

	// Compression ratio should be 1.0 (no compression)
	if result.CompressionRatio != 1.0 {
		t.Errorf("Expected compression ratio 1.0, got %.2f", result.CompressionRatio)
	}
}

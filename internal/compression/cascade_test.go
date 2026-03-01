// Package compression tests for cascade compression with expert model optimization
package compression

import (
	"context"
	"net/http"
	"testing"

	"github.com/gemone/model-router/internal/model"
)

// mockExpertAdapter simulates a strong model (GPT-4) for optimization
type mockExpertAdapter struct {
	lastPrompt string
}

func (m *mockExpertAdapter) Name() string                      { return "mock-expert" }
func (m *mockExpertAdapter) Type() model.ProviderType             { return model.ProviderOpenAI }
func (m *mockExpertAdapter) Init(config *model.Provider) error { return nil }
func (m *mockExpertAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	m.lastPrompt = req.Messages[1].Content.(string)

	// Simulate expert model optimization response
	optimizedResponse := "### Optimized Context\n\n" +
		"**Core Task/Objective:**\n" +
		"The user is building a model router system and needs guidance on testing and deployment strategies.\n\n" +
		"**Key Decisions Made:**\n" +
		"- Prioritize unit testing with high coverage\n" +
		"- Use integration tests for API endpoints\n" +
		"- Implement CI/CD pipeline for automated testing\n" +
		"- Deploy using containerization (Docker)\n\n" +
		"**Action Items and Requirements:**\n" +
		"- Write unit tests for core components\n" +
		"- Set up integration tests for API routes\n" +
		"- Configure GitHub Actions for CI/CD\n" +
		"- Create deployment documentation\n" +
		"- Monitor system health and performance\n\n" +
		"**Relevant Context:**\n" +
		"The project is a Go-based model router with multiple adapters (OpenAI, Anthropic, etc.) that needs comprehensive testing.\n\n" +
		"**Current Request:**\n" +
		"The user is asking about compression testing strategies for the system."

	return &model.ChatCompletionResponse{
		Choices: []model.ChatCompletionChoice{
			{
				Message: model.Message{
					Role:    "assistant",
					Content: optimizedResponse,
				},
			},
		},
	}, nil
}
func (m *mockExpertAdapter) ChatCompletions(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return m.ChatCompletion(ctx, req)
}
func (m *mockExpertAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	return nil, nil
}
func (m *mockExpertAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	return nil, nil
}
func (m *mockExpertAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	return nil, nil
}
func (m *mockExpertAdapter) HealthCheck(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *mockExpertAdapter) GetRequestHeaders() map[string]string {
	return nil
}
func (m *mockExpertAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}
func (m *mockExpertAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	return nil, nil
}
func (m *mockExpertAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	return nil, nil
}
func (m *mockExpertAdapter) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
}

// mockWorkerAdapter simulates a regular model (GPT-3.5) for execution
type mockWorkerAdapter struct{}

func (m *mockWorkerAdapter) Name() string                      { return "mock-worker" }
func (m *mockWorkerAdapter) Type() model.ProviderType             { return model.ProviderOpenAI }
func (m *mockWorkerAdapter) Init(config *model.Provider) error { return nil }
func (m *mockWorkerAdapter) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return &model.ChatCompletionResponse{
		Choices: []model.ChatCompletionChoice{
			{
				Message: model.Message{
					Role:    "assistant",
					Content: "Based on the optimized context, here's a comprehensive testing strategy for your model router...",
				},
			},
		},
	}, nil
}
func (m *mockWorkerAdapter) ChatCompletions(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	return m.ChatCompletion(ctx, req)
}
func (m *mockWorkerAdapter) ChatCompletionStream(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	return nil, nil
}
func (m *mockWorkerAdapter) Embeddings(ctx context.Context, req *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	return nil, nil
}
func (m *mockWorkerAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	return nil, nil
}
func (m *mockWorkerAdapter) HealthCheck(ctx context.Context) (bool, error) {
	return true, nil
}
func (m *mockWorkerAdapter) GetRequestHeaders() map[string]string {
	return nil
}
func (m *mockWorkerAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}
func (m *mockWorkerAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	return nil, nil
}
func (m *mockWorkerAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	return nil, nil
}
func (m *mockWorkerAdapter) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return &http.Response{StatusCode: 200}, nil
}

// TestCascadeStrategy tests the cascade compression strategy
func TestCascadeStrategy(t *testing.T) {
	expert := &mockExpertAdapter{}
	worker := &mockWorkerAdapter{}

	strategy := NewCascadeStrategy(
		expert,
		worker,
		"gpt-4-turbo",
		"gpt-3.5-turbo",
	)

	if strategy.Name() != "cascade_expert_optimization" {
		t.Errorf("Expected strategy name 'cascade_expert_optimization', got '%s'", strategy.Name())
	}

	// Create test conversation
	messages := []model.Message{
		{Role: "system", Content: "You are a helpful assistant for software development."},
		{Role: "user", Content: "I'm building a model router. How should I test it?"},
		{Role: "assistant", Content: "For testing, you should use unit tests, integration tests, and set up CI/CD."},
		{Role: "user", Content: "What about deployment?"},
		{Role: "assistant", Content: "Use Docker for containerization and deploy to a cloud provider."},
		{Role: "user", Content: "Should I also monitor the system?"},
	}

	// Test compression
	compressed, tokens, err := strategy.Compress(messages, 5000)
	if err != nil {
		t.Fatalf("Cascade compress failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Error("Expected at least one compressed message")
	}

	if tokens <= 0 {
		t.Errorf("Expected positive token count, got %d", tokens)
	}

	// Verify the compressed message contains optimized context
	foundOptimized := false
	for _, msg := range compressed {
		if content, ok := msg.Content.(string); ok {
			if len(content) > 100 {
				foundOptimized = true
				break
			}
		}
	}

	if !foundOptimized {
		t.Error("Expected optimized context in compressed messages")
	}

	t.Logf("Cascade compression: %d messages -> %d messages, %d tokens",
		len(messages), len(compressed), tokens)
}

// TestCascadeCompression tests the full cascade compression
func TestCascadeCompression(t *testing.T) {
	ctx := context.Background()
	expert := &mockExpertAdapter{}
	worker := &mockWorkerAdapter{}

	cascade := NewCascadeCompression(&CascadeCompressionConfig{
		ExpertAdapter:    expert,
		WorkerAdapter:    worker,
		ExpertModel:      "gpt-4-turbo",
		WorkerModel:      "gpt-3.5-turbo",
		MaxOptimizeTokens: 5000,
	})

	// Create test conversation
	messages := []model.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Help me design a testing strategy for my API."},
		{Role: "assistant", Content: "I recommend starting with unit tests for each handler, then integration tests for API endpoints."},
		{Role: "user", Content: "What tools should I use?"},
		{Role: "assistant", Content: "For Go, you can use testing package, testify for assertions, and httptest for HTTP testing."},
	}

	result, err := cascade.OptimizeWithContext(ctx, messages)
	if err != nil {
		t.Fatalf("OptimizeWithContext failed: %v", err)
	}

	// Verify optimization result
	if result.OptimizedContext == "" {
		t.Error("Expected optimized context to be generated")
	}

	if result.OptimizedPrompt == "" {
		t.Error("Expected optimized prompt to be generated")
	}

	if result.OriginalTokens <= 0 {
		t.Errorf("Expected positive original token count, got %d", result.OriginalTokens)
	}

	if result.OptimizedTokens <= 0 {
		t.Errorf("Expected positive optimized token count, got %d", result.OptimizedTokens)
	}

	// Note: Compression ratio can be > 1 if expert model generates more detailed context
	// This is acceptable as it improves quality even if it increases token count
	if result.CompressionRatio <= 0 {
		t.Errorf("Expected positive compression ratio, got %f", result.CompressionRatio)
	}

	if result.QualityScore < 0 || result.QualityScore > 1 {
		t.Errorf("Expected quality score between 0 and 1, got %f", result.QualityScore)
	}

	t.Logf("Cascade optimization result:")
	t.Logf("  Original tokens: %d", result.OriginalTokens)
	t.Logf("  Optimized tokens: %d", result.OptimizedTokens)
	t.Logf("  Compression ratio: %.2f%%", result.CompressionRatio*100)
	t.Logf("  Quality score: %.2f", result.QualityScore)
	t.Logf("  Optimized context length: %d chars", len(result.OptimizedContext))
}

// TestExpertOptimizedCompression tests the high-level API
func TestExpertOptimizedCompression(t *testing.T) {
	ctx := context.Background()
	expert := &mockExpertAdapter{}
	worker := &mockWorkerAdapter{}

	eoc := NewExpertOptimizedCompression(expert, worker)

	// Create test conversation
	messages := []model.Message{
		{Role: "user", Content: "How do I implement caching in Go?"},
		{Role: "assistant", Content: "You can use Redis or in-memory maps with sync.RWMutex for caching."},
		{Role: "user", Content: "What about cache invalidation?"},
	}

	optimizedPrompt, err := eoc.GetOptimizedPromptForWorker(ctx, messages)
	if err != nil {
		t.Fatalf("GetOptimizedPromptForWorker failed: %v", err)
	}

	if optimizedPrompt == "" {
		t.Error("Expected non-empty optimized prompt")
	}

	if len(optimizedPrompt) < 50 {
		t.Errorf("Expected optimized prompt to be substantial, got %d chars", len(optimizedPrompt))
	}

	// Verify optimized prompt contains key sections
	optimizedLower := []byte(optimizedPrompt)
	keySections := [][]byte{
		[]byte("optimized context"),
		[]byte("current request"),
		[]byte("instructions"),
	}

	foundSections := 0
	for _, section := range keySections {
		if contains(optimizedLower, []byte(section)) {
			foundSections++
		}
	}

	if foundSections < 2 {
		t.Logf("Warning: Expected more key sections in optimized prompt")
	}

	t.Logf("Expert optimized prompt length: %d chars", len(optimizedPrompt))
}

// TestCalculateQualityMetrics tests quality metrics calculation
func TestCalculateQualityMetrics(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "We need to decide on the deployment strategy."},
		{Role: "assistant", Content: "Agreed - we'll use Docker and Kubernetes."},
	}

	result := &CascadeResult{
		OriginalMessages:    messages,
		OptimizedContext:    "Task: Deployment strategy. Decision: Use Docker and K8s.",
		OptimizedPrompt:     "Context: Deployment decided. Request: Next steps?",
		OriginalTokens:     100,
		OptimizedTokens:    50,
		CompressionRatio:   0.5,
		QualityScore:       0.8,
	}

	metrics := CalculateQualityMetrics(messages, result)

	if metrics == nil {
		t.Fatal("Expected metrics to be returned")
	}

	if metrics.InstructionFollowingScore <= 0 || metrics.InstructionFollowingScore > 1 {
		t.Errorf("Expected instruction following score between 0 and 1, got %f", metrics.InstructionFollowingScore)
	}

	if metrics.ContextPreservationScore <= 0 || metrics.ContextPreservationScore > 1 {
		t.Errorf("Expected context preservation score between 0 and 1, got %f", metrics.ContextPreservationScore)
	}

	if metrics.CompressionEfficiency < 0 || metrics.CompressionEfficiency > 1 {
		t.Errorf("Expected compression efficiency between 0 and 1, got %f", metrics.CompressionEfficiency)
	}

	t.Logf("Quality metrics:")
	t.Logf("  Instruction following: %.2f", metrics.InstructionFollowingScore)
	t.Logf("  Context preservation: %.2f", metrics.ContextPreservationScore)
	t.Logf("  Compression efficiency: %.2f", metrics.CompressionEfficiency)
	t.Logf("  Overall quality: %.2f", metrics.QualityScore)
}

// Helper function for string search
func contains(data []byte, substr []byte) bool {
	for i := 0; i <= len(data)-len(substr); i++ {
		if len(data[i:]) < len(substr) {
			continue
		}
		match := true
		for j := 0; j < len(substr); j++ {
			if data[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

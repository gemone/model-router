// Package adapter provides provider adapter implementations.
package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/middleware"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/utils"
	"github.com/go-resty/resty/v2"
)

const (
	// DefaultOpenAIBaseURL is the default OpenAI API base URL
	DefaultOpenAIBaseURL = "https://api.openai.com/v1"
	// MaxRetries is the maximum number of retry attempts
	MaxRetries = 3
	// DefaultRetryWaitTime is the initial wait time between retries
	DefaultRetryWaitTime = time.Second
)

func init() {
	Register(model.ProviderOpenAI, func() Adapter { return NewOpenAIAdapter() })
}

// OpenAIAdapter implements ProviderAdapter for OpenAI API
type OpenAIAdapter struct {
	client     *resty.Client
	httpClient *http.Client
	provider   *model.Provider
	baseURL    string
	apiKey     string
}

// NewOpenAIAdapter creates a new OpenAI adapter instance
func NewOpenAIAdapter() Adapter {
	return &OpenAIAdapter{}
}

// Name returns the adapter name
func (o *OpenAIAdapter) Name() string {
	return "OpenAI"
}

// Type returns the provider type
func (o *OpenAIAdapter) Type() model.ProviderType {
	return model.ProviderOpenAI
}

// Init initializes the adapter with provider configuration
func (o *OpenAIAdapter) Init(config *model.Provider) error {
	o.provider = config

	// Use configured base URL or default (with env var fallback)
	if config.BaseURL != "" {
		o.baseURL = config.BaseURL
	} else {
		o.baseURL = DefaultOpenAIBaseURL
	}

	// Check for OPENAI_API_BASE_URL environment variable as fallback
	if envBaseURL := os.Getenv("OPENAI_API_BASE_URL"); envBaseURL != "" {
		o.baseURL = envBaseURL
	}

	// Get API key from config (supports both plain and encrypted)
	if config.APIKey != "" {
		o.apiKey = config.APIKey
	} else if config.APIKeyEnc != "" {
		// Decrypt the encrypted API key
		decrypted, err := utils.Decrypt(config.APIKeyEnc)
		if err != nil {
			return fmt.Errorf("failed to decrypt API key: %w", err)
		}
		o.apiKey = decrypted
	}

	// Check for OPENAI_API_KEY environment variable as fallback
	if o.apiKey == "" {
		if envAPIKey := os.Getenv("OPENAI_API_KEY"); envAPIKey != "" {
			o.apiKey = envAPIKey
		} else {
			return fmt.Errorf("API key is required for OpenAI provider (set OPENAI_API_KEY environment variable or provide in config)")
		}
	}

	// Configure resty client with connection pooling and retry logic
	o.client = resty.New().
		SetBaseURL(o.baseURL).
		SetHeader("Authorization", "Bearer "+o.apiKey).
		SetHeader("Content-Type", "application/json").
		SetTimeout(120 * time.Second).
		SetRetryCount(MaxRetries).
		SetRetryWaitTime(DefaultRetryWaitTime).
		SetRetryMaxWaitTime(30 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			// Retry on network errors or 5xx status codes
			return err != nil || r.StatusCode() >= 500
		})

	// Configure connection pool for HTTP client (used for streaming)
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
	o.httpClient = &http.Client{
		Timeout:   120 * time.Second,
		Transport: transport,
	}

	return nil
}

// ChatCompletion executes a non-streaming chat completion request
func (o *OpenAIAdapter) ChatCompletion(ctx context.Context, request *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	start := time.Now()
	providerName := o.provider.Name
	if providerName == "" {
		providerName = "openai"
	}

	// Debug log request
	middleware.LogAdapterRequest(providerName, request.Model, o.baseURL+"/chat/completions", request)

	var response model.ChatCompletionResponse
	var errorResp model.ErrorResponse

	resp, err := o.client.R().
		SetContext(ctx).
		SetBody(request).
		SetResult(&response).
		SetError(&errorResp).
		Post("/chat/completions")

	duration := time.Since(start)

	if err != nil {
		middleware.LogAdapterResponse(providerName, request.Model, 0, map[string]string{"error": err.Error()}, duration)
		return nil, fmt.Errorf("chat completions request failed: %w", err)
	}

	// Debug log response
	middleware.LogAdapterResponse(providerName, request.Model, resp.StatusCode(), response, duration)

	if resp.StatusCode() != http.StatusOK {
		errMsg := fmt.Sprintf("status %d", resp.StatusCode())
		if errorResp.Error.Message != "" {
			errMsg = errorResp.Error.Message
		}
		return nil, fmt.Errorf("chat completions API error (status %d): %s", resp.StatusCode(), errMsg)
	}

	return &response, nil
}

// ChatCompletionStream executes a streaming chat completion request
func (o *OpenAIAdapter) ChatCompletionStream(ctx context.Context, request *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamResponse, error) {
	start := time.Now()
	providerName := o.provider.Name
	if providerName == "" {
		providerName = "openai"
	}

	// Set stream flag
	streamReq := *request
	streamReq.Stream = true

	// Marshal request body
	body, err := json.Marshal(streamReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stream request: %w", err)
	}

	// Debug log request
	middleware.LogAdapterRequest(providerName, request.Model, o.baseURL+"/chat/completions", streamReq)

	// Create HTTP request for streaming
	url := o.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create stream request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	// Execute request
	httpResp, err := o.httpClient.Do(req)
	if err != nil {
		middleware.LogAdapterResponse(providerName, request.Model, 0, map[string]string{"error": err.Error()}, time.Since(start))
		return nil, fmt.Errorf("stream request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		middleware.LogAdapterResponse(providerName, request.Model, httpResp.StatusCode, map[string]string{"error": string(body)}, time.Since(start))
		return nil, fmt.Errorf("stream API error (status %d): %s", httpResp.StatusCode, string(body))
	}

	middleware.DebugLog("Stream request started [%s/%s]", providerName, request.Model)

	// Create response channel
	ch := make(chan *model.ChatCompletionStreamResponse)

	// Start goroutine to process SSE stream
	go func() {
		defer close(ch)
		defer httpResp.Body.Close()

		scanner := bufio.NewScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				continue
			}

			// SSE format: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				break
			}

			// Parse JSON response
			var streamResp model.ChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				// Log error but continue processing
				continue
			}

			// Send response or exit if context cancelled
			select {
			case ch <- &streamResp:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// Embeddings executes an embedding request
func (o *OpenAIAdapter) Embeddings(ctx context.Context, request *model.EmbeddingRequest) (*model.EmbeddingResponse, error) {
	var response model.EmbeddingResponse
	var errorResp model.ErrorResponse

	resp, err := o.client.R().
		SetContext(ctx).
		SetBody(request).
		SetResult(&response).
		SetError(&errorResp).
		Post("/embeddings")

	if err != nil {
		return nil, fmt.Errorf("embeddings request failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		errMsg := fmt.Sprintf("status %d", resp.StatusCode())
		if errorResp.Error.Message != "" {
			errMsg = errorResp.Error.Message
		}
		return nil, fmt.Errorf("embeddings API error (status %d): %s", resp.StatusCode(), errMsg)
	}

	return &response, nil
}

// HealthCheck verifies API connectivity
func (o *OpenAIAdapter) HealthCheck(ctx context.Context) (bool, error) {
	// Use a lightweight models list request for health check
	_, err := o.ListModels(ctx)

	if err != nil {
		return false, err
	}

	return true, nil
}

// ListModels lists available models from the provider
func (o *OpenAIAdapter) ListModels(ctx context.Context) (*model.ListModelsResponse, error) {
	var response model.ListModelsResponse
	var errorResp model.ErrorResponse

	resp, err := o.client.R().
		SetContext(ctx).
		SetResult(&response).
		SetError(&errorResp).
		Get("/models")

	if err != nil {
		return nil, fmt.Errorf("list models request failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		errMsg := fmt.Sprintf("status %d", resp.StatusCode())
		if errorResp.Error.Message != "" {
			errMsg = errorResp.Error.Message
		}
		return nil, fmt.Errorf("list models API error (status %d): %s", resp.StatusCode(), errMsg)
	}

	return &response, nil
}

// GetRequestHeaders returns the default request headers
func (o *OpenAIAdapter) GetRequestHeaders() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + o.apiKey,
	}
}

// ConvertRequest converts OpenAI format request to provider-specific format
func (o *OpenAIAdapter) ConvertRequest(req *model.ChatCompletionRequest) (interface{}, error) {
	return req, nil
}

// ConvertResponse converts provider-specific response to OpenAI format
func (o *OpenAIAdapter) ConvertResponse(resp []byte) (*model.ChatCompletionResponse, error) {
	var response model.ChatCompletionResponse
	if err := json.Unmarshal(resp, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return &response, nil
}

// ConvertStreamResponse converts provider-specific stream response to OpenAI format
func (o *OpenAIAdapter) ConvertStreamResponse(data []byte) (*model.ChatCompletionStreamResponse, error) {
	var response model.ChatCompletionStreamResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream response: %w", err)
	}
	return &response, nil
}

// DoRequest executes an HTTP request
func (o *OpenAIAdapter) DoRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := o.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range o.GetRequestHeaders() {
		req.Header.Set(key, value)
	}

	return o.httpClient.Do(req)
}

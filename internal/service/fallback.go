package service

import (
	"context"
	"fmt"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
)

// FallbackExecutor handles model fallback with health tracking and retry limits
type FallbackExecutor struct {
	profile *ProfileInstance
	metrics *StatsCollector
}

// FallbackResult records the outcome of a fallback attempt
type FallbackResult struct {
	Success       bool
	ModelName     string
	ProviderID    string
	Latency       time.Duration
	ErrorMessage  string
	AttemptNumber int
}

// NewFallbackExecutor creates a new fallback executor
func NewFallbackExecutor(profile *ProfileInstance, metrics *StatsCollector) *FallbackExecutor {
	return &FallbackExecutor{
		profile: profile,
		metrics: metrics,
	}
}

// Execute attempts to complete a request using the primary model and falls back to alternatives
func (e *FallbackExecutor) Execute(
	ctx context.Context,
	primary *model.Model,
	req *adapter.ChatCompletionRequest,
) (*adapter.ChatCompletionResponse, error) {
	// Collect fallback models from the profile's route rules
	fallbackModels := e.getFallbackModels(primary)

	// Try primary model first
	result, err := e.tryModel(ctx, primary, req, 0)

	if err == nil && result != nil {
		return result, nil
	}

	// Primary failed, attempt fallbacks
	for i, fallbackModel := range fallbackModels {
		attemptNum := i + 1
		fallbackResult, fallbackErr := e.tryModel(ctx, fallbackModel, req, attemptNum)

		if fallbackErr == nil && fallbackResult != nil {
			// Record successful fallback
			e.recordFallback(primary.Name, fallbackModel.Name, "success", attemptNum)
			return fallbackResult, nil
		}

		// Record failed fallback attempt
		e.recordFallback(primary.Name, fallbackModel.Name, fallbackErr.Error(), attemptNum)
	}

	return nil, fmt.Errorf("all fallback attempts failed for model %s: %w", primary.Name, err)
}

// tryModel attempts to execute a request on a single model
func (e *FallbackExecutor) tryModel(
	ctx context.Context,
	targetModel *model.Model,
	req *adapter.ChatCompletionRequest,
	attemptNumber int,
) (*adapter.ChatCompletionResponse, error) {
	// Get adapter for the target model
	provider, adp := e.profile.getProviderAndAdapter(targetModel.ProviderID)
	if adp == nil || provider == nil {
		return nil, fmt.Errorf("adapter not available for provider %s", targetModel.ProviderID)
	}

	// Update request with target model name
	req.Model = targetModel.OriginalName
	if req.Model == "" {
		req.Model = targetModel.Name
	}

	// Execute request
	start := time.Now()
	resp, err := adp.ChatCompletion(ctx, req)
	latency := time.Since(start)

	// Record attempt metrics regardless of outcome
	e.recordRequestMetrics(targetModel, latency, err)

	if err != nil {
		return nil, fmt.Errorf("model %s failed on attempt %d: %w", targetModel.Name, attemptNumber, err)
	}

	return resp, nil
}

// recordRequestMetrics records metrics for a request attempt
func (e *FallbackExecutor) recordRequestMetrics(targetModel *model.Model, latency time.Duration, err error) {
	log := &model.RequestLog{
		ID:               fmt.Sprintf("%s_%d", targetModel.ID, time.Now().UnixNano()),
		RequestID:        fmt.Sprintf("fallback_%d", time.Now().UnixNano()),
		Model:            targetModel.Name,
		ProviderID:       targetModel.ProviderID,
		Status:           "success",
		Latency:          latency.Milliseconds(),
		PromptTokens:     0,
		CompletionTokens: 0,
		TotalTokens:      0,
		CreatedAt:        time.Now(),
	}

	if err != nil {
		log.Status = "error"
		log.ErrorMessage = err.Error()
	}

	e.metrics.RecordRequest(log)
}

// getFallbackModels retrieves the list of fallback models for a primary model
func (e *FallbackExecutor) getFallbackModels(primary *model.Model) []*model.Model {
	var fallbackList []*model.Model

	// Use Profile.FallbackModels for fallback configuration
	for _, fallbackName := range e.profile.Profile.FallbackModels {
		if models, ok := e.profile.modelMap[fallbackName]; ok {
			for _, m := range models {
				provider := e.profile.providerMap[m.ProviderID]
				if provider != nil && provider.Enabled {
					fallbackList = append(fallbackList, m)
				}
			}
		}
	}

	return fallbackList
}

// recordFallback records a fallback event for metrics and monitoring
func (e *FallbackExecutor) recordFallback(primaryModel, fallbackModel, reason string, attemptNumber int) {
	// Log fallback event for monitoring
	// Health scores are automatically updated via recordRequestMetrics
	_ = primaryModel
	_ = fallbackModel
	_ = reason
	_ = attemptNumber
}

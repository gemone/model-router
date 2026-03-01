// Package ensemble provides retry logic with fallback for model requests.
package ensemble

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries int     // Maximum number of retry attempts
	BackoffMs  []int   // Backoff durations in milliseconds for each retry attempt
}

// DefaultRetryConfig returns a config with 3 retries and exponential backoff
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 3,
		BackoffMs:  []int{100, 500, 2000}, // Exponential backoff: 100ms, 500ms, 2s
	}
}

// RetryStats tracks retry attempts and statistics
type RetryStats struct {
	mu             sync.Mutex
	AttemptCount   int            // Total number of attempts made
	RetryCount     int            // Number of retries performed
	FallbackCount  int            // Number of fallbacks to alternative models
	ModelAttempts  map[string]int // Attempts per model identifier
	SuccessOnRetry bool           // Whether success was achieved after retry
}

// NewRetryStats creates a new RetryStats instance
func NewRetryStats() *RetryStats {
	return &RetryStats{
		ModelAttempts: make(map[string]int),
	}
}

// RecordAttempt records an attempt for a specific model
func (rs *RetryStats) RecordAttempt(model string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.AttemptCount++
	rs.ModelAttempts[model]++
}

// RecordRetry records a retry attempt
func (rs *RetryStats) RecordRetry() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.RetryCount++
}

// RecordFallback records a fallback to an alternative model
func (rs *RetryStats) RecordFallback() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.FallbackCount++
}

// MarkSuccessOnRetry marks that success was achieved after retry
func (rs *RetryStats) MarkSuccessOnRetry() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.SuccessOnRetry = true
}

// String returns a string representation of the stats
func (rs *RetryStats) String() string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return fmt.Sprintf("attempts=%d retries=%d fallbacks=%d success_on_retry=%v",
		rs.AttemptCount, rs.RetryCount, rs.FallbackCount, rs.SuccessOnRetry)
}

// RequestFunc represents a function that can be retried
type RequestFunc func(ctx context.Context) (interface{}, error)

// RetryWithBackoff executes a request with retry logic and exponential backoff
func RetryWithBackoff(ctx context.Context, config *RetryConfig, stats *RetryStats, model string, fn RequestFunc) (interface{}, error) {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Record attempt
		if stats != nil {
			stats.RecordAttempt(model)
		}

		// Attempt the request
		result, err := fn(ctx)
		if err == nil {
			// Success
			if attempt > 0 && stats != nil {
				stats.MarkSuccessOnRetry()
			}
			return result, nil
		}

		lastErr = err

		// If this is not the last attempt, apply backoff
		if attempt < config.MaxRetries {
			if stats != nil {
				stats.RecordRetry()
			}

			// Get backoff duration for this attempt
			backoffIdx := attempt
			if backoffIdx >= len(config.BackoffMs) {
				backoffIdx = len(config.BackoffMs) - 1
			}
			backoffDuration := time.Duration(config.BackoffMs[backoffIdx]) * time.Millisecond

			// Wait for backoff or context cancellation
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("request canceled during backoff: %w", ctx.Err())
			case <-time.After(backoffDuration):
				// Continue to next attempt
			}
		}
	}

	return nil, fmt.Errorf("max retries (%d) exceeded, last error: %w", config.MaxRetries, lastErr)
}

// ModelProvider represents a model that can be used as a fallback
type ModelProvider interface {
	Execute(ctx context.Context, request interface{}) (interface{}, error)
	Name() string
}

// FallbackResult contains the result and execution details
type FallbackResult struct {
	Result       interface{}     // The successful result
	Stats        *RetryStats     // Retry statistics
	UsedModel    string          // The model that succeeded
	AllAttempts  []string        // List of models attempted in order
	PartialError error           // Error if partial failure occurred
}

// Fallback tries alternative models/providers until one succeeds
func Fallback(ctx context.Context, config *RetryConfig, models []ModelProvider, request interface{}) *FallbackResult {
	stats := NewRetryStats()
	attempts := []string{}
	var lastErr error

	for _, model := range models {
		modelName := model.Name()
		attempts = append(attempts, modelName)

		// Try with retry logic
		result, err := RetryWithBackoff(ctx, config, stats, modelName, func(ctx context.Context) (interface{}, error) {
			return model.Execute(ctx, request)
		})

		if err == nil {
			// Success
			return &FallbackResult{
				Result:      result,
				Stats:       stats,
				UsedModel:   modelName,
				AllAttempts: attempts,
			}
		}

		lastErr = err
		stats.RecordFallback()

		// Check if context is canceled
		if ctx.Err() != nil {
			return &FallbackResult{
				Stats:        stats,
				AllAttempts:  attempts,
				PartialError: fmt.Errorf("fallback canceled after %d models: %w", len(attempts), ctx.Err()),
			}
		}
	}

	// All models failed
	return &FallbackResult{
		Stats:        stats,
		AllAttempts:  attempts,
		PartialError: fmt.Errorf("all %d models failed, last error: %w", len(models), lastErr),
	}
}

// IsPartialFailure checks if the result represents a partial failure
func (fr *FallbackResult) IsPartialFailure() bool {
	return fr.PartialError != nil && fr.Result != nil
}

// IsTotalFailure checks if the result represents a total failure
func (fr *FallbackResult) IsTotalFailure() bool {
	return fr.Result == nil
}

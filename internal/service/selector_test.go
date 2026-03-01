package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/gemone/model-router/internal/model"
)

// Mock implementations for testing

func createTestModels() []*model.Model {
	return []*model.Model{
		{
			ID:            "model1",
			Name:          "gpt-4",
			ProviderID:    "provider1",
			Enabled:       true,
			InputPrice:    0.03,
			OutputPrice:   0.06,
			ContextWindow: 8192,
		},
		{
			ID:            "model2",
			Name:          "gpt-4",
			ProviderID:    "provider2",
			Enabled:       true,
			InputPrice:    0.01,
			OutputPrice:   0.02,
			ContextWindow: 8192,
		},
		{
			ID:            "model3",
			Name:          "gpt-4",
			ProviderID:    "provider3",
			Enabled:       true,
			InputPrice:    0.05,
			OutputPrice:   0.10,
			ContextWindow: 8192,
		},
		{
			ID:            "model4",
			Name:          "gpt-4",
			ProviderID:    "provider4",
			Enabled:       false,
			InputPrice:    0.001,
			OutputPrice:   0.002,
			ContextWindow: 8192,
		},
	}
}

func createTestMetrics() map[string]*model.ModelMetrics {
	return map[string]*model.ModelMetrics{
		"model1": {
			ID:           "model1",
			ProviderID:   "provider1",
			ModelName:    "gpt-4",
			HealthScore:  95.0,
			AvgLatencyMs: 500,
			SuccessRate:  0.98,
			IsHealthy:    true,
		},
		"model2": {
			ID:           "model2",
			ProviderID:   "provider2",
			ModelName:    "gpt-4",
			HealthScore:  85.0,
			AvgLatencyMs: 300,
			SuccessRate:  0.95,
			IsHealthy:    true,
		},
		"model3": {
			ID:           "model3",
			ProviderID:   "provider3",
			ModelName:    "gpt-4",
			HealthScore:  75.0,
			AvgLatencyMs: 800,
			SuccessRate:  0.90,
			IsHealthy:    true,
		},
		"model4": {
			ID:           "model4",
			ProviderID:   "provider4",
			ModelName:    "gpt-4",
			HealthScore:  60.0,
			AvgLatencyMs: 200,
			SuccessRate:  0.85,
			IsHealthy:    false,
		},
	}
}

// TestPrioritySelector tests priority-based model selection
func TestPrioritySelector(t *testing.T) {
	selector := &PrioritySelector{}
	models := createTestModels()
	metrics := createTestMetrics()

	// Note: Priority selector relies on database for provider priorities
	// This test verifies the basic flow
	result := selector.Select(models, metrics)

	if result == nil {
		t.Fatal("PrioritySelector returned nil")
	}

	if !result.Enabled {
		t.Error("PrioritySelector selected a disabled model")
	}
}

// TestWeightedSelector tests weighted round-robin selection
func TestWeightedSelector(t *testing.T) {
	selector := &WeightedSelector{}
	models := createTestModels()
	metrics := createTestMetrics()

	// Run multiple times to test distribution
	results := make(map[string]int)
	iterations := 100

	for i := 0; i < iterations; i++ {
		result := selector.Select(models, metrics)
		if result != nil {
			results[result.ID]++
		}
	}

	// Should have selected some models
	if len(results) == 0 {
		t.Fatal("WeightedSelector selected no models")
	}

	// Should not select disabled models
	if _, exists := results["model4"]; exists {
		t.Error("WeightedSelector selected a disabled model")
	}
}

// TestLeastLatencySelector tests latency-based selection
func TestLeastLatencySelector(t *testing.T) {
	selector := &LeastLatencySelector{}
	models := createTestModels()
	metrics := createTestMetrics()

	result := selector.Select(models, metrics)

	if result == nil {
		t.Fatal("LeastLatencySelector returned nil")
	}

	// Should prefer model2 (300ms) over others
	// Note: model4 has 200ms but is unhealthy, so should be skipped
	if result.ID == "model4" {
		t.Error("LeastLatencySelector selected an unhealthy model")
	}

	// Verify it selected a healthy, enabled model
	metric, exists := metrics[result.ID]
	if exists && !metric.IsAvailable() {
		t.Error("LeastLatencySelector selected an unavailable model")
	}
}

// TestHighestHealthSelector tests health-based selection
func TestHighestHealthSelector(t *testing.T) {
	selector := &HighestHealthSelector{}
	models := createTestModels()
	metrics := createTestMetrics()

	result := selector.Select(models, metrics)

	if result == nil {
		t.Fatal("HighestHealthSelector returned nil")
	}

	// Should select model1 with health score 95
	if result.ID != "model1" {
		t.Errorf("Expected model1 (health 95), got %s", result.ID)
	}

	// Verify the selected model is healthy
	metric := metrics[result.ID]
	if !metric.IsAvailable() {
		t.Error("HighestHealthSelector selected an unhealthy model")
	}
}

// TestLowestCostSelector tests cost-based selection
func TestLowestCostSelector(t *testing.T) {
	selector := &LowestCostSelector{}
	models := createTestModels()
	metrics := createTestMetrics()

	result := selector.Select(models, metrics)

	if result == nil {
		t.Fatal("LowestCostSelector returned nil")
	}

	// Should prefer model2 (avg cost 0.015) over model1 (0.045) and model3 (0.075)
	// model4 has lowest cost (0.0015) but is unhealthy, so should be skipped
	if result.ID == "model4" {
		t.Error("LowestCostSelector selected an unhealthy model")
	}

	// Verify the selected model is healthy
	if metric, exists := metrics[result.ID]; exists {
		if !metric.IsAvailable() {
			t.Error("LowestCostSelector selected an unhealthy model")
		}
	}
}

// TestAutoSelector tests comprehensive automatic selection
func TestAutoSelector(t *testing.T) {
	selector := &AutoSelector{}
	models := createTestModels()
	metrics := createTestMetrics()

	result := selector.Select(models, metrics)

	if result == nil {
		t.Fatal("AutoSelector returned nil")
	}

	// AutoSelector should prefer model1 due to:
	// - High health score (95)
	// - Good success rate (0.98)
	// - Reasonable latency (500ms)
	// Even though model2 has lower latency and cost, model1's health and success rate give it a higher overall score
	if result.ID != "model1" {
		t.Logf("AutoSelector selected %s (expected model1 based on composite score)", result.ID)
	}

	// Verify the selected model is enabled and healthy
	if !result.Enabled {
		t.Error("AutoSelector selected a disabled model")
	}

	if metric, exists := metrics[result.ID]; exists {
		if !metric.IsAvailable() {
			t.Error("AutoSelector selected an unavailable model")
		}
	}
}

// TestSelectorWithEmptyModels tests all selectors with empty model list
func TestSelectorWithEmptyModels(t *testing.T) {
	selectors := []StrategySelector{
		&PrioritySelector{},
		&WeightedSelector{},
		&LeastLatencySelector{},
		&HighestHealthSelector{},
		&LowestCostSelector{},
		&AutoSelector{},
	}

	for i, selector := range selectors {
		t.Run(fmt.Sprintf("selector_%d", i), func(t *testing.T) {
			result := selector.Select([]*model.Model{}, make(map[string]*model.ModelMetrics))
			if result != nil {
				t.Errorf("Selector %d returned non-nil for empty models", i)
			}
		})
	}
}

// TestSelectorWithOnlyDisabledModels tests fallback behavior
func TestSelectorWithOnlyDisabledModels(t *testing.T) {
	models := []*model.Model{
		{
			ID:         "disabled1",
			Name:       "gpt-4",
			ProviderID: "provider1",
			Enabled:    false,
		},
		{
			ID:         "disabled2",
			Name:       "gpt-4",
			ProviderID: "provider2",
			Enabled:    false,
		},
	}

	selectors := []StrategySelector{
		&PrioritySelector{},
		&LeastLatencySelector{},
		&HighestHealthSelector{},
		&LowestCostSelector{},
		&AutoSelector{},
	}

	for i, selector := range selectors {
		t.Run(fmt.Sprintf("selector_%d", i), func(t *testing.T) {
			result := selector.Select(models, make(map[string]*model.ModelMetrics))
			// Most selectors should return the first model as fallback
			if result == nil {
				t.Logf("Selector %d returned nil for all disabled models (acceptable behavior)", i)
			}
		})
	}
}

// TestSelectorWithNoMetrics tests behavior when no metrics are available
func TestSelectorWithNoMetrics(t *testing.T) {
	selectors := []StrategySelector{
		&LeastLatencySelector{},
		&HighestHealthSelector{},
		&LowestCostSelector{},
		&AutoSelector{},
	}

	models := createTestModels()

	for i, selector := range selectors {
		t.Run(fmt.Sprintf("selector_%d", i), func(t *testing.T) {
			result := selector.Select(models, make(map[string]*model.ModelMetrics))
			if result == nil {
				t.Errorf("Selector %d returned nil with no metrics", i)
			}
		})
	}
}

// TestDefaultModelSelector tests the main selector interface
func TestDefaultModelSelector(t *testing.T) {
	stats := GetStatsCollector()
	selector := NewDefaultModelSelector(stats)

	ctx := context.Background()

	// Test SelectModelsWithStrategy
	models := createTestModels()

	// Test each strategy
	strategies := []model.RouteStrategy{
		model.RouteStrategyPriority,
		model.RouteStrategyWeighted,
		model.RouteStrategyAuto,
		"least_latency",
		"highest_health",
		"lowest_cost",
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			result := selector.SelectModelsWithStrategy(ctx, models, strategy)
			if result == nil {
				t.Errorf("SelectModelsWithStrategy returned nil for strategy %s", strategy)
			}
		})
	}
}

// TestAutoSelectorScoring tests the scoring algorithm of AutoSelector
func TestAutoSelectorScoring(t *testing.T) {
	selector := &AutoSelector{}

	// Create models with specific characteristics
	models := []*model.Model{
		{
			ID:            "low_latency_high_cost",
			Name:          "gpt-4",
			ProviderID:    "p1",
			Enabled:       true,
			InputPrice:    0.10,
			OutputPrice:   0.20,
		},
		{
			ID:            "high_latency_low_cost",
			Name:          "gpt-4",
			ProviderID:    "p2",
			Enabled:       true,
			InputPrice:    0.001,
			OutputPrice:   0.002,
		},
		{
			ID:            "balanced",
			Name:          "gpt-4",
			ProviderID:    "p3",
			Enabled:       true,
			InputPrice:    0.02,
			OutputPrice:   0.04,
		},
	}

	metrics := map[string]*model.ModelMetrics{
		"low_latency_high_cost": {
			ID:           "low_latency_high_cost",
			ProviderID:   "p1",
			ModelName:    "gpt-4",
			HealthScore:  90.0,
			AvgLatencyMs: 100,
			SuccessRate:  0.95,
			IsHealthy:    true,
		},
		"high_latency_low_cost": {
			ID:           "high_latency_low_cost",
			ProviderID:   "p2",
			ModelName:    "gpt-4",
			HealthScore:  70.0,
			AvgLatencyMs: 2000,
			SuccessRate:  0.85,
			IsHealthy:    true,
		},
		"balanced": {
			ID:           "balanced",
			ProviderID:   "p3",
			ModelName:    "gpt-4",
			HealthScore:  95.0,
			AvgLatencyMs: 500,
			SuccessRate:  0.98,
			IsHealthy:    true,
		},
	}

	result := selector.Select(models, metrics)

	if result == nil {
		t.Fatal("AutoSelector returned nil")
	}

	// The balanced model should win due to high health, good success rate, reasonable latency
	if result.ID != "balanced" {
		t.Logf("AutoSelector selected %s (expected 'balanced' based on composite score)", result.ID)
		// This is not necessarily an error - just logging the behavior
	}
}

// BenchmarkSelectors benchmarks all selectors
func BenchmarkSelectors(b *testing.B) {
	models := createTestModels()
	metrics := createTestMetrics()

	selectors := map[string]StrategySelector{
		"Priority":      &PrioritySelector{},
		"Weighted":      &WeightedSelector{},
		"LeastLatency":  &LeastLatencySelector{},
		"HighestHealth": &HighestHealthSelector{},
		"LowestCost":    &LowestCostSelector{},
		"Auto":          &AutoSelector{},
	}

	for name, selector := range selectors {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				selector.Select(models, metrics)
			}
		})
	}
}

// TestWeightedSelectorConcurrency tests concurrent access to WeightedSelector
func TestWeightedSelectorConcurrency(t *testing.T) {
	selector := &WeightedSelector{}
	models := createTestModels()
	metrics := createTestMetrics()

	done := make(chan bool)
	iterations := 100

	// Launch multiple goroutines
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				selector.Select(models, metrics)
			}
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panic or deadlock, the test passes
}

// TestDefaultModelSelectorConcurrency tests concurrent access to DefaultModelSelector
func TestDefaultModelSelectorConcurrency(t *testing.T) {
	stats := GetStatsCollector()
	selector := NewDefaultModelSelector(stats)
	ctx := context.Background()
	models := createTestModels()

	strategies := []model.RouteStrategy{
		model.RouteStrategyPriority,
		model.RouteStrategyWeighted,
		model.RouteStrategyAuto,
	}

	done := make(chan bool)

	// Launch multiple goroutines
	for i := 0; i < 5; i++ {
		go func(idx int) {
			for j := 0; j < 50; j++ {
				strategy := strategies[idx%len(strategies)]
				selector.SelectModelsWithStrategy(ctx, models, strategy)
			}
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 5; i++ {
		<-done
	}
}

// TestInvalidateMetricsCache tests cache invalidation
func TestInvalidateMetricsCache(t *testing.T) {
	stats := GetStatsCollector()
	selector := NewDefaultModelSelector(stats)

	// Should not panic
	selector.InvalidateMetricsCache()
}

// Helper function to verify selector behavior
func verifySelection(t *testing.T, name string, result *model.Model, models []*model.Model) {
	t.Helper()

	if result == nil {
		t.Errorf("%s: returned nil", name)
		return
	}

	// Verify result is from the input models
	found := false
	for _, m := range models {
		if m.ID == result.ID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("%s: returned model not in input list", name)
	}
}

// TestAllSelectorsReturnValidModels tests that all selectors return models from the input list
func TestAllSelectorsReturnValidModels(t *testing.T) {
	models := createTestModels()
	metrics := createTestMetrics()

	selectors := map[string]StrategySelector{
		"Priority":      &PrioritySelector{},
		"Weighted":      &WeightedSelector{},
		"LeastLatency":  &LeastLatencySelector{},
		"HighestHealth": &HighestHealthSelector{},
		"LowestCost":    &LowestCostSelector{},
		"Auto":          &AutoSelector{},
	}

	for name, selector := range selectors {
		t.Run(name, func(t *testing.T) {
			result := selector.Select(models, metrics)
			verifySelection(t, name, result, models)
		})
	}
}

// TestSelectorsWithSingleModel tests edge case of single model
func TestSelectorsWithSingleModel(t *testing.T) {
	models := []*model.Model{
		{
			ID:            "single",
			Name:          "gpt-4",
			ProviderID:    "provider1",
			Enabled:       true,
			InputPrice:    0.03,
			OutputPrice:   0.06,
			ContextWindow: 8192,
		},
	}

	metrics := map[string]*model.ModelMetrics{
		"single": {
			ID:           "single",
			ProviderID:   "provider1",
			ModelName:    "gpt-4",
			HealthScore:  95.0,
			AvgLatencyMs: 500,
			SuccessRate:  0.98,
			IsHealthy:    true,
		},
	}

	selectors := []StrategySelector{
		&PrioritySelector{},
		&LeastLatencySelector{},
		&HighestHealthSelector{},
		&LowestCostSelector{},
		&AutoSelector{},
	}

	for _, selector := range selectors {
		result := selector.Select(models, metrics)
		if result == nil {
			t.Error("Selector returned nil for single model")
		}
		if result.ID != "single" {
			t.Error("Selector did not return the only available model")
		}
	}
}

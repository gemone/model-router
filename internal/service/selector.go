package service

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
)

// StrategySelector defines the interface for model selection strategies
type StrategySelector interface {
	Select(models []*model.Model, metrics map[string]*model.ModelMetrics) *model.Model
}

// PrioritySelector selects models by provider priority
type PrioritySelector struct{}

// Select implements StrategySelector for priority-based selection
func (s *PrioritySelector) Select(models []*model.Model, metrics map[string]*model.ModelMetrics) *model.Model {
	if len(models) == 0 {
		return nil
	}

	// Get providers to determine priority
	db := database.GetDB()
	if db == nil {
		// Fallback: return first enabled model
		for _, m := range models {
			if m.Enabled {
				return m
			}
		}
		return models[0]
	}

	var providers []model.Provider
	if err := db.Find(&providers).Error; err != nil {
		// Fallback: return first enabled model
		for _, m := range models {
			if m.Enabled {
				return m
			}
		}
		return models[0]
	}

	// Build priority map
	providerPriority := make(map[string]int)
	for _, p := range providers {
		providerPriority[p.ID] = p.Priority
	}

	// Select model with highest provider priority
	var bestModel *model.Model
	bestPriority := math.MinInt

	for _, m := range models {
		if !m.Enabled {
			continue
		}
		priority := providerPriority[m.ProviderID]
		if priority > bestPriority {
			bestPriority = priority
			bestModel = m
		}
	}

	if bestModel != nil {
		return bestModel
	}

	// Fallback to first enabled model
	for _, m := range models {
		if m.Enabled {
			return m
		}
	}
	return models[0]
}

// WeightedSelector implements weighted round-robin selection
type WeightedSelector struct {
	mu     sync.Mutex
	offset uint64
}

// Select implements StrategySelector for weighted selection
func (s *WeightedSelector) Select(models []*model.Model, metrics map[string]*model.ModelMetrics) *model.Model {
	if len(models) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get providers to determine weights
	db := database.GetDB()
	if db == nil {
		// Fallback: return first enabled model
		for _, m := range models {
			if m.Enabled {
				return m
			}
		}
		return models[0]
	}

	var providers []model.Provider
	if err := db.Find(&providers).Error; err != nil {
		// Fallback: return first enabled model
		for _, m := range models {
			if m.Enabled {
				return m
			}
		}
		return models[0]
	}

	// Build weight map
	providerWeight := make(map[string]int)
	for _, p := range providers {
		weight := p.Weight
		if weight <= 0 {
			weight = 100
		}
		providerWeight[p.ID] = weight
	}

	// Collect enabled models with their weights
	type weightedModel struct {
		model  *model.Model
		weight int
	}

	var weightedModels []weightedModel
	totalWeight := 0

	for _, m := range models {
		if !m.Enabled {
			continue
		}
		weight := providerWeight[m.ProviderID]
		weightedModels = append(weightedModels, weightedModel{model: m, weight: weight})
		totalWeight += weight
	}

	if len(weightedModels) == 0 {
		return models[0]
	}

	// Weighted round-robin selection
	s.offset++
	randVal := int(s.offset) % totalWeight

	cumulative := 0
	for _, wm := range weightedModels {
		cumulative += wm.weight
		if randVal < cumulative {
			return wm.model
		}
	}

	return weightedModels[0].model
}

// LeastLatencySelector selects the model with lowest average latency
type LeastLatencySelector struct{}

// Select implements StrategySelector for latency-based selection
func (s *LeastLatencySelector) Select(models []*model.Model, metrics map[string]*model.ModelMetrics) *model.Model {
	if len(models) == 0 {
		return nil
	}

	var bestModel *model.Model
	bestLatency := int64(math.MaxInt64)

	for _, m := range models {
		if !m.Enabled {
			continue
		}

		latency := int64(0)
		if metric, ok := metrics[m.ID]; ok && metric != nil {
			latency = metric.AvgLatencyMs
		} else {
			// No metrics available, use default high latency
			latency = 10000
		}

		// Consider health score as a factor
		if metric, ok := metrics[m.ID]; ok && metric != nil {
			if !metric.IsAvailable() {
				continue
			}
			// Adjust latency by health score (unhealthy models appear slower)
			if metric.HealthScore < 100 {
				latency = int64(float64(latency) * (100.0 / (metric.HealthScore + 1)))
			}
		}

		if latency < bestLatency {
			bestLatency = latency
			bestModel = m
		}
	}

	if bestModel != nil {
		return bestModel
	}

	// Fallback to first enabled model
	for _, m := range models {
		if m.Enabled {
			return m
		}
	}
	return models[0]
}

// HighestHealthSelector selects the model with highest health score
type HighestHealthSelector struct{}

// Select implements StrategySelector for health-based selection
func (s *HighestHealthSelector) Select(models []*model.Model, metrics map[string]*model.ModelMetrics) *model.Model {
	if len(models) == 0 {
		return nil
	}

	var bestModel *model.Model
	bestHealth := -1.0

	for _, m := range models {
		if !m.Enabled {
			continue
		}

		healthScore := 100.0 // Default health score
		if metric, ok := metrics[m.ID]; ok && metric != nil {
			healthScore = metric.HealthScore
			if !metric.IsAvailable() {
				continue
			}
		}

		if healthScore > bestHealth {
			bestHealth = healthScore
			bestModel = m
		}
	}

	if bestModel != nil {
		return bestModel
	}

	// Fallback to first enabled model
	for _, m := range models {
		if m.Enabled {
			return m
		}
	}
	return models[0]
}

// LowestCostSelector selects the model with lowest cost
type LowestCostSelector struct{}

// Select implements StrategySelector for cost-based selection
func (s *LowestCostSelector) Select(models []*model.Model, metrics map[string]*model.ModelMetrics) *model.Model {
	if len(models) == 0 {
		return nil
	}

	var bestModel *model.Model
	bestCost := math.MaxFloat64

	for _, m := range models {
		if !m.Enabled {
			continue
		}

		// Skip if model is unhealthy
		if metric, ok := metrics[m.ID]; ok && metric != nil {
			if !metric.IsAvailable() {
				continue
			}
		}

		// Calculate average cost per 1K tokens (input + output)
		avgCost := (m.InputPrice + m.OutputPrice) / 2.0

		if avgCost < bestCost {
			bestCost = avgCost
			bestModel = m
		}
	}

	if bestModel != nil {
		return bestModel
	}

	// Fallback to first enabled model
	for _, m := range models {
		if m.Enabled {
			return m
		}
	}
	return models[0]
}

// AutoSelector implements comprehensive automatic selection considering multiple factors
type AutoSelector struct{}

// Select implements StrategySelector for automatic selection
func (s *AutoSelector) Select(models []*model.Model, metrics map[string]*model.ModelMetrics) *model.Model {
	if len(models) == 0 {
		return nil
	}

	// Score each model based on multiple factors
	type modelScore struct {
		model *model.Model
		score float64
	}

	var scoredModels []modelScore

	for _, m := range models {
		if !m.Enabled {
			continue
		}

		score := 0.0

		// Health score (40% weight)
		healthScore := 100.0
		isAvailable := true
		if metric, ok := metrics[m.ID]; ok && metric != nil {
			healthScore = metric.HealthScore
			isAvailable = metric.IsAvailable()
		}

		if !isAvailable {
			continue
		}
		score += healthScore * 0.4

		// Latency score (30% weight) - lower is better
		latencyScore := 100.0
		if metric, ok := metrics[m.ID]; ok && metric != nil && metric.AvgLatencyMs > 0 {
			// Normalize: 1000ms = 0 points, 0ms = 100 points
			latencyScore = math.Max(0, 100-float64(metric.AvgLatencyMs)/10.0)
		}
		score += latencyScore * 0.3

		// Success rate score (20% weight)
		successRate := 1.0
		if metric, ok := metrics[m.ID]; ok && metric != nil {
			successRate = metric.SuccessRate
		}
		score += successRate * 100 * 0.2

		// Cost score (10% weight) - lower cost is better
		avgCost := (m.InputPrice + m.OutputPrice) / 2.0
		costScore := math.Max(0, 100-avgCost*10)
		score += costScore * 0.1

		scoredModels = append(scoredModels, modelScore{model: m, score: score})
	}

	// Select model with highest score
	if len(scoredModels) == 0 {
		// Fallback to first enabled model
		for _, m := range models {
			if m.Enabled {
				return m
			}
		}
		return models[0]
	}

	best := scoredModels[0]
	for _, sm := range scoredModels[1:] {
		if sm.score > best.score {
			best = sm
		}
	}

	return best.model
}

// DefaultModelSelector implements the main model selection logic
type DefaultModelSelector struct {
	mu        sync.RWMutex
	selectors map[model.RouteStrategy]StrategySelector
	stats     *StatsCollector
}

// NewDefaultModelSelector creates a new model selector
func NewDefaultModelSelector(stats *StatsCollector) *DefaultModelSelector {
	return &DefaultModelSelector{
		selectors: make(map[model.RouteStrategy]StrategySelector),
		stats:     stats,
	}
}

// SelectModel selects the best model based on profile configuration and metrics
func (s *DefaultModelSelector) SelectModel(
	ctx context.Context,
	profile *model.Profile,
	requestedModel string,
) (*model.Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get candidate models for the requested model name
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var models []model.Model
	if err := db.Where("name = ? AND enabled = ?", requestedModel, true).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no available models found for: %s", requestedModel)
	}

	// Convert to pointers
	modelPtrs := make([]*model.Model, 0, len(models))
	for i := range models {
		modelPtrs = append(modelPtrs, &models[i])
	}

	// Collect metrics for all models
	metrics := s.collectMetrics(ctx, modelPtrs)

	// Determine strategy from profile (default to auto)
	strategy := profile.DefaultRouteStrategy
	if strategy == "" {
		strategy = model.RouteStrategyAuto
	}

	// Get or create selector for the strategy
	selector := s.getSelector(strategy)
	if selector == nil {
		selector = &AutoSelector{} // Fallback to auto
	}

	// Select model using the strategy
	selected := selector.Select(modelPtrs, metrics)
	if selected == nil {
		return nil, fmt.Errorf("no model selected by strategy %s", strategy)
	}

	return selected, nil
}

// SelectModelsWithStrategy selects a model from a list using a specific strategy
func (s *DefaultModelSelector) SelectModelsWithStrategy(
	ctx context.Context,
	models []*model.Model,
	strategy model.RouteStrategy,
) *model.Model {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(models) == 0 {
		return nil
	}

	// Collect metrics
	metrics := s.collectMetrics(ctx, models)

	// Get selector for strategy
	selector := s.getSelector(strategy)
	if selector == nil {
		selector = &AutoSelector{}
	}

	return selector.Select(models, metrics)
}

// getSelector returns the selector for a strategy, creating it if necessary
func (s *DefaultModelSelector) getSelector(strategy model.RouteStrategy) StrategySelector {
	s.mu.RLock()
	if selector, ok := s.selectors[strategy]; ok {
		s.mu.RUnlock()
		return selector
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if selector, ok := s.selectors[strategy]; ok {
		return selector
	}

	var selector StrategySelector
	switch strategy {
	case model.RouteStrategyPriority:
		selector = &PrioritySelector{}
	case model.RouteStrategyWeighted:
		selector = &WeightedSelector{}
	case model.RouteStrategyAuto:
		selector = &AutoSelector{}
	case "least_latency":
		selector = &LeastLatencySelector{}
	case "highest_health":
		selector = &HighestHealthSelector{}
	case "lowest_cost":
		selector = &LowestCostSelector{}
	default:
		selector = &AutoSelector{}
	}

	s.selectors[strategy] = selector
	return selector
}

// collectMetrics collects metrics for all models
func (s *DefaultModelSelector) collectMetrics(ctx context.Context, models []*model.Model) map[string]*model.ModelMetrics {
	metrics := make(map[string]*model.ModelMetrics)

	db := database.GetDB()
	if db == nil {
		return metrics
	}

	for _, m := range models {
		var metric model.ModelMetrics
		err := db.Where("model_name = ? AND provider_id = ?", m.Name, m.ProviderID).
			Order("updated_at DESC").
			First(&metric).Error

		if err == nil {
			metrics[m.ID] = &metric
		}
	}

	return metrics
}

// InvalidateMetricsCache invalidates any cached metrics (for future use)
func (s *DefaultModelSelector) InvalidateMetricsCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// No-op for now - can add cache later if needed
}

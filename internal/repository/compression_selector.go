package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/cache"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
)

// MetricsRecorder defines the interface for recording compression metrics
type MetricsRecorder interface {
	RecordSelectionLatency(groupName, profileID string, duration time.Duration)
	RecordUsage(groupName, modelSelected, profileID string)
	RecordFallback(groupName, reason, profileID string)
}

// HealthScoreChecker defines the interface for checking health scores
type HealthScoreChecker interface {
	GetHealthScore(providerID, modelName string) float64
}

// CompressionGroupSelector selects models from compression groups based on health scores
type CompressionGroupSelector struct {
	profileID   string
	groupCache  *cache.L2Cache
	healthCache *cache.L2Cache
	stats       HealthScoreChecker
	metrics     MetricsRecorder
}

// NewCompressionGroupSelector initializes groupCache (5min TTL) and healthCache (30sec TTL) using cache.NewL2CacheWithConfig
func NewCompressionGroupSelector(profileID string, stats HealthScoreChecker, metrics MetricsRecorder) (*CompressionGroupSelector, error) {
	groupCache, err := cache.NewL2CacheWithConfig(&cache.L2CacheConfig{
		Addr:       "localhost:6379",
		Password:   "",
		DB:         0,
		DefaultTTL: 5 * time.Minute,
		KeyPrefix:  "model-router:compression-group:",
		MaxRetries: 3,
		PoolSize:   10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create group cache: %w", err)
	}

	healthCache, err := cache.NewL2CacheWithConfig(&cache.L2CacheConfig{
		Addr:       "localhost:6379",
		Password:   "",
		DB:         0,
		DefaultTTL: 30 * time.Second,
		KeyPrefix:  "model-router:compression-health:",
		MaxRetries: 3,
		PoolSize:   10,
	})
	if err != nil {
		groupCache.Close()
		return nil, fmt.Errorf("failed to create health cache: %w", err)
	}

	return &CompressionGroupSelector{
		profileID:   profileID,
		groupCache:  groupCache,
		healthCache: healthCache,
		stats:       stats,
		metrics:     metrics,
	}, nil
}

// SelectAdapter selects the best adapter from a compression group based on health scores
func (s *CompressionGroupSelector) SelectAdapter(ctx context.Context, groupName string) (adapter.Adapter, *model.Model, *model.CompressionMetadata, error) {
	// Start time tracking for metrics
	startTime := time.Now()

	// Get group configuration
	group, err := s.getGroupConfig(ctx, groupName)
	if err != nil {
		s.metrics.RecordFallback(groupName, "failed_to_get_group_config", "unknown")
		return nil, nil, nil, fmt.Errorf("failed to get group config: %w", err)
	}

	if !group.Enabled {
		s.metrics.RecordFallback(groupName, "group_disabled", "unknown")
		return nil, nil, nil, fmt.Errorf("compression group %s is disabled", groupName)
	}

	if len(group.Models) == 0 {
		s.metrics.RecordFallback(groupName, "no_models_in_group", "unknown")
		return nil, nil, nil, fmt.Errorf("no models in compression group %s", groupName)
	}

	// Find the healthiest model
	var bestModelRef *model.ModelReference
	bestHealthScore := -1.0

	for i := range group.Models {
		modelRef := &group.Models[i]

		// Check if model is healthy
		if s.isModelHealthy(ctx, modelRef, group.HealthThreshold) {
			// Get health score from cache or stats
			healthScore := s.getModelHealthScore(ctx, modelRef)
			if healthScore > bestHealthScore {
				bestHealthScore = healthScore
				bestModelRef = modelRef
			}
		}
	}

	if bestModelRef == nil {
		s.metrics.RecordFallback(groupName, "no_healthy_models", group.ProfileID)
		return nil, nil, nil, fmt.Errorf("no healthy models found in compression group %s", groupName)
	}

	// Load the model and provider from database
	db := database.GetDB()
	var dbModel model.Model
	if err := db.Where("name = ? AND provider_id = ?", bestModelRef.ModelName, bestModelRef.ProviderID).First(&dbModel).Error; err != nil {
		s.metrics.RecordFallback(groupName, "model_not_found", group.ProfileID)
		return nil, nil, nil, fmt.Errorf("model not found: %s@%s", bestModelRef.ModelName, bestModelRef.ProviderID)
	}

	var provider model.Provider
	if err := db.First(&provider, "id = ?", bestModelRef.ProviderID).Error; err != nil {
		s.metrics.RecordFallback(groupName, "provider_not_found", group.ProfileID)
		return nil, nil, nil, fmt.Errorf("provider not found: %s", bestModelRef.ProviderID)
	}

	// Create adapter
	adp := adapter.Create(provider.Type)
	if adp == nil {
		s.metrics.RecordFallback(groupName, "unsupported_provider_type", group.ProfileID)
		return nil, nil, nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
	if err := adp.Init(&provider); err != nil {
		s.metrics.RecordFallback(groupName, "adapter_init_failed", group.ProfileID)
		return nil, nil, nil, fmt.Errorf("failed to initialize adapter: %w", err)
	}

	// Calculate selection duration and record metrics
	selectionDuration := time.Since(startTime)
	s.metrics.RecordSelectionLatency(groupName, group.ProfileID, selectionDuration)
	s.metrics.RecordUsage(groupName, bestModelRef.ModelName, group.ProfileID)

	// Build metadata
	metadata := &model.CompressionMetadata{
		GroupUsed:     groupName,
		ModelSelected: bestModelRef.ModelName,
		ProviderID:    bestModelRef.ProviderID,
		FallbackUsed:  false,
	}

	return adp, &dbModel, metadata, nil
}

// getGroupConfig uses groupCache to retrieve compression group configuration
func (s *CompressionGroupSelector) getGroupConfig(ctx context.Context, groupName string) (*model.CompressionModelGroup, error) {
	cacheKey := fmt.Sprintf("group:%s:%s", s.profileID, groupName)

	var group model.CompressionModelGroup
	err := s.groupCache.Get(ctx, cacheKey, &group)
	if err == nil {
		return &group, nil
	}

	// Cache miss - load from database
	db := database.GetDB()
	if err := db.Where("name = ? AND profile_id = ?", groupName, s.profileID).First(&group).Error; err != nil {
		return nil, fmt.Errorf("compression group not found: %s for profile %s", groupName, s.profileID)
	}

	// Store in cache
	if err := s.groupCache.Set(ctx, cacheKey, &group); err != nil {
		// Log but don't fail on cache set error
		fmt.Printf("Warning: failed to cache group config: %v\n", err)
	}

	return &group, nil
}

// isModelHealthy uses healthCache to check if a model meets the health threshold
func (s *CompressionGroupSelector) isModelHealthy(ctx context.Context, modelRef *model.ModelReference, threshold float64) bool {
	cacheKey := fmt.Sprintf("%s:%s", modelRef.ProviderID, modelRef.ModelName)

	var cachedHealth struct {
		Score     float64
		Timestamp time.Time
	}

	err := s.healthCache.Get(ctx, cacheKey, &cachedHealth)
	if err == nil {
		// Cache hit - check if cache is fresh (within 30 seconds)
		if time.Since(cachedHealth.Timestamp) < 30*time.Second {
			return cachedHealth.Score >= threshold
		}
	}

	// Cache miss or stale - get fresh health score
	score := s.stats.GetHealthScore(modelRef.ProviderID, modelRef.ModelName)

	// Update cache
	cachedHealth.Score = score
	cachedHealth.Timestamp = time.Now()
	if err := s.healthCache.Set(ctx, cacheKey, &cachedHealth); err != nil {
		fmt.Printf("Warning: failed to cache health score: %v\n", err)
	}

	return score >= threshold
}

// getModelHealthScore gets the health score for a model reference
func (s *CompressionGroupSelector) getModelHealthScore(ctx context.Context, modelRef *model.ModelReference) float64 {
	return s.stats.GetHealthScore(modelRef.ProviderID, modelRef.ModelName)
}

// InvalidateGroupCache invalidates cache for a specific compression group
func (s *CompressionGroupSelector) InvalidateGroupCache(ctx context.Context, groupName string) error {
	cacheKey := fmt.Sprintf("group:%s:%s", s.profileID, groupName)
	if err := s.groupCache.Delete(ctx, cacheKey); err != nil {
		return fmt.Errorf("failed to invalidate group cache: %w", err)
	}

	// Also invalidate related health cache entries
	group, err := s.getGroupConfig(ctx, groupName)
	if err != nil {
		return err
	}

	for _, modelRef := range group.Models {
		healthKey := fmt.Sprintf("%s:%s", modelRef.ProviderID, modelRef.ModelName)
		if err := s.healthCache.Delete(ctx, healthKey); err != nil {
			fmt.Printf("Warning: failed to invalidate health cache for %s: %v\n", healthKey, err)
		}
	}

	return nil
}

// GroupExists checks if a compression group exists without side effects
func (s *CompressionGroupSelector) GroupExists(ctx context.Context, groupName string) bool {
	_, err := s.getGroupConfig(ctx, groupName)
	return err == nil
}

// InvalidateAllCache clears all compression selector caches
func (s *CompressionGroupSelector) InvalidateAllCache(ctx context.Context) error {
	if err := s.groupCache.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear group cache: %w", err)
	}

	if err := s.healthCache.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear health cache: %w", err)
	}

	return nil
}

// Close closes the cache connections
func (s *CompressionGroupSelector) Close() error {
	var errs []error

	if err := s.groupCache.Close(); err != nil {
		errs = append(errs, fmt.Errorf("group cache close error: %w", err))
	}

	if err := s.healthCache.Close(); err != nil {
		errs = append(errs, fmt.Errorf("health cache close error: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}

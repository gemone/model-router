package service

import (
	"context"
	"fmt"
	"time"

	"github.com/gemone/model-router/internal/cache"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/ensemble"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/repository"
)

// CompositeRouter handles routing logic for composite models
type CompositeRouter struct {
	dispatcher  *ensemble.Dispatcher
	synthesizer *ensemble.Synthesizer
}

// NewCompositeRouter creates a new CompositeRouter
func NewCompositeRouter(dispatcher *ensemble.Dispatcher, synthesizer *ensemble.Synthesizer) *CompositeRouter {
	return &CompositeRouter{
		dispatcher:  dispatcher,
		synthesizer: synthesizer,
	}
}

// CompositeService handles composite model routing with ensemble capabilities
type CompositeService struct {
	repo        repository.CompositeModelRepository
	router      *CompositeRouter
	profileID   string          // For filtering composite models
	configCache *cache.L2Cache  // Cache for composite model config (5 min TTL)
	stats       *StatsCollector
}

// NewCompositeModelService creates a new CompositeService with 5min TTL config cache
func NewCompositeModelService(profileID string) (*CompositeService, error) {
	// Create L2 cache with 5min TTL for composite model config
	configCache, err := cache.NewL2CacheWithConfig(&cache.L2CacheConfig{
		Addr:       "localhost:6379",
		Password:   "",
		DB:         0,
		DefaultTTL: 5 * time.Minute,
		KeyPrefix:  "model-router:composite-model:",
		MaxRetries: 3,
		PoolSize:   10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create config cache: %w", err)
	}

	// Create ensemble components
	dispatcher := ensemble.NewDispatcher()
	synthesizer := ensemble.NewSynthesizer(&ensemble.SynthesizerConfig{
		MaxTokens: 100000, // 100k tokens output for synthesis
	})

	// Create composite router
	router := NewCompositeRouter(dispatcher, synthesizer)

	// Create repository
	db := database.GetDB()
	repo := repository.NewCompositeModelRepository(db)

	return &CompositeService{
		repo:        repo,
		router:      router,
		profileID:   profileID,
		configCache: configCache,
		stats:       GetStatsCollector(),
	}, nil
}

// Route selects the best backend model based on composite model strategy
func (s *CompositeService) Route(ctx context.Context, profile *ProfileInstance, modelName string) (*RouteResult, error) {
	// Load composite model from cache or repository
	composite, err := s.getCompositeModel(ctx, modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to load composite model: %w", err)
	}

	// Validate composite model
	if !composite.Enabled {
		return nil, fmt.Errorf("composite model %s is disabled", modelName)
	}

	if len(composite.BackendModels) == 0 {
		return nil, fmt.Errorf("composite model %s has no backend models", modelName)
	}

	// Delegate to CompositeRouter for routing logic
	return s.router.Route(ctx, profile, composite, s.stats)
}

// getCompositeModel loads composite model from cache or repository
func (s *CompositeService) getCompositeModel(ctx context.Context, modelName string) (*model.CompositeAutoModel, error) {
	cacheKey := fmt.Sprintf("composite:%s:%s", s.profileID, modelName)

	var composite model.CompositeAutoModel
	err := s.configCache.Get(ctx, cacheKey, &composite)
	if err == nil {
		return &composite, nil
	}

	// Cache miss - load from repository
	compositePtr, err := s.repo.GetByProfileAndName(ctx, s.profileID, modelName)
	if err != nil {
		return nil, fmt.Errorf("composite model not found: %s for profile %s", modelName, s.profileID)
	}
	composite = *compositePtr

	// Store in cache
	if err := s.configCache.Set(ctx, cacheKey, &composite); err != nil {
		fmt.Printf("Warning: failed to cache composite model config: %v\n", err)
	}

	return &composite, nil
}

// HealthCheck checks the health of all backend models in the composite
func (s *CompositeService) HealthCheck(ctx context.Context, modelName string) (map[string]bool, error) {
	composite, err := s.getCompositeModel(ctx, modelName)
	if err != nil {
		return nil, err
	}

	results := make(map[string]bool)

	for _, backend := range composite.BackendModels {
		// Check health using stats collector
		healthScore := s.stats.GetHealthScore(backend.ProviderID, backend.ModelName)
		results[backend.ModelName+"@"+backend.ProviderID] = healthScore >= composite.HealthThreshold
	}

	return results, nil
}

// InvalidateConfigCache invalidates cache for a specific composite model
func (s *CompositeService) InvalidateConfigCache(ctx context.Context, modelName string) error {
	cacheKey := fmt.Sprintf("composite:%s:%s", s.profileID, modelName)
	if err := s.configCache.Delete(ctx, cacheKey); err != nil {
		return fmt.Errorf("failed to invalidate config cache: %w", err)
	}
	return nil
}

// InvalidateAllCache clears all composite service caches
func (s *CompositeService) InvalidateAllCache(ctx context.Context) error {
	if err := s.configCache.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear config cache: %w", err)
	}
	return nil
}

// Close closes the cache connections
func (s *CompositeService) Close() error {
	if err := s.configCache.Close(); err != nil {
		return fmt.Errorf("config cache close error: %w", err)
	}
	return nil
}

// Route implements the routing logic for composite models
func (r *CompositeRouter) Route(ctx context.Context, profile *ProfileInstance, composite *model.CompositeAutoModel, stats *StatsCollector) (*RouteResult, error) {
	switch composite.Strategy {
	case model.CompositeStrategyCascade:
		return r.routeCascade(ctx, profile, composite, stats)
	case model.CompositeStrategyParallel:
		return r.routeParallel(ctx, profile, composite, stats)
	case model.CompositeStrategyContent:
		return r.routeByContent(ctx, profile, composite, stats)
	case model.CompositeStrategyRule:
		return r.routeByRule(ctx, profile, composite, stats)
	default:
		return nil, fmt.Errorf("unsupported composite strategy: %s", composite.Strategy)
	}
}

// routeCascade tries backend models in order, returns first success
func (r *CompositeRouter) routeCascade(ctx context.Context, profile *ProfileInstance, composite *model.CompositeAutoModel, stats *StatsCollector) (*RouteResult, error) {
	for _, backend := range composite.BackendModels {
		// Get adapter for backend model
		adp, modelInfo, err := profile.GetAdapterForModel(backend.ModelName, backend.ProviderID)
		if err != nil {
			RecordCompositeFallback(composite.Name, backend.ModelName, "none")
			continue
		}

		// Check health score
		healthScore := stats.GetHealthScore(backend.ProviderID, backend.ModelName)
		if healthScore < composite.HealthThreshold {
			RecordCompositeFallback(composite.Name, backend.ModelName, "unhealthy")
			continue
		}

		// Get provider
		provider := profile.providerMap[backend.ProviderID]
		if provider == nil || !provider.Enabled {
			RecordCompositeFallback(composite.Name, backend.ModelName, "provider_disabled")
			continue
		}

		// Found healthy backend
		return &RouteResult{
			Adapter:      adp,
			Model:        modelInfo,
			Provider:     provider,
			Profile:      profile.Profile,
			FallbackUsed: false,
		}, nil
	}

	return nil, fmt.Errorf("no healthy backend models available for composite %s", composite.Name)
}

// routeParallel selects all backends for parallel execution (aggregation happens elsewhere)
func (r *CompositeRouter) routeParallel(ctx context.Context, profile *ProfileInstance, composite *model.CompositeAutoModel, stats *StatsCollector) (*RouteResult, error) {
	// For parallel strategy, return the first healthy backend as primary
	// The actual parallel dispatch and aggregation is handled by the API layer
	return r.routeCascade(ctx, profile, composite, stats)
}

// routeByContent analyzes content to select best backend
func (r *CompositeRouter) routeByContent(ctx context.Context, profile *ProfileInstance, composite *model.CompositeAutoModel, stats *StatsCollector) (*RouteResult, error) {
	// For MVP: use cascade routing
	// TODO: Implement content analysis using router.ContentAnalyzer
	return r.routeCascade(ctx, profile, composite, stats)
}

// routeByRule uses rule-based matching to select backend
func (r *CompositeRouter) routeByRule(ctx context.Context, profile *ProfileInstance, composite *model.CompositeAutoModel, stats *StatsCollector) (*RouteResult, error) {
	// For MVP: use cascade routing
	// TODO: Implement rule-based routing using composite.RoutingRules
	return r.routeCascade(ctx, profile, composite, stats)
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
)

// RouteService handles route loading and model selection with weight/priority
type RouteService struct {
	sync.RWMutex
	routes        map[string]*RouteInstance // routeID -> RouteInstance
	stats         *StatsCollector
	selector      *DefaultModelSelector
	weightCounter uint64 // For weighted round-robin selection
}

// RouteInstance holds a loaded route with resolved models
type RouteInstance struct {
	Route        *model.Route
	ModelEntries []RouteModelEntryWithModel // Models with weight and priority
}

// RouteModelEntryWithModel combines route model entry with the actual model
type RouteModelEntryWithModel struct {
	*model.Model
	Weight   int
	Priority int
	Enabled  bool
	route    *model.Route // internal reference to parent route
}

var (
	routeService     *RouteService
	routeServiceOnce sync.Once
)

// GetRouteService returns the singleton RouteService
func GetRouteService() *RouteService {
	routeServiceOnce.Do(func() {
		routeService = &RouteService{
			routes:   make(map[string]*RouteInstance),
			stats:    GetStatsCollector(),
			selector: NewDefaultModelSelector(GetStatsCollector()),
		}
		routeService.LoadFromDB()
	})
	return routeService
}

// LoadFromDB loads all routes from database
func (s *RouteService) LoadFromDB() error {
	s.Lock()
	defer s.Unlock()
	return s.loadFromDB_unlocked()
}

func (s *RouteService) loadFromDB_unlocked() error {
	db := database.GetDB()

	var routes []model.Route
	if err := db.Find(&routes).Error; err != nil {
		return fmt.Errorf("failed to load routes: %w", err)
	}

	s.routes = make(map[string]*RouteInstance)

	for i := range routes {
		r := &routes[i]
		if !r.Enabled {
			continue
		}

		instance, err := s.loadRouteInstance(r)
		if err != nil {
			fmt.Printf("Warning: failed to load route %s: %v\n", r.ID, err)
			continue
		}

		s.routes[r.ID] = instance
	}

	return nil
}

// loadRouteInstance loads a single route with its models
func (s *RouteService) loadRouteInstance(route *model.Route) (*RouteInstance, error) {
	db := database.GetDB()

	// Parse ModelConfig JSON
	var config model.RouteModelConfig
	if route.ModelConfig != "" {
		if err := json.Unmarshal([]byte(route.ModelConfig), &config); err != nil {
			return nil, fmt.Errorf("failed to parse model config: %w", err)
		}
	}

	instance := &RouteInstance{
		Route:        route,
		ModelEntries: make([]RouteModelEntryWithModel, 0),
	}

	// Load models for each entry
	for _, entry := range config.Models {
		if !entry.Enabled {
			continue
		}

		var m model.Model
		if err := db.Where("id = ? AND enabled = ?", entry.ModelID, true).First(&m).Error; err != nil {
			fmt.Printf("Warning: model %s not found for route %s\n", entry.ModelID, route.ID)
			continue
		}

		instance.ModelEntries = append(instance.ModelEntries, RouteModelEntryWithModel{
			Model:    &m,
			Weight:   entry.Weight,
			Priority: entry.Priority,
			Enabled:  entry.Enabled,
			route:    route,
		})
	}

	return instance, nil
}

// GetRoute returns a route instance by ID
func (s *RouteService) GetRoute(routeID string) *RouteInstance {
	s.RLock()
	defer s.RUnlock()
	return s.routes[routeID]
}

// GetRoutesByIDs returns multiple route instances by IDs
func (s *RouteService) GetRoutesByIDs(routeIDs []string) []*RouteInstance {
	s.RLock()
	defer s.RUnlock()

	var routes []*RouteInstance
	for _, id := range routeIDs {
		if route, ok := s.routes[id]; ok {
			routes = append(routes, route)
		}
	}
	return routes
}

// SelectModelFromRoute selects a model from a route using the route's strategy
func (s *RouteService) SelectModelFromRoute(ctx context.Context, routeID string) (*RouteModelEntryWithModel, error) {
	s.RLock()
	route, ok := s.routes[routeID]
	s.RUnlock()

	if !ok {
		return nil, fmt.Errorf("route not found: %s", routeID)
	}

	return s.SelectModelFromRouteInstance(ctx, route)
}

// SelectModelFromRouteInstance selects a model from a route instance (exported)
func (s *RouteService) SelectModelFromRouteInstance(ctx context.Context, route *RouteInstance) (*RouteModelEntryWithModel, error) {
	if len(route.ModelEntries) == 0 {
		return nil, fmt.Errorf("no models available in route: %s", route.Route.ID)
	}

	// Filter enabled entries
	var enabledEntries []RouteModelEntryWithModel
	for _, entry := range route.ModelEntries {
		if entry.Enabled && entry.Model != nil && entry.Model.Enabled {
			enabledEntries = append(enabledEntries, entry)
		}
	}

	if len(enabledEntries) == 0 {
		return nil, fmt.Errorf("no enabled models in route: %s", route.Route.ID)
	}

	// Select based on route strategy
	switch route.Route.Strategy {
	case model.RouteStrategyPriority:
		return s.selectByPriority(enabledEntries, route.Route.HealthThreshold), nil
	case model.RouteStrategyWeighted:
		return s.selectByWeight(enabledEntries, route.Route.HealthThreshold), nil
	case model.RouteStrategyAuto:
		return s.selectByAuto(ctx, enabledEntries, route.Route.HealthThreshold), nil
	case model.RouteStrategyRandom:
		return s.selectByRandom(enabledEntries, route.Route.HealthThreshold), nil
	default:
		return s.selectByAuto(ctx, enabledEntries, route.Route.HealthThreshold), nil
	}
}

// selectByPriority selects model with highest priority
func (s *RouteService) selectByPriority(entries []RouteModelEntryWithModel, healthThreshold float64) *RouteModelEntryWithModel {
	var best *RouteModelEntryWithModel
	highestPriority := -1

	for i := range entries {
		entry := &entries[i]
		// Check health score
		healthScore := s.stats.GetHealthScore(entry.Model.ProviderID, entry.Model.Name)
		if healthScore < healthThreshold {
			continue
		}

		if entry.Priority > highestPriority {
			highestPriority = entry.Priority
			best = entry
		}
	}

	// Fallback to first entry if all failed health check
	if best == nil && len(entries) > 0 {
		return &entries[0]
	}

	return best
}

// selectByWeight selects model using weighted round-robin
func (s *RouteService) selectByWeight(entries []RouteModelEntryWithModel, healthThreshold float64) *RouteModelEntryWithModel {
	// Filter healthy entries and calculate total weight
	type weightedEntry struct {
		entry  *RouteModelEntryWithModel
		weight int
	}

	var healthyEntries []weightedEntry
	totalWeight := 0

	for i := range entries {
		entry := &entries[i]
		healthScore := s.stats.GetHealthScore(entry.Model.ProviderID, entry.Model.Name)
		if healthScore >= healthThreshold {
			healthyEntries = append(healthyEntries, weightedEntry{
				entry:  entry,
				weight: entry.Weight,
			})
			totalWeight += entry.Weight
		}
	}

	if len(healthyEntries) == 0 {
		// Fallback to first entry if all unhealthy
		if len(entries) > 0 {
			return &entries[0]
		}
		return nil
	}

	// Weighted round-robin selection using atomic counter
	if totalWeight > 0 {
		// Atomically increment counter for thread-safe weighted selection
		offset := atomic.AddUint64(&s.weightCounter, 1)
		target := int(offset % uint64(totalWeight))

		cumulative := 0
		for _, we := range healthyEntries {
			cumulative += we.weight
			if target < cumulative {
				return we.entry
			}
		}
	}

	// Fallback
	return healthyEntries[0].entry
}

// selectByAuto selects model using comprehensive scoring
func (s *RouteService) selectByAuto(ctx context.Context, entries []RouteModelEntryWithModel, healthThreshold float64) *RouteModelEntryWithModel {
	type scoredEntry struct {
		entry *RouteModelEntryWithModel
		score float64
	}

	var scoredEntries []scoredEntry

	for i := range entries {
		entry := &entries[i]

		// Get health score
		healthScore := s.stats.GetHealthScore(entry.Model.ProviderID, entry.Model.Name)

		// Skip if below health threshold
		if healthScore < healthThreshold {
			continue
		}

		// Calculate composite score
		// 40% health, 30% priority (normalized), 20% latency, 10% weight
		score := 0.0

		// Health score (40%)
		score += healthScore * 0.4

		// Priority score (30%) - normalize to 0-100
		priorityScore := float64(entry.Priority) * 10 // Assuming max priority ~10
		if priorityScore > 100 {
			priorityScore = 100
		}
		score += priorityScore * 0.3

		// Latency score (20%) - lower is better
		avgLatency := s.stats.GetAvgLatency(entry.Model.ProviderID, entry.Model.Name, time.Minute)
		latencyScore := 100.0
		if avgLatency > 0 {
			// Normalize: 5000ms = 0 points, 0ms = 100 points
			latencyScore = max(0, 100-avgLatency/50.0)
		}
		score += latencyScore * 0.2

		// Weight score (10%) - normalize to 0-100
		weightScore := float64(entry.Weight)
		if weightScore > 100 {
			weightScore = 100
		}
		score += weightScore * 0.1

		scoredEntries = append(scoredEntries, scoredEntry{
			entry: entry,
			score: score,
		})
	}

	if len(scoredEntries) == 0 {
		// Fallback to first entry if all filtered out
		if len(entries) > 0 {
			return &entries[0]
		}
		return nil
	}

	// Select entry with highest score
	best := scoredEntries[0]
	for _, se := range scoredEntries[1:] {
		if se.score > best.score {
			best = se
		}
	}

	return best.entry
}

// selectByRandom selects a random model
func (s *RouteService) selectByRandom(entries []RouteModelEntryWithModel, healthThreshold float64) *RouteModelEntryWithModel {
	// Filter healthy entries
	var healthyEntries []*RouteModelEntryWithModel
	for i := range entries {
		entry := &entries[i]
		healthScore := s.stats.GetHealthScore(entry.Model.ProviderID, entry.Model.Name)
		if healthScore >= healthThreshold {
			healthyEntries = append(healthyEntries, entry)
		}
	}

	if len(healthyEntries) == 0 {
		if len(entries) > 0 {
			return &entries[0]
		}
		return nil
	}

	// Random selection using math/rand
	idx := rand.Intn(len(healthyEntries))
	return healthyEntries[idx]
}

// SelectModelFromRoutes selects a model from multiple routes (tries each route in order)
func (s *RouteService) SelectModelFromRoutes(ctx context.Context, routeIDs []string) (*RouteModelEntryWithModel, *RouteInstance, error) {
	routes := s.GetRoutesByIDs(routeIDs)

	for _, route := range routes {
		entry, err := s.SelectModelFromRouteInstance(ctx, route)
		if err == nil && entry != nil {
			return entry, route, nil
		}
		// Continue to next route on error
	}

	return nil, nil, fmt.Errorf("no available model found in any of the routes: %v", routeIDs)
}

// Refresh refreshes a specific route from database
func (s *RouteService) Refresh(routeID string) error {
	s.Lock()
	defer s.Unlock()

	db := database.GetDB()
	var route model.Route
	if err := db.Where("id = ?", routeID).First(&route).Error; err != nil {
		return fmt.Errorf("route not found: %s", routeID)
	}

	if !route.Enabled {
		delete(s.routes, routeID)
		return nil
	}

	instance, err := s.loadRouteInstance(&route)
	if err != nil {
		return err
	}

	s.routes[routeID] = instance
	return nil
}

// RefreshAll refreshes all routes from database
func (s *RouteService) RefreshAll() error {
	return s.LoadFromDB()
}

// GetAllRoutes returns all route instances
func (s *RouteService) GetAllRoutes() []*RouteInstance {
	s.RLock()
	defer s.RUnlock()

	routes := make([]*RouteInstance, 0, len(s.routes))
	for _, r := range s.routes {
		routes = append(routes, r)
	}
	return routes
}

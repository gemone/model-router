// Package router provides profile-based routing for model requests.
package router

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/service"
)

// ProfileRouter handles profile-based routing with caching and fallback.
type ProfileRouter struct {
	sync.RWMutex
	profiles          map[string]*Profile // profileID -> Profile
	patternCache      map[string][]string // modelName -> matching profileIDs (pattern cache)
	healthScores      map[string]float64  // providerID_model -> health score
	healthScoreExpiry map[string]time.Time
	cacheTTL          time.Duration
	stats             *service.StatsCollector
}

// Profile wraps a model.Profile with runtime routing information.
type Profile struct {
	*model.Profile
	providers    map[string]*model.Provider // providerID -> Provider
	models       map[string][]*model.Model  // modelName -> Models
	routes       map[string]*service.RouteInstance // routeID -> Route (NEW: route-based access)
	adapterMap   map[string]adapter.Adapter // providerID -> Adapter
	lastLoadTime time.Time
}

// RouteResult contains the routing destination for a request.
type RouteResult struct {
	Provider  *model.Provider
	Model     *model.Model
	Adapter   adapter.Adapter
	Profile   *model.Profile
	IsFallback bool
}

// NewProfileRouter creates a new ProfileRouter instance.
func NewProfileRouter() *ProfileRouter {
	return &ProfileRouter{
		profiles:          make(map[string]*Profile),
		patternCache:      make(map[string][]string),
		healthScores:      make(map[string]float64),
		healthScoreExpiry: make(map[string]time.Time),
		cacheTTL:          5 * time.Minute,
		stats:             service.GetStatsCollector(),
	}
}

// LoadProfiles loads all profiles from the database.
func (r *ProfileRouter) LoadProfiles() error {
	r.Lock()
	defer r.Unlock()

	db := database.GetDB()

	var profiles []model.Profile
	if err := db.Find(&profiles).Error; err != nil {
		return fmt.Errorf("failed to load profiles: %w", err)
	}

	// Clear existing data
	r.profiles = make(map[string]*Profile)
	r.patternCache = make(map[string][]string)

	// Load each profile
	for i := range profiles {
		p := &profiles[i]
		if !p.Enabled {
			continue
		}

		profile := &Profile{
			Profile:      p,
			providers:    make(map[string]*model.Provider),
			models:       make(map[string][]*model.Model),
			adapterMap:   make(map[string]adapter.Adapter),
			lastLoadTime: time.Now(),
		}

		// Load associated providers
		var providers []model.Provider
		if err := db.Find(&providers).Error; err != nil {
			return fmt.Errorf("failed to load providers: %w", err)
		}

		for j := range providers {
			prov := &providers[j]
			if !prov.Enabled {
				continue
			}
			profile.providers[prov.ID] = prov

			// Create adapter for this provider
			adapt := adapter.Create(prov.Type)
			if adapt != nil {
				if err := adapt.Init(prov); err == nil {
					profile.adapterMap[prov.ID] = adapt
				}
			}
		}

		// Load models via ModelIDs
		if len(p.ModelIDs) > 0 {
			var models []model.Model
			if err := db.Where("id IN ?", p.ModelIDs).Find(&models).Error; err == nil {
				for j := range models {
					m := &models[j]
					if m.Enabled {
						profile.models[m.Name] = append(profile.models[m.Name], m)
					}
				}
			}
		}

		// Load routes via RouteIDs (NEW: route-based access with weight and priority)
		if len(p.RouteIDs) > 0 {
			profile.routes = make(map[string]*service.RouteInstance)
			routeService := service.GetRouteService()
			for _, routeID := range p.RouteIDs {
				if route := routeService.GetRoute(routeID); route != nil {
					profile.routes[routeID] = route
				}
			}
		}

		r.profiles[p.ID] = profile
	}

	return nil
}

// MatchProfile finds the best matching profile for a given model name.
func (r *ProfileRouter) MatchProfile(modelName string) (*Profile, error) {
	r.RLock()
	defer r.RUnlock()

	// Check pattern cache first
	var cachedProfileIDs []string
	var found bool

	if r.patternCache != nil {
		cachedProfileIDs, found = r.patternCache[modelName]
	}

	if !found {
		// No cache hit, need to find matching profiles
		var matchingProfiles []*Profile
		for _, p := range r.profiles {
			if r.matchesAnyPattern(p, modelName) {
				matchingProfiles = append(matchingProfiles, p)
			}
		}

		if len(matchingProfiles) == 0 {
			// Try to find a profile with the model directly
			for _, p := range r.profiles {
				if _, ok := p.models[modelName]; ok {
					matchingProfiles = append(matchingProfiles, p)
				}
			}
		}

		if len(matchingProfiles) == 0 {
			return nil, fmt.Errorf("no profile found for model: %s", modelName)
		}

		// Select profile by priority
		selected := r.selectByPriority(matchingProfiles)
		cachedProfileIDs = []string{selected.Profile.ID}
		r.patternCache[modelName] = cachedProfileIDs
	}

	// Return the first cached profile
	if len(cachedProfileIDs) > 0 {
		if p, ok := r.profiles[cachedProfileIDs[0]]; ok {
			return p, nil
		}
	}

	// Fallback: return any enabled profile
	for _, p := range r.profiles {
		return p, nil
	}

	return nil, fmt.Errorf("no enabled profiles available")
}

// matchesAnyPattern checks if the model name matches any pattern in the profile.
func (r *ProfileRouter) matchesAnyPattern(p *Profile, modelName string) bool {
	// Check if profile has this model directly
	_, ok := p.models[modelName]
	return ok
}

// selectByPriority selects the first available profile.
// In the new architecture, Profile doesn't have priority - it's an API entry point.
func (r *ProfileRouter) selectByPriority(profiles []*Profile) *Profile {
	// Return the first available profile
	for _, p := range profiles {
		if p.Profile != nil && p.Profile.Enabled {
			return p
		}
	}
	return nil
}

// Route determines the best provider and model for a given request.
func (r *ProfileRouter) Route(ctx context.Context, modelName string) (*RouteResult, error) {
	profile, err := r.MatchProfile(modelName)
	if err != nil {
		return nil, err
	}

	// 1. Direct model lookup (via ModelIDs)
	if models, ok := profile.models[modelName]; ok && len(models) > 0 {
		selected := r.selectBestModel(profile, models)
		if selected != nil {
			provider := profile.providers[selected.ProviderID]
			adapt := profile.adapterMap[selected.ProviderID]
			if provider != nil && adapt != nil && provider.Enabled {
				return &RouteResult{
					Provider:   provider,
					Model:      selected,
					Adapter:    adapt,
					Profile:    profile.Profile,
					IsFallback: false,
				}, nil
			}
		}
	}

	// 2. Route-based model lookup (via RouteIDs, using weight and priority)
	if len(profile.routes) > 0 {
		result, err := r.routeViaRoutes(ctx, profile, modelName)
		if err == nil && result != nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("no available route for model: %s", modelName)
}

// routeViaRoutes attempts to find a model through configured routes
func (r *ProfileRouter) routeViaRoutes(ctx context.Context, profile *Profile, modelName string) (*RouteResult, error) {
	routeService := service.GetRouteService()

	for routeID, routeInstance := range profile.routes {
		// Check if this route contains the requested model
		for _, entry := range routeInstance.ModelEntries {
			if entry.Enabled && entry.Model != nil && entry.Model.Name == modelName {
				// Use route's strategy to select the best model
				selectedEntry, err := routeService.SelectModelFromRoute(ctx, routeID)
				if err != nil {
					continue
				}

				if selectedEntry != nil && selectedEntry.Model != nil {
					provider := profile.providers[selectedEntry.Model.ProviderID]
					adapt := profile.adapterMap[selectedEntry.Model.ProviderID]
					if provider != nil && adapt != nil && provider.Enabled {
						return &RouteResult{
							Provider:   provider,
							Model:      selectedEntry.Model,
							Adapter:    adapt,
							Profile:    profile.Profile,
							IsFallback: false,
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("model %s not found in any route", modelName)
}

// selectBestModel selects the best model from a list based on priority and health.
func (r *ProfileRouter) selectBestModel(profile *Profile, models []*model.Model) *model.Model {
	if len(models) == 0 {
		return nil
	}
	if len(models) == 1 {
		return models[0]
	}

		// In the new architecture, priority is configured at Route level.
	// For direct model selection, use health score only.
	var best *model.Model
	bestHealthScore := -1.0

	for _, m := range models {
		provider := profile.providers[m.ProviderID]
		if provider == nil || !provider.Enabled {
			continue
		}

		healthScore := r.getHealthScore(m.ProviderID, m.Name)

		if healthScore > bestHealthScore {
			best = m
			bestHealthScore = healthScore
		}
	}

	return best
}

// getHealthScore retrieves the health score with caching.
func (r *ProfileRouter) getHealthScore(providerID, modelName string) float64 {
	key := providerID + "_" + modelName

	r.RLock()
	expiry, hasExpiry := r.healthScoreExpiry[key]
	score, hasScore := r.healthScores[key]
	r.RUnlock()

	// Check if cached score is still valid
	if hasScore && hasExpiry && time.Now().Before(expiry) {
		return score
	}

	// Get fresh score from stats collector
	freshScore := r.stats.GetHealthScore(providerID, modelName)

	r.Lock()
	r.healthScores[key] = freshScore
	r.healthScoreExpiry[key] = time.Now().Add(r.cacheTTL)
	r.Unlock()

	return freshScore
}

// RouteWithFallback routes a request with automatic fallback on errors.
func (r *ProfileRouter) RouteWithFallback(ctx context.Context, modelName string, lastError error) (*RouteResult, error) {
	result, err := r.Route(ctx, modelName)
	if err != nil {
		return nil, err
	}

	// Check if we should fallback based on health
	if r.shouldFallback(result.Provider.ID, result.Model.Name, lastError) {
		fallbackResult := r.tryFallback(ctx, modelName, result.Profile.ID)
		if fallbackResult != nil {
			return fallbackResult, nil
		}
	}

	return result, nil
}

// shouldFallback determines if a fallback should occur.
func (r *ProfileRouter) shouldFallback(providerID, modelName string, lastError error) bool {
	if lastError != nil {
		return true
	}

	errorRate := r.stats.GetErrorRate(providerID, modelName, time.Minute)
	if errorRate > 0.5 {
		return true
	}

	avgLatency := r.stats.GetAvgLatency(providerID, modelName, time.Minute)
	if avgLatency > 10000 {
		return true
	}

	currentRPM := r.stats.GetCurrentRPM(providerID, modelName)
	r.RLock()
	// Check model-level rate limit first
	rateLimit := r.getModelRateLimit(providerID, modelName)
	r.RUnlock()

	if rateLimit > 0 && currentRPM >= int64(rateLimit) {
		return true
	}

	return false
}

// getModelRateLimit gets the rate limit for a specific model.
// Returns the model's rate_limit if set (> 0), otherwise returns 0 (no limit).
func (r *ProfileRouter) getModelRateLimit(providerID, modelName string) int {
	// Find the model in all profiles
	for _, p := range r.profiles {
		if models, ok := p.models[modelName]; ok {
			for _, m := range models {
				if m.ProviderID == providerID && m.RateLimit > 0 {
					return m.RateLimit
				}
			}
		}
	}
	return 0
}

// tryFallback attempts to find a fallback route.
func (r *ProfileRouter) tryFallback(ctx context.Context, modelName, excludeProfileID string) *RouteResult {
	r.RLock()
	defer r.RUnlock()

	// Try other profiles
	for _, profile := range r.profiles {
		if profile.Profile.ID == excludeProfileID {
			continue
		}

		// Check for direct model match
		if models, ok := profile.models[modelName]; ok {
			for _, m := range models {
				provider := profile.providers[m.ProviderID]
				adapt := profile.adapterMap[m.ProviderID]
				if provider != nil && adapt != nil && provider.Enabled {
					return &RouteResult{
						Provider:   provider,
						Model:      m,
						Adapter:    adapt,
						Profile:    profile.Profile,
						IsFallback: true,
					}
				}
			}
		}
	}

	return nil
}

// getProviderByID retrieves a provider by ID from all profiles.
func (r *ProfileRouter) getProviderByID(providerID string) *model.Provider {
	for _, profile := range r.profiles {
		if provider, ok := profile.providers[providerID]; ok {
			return provider
		}
	}
	return nil
}

// GetProfile retrieves a profile by ID.
func (r *ProfileRouter) GetProfile(profileID string) (*Profile, error) {
	r.RLock()
	defer r.RUnlock()

	if profile, ok := r.profiles[profileID]; ok {
		return profile, nil
	}
	return nil, fmt.Errorf("profile not found: %s", profileID)
}

// GetAllProfiles returns all loaded profiles.
func (r *ProfileRouter) GetAllProfiles() []*Profile {
	r.RLock()
	defer r.RUnlock()

	profiles := make([]*Profile, 0, len(r.profiles))
	for _, p := range r.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// RefreshCache refreshes the pattern cache.
func (r *ProfileRouter) RefreshCache() {
	r.Lock()
	defer r.Unlock()

	r.patternCache = make(map[string][]string)
}

// matchPattern checks if a model name matches a pattern with wildcard support.
func matchPattern(modelName, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		regex := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, `.*`)
		matched, _ := regexp.MatchString("^"+regex+"$", modelName)
		return matched
	}
	return modelName == pattern
}

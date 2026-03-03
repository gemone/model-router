package router

import (
	"context"
	"testing"
	"time"

	"github.com/gemone/model-router/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		pattern  string
		expected bool
	}{
		{"exact match", "gpt-4", "gpt-4", true},
		{"exact mismatch", "gpt-4", "gpt-3", false},
		{"wildcard all", "gpt-4", "*", true},
		{"prefix wildcard", "gpt-4-turbo", "gpt-*", true},
		{"prefix wildcard mismatch", "claude-3", "gpt-*", false},
		{"suffix wildcard", "gpt-4-turbo", "*-turbo", true},
		{"middle wildcard", "gpt-4-turbo", "gpt-*-turbo", true},
		{"multiple wildcards", "gpt-4-turbo-preview", "gpt-*-preview", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.model, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewProfileRouter(t *testing.T) {
	router := NewProfileRouter()
	assert.NotNil(t, router)
	assert.NotNil(t, router.profiles)
	assert.NotNil(t, router.patternCache)
	assert.NotNil(t, router.healthScores)
}

func TestProfileRouter_SelectByPriority(t *testing.T) {
	router := NewProfileRouter()

	// In the new architecture, Profile doesn't have Priority.
	// Priority is configured at Route level.
	// selectByPriority returns the first enabled profile.
	profiles := []*Profile{
		{Profile: &model.Profile{ID: "p1", Enabled: true}},
		{Profile: &model.Profile{ID: "p2", Enabled: true}},
		{Profile: &model.Profile{ID: "p3", Enabled: true}},
	}

	result := router.selectByPriority(profiles)
	assert.NotNil(t, result)
	// Should return first enabled profile
	assert.Equal(t, "p1", result.Profile.ID)
}

func TestProfileRouter_ReloadCache(t *testing.T) {
	router := NewProfileRouter()
	router.patternCache["test-model"] = []string{"profile1"}

	assert.Len(t, router.patternCache, 1)

	router.RefreshCache()
	assert.Empty(t, router.patternCache)
}

func TestProfileRouter_GetHealthScore(t *testing.T) {
	router := NewProfileRouter()

	// First call should cache
	score1 := router.getHealthScore("provider1", "model1")
	assert.GreaterOrEqual(t, score1, 0.0)
	assert.LessOrEqual(t, score1, 100.0)

	// Second call should use cache
	score2 := router.getHealthScore("provider1", "model1")
	assert.Equal(t, score1, score2)
}

func TestProfileRouter_RouteWithFallback_NoFallbackNeeded(t *testing.T) {
	router := NewProfileRouter()

	// Test with no last error, no image
	result, err := router.RouteWithFallback(context.Background(), "nonexistent-model", nil, false)
	// Should fail because no profiles are loaded
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestProfileRouter_ShouldFallback(t *testing.T) {
	router := NewProfileRouter()

	tests := []struct {
		name       string
		providerID string
		modelName  string
		lastError  error
		expected   bool
	}{
		{
			name:       "no error, no fallback",
			providerID: "test-provider",
			modelName:  "test-model",
			lastError:  nil,
			expected:   false,
		},
		{
			name:       "with error, should fallback",
			providerID: "test-provider",
			modelName:  "test-model",
			lastError:  assert.AnError,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.shouldFallback(tt.providerID, tt.modelName, tt.lastError)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestProfileRouter_GetProfile tests getting a profile by ID
func TestProfileRouter_GetProfile(t *testing.T) {
	router := NewProfileRouter()

	// Test non-existent profile
	profile, err := router.GetProfile("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, profile)
}

// TestProfileRouter_GetAllProfiles tests getting all profiles
func TestProfileRouter_GetAllProfiles(t *testing.T) {
	router := NewProfileRouter()

	profiles := router.GetAllProfiles()
	assert.NotNil(t, profiles)
	// Empty because no profiles loaded
	assert.Empty(t, profiles)
}

// TestProfileRouter_MatchesAnyPattern tests pattern matching for profiles
func TestProfileRouter_MatchesAnyPattern(t *testing.T) {
	router := NewProfileRouter()

	profile := &Profile{
		Profile: &model.Profile{ID: "test-profile"},
		models:  make(map[string][]*model.Model),
	}

	// Add some models to the profile
	profile.models["gpt-4"] = []*model.Model{{ID: "gpt-4", Name: "gpt-4"}}
	profile.models["gpt-3.5-turbo"] = []*model.Model{{ID: "gpt-3.5-turbo", Name: "gpt-3.5-turbo"}}

	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{"model exists", "gpt-4", true},
		{"model exists variant", "gpt-3.5-turbo", true},
		{"model does not exist", "claude-3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.matchesAnyPattern(profile, tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// BenchmarkMatchPattern benchmarks pattern matching
func BenchmarkMatchPattern(b *testing.B) {
	patterns := []string{"*", "gpt-*", "*-turbo", "claude-*-opuss"}
	models := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-opuss", "deepseek-chat"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, pattern := range patterns {
			for _, model := range models {
				matchPattern(model, pattern)
			}
		}
	}
}

// BenchmarkGetHealthScore benchmarks health score retrieval with caching
func BenchmarkGetHealthScore(b *testing.B) {
	router := NewProfileRouter()
	router.cacheTTL = time.Minute

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.getHealthScore("provider1", "model1")
	}
}

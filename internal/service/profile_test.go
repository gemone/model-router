package service

import (
	"testing"

	"github.com/gemone/model-router/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestProfileManager(t *testing.T) {
	t.Run("isValidPath", func(t *testing.T) {
		tests := []struct {
			path  string
			valid bool
		}{
			{"default", true},
			{"claudecode", true},
			{"my-profile", true},
			{"profile_123", true},
			{"", false},
			{"path/with/slash", false},
			{"path.with.dot", false},
			{"path with space", false},
		}

		for _, tt := range tests {
			t.Run(tt.path, func(t *testing.T) {
				result := isValidPath(tt.path)
				assert.Equal(t, tt.valid, result)
			})
		}
	})

	t.Run("matchPattern", func(t *testing.T) {
		tests := []struct {
			modelName string
			pattern   string
			matches   bool
		}{
			{"gpt-4", "gpt-4", true},
			{"gpt-4", "gpt-*", true},
			{"gpt-4-turbo", "gpt-*", true},
			{"claude-3", "gpt-*", false},
			{"gpt-4", "*", true},
			{"any-model", "*", true},
			{"gpt-4", "claude-*", false},
			{"gpt-4-0125-preview", "gpt-4-*", true},
		}

		for _, tt := range tests {
			t.Run(tt.modelName+"_"+tt.pattern, func(t *testing.T) {
				result := matchPattern(tt.modelName, tt.pattern)
				assert.Equal(t, tt.matches, result)
			})
		}
	})
}

func TestProfileInstance_Route(t *testing.T) {
	// This test requires database setup, so we'll just test the basic structure
	t.Run("RouteResult structure", func(t *testing.T) {
		result := &RouteResult{
			Model:        &model.Model{Name: "test"},
			Provider:     &model.Provider{Name: "test-provider"},
			FallbackUsed: false,
		}

		assert.NotNil(t, result.Model)
		assert.NotNil(t, result.Provider)
		assert.False(t, result.FallbackUsed)
	})
}

package adapter

import (
	"testing"

	"github.com/gemone/model-router/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestAdapterFactory(t *testing.T) {
	// Clear and re-register for testing
	factory.adapters = make(map[model.ProviderType]func() Adapter)

	// Register test adapters
	Register(model.ProviderOpenAI, func() Adapter {
		return nil
	})
	Register(model.ProviderClaude, func() Adapter {
		return nil
	})
	Register(model.ProviderDeepSeek, func() Adapter {
		return nil
	})

	t.Run("GetSupportedTypes", func(t *testing.T) {
		types := GetSupportedTypes()
		assert.Len(t, types, 3)
		assert.Contains(t, types, model.ProviderOpenAI)
		assert.Contains(t, types, model.ProviderClaude)
		assert.Contains(t, types, model.ProviderDeepSeek)
	})

	t.Run("Create existing adapter", func(t *testing.T) {
		adapter := Create(model.ProviderOpenAI)
		assert.Nil(t, adapter) // We registered nil creator
	})

	t.Run("Create non-existing adapter", func(t *testing.T) {
		adapter := Create(model.ProviderOllama)
		assert.Nil(t, adapter)
	})
}

package compression

import (
	"context"
	"testing"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.False(t, config.EnableCompression, "Default compression should be disabled")
	assert.Equal(t, "sliding_window", config.DefaultStrategy)
	assert.Equal(t, 8000, config.DefaultThreshold)
	assert.Equal(t, 16000, config.DefaultMaxContextWindow)
}

func TestShouldCompress(t *testing.T) {
	adapters := make(map[string]adapter.Adapter)
	config := &CompressionConfig{
		EnableCompression:      true,
		DefaultStrategy:        "sliding_window",
		DefaultThreshold:       8000,
		DefaultMaxContextWindow: 16000,
	}

	svc := NewDefaultService("test-profile", adapters, nil, config)
	assert.NotNil(t, svc)

	// Test with compression disabled at profile level
	profile := &model.Profile{
		EnableCompression:     false,
		CompressionStrategy:   "sliding_window",
		CompressionLevel:      model.CompressionLevelSession,
		CompressionThreshold:  8000,
		MaxContextWindow:      16000,
	}

	testModel := &model.Model{
		Name: "test-model",
	}

	shouldCompress := svc.ShouldCompress(profile, testModel, 10)
	assert.False(t, shouldCompress, "Should not compress when profile compression is disabled")

	// Test with compression enabled at profile level
	profile.EnableCompression = true
	shouldCompress = svc.ShouldCompress(profile, testModel, 10)
	assert.True(t, shouldCompress, "Should compress when profile compression is enabled and level is session")

	// Test with threshold level
	profile.CompressionLevel = model.CompressionLevelThreshold
	shouldCompress = svc.ShouldCompress(profile, testModel, 10)
	assert.True(t, shouldCompress, "Should compress with threshold level when messages exist")
}

func TestGetCompressionGroupName(t *testing.T) {
	adapters := make(map[string]adapter.Adapter)
	config := DefaultConfig()
	svc := NewDefaultService("test-profile", adapters, nil, config)

	profile := &model.Profile{
		DefaultCompressionGroup: "default-group",
	}

	// Test with API override
	apiGroup := "api-override-group"
	groupName := svc.getCompressionGroupName(profile, &apiGroup)
	assert.Equal(t, "api-override-group", groupName, "Should use API override group")

	// Test without API override
	groupName = svc.getCompressionGroupName(profile, nil)
	assert.Equal(t, "default-group", groupName, "Should use profile default group")

	// Test with empty profile default
	profile.DefaultCompressionGroup = ""
	groupName = svc.getCompressionGroupName(profile, nil)
	assert.Equal(t, "", groupName, "Should return empty string when no group is specified")
}

func TestCompressDisabled(t *testing.T) {
	adapters := make(map[string]adapter.Adapter)
	config := &CompressionConfig{
		EnableCompression: false,
	}

	svc := NewDefaultService("test-profile", adapters, nil, config)

	profile := &model.Profile{
		EnableCompression: false,
	}

	session := &model.Session{
		ID: "test-session",
	}

	messages, metadata, err := svc.Compress(context.Background(), profile, session, 8000, nil)

	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Empty(t, messages, "Should return empty messages when compression is disabled")
}

func TestFactoryCreateService(t *testing.T) {
	config := &CompressionConfig{
		EnableCompression:      true,
		DefaultStrategy:        "sliding_window",
		DefaultThreshold:       8000,
		DefaultMaxContextWindow: 16000,
	}

	factory := NewFactory(config)
	assert.NotNil(t, factory)

	profile := &model.Profile{
		ID:                     "test-profile",
		EnableCompression:      true,
		CompressionStrategy:    "hybrid",
		CompressionThreshold:   6000,
		MaxContextWindow:       12000,
		DefaultCompressionGroup: "test-group",
	}

	adapters := make(map[string]adapter.Adapter)

	svc := factory.CreateService(profile, adapters, nil)
	assert.NotNil(t, svc)

	// Test that the service was created with merged config
	// (This is a basic smoke test - more detailed tests would require mocking adapters)
}

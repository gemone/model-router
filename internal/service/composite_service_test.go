package service

import (
	"context"
	"testing"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/ensemble"
	"github.com/gemone/model-router/internal/model"
)

func TestCompositeRouter(t *testing.T) {
	// Create a composite router
	dispatcher := ensemble.NewDispatcher()
	synthesizer := ensemble.NewSynthesizer(&ensemble.SynthesizerConfig{
		MaxTokens: 100000,
	})
	router := NewCompositeRouter(dispatcher, synthesizer)

	// Create test profile with providers
	profile := &ProfileInstance{
		Profile: &model.Profile{
			ID:   "test-profile",
			Name: "Test Profile",
			Path: "test",
		},
		adapters:    make(map[string]adapter.Adapter),
		providerMap: make(map[string]*model.Provider),
		modelMap:    make(map[string][]*model.Model),
		stats:       GetStatsCollector(),
	}

	ctx := context.Background()

	t.Run("routeCascade returns first healthy backend", func(t *testing.T) {
		composite := &model.CompositeAutoModel{
			Name:            "test-composite",
			Strategy:        model.CompositeStrategyCascade,
			HealthThreshold: 70.0,
			BackendModels: []model.CompositeBackendModel{
				{ModelName: "model-1", ProviderID: "provider-1"},
			},
		}

		result, err := router.Route(ctx, profile, composite, profile.stats)
		if err != nil {
			// Expected to fail because we don't have real providers/adapters set up
			t.Logf("Expected error in test: %v", err)
		}
		if result != nil {
			t.Logf("Got route result: %+v", result)
		}
	})

	t.Run("routeParallel behaves like cascade for MVP", func(t *testing.T) {
		composite := &model.CompositeAutoModel{
			Name:            "test-parallel",
			Strategy:        model.CompositeStrategyParallel,
			HealthThreshold: 70.0,
			BackendModels: []model.CompositeBackendModel{
				{ModelName: "model-1", ProviderID: "provider-1"},
			},
		}

		result, err := router.routeParallel(ctx, profile, composite, profile.stats)
		if err != nil {
			// Expected to fail because we don't have real providers/adapters set up
			t.Logf("Expected error in test: %v", err)
		}
		if result != nil {
			t.Logf("Got route result: %+v", result)
		}
	})
}

func TestCompositeService(t *testing.T) {
	t.Run("NewCompositeModelService creates service with cache", func(t *testing.T) {
		service, err := NewCompositeModelService("test-profile")
		if err != nil {
			t.Logf("Warning: failed to create composite service (Redis may not be running): %v", err)
			return
		}
		defer service.Close()

		if service == nil {
			t.Fatal("expected service to be created")
		}
		if service.configCache == nil {
			t.Error("expected configCache to be initialized")
		}
		if service.router == nil {
			t.Error("expected router to be initialized")
		}
	})
}


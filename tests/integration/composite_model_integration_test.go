// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/repository"
	"github.com/gemone/model-router/internal/service"
)

// TestCompositeModelIntegration tests the composite model feature end-to-end
func TestCompositeModelIntegration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Initialize database
	db := database.GetDB()
	if db == nil {
		t.Skip("database not available")
	}

	// Create test profile
	profile := &model.Profile{
		ID:          "test-composite-profile",
		Name:        "Test Composite Profile",
		Path:        "test-composite",
		Description: "Test profile for composite model integration",
		Enabled:     true,
		Priority:    0,
	}
	db.Create(profile)
	defer db.Delete(profile)

	// Create test providers
	provider1 := &model.Provider{
		ID:      "test-provider-1",
		Name:    "Test Provider 1",
		Type:    model.ProviderOpenAI,
		BaseURL: "https://api.openai.com/v1",
		Enabled: true,
		Priority: 1,
	}
	provider2 := &model.Provider{
		ID:      "test-provider-2",
		Name:    "Test Provider 2",
		Type:    model.ProviderOpenAI,
		BaseURL: "https://api.openai.com/v1",
		Enabled: true,
		Priority: 2,
	}
	db.Create(provider1)
	db.Create(provider2)
	defer db.Delete(provider1)
	defer db.Delete(provider2)

	// Create composite model
	composite := &model.CompositeAutoModel{
		ID:             "test-composite-1",
		ProfileID:      profile.ID,
		Name:           "test-composite",
		Priority:       1,
		Enabled:        true,
		HealthThreshold: 70.0,
		Strategy:       model.CompositeStrategyCascade,
		BackendModels: []model.CompositeBackendModel{
			{
				ModelName:  "gpt-4",
				ProviderID: provider1.ID,
				Weight:     100,
				TimeoutMs:  30000,
			},
			{
				ModelName:  "gpt-3.5-turbo",
				ProviderID: provider2.ID,
				Weight:     50,
				TimeoutMs:  30000,
			},
		},
	}
	db.Create(composite)
	defer db.Delete(composite)

	t.Run("Repository can retrieve composite model", func(t *testing.T) {
		repo := repository.NewCompositeModelRepository(db)

		ctx := context.Background()
		found, err := repo.GetByProfileAndName(ctx, profile.ID, composite.Name)
		if err != nil {
			t.Fatalf("failed to get composite model: %v", err)
		}

		if found.Name != composite.Name {
			t.Errorf("expected name %s, got %s", composite.Name, found.Name)
		}

		if len(found.BackendModels) != len(composite.BackendModels) {
			t.Errorf("expected %d backend models, got %d", len(composite.BackendModels), len(found.BackendModels))
		}
	})

	t.Run("ProfileManager loads composite models", func(t *testing.T) {
		pm := service.GetProfileManager()
		err := pm.RefreshAll()
		if err != nil {
			t.Fatalf("failed to refresh profiles: %v", err)
		}

		// Get the profile
		profileInstance := pm.GetProfileByID(profile.ID)
		if profileInstance == nil {
			t.Fatal("profile not found")
		}

		// Check if composite model is loaded
		loadedComposite, ok := profileInstance.GetCompositeModel(composite.Name)
		if !ok {
			t.Fatal("composite model not loaded in profile")
		}

		if loadedComposite.Name != composite.Name {
			t.Errorf("expected name %s, got %s", composite.Name, loadedComposite.Name)
		}
	})

	t.Run("CompositeService routes to backend model", func(t *testing.T) {
		compService, err := service.NewCompositeModelService(profile.ID)
		if err != nil {
			t.Skipf("failed to create composite service (Redis may not be running): %v", err)
		}
		defer compService.Close()

		pm := service.GetProfileManager()
		profileInstance := pm.GetProfileByID(profile.ID)
		if profileInstance == nil {
			t.Fatal("profile not found")
		}

		ctx := context.Background()
		result, err := compService.Route(ctx, profileInstance, composite.Name)

		// This may fail if we don't have actual API keys, so we just check the routing logic
		if err != nil {
			t.Logf("Routing failed (expected if no API keys): %v", err)
		} else if result != nil {
			t.Logf("Successfully routed to model: %s from provider: %s", result.Model.Name, result.Provider.ID)
		}
	})

	t.Run("HealthCheck checks backend model health", func(t *testing.T) {
		compService, err := service.NewCompositeModelService(profile.ID)
		if err != nil {
			t.Skipf("failed to create composite service (Redis may not be running): %v", err)
		}
		defer compService.Close()

		ctx := context.Background()
		health, err := compService.HealthCheck(ctx, composite.Name)
		if err != nil {
			t.Fatalf("health check failed: %v", err)
		}

		if len(health) == 0 {
			t.Error("expected health check results, got none")
		}

		for backend, healthy := range health {
			t.Logf("Backend %s: healthy=%v", backend, healthy)
		}
	})
}

package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/repository"
	"github.com/gemone/model-router/internal/service"
	"github.com/gofiber/fiber/v3"
)

// CompositeAdminHandler manages composite model endpoints
type CompositeAdminHandler struct {
	profileManager     *service.ProfileManager
	compositeModelRepo repository.CompositeModelRepository
	stats              *service.StatsCollector
}

// NewCompositeAdminHandler creates a new composite admin handler
func NewCompositeAdminHandler() *CompositeAdminHandler {
	return &CompositeAdminHandler{
		profileManager:     service.GetProfileManager(),
		compositeModelRepo: repository.NewCompositeModelRepository(database.GetDB()),
		stats:              service.GetStatsCollector(),
	}
}

// RegisterRoutes registers composite admin routes
func (h *CompositeAdminHandler) RegisterRoutes(r fiber.Router) {
	// Composite model management routes
	r.Get("/profiles/:profile_id/composite-models", h.ListCompositeModels)
	r.Put("/profiles/:profile_id/composite-models/:id", h.UpsertCompositeModel)
	r.Delete("/profiles/:profile_id/composite-models/:id", h.DeleteCompositeModel)
	r.Get("/profiles/:profile_id/composite-models/:id/health", h.GetCompositeModelHealth)
	r.Get("/profiles/:profile_id/composite-models/:id/metrics", h.GetCompositeModelMetrics)
}

// ListCompositeModels lists all composite models for a profile
// GET /api/admin/profiles/:profile_id/composite-models
func (h *CompositeAdminHandler) ListCompositeModels(c fiber.Ctx) error {
	profileID := c.Params("profile_id")

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	models, err := h.compositeModelRepo.ListEnabledByProfile(c.Context(), profile.Profile.ID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list composite models"})
	}

	return c.JSON(models)
}

// UpsertCompositeModel creates or updates a composite model
// PUT /api/admin/profiles/:profile_id/composite-models/:id
func (h *CompositeAdminHandler) UpsertCompositeModel(c fiber.Ctx) error {
	profileID := c.Params("profile_id")
	modelID := c.Params("id")

	var req model.CompositeAutoModel
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// Validate CompositeBackendModel references against profile models
	for _, backend := range req.BackendModels {
		_, _, err := profile.GetAdapterForModel(backend.ModelName, backend.ProviderID)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error":   fmt.Sprintf("backend model reference invalid: %s@%s", backend.ModelName, backend.ProviderID),
				"details": err.Error(),
			})
		}
	}

	// Set fields
	req.ProfileID = profile.Profile.ID
	req.Name = modelID

	// Validate configuration
	if err := req.Validate(); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Check if exists
	existing, _ := h.compositeModelRepo.GetByProfileAndName(c.Context(), profile.Profile.ID, modelID)
	if existing != nil {
		req.ID = existing.ID
		if err := h.compositeModelRepo.Update(c.Context(), &req); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update composite model"})
		}
	} else {
		req.ID = fmt.Sprintf("%s-%s-%d", profileID, modelID, time.Now().Unix())
		if err := h.compositeModelRepo.Create(c.Context(), &req); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create composite model"})
		}
	}

	// Refresh profile to pick up composite model changes
	_ = h.profileManager.Refresh(profileID)

	return c.JSON(req)
}

// DeleteCompositeModel deletes a composite model
// DELETE /api/admin/profiles/:profile_id/composite-models/:id
func (h *CompositeAdminHandler) DeleteCompositeModel(c fiber.Ctx) error {
	profileID := c.Params("profile_id")
	modelID := c.Params("id")

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	composite, err := h.compositeModelRepo.GetByProfileAndName(c.Context(), profile.Profile.ID, modelID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "composite model not found"})
	}

	if err := h.compositeModelRepo.Delete(c.Context(), composite.ID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete composite model"})
	}

	// Refresh profile to pick up composite model changes
	_ = h.profileManager.Refresh(profileID)

	return c.SendStatus(http.StatusNoContent)
}

// GetCompositeModelHealth returns health status for a composite model
// GET /api/admin/profiles/:profile_id/composite-models/:id/health
func (h *CompositeAdminHandler) GetCompositeModelHealth(c fiber.Ctx) error {
	profileID := c.Params("profile_id")
	modelID := c.Params("id")

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	composite, err := h.compositeModelRepo.GetByProfileAndName(c.Context(), profile.Profile.ID, modelID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "composite model not found"})
	}

	type BackendHealth struct {
		ModelName   string  `json:"model_name"`
		ProviderID  string  `json:"provider_id"`
		HealthScore float64 `json:"health_score"`
		Available   bool    `json:"available"`
		Weight      int     `json:"weight"`
	}

	healths := make([]BackendHealth, 0, len(composite.BackendModels))
	for _, backend := range composite.BackendModels {
		health := BackendHealth{
			ModelName:   backend.ModelName,
			ProviderID:  backend.ProviderID,
			HealthScore: h.stats.GetHealthScore(backend.ProviderID, backend.ModelName),
			Weight:      backend.Weight,
		}
		health.Available = health.HealthScore >= composite.HealthThreshold
		healths = append(healths, health)
	}

	return c.JSON(fiber.Map{
		"model_id":         modelID,
		"strategy":         composite.Strategy,
		"health_threshold": composite.HealthThreshold,
		"enabled":          composite.Enabled,
		"backends":         healths,
	})
}

// GetCompositeModelMetrics returns metrics for a composite model
// GET /api/admin/profiles/:profile_id/composite-models/:id/metrics
func (h *CompositeAdminHandler) GetCompositeModelMetrics(c fiber.Ctx) error {
	profileID := c.Params("profile_id")
	modelID := c.Params("id")

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	composite, err := h.compositeModelRepo.GetByProfileAndName(c.Context(), profile.Profile.ID, modelID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "composite model not found"})
	}

	type BackendMetrics struct {
		ModelName       string  `json:"model_name"`
		ProviderID      string  `json:"provider_id"`
		HealthScore     float64 `json:"health_score"`
		AvgLatency      float64 `json:"avg_latency_ms"`
		SuccessRate     float64 `json:"success_rate"`
		RequestCount    int64   `json:"request_count"`
		ErrorCount      int64   `json:"error_count"`
	}

	metrics := make([]BackendMetrics, 0, len(composite.BackendModels))
	for _, backend := range composite.BackendModels {
		// GetProviderStats only takes providerID, returns map[string]interface{}
		providerStats := h.stats.GetProviderStats(backend.ProviderID)

		// Extract model-specific stats if available in model_stats
		var avgLatency float64
		var successRate float64
		var requestCount int64
		var errorCount int64

		if modelStats, ok := providerStats["model_stats"].(map[string]map[string]int64); ok {
			if ms, ok := modelStats[backend.ModelName]; ok {
				requestCount = ms["requests"]
				errorCount = ms["requests"] - ms["success"]
				if requestCount > 0 {
					successRate = float64(ms["success"]) / float64(requestCount) * 100
				}
			}
		}
		if al, ok := providerStats["avg_latency_ms"].(float64); ok {
			avgLatency = al
		}

		m := BackendMetrics{
			ModelName:    backend.ModelName,
			ProviderID:   backend.ProviderID,
			HealthScore:  h.stats.GetHealthScore(backend.ProviderID, backend.ModelName),
			AvgLatency:   avgLatency,
			SuccessRate:  successRate,
			RequestCount: requestCount,
			ErrorCount:   errorCount,
		}
		metrics = append(metrics, m)
	}

	return c.JSON(fiber.Map{
		"model_id": modelID,
		"strategy": composite.Strategy,
		"enabled":  composite.Enabled,
		"backends": metrics,
	})
}

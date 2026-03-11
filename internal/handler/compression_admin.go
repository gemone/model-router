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

// CompressionAdminHandler manages compression group endpoints
type CompressionAdminHandler struct {
	profileManager      *service.ProfileManager
	compressionGroupRepo repository.CompressionGroupRepository
	stats               *service.StatsCollector
}

// NewCompressionAdminHandler creates a new compression admin handler
func NewCompressionAdminHandler() *CompressionAdminHandler {
	return &CompressionAdminHandler{
		profileManager:      service.GetProfileManager(),
		compressionGroupRepo: repository.NewCompressionGroupRepository(database.GetDB()),
		stats:               service.GetStatsCollector(),
	}
}

// RegisterRoutes registers compression admin routes
func (h *CompressionAdminHandler) RegisterRoutes(r fiber.Router) {
	// Compression group management routes
	r.Get("/profiles/:profile_id/compression-groups", h.ListCompressionGroups)
	r.Put("/profiles/:profile_id/compression-groups/:group_name", h.UpsertCompressionGroup)
	r.Delete("/profiles/:profile_id/compression-groups/:group_name", h.DeleteCompressionGroup)
	r.Get("/profiles/:profile_id/compression-groups/:group_name/health", h.GetCompressionGroupHealth)
}

// ListCompressionGroups lists all compression groups for a profile
// GET /api/admin/profiles/:profile_id/compression-groups
func (h *CompressionAdminHandler) ListCompressionGroups(c fiber.Ctx) error {
	profileID := c.Params("profile_id")

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	groups, err := h.compressionGroupRepo.ListEnabledByProfile(c.Context(), profile.Profile.ID)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list groups"})
	}

	return c.JSON(groups)
}

// UpsertCompressionGroup creates or updates a compression group
// PUT /api/admin/profiles/:profile_id/compression-groups/:group_name
func (h *CompressionAdminHandler) UpsertCompressionGroup(c fiber.Ctx) error {
	profileID := c.Params("profile_id")
	groupName := c.Params("group_name")

	var req model.CompressionModelGroup
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// Validate model references exist
	for _, modelRef := range req.Models {
		_, _, err := profile.GetAdapterForModel(modelRef.ModelName, modelRef.ProviderID)
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error":   fmt.Sprintf("model reference invalid: %s@%s", modelRef.ModelName, modelRef.ProviderID),
				"details": err.Error(),
			})
		}
	}

	// Set fields
	req.ProfileID = profile.Profile.ID
	req.Name = groupName

	// Check if exists
	existing, _ := h.compressionGroupRepo.GetByProfileAndName(c.Context(), profile.Profile.ID, groupName)
	if existing != nil {
		req.ID = existing.ID
		if err := h.compressionGroupRepo.Update(c.Context(), &req); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update group"})
		}
	} else {
		req.ID = fmt.Sprintf("%s-%s-%d", profileID, groupName, time.Now().Unix())
		if err := h.compressionGroupRepo.Create(c.Context(), &req); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create group"})
		}
	}

	// Invalidate cache
	if profile.CompressionSelector != nil {
		_ = profile.CompressionSelector.InvalidateGroupCache(c.Context(), groupName)
	}

	return c.JSON(req)
}

// DeleteCompressionGroup deletes a compression group
// DELETE /api/admin/profiles/:profile_id/compression-groups/:group_name
func (h *CompressionAdminHandler) DeleteCompressionGroup(c fiber.Ctx) error {
	profileID := c.Params("profile_id")
	groupName := c.Params("group_name")

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	group, err := h.compressionGroupRepo.GetByProfileAndName(c.Context(), profile.Profile.ID, groupName)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}

	if err := h.compressionGroupRepo.Delete(c.Context(), group.ID); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete group"})
	}

	// Invalidate cache
	if profile.CompressionSelector != nil {
		_ = profile.CompressionSelector.InvalidateGroupCache(c.Context(), groupName)
	}

	return c.SendStatus(http.StatusNoContent)
}

// GetCompressionGroupHealth returns health status for a compression group
// GET /api/admin/profiles/:profile_id/compression-groups/:group_name/health
func (h *CompressionAdminHandler) GetCompressionGroupHealth(c fiber.Ctx) error {
	profileID := c.Params("profile_id")
	groupName := c.Params("group_name")

	profile := h.profileManager.GetProfile(profileID)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	group, err := h.compressionGroupRepo.GetByProfileAndName(c.Context(), profile.Profile.ID, groupName)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "group not found"})
	}

	type ModelHealth struct {
		ModelName    string  `json:"model_name"`
		ProviderID   string  `json:"provider_id"`
		HealthScore  float64 `json:"health_score"`
		Available    bool    `json:"available"`
	}

	healths := make([]ModelHealth, 0, len(group.Models))
	for _, modelRef := range group.Models {
		health := ModelHealth{
			ModelName:   modelRef.ModelName,
			ProviderID:  modelRef.ProviderID,
			HealthScore: h.stats.GetHealthScore(modelRef.ProviderID, modelRef.ModelName),
		}
		health.Available = health.HealthScore >= group.HealthThreshold
		healths = append(healths, health)
	}

	return c.JSON(fiber.Map{
		"group_name":        groupName,
		"health_threshold":  group.HealthThreshold,
		"enabled":           group.Enabled,
		"models":            healths,
	})
}

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/middleware"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/service"
	"github.com/gemone/model-router/internal/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ErrorResponse is a standardized error response format
// Use this for consistent error responses across all endpoints
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

// errorResponse creates a standardized error response
// Usage: return c.Status(status).JSON(errorResponse("message", "details", "hint"))
// Omit details or hint by passing empty string
func errorResponse(errorMsg, details, hint string) ErrorResponse {
	return ErrorResponse{
		Error:   errorMsg,
		Details: details,
		Hint:    hint,
	}
}

// AdminHandler manages the admin backend
type AdminHandler struct {
	profileManager *service.ProfileManager
	stats          *service.StatsCollector
}

// NewAdminHandler creates a new Admin handler
func NewAdminHandler() *AdminHandler {
	return &AdminHandler{
		profileManager: service.GetProfileManager(),
		stats:          service.GetStatsCollector(),
	}
}

// RegisterRoutes registers admin routes
// Note: Authentication endpoints (login, logout, auth/status) are registered separately
// with stricter rate limiting in serve.go
func (h *AdminHandler) RegisterRoutes(r fiber.Router) {
	// Profile management
	r.Get("/profiles", h.ListProfiles)
	r.Post("/profiles", h.CreateProfile)
	r.Get("/profiles/:id", h.GetProfile)
	r.Put("/profiles/:id", h.UpdateProfile)
	r.Delete("/profiles/:id", h.DeleteProfile)

	// Provider management
	r.Get("/providers", h.ListProviders)
	r.Post("/providers", h.CreateProvider)
	r.Get("/providers/:id", h.GetProvider)
	r.Put("/providers/:id", h.UpdateProvider)
	r.Delete("/providers/:id", h.DeleteProvider)

	// Model management
	r.Get("/models", h.ListModels)
	r.Post("/models", h.CreateModel)
	r.Get("/models/:id", h.GetModel)
	r.Put("/models/:id", h.UpdateModel)
	r.Delete("/models/:id", h.DeleteModel)

	// Route management
	r.Get("/routes", h.ListRoutes)
	r.Post("/routes", h.CreateRoute)
	r.Get("/routes/:id", h.GetRoute)
	r.Put("/routes/:id", h.UpdateRoute)
	r.Delete("/routes/:id", h.DeleteRoute)

	// Statistics
	r.Get("/stats/dashboard", h.GetDashboardStats)
	r.Get("/stats/trend", h.GetTrendStats)
	r.Get("/stats/all", h.GetAllProviderModelStats)
	r.Get("/stats/provider/:id", h.GetProviderStats)
	r.Get("/stats/model/:name", h.GetModelStats)

	// Logs
	r.Get("/logs", h.GetLogs)
	r.Delete("/logs", h.ClearLogs)

	// Testing
	r.Post("/test", h.TestModel)

	// Model capability detection
	r.Post("/models/detect-capabilities", h.DetectModelCapabilities)

	// Settings management
	r.Get("/settings", h.GetSettings)
	r.Put("/settings", h.UpdateSettings)

	// Log level management
	r.Get("/log-level", h.GetLogLevel)
	r.Put("/log-level", h.SetLogLevel)

	// Server log management
	r.Get("/server-logs", h.GetServerLogs)
	r.Get("/server-logs/:request_id", h.GetServerLogDetail)
	r.Delete("/server-logs", h.ClearServerLogs)
}

// ==================== Profile Management ====================

// ListProfiles lists all profiles
// Note: API tokens are intentionally omitted from responses for security.
func (h *AdminHandler) ListProfiles(c fiber.Ctx) error {
	profiles := h.profileManager.GetAllProfiles()
	// Clear API Token and encrypted fields
	for i := range profiles {
		profiles[i].APIToken = ""
		profiles[i].APITokenEnc = ""
	}
	return c.JSON(profiles)
}

// GetProfile gets a single profile
func (h *AdminHandler) GetProfile(c fiber.Ctx) error {
	id := c.Params("id")
	profileInstance := h.profileManager.GetProfileByID(id)
	if profileInstance == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}
	// Clear API Token and encrypted fields
	profileInstance.Profile.APIToken = ""
	profileInstance.Profile.APITokenEnc = ""
	return c.JSON(profileInstance.Profile)
}

// CreateProfile creates a new profile
func (h *AdminHandler) CreateProfile(c fiber.Ctx) error {
	var req struct {
		model.Profile
		APIToken string `json:"api_token"` // Receive plaintext token
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	profile := req.Profile
	profile.ID = uuid.New().String()

	// If API Token is provided, encrypt it for storage
	if req.APIToken != "" {
		encrypted, err := utils.Encrypt(req.APIToken)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encrypt API token"})
		}
		profile.APITokenEnc = encrypted
	}

	if err := h.profileManager.CreateProfile(&profile); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Clear sensitive fields when returning
	profile.APIToken = ""
	profile.APITokenEnc = ""
	return c.Status(http.StatusCreated).JSON(profile)
}

// UpdateProfile updates a profile
func (h *AdminHandler) UpdateProfile(c fiber.Ctx) error {
	id := c.Params("id")

	// Get existing profile first
	db := database.GetDB()
	var existingProfile model.Profile
	if err := db.First(&existingProfile, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// Parse only the fields we want to update
	var updates struct {
		Name                 *string  `json:"name"`
		Path                 *string  `json:"path"`
		Description          *string  `json:"description"`
		Enabled              *bool    `json:"enabled"`
		Priority             *int     `json:"priority"`
		EnableCompression    *bool    `json:"enable_compression"`
		CompressionStrategy  *string  `json:"compression_strategy"`
		CompressionLevel     *string  `json:"compression_level"`
		CompressionThreshold *int     `json:"compression_threshold"`
		MaxContextWindow     *int     `json:"max_context_window"`
		ModelIDs             []string `json:"model_ids"`
		FallbackModels       []string `json:"fallback_models"`
		RouteIDs             []string `json:"route_ids"`
		APIToken             *string  `json:"api_token"` // API Token update field
	}
	if err := c.Bind().Body(&updates); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Update only provided fields
	if updates.Name != nil {
		existingProfile.Name = *updates.Name
	}
	if updates.Path != nil && *updates.Path != "" {
		existingProfile.Path = *updates.Path
	}
	if updates.Description != nil {
		existingProfile.Description = *updates.Description
	}
	if updates.Enabled != nil {
		existingProfile.Enabled = *updates.Enabled
	}
	if updates.EnableCompression != nil {
		existingProfile.EnableCompression = *updates.EnableCompression
	}
	if updates.CompressionStrategy != nil {
		existingProfile.CompressionStrategy = *updates.CompressionStrategy
	}
	if updates.CompressionLevel != nil {
		existingProfile.CompressionLevel = *updates.CompressionLevel
	}
	if updates.CompressionThreshold != nil {
		existingProfile.CompressionThreshold = *updates.CompressionThreshold
	}
	if updates.MaxContextWindow != nil {
		existingProfile.MaxContextWindow = *updates.MaxContextWindow
	}
	if updates.ModelIDs != nil {
		existingProfile.ModelIDs = updates.ModelIDs
	}
	if updates.FallbackModels != nil {
		existingProfile.FallbackModels = updates.FallbackModels
	}
	if updates.RouteIDs != nil {
		existingProfile.RouteIDs = updates.RouteIDs
	}

	// Update API Token (if provided)
	if updates.APIToken != nil {
		if *updates.APIToken != "" {
			// Encrypt the new token
			encrypted, err := utils.Encrypt(*updates.APIToken)
			if err != nil {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encrypt API token"})
			}
			existingProfile.APITokenEnc = encrypted
		} else {
			// Empty string means clear the token
			existingProfile.APITokenEnc = ""
		}
	}

	if err := h.profileManager.UpdateProfile(&existingProfile); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(existingProfile)
}

// DeleteProfile deletes a profile
func (h *AdminHandler) DeleteProfile(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.profileManager.DeleteProfile(id); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(http.StatusNoContent)
}

// ==================== Provider Management ====================

// ListProviders lists all providers
// Note: API keys are intentionally omitted from responses for security.
// The APIKey field is cleared and APIKeyEnc (encrypted) is never returned.
func (h *AdminHandler) ListProviders(c fiber.Ctx) error {
	db := database.GetDB()
	var providers []model.Provider
	if err := db.Find(&providers).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Don't return encrypted API keys (API keys are redacted for security)
	for i := range providers {
		providers[i].APIKey = ""
		providers[i].APIKeyEnc = ""
	}

	return c.JSON(providers)
}

// GetProvider gets a single provider
func (h *AdminHandler) GetProvider(c fiber.Ctx) error {
	id := c.Params("id")
	db := database.GetDB()

	var provider model.Provider
	if err := db.First(&provider, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "provider not found"})
	}

	provider.APIKey = ""
	provider.APIKeyEnc = ""

	return c.JSON(provider)
}

// CreateProvider creates a new provider
func (h *AdminHandler) CreateProvider(c fiber.Ctx) error {
	var req struct {
		model.Provider
		APIKey string `json:"api_key"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	provider := req.Provider
	provider.ID = uuid.New().String()

	// Generate IDs for nested models
	for i := range provider.Models {
		if provider.Models[i].ID == "" {
			provider.Models[i].ID = uuid.New().String()
		}
		provider.Models[i].ProviderID = provider.ID
	}

	// Encrypt API key
	if req.APIKey != "" {
		encrypted, err := utils.Encrypt(req.APIKey)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encrypt API key"})
		}
		provider.APIKeyEnc = encrypted
	}

	db := database.GetDB()
	if err := db.Create(&provider).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh profiles
	h.profileManager.RefreshAll()

	provider.APIKey = ""
	provider.APIKeyEnc = ""
	return c.Status(http.StatusCreated).JSON(provider)
}

// UpdateProvider updates a provider
func (h *AdminHandler) UpdateProvider(c fiber.Ctx) error {
	id := c.Params("id")

	var req struct {
		model.Provider
		APIKey string `json:"api_key"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	db := database.GetDB()

	var provider model.Provider
	if err := db.First(&provider, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "provider not found"})
	}

	// Update fields
	provider.Name = req.Name
	provider.Type = req.Type
	provider.BaseURL = req.BaseURL
	provider.Enabled = req.Enabled

	// If new API key is provided, update it
	if req.APIKey != "" {
		encrypted, err := utils.Encrypt(req.APIKey)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encrypt API key"})
		}
		provider.APIKeyEnc = encrypted
	}

	if err := db.Save(&provider).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh profiles
	h.profileManager.RefreshAll()

	provider.APIKey = ""
	provider.APIKeyEnc = ""
	return c.JSON(provider)
}

// DeleteProvider deletes a provider
func (h *AdminHandler) DeleteProvider(c fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()

	// Delete associated models
	db.Where("provider_id = ?", id).Delete(&model.Model{})

	// Delete provider
	if err := db.Delete(&model.Provider{}, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh profiles
	h.profileManager.RefreshAll()

	return c.SendStatus(http.StatusNoContent)
}

// ==================== Model Management ====================

// ListModels lists all models
func (h *AdminHandler) ListModels(c fiber.Ctx) error {
	db := database.GetDB()
	var models []model.Model

	query := db
	if profileID := c.Query("profile_id"); profileID != "" {
		query = query.Where("profile_id = ?", profileID)
	}
	if providerID := c.Query("provider_id"); providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}

	if err := query.Find(&models).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(models)
}

// GetModel gets a single model
func (h *AdminHandler) GetModel(c fiber.Ctx) error {
	id := c.Params("id")
	db := database.GetDB()

	var m model.Model
	if err := db.First(&m, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "model not found"})
	}

	return c.JSON(m)
}

// CreateModel creates a new model
func (h *AdminHandler) CreateModel(c fiber.Ctx) error {
	var m model.Model
	if err := c.Bind().Body(&m); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	m.ID = uuid.New().String()

	db := database.GetDB()
	if err := db.Create(&m).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh profiles
	h.profileManager.RefreshAll()

	return c.Status(http.StatusCreated).JSON(m)
}

// UpdateModel updates a model
func (h *AdminHandler) UpdateModel(c fiber.Ctx) error {
	id := c.Params("id")

	var m model.Model
	if err := c.Bind().Body(&m); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	m.ID = id

	db := database.GetDB()

	var existing model.Model
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "model not found"})
	}

	if err := db.Save(&m).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh profiles
	h.profileManager.RefreshAll()

	return c.JSON(m)
}

// DeleteModel deletes a model
func (h *AdminHandler) DeleteModel(c fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()
	if err := db.Delete(&model.Model{}, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh profiles
	h.profileManager.RefreshAll()

	return c.SendStatus(http.StatusNoContent)
}

// ==================== Route Management ====================

// ListRoutes lists all routes
func (h *AdminHandler) ListRoutes(c fiber.Ctx) error {
	db := database.GetDB()
	var routes []model.Route
	if err := db.Find(&routes).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// First pass: collect all model IDs for batch query (avoid N+1)
	var allModelIDs []string
	for _, r := range routes {
		if r.ModelConfig != "" {
			var config model.RouteModelConfig
			if err := json.Unmarshal([]byte(r.ModelConfig), &config); err != nil {
				// Log error but continue processing
				continue
			}
			for _, entry := range config.Models {
				if entry.ModelID != "" {
					allModelIDs = append(allModelIDs, entry.ModelID)
				}
			}
		}
	}

	// Fetch all models in a single query
	var allModels []model.Model
	if len(allModelIDs) > 0 {
		db.Where("id IN ?", allModelIDs).Find(&allModels)
	}
	modelMap := make(map[string]model.Model)
	for _, m := range allModels {
		modelMap[m.ID] = m
	}

	// Convert to frontend format
	var result []fiber.Map
	for _, r := range routes {
		// Parse model config to get target models
		var config model.RouteModelConfig
		var targetModels []string
		if r.ModelConfig != "" {
			if err := json.Unmarshal([]byte(r.ModelConfig), &config); err != nil {
				// Log error but continue with empty target models
				middleware.WarnLog("Failed to parse model config for route %s: %v", r.ID, err)
			} else {
				for _, entry := range config.Models {
					if m, ok := modelMap[entry.ModelID]; ok {
						targetModels = append(targetModels, m.Name)
					}
				}
			}
		}

		fallbackEnabled := r.FallbackPolicy != "none"

		result = append(result, fiber.Map{
			"id":               r.ID,
			"name":             r.Name,
			"description":      r.Description,
			"enabled":          r.Enabled,
			"strategy":         r.Strategy,
			"content_type":     r.ContentType,
			"model_pattern":    "*", // Default pattern
			"target_models":    targetModels,
			"fallback_enabled": fallbackEnabled,
			"fallback_models":  []string{},
			"models":           len(targetModels),
		})
	}

	return c.JSON(result)
}

// GetRoute gets a single route
func (h *AdminHandler) GetRoute(c fiber.Ctx) error {
	id := c.Params("id")
	db := database.GetDB()

	var r model.Route
	if err := db.First(&r, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "route not found"})
	}

	// Parse model config to get target models (using batch query to avoid N+1)
	var config model.RouteModelConfig
	var targetModels []string
	var models []fiber.Map
	if r.ModelConfig != "" {
		if err := json.Unmarshal([]byte(r.ModelConfig), &config); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to parse model config for route %s: %v", r.ID, err),
			})
		}

		// Collect all model IDs for batch query
		var modelIDs []string
		for _, entry := range config.Models {
			if entry.ModelID != "" {
				modelIDs = append(modelIDs, entry.ModelID)
			}
		}

		// Fetch all models in a single query
		var fetchedModels []model.Model
		if len(modelIDs) > 0 {
			db.Where("id IN ?", modelIDs).Find(&fetchedModels)
		}

		// Build a map for quick lookup
		modelMap := make(map[string]model.Model)
		for _, m := range fetchedModels {
			modelMap[m.ID] = m
		}

		// Build response using the map
		for _, entry := range config.Models {
			if m, ok := modelMap[entry.ModelID]; ok {
				targetModels = append(targetModels, m.Name)
				models = append(models, fiber.Map{
					"model_id": entry.ModelID,
					"name":     m.Name,
					"weight":   entry.Weight,
					"priority": entry.Priority,
					"enabled":  entry.Enabled,
				})
			}
		}
	}

	fallbackEnabled := r.FallbackPolicy != "none"

	return c.JSON(fiber.Map{
		"id":               r.ID,
		"name":             r.Name,
		"description":      r.Description,
		"enabled":          r.Enabled,
		"strategy":         r.Strategy,
		"content_type":     r.ContentType,
		"model_pattern":    "*",
		"target_models":    targetModels,
		"fallback_enabled": fallbackEnabled,
		"fallback_models":  []string{},
		"models":           models,
	})
}

// CreateRouteRequest frontend route creation request format
type CreateRouteRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	ModelPattern    string   `json:"model_pattern"`
	Strategy        string   `json:"strategy"`
	ContentType     string   `json:"content_type"`
	TargetModels    []string `json:"target_models"`
	FallbackEnabled bool     `json:"fallback_enabled"`
	FallbackModels  []string `json:"fallback_models"`
	Enabled         bool     `json:"enabled"`
}

// CreateRoute creates a route
func (h *AdminHandler) CreateRoute(c fiber.Ctx) error {
	var req CreateRouteRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validate content_type FIRST (before any processing or DB operations)
	contentType := model.ContentType(req.ContentType)
	if contentType == "" {
		contentType = model.ContentTypeAll
	}

	validContentTypes := map[model.ContentType]bool{
		model.ContentTypeText:  true,
		model.ContentTypeImage: true,
		model.ContentTypeAll:   true,
	}
	if !validContentTypes[contentType] {
		return c.Status(http.StatusBadRequest).JSON(errorResponse(
			"invalid content_type",
			fmt.Sprintf("got '%s', but must be one of: text, image, all", req.ContentType),
			"See API documentation for valid content_type values",
		))
	}

	// Build model config from target_models (batch fetch to avoid N+1 query)
	db := database.GetDB()

	// Fetch all models in a single query to avoid N+1 problem
	var models []model.Model
	if len(req.TargetModels) > 0 {
		if err := db.Where("name IN ?", req.TargetModels).Find(&models).Error; err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch models"})
		}
	}

	// Build a map for quick lookup
	modelMap := make(map[string]model.Model)
	for _, m := range models {
		modelMap[m.Name] = m
	}

	// Build model entries from the fetched models
	var modelEntries []model.RouteModelEntry
	for i, modelName := range req.TargetModels {
		m, exists := modelMap[modelName]
		if !exists {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "model not found: " + modelName})
		}
		modelEntries = append(modelEntries, model.RouteModelEntry{
			ModelID:  m.ID,
			Weight:   100,
			Priority: len(req.TargetModels) - i, // Higher priority for earlier models
			Enabled:  true,
		})
	}

	modelConfig := model.RouteModelConfig{Models: modelEntries}
	modelConfigJSON, _ := json.Marshal(modelConfig)

	// Determine fallback policy
	fallbackPolicy := "none"
	if req.FallbackEnabled {
		fallbackPolicy = "next_model"
	}

	route := model.Route{
		ID:              uuid.New().String(),
		Name:            req.Name,
		Description:     req.Description,
		Enabled:         req.Enabled,
		Strategy:        model.RouteStrategy(req.Strategy),
		ContentType:     contentType,
		ModelConfig:     string(modelConfigJSON),
		HealthThreshold: 70,
		FallbackPolicy:  fallbackPolicy,
	}

	// Set default strategy if not specified
	if route.Strategy == "" {
		route.Strategy = model.RouteStrategyPriority
	}

	if err := db.Create(&route).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh route service
	service.GetRouteService().Refresh(route.ID)

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"id":               route.ID,
		"name":             route.Name,
		"description":      route.Description,
		"enabled":          route.Enabled,
		"strategy":         route.Strategy,
		"content_type":     route.ContentType,
		"model_config":     route.ModelConfig,
		"health_threshold": route.HealthThreshold,
		"fallback_policy":  route.FallbackPolicy,
		"model_pattern":    req.ModelPattern,
		"target_models":    req.TargetModels,
		"fallback_enabled": req.FallbackEnabled,
		"fallback_models":  req.FallbackModels,
	})
}

// UpdateRouteRequest frontend route update request format
type UpdateRouteRequest struct {
	Name            *string  `json:"name"`
	Description     *string  `json:"description"`
	Enabled         *bool    `json:"enabled"`
	Strategy        *string  `json:"strategy"`
	ContentType     *string  `json:"content_type"`
	TargetModels    []string `json:"target_models"`
	FallbackEnabled *bool    `json:"fallback_enabled"`
}

// UpdateRoute updates a route
func (h *AdminHandler) UpdateRoute(c fiber.Ctx) error {
	id := c.Params("id")

	var req UpdateRouteRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	db := database.GetDB()
	var route model.Route
	if err := db.First(&route, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "route not found"})
	}

	// Update fields
	if req.Name != nil {
		route.Name = *req.Name
	}
	if req.Description != nil {
		route.Description = *req.Description
	}
	if req.Enabled != nil {
		route.Enabled = *req.Enabled
	}
	if req.Strategy != nil {
		route.Strategy = model.RouteStrategy(*req.Strategy)
	}
	if req.ContentType != nil {
		route.ContentType = model.ContentType(*req.ContentType)
	}
	if req.FallbackEnabled != nil {
		if *req.FallbackEnabled {
			route.FallbackPolicy = "next_model"
		} else {
			route.FallbackPolicy = "none"
		}
	}

	// Update target models if provided (batch fetch to avoid N+1 query)
	if req.TargetModels != nil {
		// Fetch all models in a single query to avoid N+1 problem
		var models []model.Model
		if len(req.TargetModels) > 0 {
			if err := db.Where("name IN ?", req.TargetModels).Find(&models).Error; err != nil {
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch models"})
			}
		}

		// Build a map for quick lookup
		modelMap := make(map[string]model.Model)
		for _, m := range models {
			modelMap[m.Name] = m
		}

		var modelEntries []model.RouteModelEntry
		for i, modelName := range req.TargetModels {
			m, exists := modelMap[modelName]
			if !exists {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "model not found: " + modelName})
			}
			modelEntries = append(modelEntries, model.RouteModelEntry{
				ModelID:  m.ID,
				Weight:   100,
				Priority: len(req.TargetModels) - i,
				Enabled:  true,
			})
		}
		modelConfig := model.RouteModelConfig{Models: modelEntries}
		modelConfigJSON, _ := json.Marshal(modelConfig)
		route.ModelConfig = string(modelConfigJSON)
	}

	if err := db.Save(&route).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh route service
	service.GetRouteService().Refresh(route.ID)

	// Parse model config for response (using batch query to avoid N+1)
	var config model.RouteModelConfig
	var targetModels []string
	if route.ModelConfig != "" {
		if err := json.Unmarshal([]byte(route.ModelConfig), &config); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to parse model config for route %s: %v", route.ID, err),
			})
		}

		// Collect all model IDs for batch query
		var modelIDs []string
		for _, entry := range config.Models {
			if entry.ModelID != "" {
				modelIDs = append(modelIDs, entry.ModelID)
			}
		}

		// Fetch all models in a single query
		var fetchedModels []model.Model
		if len(modelIDs) > 0 {
			db.Where("id IN ?", modelIDs).Find(&fetchedModels)
		}

		// Build a map for quick lookup
		modelMap := make(map[string]model.Model)
		for _, m := range fetchedModels {
			modelMap[m.ID] = m
		}

		// Build response using the map
		for _, entry := range config.Models {
			if m, ok := modelMap[entry.ModelID]; ok {
				targetModels = append(targetModels, m.Name)
			}
		}
	}

	fallbackEnabled := route.FallbackPolicy != "none"

	return c.JSON(fiber.Map{
		"id":               route.ID,
		"name":             route.Name,
		"description":      route.Description,
		"enabled":          route.Enabled,
		"strategy":         route.Strategy,
		"content_type":     route.ContentType,
		"model_pattern":    "*",
		"target_models":    targetModels,
		"fallback_enabled": fallbackEnabled,
		"fallback_models":  []string{},
	})
}

// DeleteRoute deletes a route
func (h *AdminHandler) DeleteRoute(c fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()
	if err := db.Delete(&model.Route{}, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Refresh route service
	service.GetRouteService().RefreshAll()

	return c.SendStatus(http.StatusNoContent)
}

// DetectModelCapabilities detects model capabilities
func (h *AdminHandler) DetectModelCapabilities(c fiber.Ctx) error {
	var req struct {
		ProviderID string `json:"provider_id"`
		ModelName  string `json:"model_name"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Get Provider type to determine capability detection strategy
	db := database.GetDB()
	var provider model.Provider
	if err := db.First(&provider, "id = ?", req.ProviderID).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "provider not found"})
	}

	// Detect capabilities based on model name and Provider type
	supportsFunc := detectFunctionCapability(string(provider.Type), req.ModelName)
	supportsVision := detectVisionCapability(string(provider.Type), req.ModelName)

	return c.JSON(fiber.Map{
		"supports_function": supportsFunc,
		"supports_vision":   supportsVision,
		"message":           "Capabilities detected based on model name patterns",
	})
}

// detectFunctionCapability detects if a model supports function calling
func detectFunctionCapability(providerType, modelName string) bool {
	// OpenAI models
	if providerType == "openai" || providerType == "azure" || providerType == "openai_compatible" || providerType == "openai-compatible" {
		// GPT-4 series supports function calling
		if contains(modelName, "gpt-4") || contains(modelName, "gpt-3.5-turbo") || contains(modelName, "gpt-4o") {
			return true
		}
	}

	// Anthropic/Claude models
	if providerType == "claude" || providerType == "anthropic" {
		// Claude 3 series supports function calling
		if contains(modelName, "claude-3") {
			return true
		}
	}

	// DeepSeek models
	if providerType == "deepseek" {
		// deepseek-chat and deepseek-coder support function calling
		if contains(modelName, "deepseek-chat") || contains(modelName, "deepseek-coder") {
			return true
		}
	}

	return false
}

// detectVisionCapability detects if a model supports vision
func detectVisionCapability(providerType, modelName string) bool {
	// OpenAI models - versions with vision or v
	if providerType == "openai" || providerType == "azure" || providerType == "openai_compatible" || providerType == "openai-compatible" {
		if contains(modelName, "vision") || contains(modelName, "-4o") || contains(modelName, "-4-turbo") {
			return true
		}
	}

	// Anthropic/Claude models - Claude 3 series all support vision
	if providerType == "claude" || providerType == "anthropic" {
		if contains(modelName, "claude-3") {
			return true
		}
	}

	// DeepSeek - VL models support vision
	if providerType == "deepseek" {
		if contains(modelName, "vl") || contains(modelName, "vision") {
			return true
		}
	}

	// Ollama - multimodal models
	if providerType == "ollama" {
		if contains(modelName, "llava") || contains(modelName, "vision") || contains(modelName, "mm") {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ==================== Statistics ====================

// GetDashboardStats gets dashboard statistics
func (h *AdminHandler) GetDashboardStats(c fiber.Ctx) error {
	stats := h.stats.GetDashboardStats()

	// Add active model and provider counts
	db := database.GetDB()
	var modelCount, providerCount int64
	db.Model(&model.Model{}).Where("enabled = ?", true).Count(&modelCount)
	db.Model(&model.Provider{}).Where("enabled = ?", true).Count(&providerCount)

	stats["active_models"] = modelCount
	stats["active_providers"] = providerCount

	return c.JSON(stats)
}

// GetTrendStats gets trend statistics (last 24 hours)
func (h *AdminHandler) GetTrendStats(c fiber.Ctx) error {
	stats := h.stats.GetTrendStats()
	return c.JSON(stats)
}

// GetProviderStats gets provider statistics
func (h *AdminHandler) GetProviderStats(c fiber.Ctx) error {
	id := c.Params("id")
	stats := h.stats.GetProviderStats(id)
	return c.JSON(stats)
}

// GetModelStats gets model statistics
func (h *AdminHandler) GetModelStats(c fiber.Ctx) error {
	name := c.Params("name")
	stats := h.stats.GetModelStats(name)
	return c.JSON(stats)
}

// GetAllProviderModelStats gets detailed statistics for all providers and models
func (h *AdminHandler) GetAllProviderModelStats(c fiber.Ctx) error {
	stats := h.stats.GetAllProviderModelStats()
	return c.JSON(stats)
}

// ==================== Logs ====================

// GetLogs gets logs
func (h *AdminHandler) GetLogs(c fiber.Ctx) error {
	page := 1
	pageSize := 50

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil {
			pageSize = parsed
		}
	}

	logs, total := h.stats.GetRequestLogs(page, pageSize)
	return c.JSON(fiber.Map{
		"logs":  logs,
		"total": total,
	})
}

// ClearLogs clears logs
func (h *AdminHandler) ClearLogs(c fiber.Ctx) error {
	if err := h.stats.ClearLogs(); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(http.StatusNoContent).SendString("")
}

// ==================== Testing ====================

// TestModel tests a model
func (h *AdminHandler) TestModel(c fiber.Ctx) error {
	var req struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
	}
	if err := c.Bind().Body(&req); err != nil {
		middleware.ErrorLog("TestModel BodyParser failed: %v", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if req.Model == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "model is required"})
	}

	var result *model.TestResult
	var err error

	// If provider_id is provided, use ProfileInstance.TestModel with specific provider
	if req.ProviderID != "" {
		// Find the profile instance that contains this provider
		result, err = h.testModelWithProvider(c.Context(), req.ProviderID, req.Model)
	} else {
		// Auto-detect provider
		result, err = h.profileManager.TestModel(c.Context(), req.Model)
	}

	if err != nil {
		middleware.ErrorLog("TestModel failed: provider=%s model=%s error=%v", req.ProviderID, req.Model, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	return c.JSON(result)
}

// testModelWithProvider tests a model using a specific provider
func (h *AdminHandler) testModelWithProvider(ctx context.Context, providerID, modelName string) (*model.TestResult, error) {
	// Query provider from database
	db := database.GetDB()
	var provider model.Provider
	if err := db.Where("id = ? AND enabled = ?", providerID, true).First(&provider).Error; err != nil {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	// Query model from database
	var targetModel model.Model
	if err := db.Where("name = ? AND provider_id = ? AND enabled = ?", modelName, providerID, true).First(&targetModel).Error; err != nil {
		return nil, fmt.Errorf("model not found: %s", modelName)
	}

	// Create adapter
	adp := adapter.Create(provider.Type)
	if adp == nil {
		return nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
	if err := adp.Init(&provider); err != nil {
		if strings.Contains(err.Error(), "decrypt") || strings.Contains(err.Error(), "encryption") {
			return nil, fmt.Errorf("failed to decrypt API key. Please re-configure the provider's API key. Error: %v", err)
		}
		return nil, fmt.Errorf("failed to init adapter: %v", err)
	}

	// Use OriginalName for the actual API call, fallback to modelName if empty
	actualModelName := modelName
	if targetModel.OriginalName != "" {
		actualModelName = targetModel.OriginalName
	}

	// Build request
	req := &model.ChatCompletionRequest{
		Model: actualModelName,
		Messages: []model.Message{
			{Role: "user", Content: "Hello, this is a test message. Please respond with 'OK'."},
		},
		MaxTokens: 50,
	}

	// Send request
	start := time.Now()
	_, err := adp.ChatCompletion(ctx, req)
	latency := time.Since(start).Milliseconds()

	testResult := &model.TestResult{
		ProviderID: providerID,
		Model:      modelName,
		Latency:    latency,
		CreatedAt:  time.Now(),
	}

	if err != nil {
		testResult.Success = false
		testResult.Error = err.Error()
	} else {
		testResult.Success = true
	}

	return testResult, nil
}

// ==================== Settings Management ====================

// SettingsRequest settings request structure
// SECURITY NOTE: AdminToken and JWTSecret are sensitive fields that must never be
// included in any JSON response. This struct is ONLY used for parsing input.
// All responses use fiber.Map with explicit empty strings for sensitive fields.
type SettingsRequest struct {
	Port           int    `json:"port"`
	Host           string `json:"host"`
	Language       string `json:"language"`
	EnableCORS     bool   `json:"enable_cors"`
	EnableStats    bool   `json:"enable_stats"`
	EnableFallback bool   `json:"enable_fallback"`
	AdminToken     string `json:"admin_token"` // SENSITIVE - Never include in responses
	JWTSecret      string `json:"jwt_secret"`  // SENSITIVE - Never include in responses
	LogLevel       string `json:"log_level"`
	MaxRetries     int    `json:"max_retries"`
	DBPath         string `json:"db_path"`
}

// GetSettings gets system settings
func (h *AdminHandler) GetSettings(c fiber.Ctx) error {
	cfg := config.Get()

	settings := fiber.Map{
		"port":            cfg.Server.Port,
		"host":            cfg.Server.Host,
		"language":        "zh-CN",
		"enable_cors":     cfg.CORS.Enabled,
		"enable_stats":    cfg.Features.EnableStats,
		"enable_fallback": cfg.Features.EnableFallback,
		"admin_token":     "",
		"jwt_secret":      "",
		"log_level":       cfg.Logging.Level,
		"max_retries":     cfg.Features.MaxRetries,
		"db_path":         cfg.Database.Path,
	}

	return c.JSON(settings)
}

// UpdateSettings updates system settings
func (h *AdminHandler) UpdateSettings(c fiber.Ctx) error {
	var req SettingsRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	cfg := config.Get()

	if req.Port > 0 && req.Port <= 65535 {
		cfg.Server.Port = req.Port
	}
	if req.Host != "" {
		cfg.Server.Host = req.Host
	}
	if req.AdminToken != "" {
		cfg.Security.AdminToken = req.AdminToken
	}
	if req.JWTSecret != "" {
		cfg.Security.JWTSecret = req.JWTSecret
	}
	cfg.CORS.Enabled = req.EnableCORS
	cfg.Features.EnableStats = req.EnableStats
	cfg.Features.EnableFallback = req.EnableFallback
	if req.LogLevel != "" {
		cfg.Logging.Level = req.LogLevel
	}
	if req.MaxRetries >= 0 {
		cfg.Features.MaxRetries = req.MaxRetries
	}
	if req.DBPath != "" {
		cfg.Database.Path = req.DBPath
	}

	response := fiber.Map{
		"port":            cfg.Server.Port,
		"host":            cfg.Server.Host,
		"language":        req.Language,
		"enable_cors":     cfg.CORS.Enabled,
		"enable_stats":    cfg.Features.EnableStats,
		"enable_fallback": cfg.Features.EnableFallback,
		"admin_token":     "",
		"jwt_secret":      "",
		"log_level":       cfg.Logging.Level,
		"max_retries":     cfg.Features.MaxRetries,
		"db_path":         cfg.Database.Path,
		"message":         "Settings updated. Some changes may require server restart.",
	}

	return c.JSON(response)
}

// ==================== Log Level Management ====================

// GetLogLevel gets current log level
func (h *AdminHandler) GetLogLevel(c fiber.Ctx) error {
	level := middleware.GetLogLevelString()
	return c.JSON(fiber.Map{
		"level":            level,
		"description":      getLogLevelDescription(level),
		"available_levels": []string{"debug", "info", "warn", "error"},
	})
}

// SetLogLevelRequest set log level request
type SetLogLevelRequest struct {
	Level string `json:"level"`
}

// SetLogLevel sets log level
func (h *AdminHandler) SetLogLevel(c fiber.Ctx) error {
	var req SetLogLevelRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error":   "invalid request body",
			"message": err.Error(),
		})
	}

	// Validate log level
	level := strings.ToLower(req.Level)
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[level] {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error":            "invalid log level",
			"message":          "valid levels are: debug, info, warn, error",
			"available_levels": []string{"debug", "info", "warn", "error"},
		})
	}

	// Set log level
	middleware.SetLogLevel(level)

	return c.JSON(fiber.Map{
		"level":       level,
		"description": getLogLevelDescription(level),
		"message":     "Log level updated successfully",
	})
}

// getLogLevelDescription gets log level description
func getLogLevelDescription(level string) string {
	switch level {
	case "debug":
		return "Debug mode - All requests and responses will be logged with full details"
	case "info":
		return "Info mode - Basic request information will be logged"
	case "warn":
		return "Warn mode - Only warnings and errors will be logged"
	case "error":
		return "Error mode - Only errors will be logged"
	default:
		return "Unknown level"
	}
}

// GetServerLogs gets server real-time logs (supports pagination and search)
func (h *AdminHandler) GetServerLogs(c fiber.Ctx) error {
	// Parse query parameters
	level := c.Query("level", "")
	keyword := c.Query("keyword", "")
	requestID := c.Query("request_id", "")
	groupByRequest := c.Query("group_by_request", "") == "true"

	page := 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 50
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 200 {
			pageSize = parsed
		}
	}

	// If querying specific request logs by request_id
	if requestID != "" && !groupByRequest {
		entries := middleware.GetLogStore().GetRequestLogs(requestID)
		return c.JSON(fiber.Map{
			"entries":    entries,
			"total":      len(entries),
			"request_id": requestID,
		})
	}

	// If grouping by request
	if groupByRequest {
		groups := middleware.GetRequestGroups(keyword, pageSize)
		return c.JSON(fiber.Map{
			"groups":  groups,
			"total":   len(groups),
			"grouped": true,
		})
	}

	// Normal query
	result := middleware.QueryLogs(level, keyword, "", page, pageSize)
	return c.JSON(fiber.Map{
		"entries":   result.Entries,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
		"has_more":  result.HasMore,
	})
}

// GetServerLogDetail gets log details for a specific request
func (h *AdminHandler) GetServerLogDetail(c fiber.Ctx) error {
	requestID := c.Params("request_id")
	if requestID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "request_id is required",
		})
	}

	entries := middleware.GetLogStore().GetRequestLogs(requestID)
	if len(entries) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "request not found",
		})
	}

	return c.JSON(fiber.Map{
		"request_id": requestID,
		"entries":    entries,
		"total":      len(entries),
	})
}

// ClearServerLogs clears server log buffer
func (h *AdminHandler) ClearServerLogs(c fiber.Ctx) error {
	middleware.GetLogStore().Clear()
	middleware.ClearBuffer()
	return c.JSON(fiber.Map{
		"message": "Server logs cleared successfully",
	})
}

// ==================== Authentication Management ====================

// LoginRequest login request
type LoginRequest struct {
	Password string `json:"password"`
}

// LoginResponse login response
type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// Login handles login request
func (h *AdminHandler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "invalid request body",
			"message": err.Error(),
		})
	}

	if req.Password == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "password_required",
			"message": "Password is required",
		})
	}

	// Validate password
	if middleware.ValidateAdminToken(req.Password) {
		// Login successful - return success without returning the token
		// The frontend already has the password/token from the user input
		return c.JSON(LoginResponse{
			Success: true,
			Message: "Login successful",
		})
	}

	// Login failed
	middleware.WarnLog("Failed login attempt from %s", c.IP())
	return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
		"success": false,
		"error":   "invalid_credentials",
		"message": "Invalid password",
	})
}

// Logout handles logout request
func (h *AdminHandler) Logout(c fiber.Ctx) error {
	// Since we use stateless token authentication, logout is mainly handled on the frontend
	// Backend just returns a success response
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logout successful",
	})
}

// GetAuthStatus gets authentication status
func (h *AdminHandler) GetAuthStatus(c fiber.Ctx) error {
	cfg := config.GetConfig()

	return c.JSON(fiber.Map{
		"enabled": cfg.Security.AdminToken != "",
		"message": func() string {
			if cfg.Security.AdminToken == "" {
				return "Admin authentication is not configured"
			}
			return "Admin authentication is enabled"
		}(),
	})
}

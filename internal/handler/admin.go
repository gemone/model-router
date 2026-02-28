package handler

import (
	"net/http"
	"strconv"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/service"
	"github.com/gemone/model-router/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// AdminHandler 管理后台处理器
type AdminHandler struct {
	profileManager *service.ProfileManager
	stats          *service.StatsCollector
}

// NewAdminHandler 创建 Admin 处理器
func NewAdminHandler() *AdminHandler {
	return &AdminHandler{
		profileManager: service.GetProfileManager(),
		stats:          service.GetStatsCollector(),
	}
}

// RegisterRoutes 注册管理路由
func (h *AdminHandler) RegisterRoutes(r fiber.Router) {
	// Profile 管理
	r.Get("/profiles", h.ListProfiles)
	r.Post("/profiles", h.CreateProfile)
	r.Get("/profiles/:id", h.GetProfile)
	r.Put("/profiles/:id", h.UpdateProfile)
	r.Delete("/profiles/:id", h.DeleteProfile)

	// Provider 管理
	r.Get("/providers", h.ListProviders)
	r.Post("/providers", h.CreateProvider)
	r.Get("/providers/:id", h.GetProvider)
	r.Put("/providers/:id", h.UpdateProvider)
	r.Delete("/providers/:id", h.DeleteProvider)

	// Model 管理
	r.Get("/models", h.ListModels)
	r.Post("/models", h.CreateModel)
	r.Get("/models/:id", h.GetModel)
	r.Put("/models/:id", h.UpdateModel)
	r.Delete("/models/:id", h.DeleteModel)

	// Route 管理
	r.Get("/routes", h.ListRoutes)
	r.Post("/routes", h.CreateRoute)
	r.Get("/routes/:id", h.GetRoute)
	r.Put("/routes/:id", h.UpdateRoute)
	r.Delete("/routes/:id", h.DeleteRoute)

	// 统计数据
	r.Get("/stats/dashboard", h.GetDashboardStats)
	r.Get("/stats/provider/:id", h.GetProviderStats)
	r.Get("/stats/model/:name", h.GetModelStats)

	// 日志
	r.Get("/logs", h.GetLogs)

	// 测试
	r.Post("/test", h.TestModel)

	// 模型能力检测
	r.Post("/models/detect-capabilities", h.DetectModelCapabilities)

	// 设置管理
	r.Get("/settings", h.GetSettings)
	r.Put("/settings", h.UpdateSettings)
}

// ==================== Profile 管理 ====================

// ListProfiles 列出所有 Profile
func (h *AdminHandler) ListProfiles(c *fiber.Ctx) error {
	profiles := h.profileManager.GetAllProfiles()
	return c.JSON(profiles)
}

// GetProfile 获取单个 Profile
func (h *AdminHandler) GetProfile(c *fiber.Ctx) error {
	id := c.Params("id")
	profile := h.profileManager.GetProfileByID(id)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}
	return c.JSON(profile.Profile)
}

// CreateProfile 创建 Profile
func (h *AdminHandler) CreateProfile(c *fiber.Ctx) error {
	var profile model.Profile
	if err := c.BodyParser(&profile); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	profile.ID = uuid.New().String()

	if err := h.profileManager.CreateProfile(&profile); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(http.StatusCreated).JSON(profile)
}

// UpdateProfile 更新 Profile
func (h *AdminHandler) UpdateProfile(c *fiber.Ctx) error {
	id := c.Params("id")
	var profile model.Profile
	if err := c.BodyParser(&profile); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	profile.ID = id

	if err := h.profileManager.UpdateProfile(&profile); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(profile)
}

// DeleteProfile 删除 Profile
func (h *AdminHandler) DeleteProfile(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.profileManager.DeleteProfile(id); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(http.StatusNoContent)
}

// ==================== Provider 管理 ====================

// ListProviders 列出 Provider
func (h *AdminHandler) ListProviders(c *fiber.Ctx) error {
	db := database.GetDB()
	var providers []model.Provider
	if err := db.Find(&providers).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 不返回加密的API密钥
	for i := range providers {
		providers[i].APIKey = ""
		providers[i].APIKeyEnc = ""
	}

	return c.JSON(providers)
}

// GetProvider 获取单个 Provider
func (h *AdminHandler) GetProvider(c *fiber.Ctx) error {
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

// CreateProvider 创建 Provider
func (h *AdminHandler) CreateProvider(c *fiber.Ctx) error {
	var req struct {
		model.Provider
		APIKey string `json:"api_key"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	provider := req.Provider
	provider.ID = uuid.New().String()

	// 加密API密钥
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

	// 刷新 Profile
	h.profileManager.RefreshAll()

	provider.APIKey = ""
	provider.APIKeyEnc = ""
	return c.Status(http.StatusCreated).JSON(provider)
}

// UpdateProvider 更新 Provider
func (h *AdminHandler) UpdateProvider(c *fiber.Ctx) error {
	id := c.Params("id")

	var req struct {
		model.Provider
		APIKey string `json:"api_key"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	db := database.GetDB()

	var provider model.Provider
	if err := db.First(&provider, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "provider not found"})
	}

	// 更新字段
	provider.Name = req.Name
	provider.Type = req.Type
	provider.BaseURL = req.BaseURL
	provider.Enabled = req.Enabled
	provider.Priority = req.Priority
	provider.Weight = req.Weight
	provider.RateLimit = req.RateLimit

	// 如果提供了新的API密钥，更新它
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

	// 刷新 Profile
	h.profileManager.RefreshAll()

	provider.APIKey = ""
	provider.APIKeyEnc = ""
	return c.JSON(provider)
}

// DeleteProvider 删除 Provider
func (h *AdminHandler) DeleteProvider(c *fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()

	// 删除关联的模型
	db.Where("provider_id = ?", id).Delete(&model.Model{})

	// 删除 Provider
	if err := db.Delete(&model.Provider{}, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 刷新 Profile
	h.profileManager.RefreshAll()

	return c.SendStatus(http.StatusNoContent)
}

// ==================== Model 管理 ====================

// ListModels 列出 Model
func (h *AdminHandler) ListModels(c *fiber.Ctx) error {
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

// GetModel 获取单个 Model
func (h *AdminHandler) GetModel(c *fiber.Ctx) error {
	id := c.Params("id")
	db := database.GetDB()

	var m model.Model
	if err := db.First(&m, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "model not found"})
	}

	return c.JSON(m)
}

// CreateModel 创建 Model
func (h *AdminHandler) CreateModel(c *fiber.Ctx) error {
	var m model.Model
	if err := c.BodyParser(&m); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	m.ID = uuid.New().String()

	db := database.GetDB()
	if err := db.Create(&m).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 刷新 Profile
	h.profileManager.RefreshAll()

	return c.Status(http.StatusCreated).JSON(m)
}

// UpdateModel 更新 Model
func (h *AdminHandler) UpdateModel(c *fiber.Ctx) error {
	id := c.Params("id")

	var m model.Model
	if err := c.BodyParser(&m); err != nil {
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

	// 刷新 Profile
	h.profileManager.RefreshAll()

	return c.JSON(m)
}

// DeleteModel 删除 Model
func (h *AdminHandler) DeleteModel(c *fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()
	if err := db.Delete(&model.Model{}, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 刷新 Profile
	h.profileManager.RefreshAll()

	return c.SendStatus(http.StatusNoContent)
}

// DetectModelCapabilities 检测模型能力
func (h *AdminHandler) DetectModelCapabilities(c *fiber.Ctx) error {
	var req struct {
		ProviderID string `json:"provider_id"`
		ModelName  string `json:"model_name"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"supports_function": false,
		"supports_vision":   false,
		"message":          "Capabilities detection not implemented yet",
	})
}

// ==================== Route 管理 ====================

// ListRoutes 列出 Route Rules
func (h *AdminHandler) ListRoutes(c *fiber.Ctx) error {
	db := database.GetDB()
	var rules []model.RouteRule

	if err := db.Find(&rules).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(rules)
}

// GetRoute 获取单个 Route
func (h *AdminHandler) GetRoute(c *fiber.Ctx) error {
	id := c.Params("id")
	db := database.GetDB()

	var rule model.RouteRule
	if err := db.First(&rule, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "route not found"})
	}

	return c.JSON(rule)
}

// CreateRoute 创建 Route Rule
func (h *AdminHandler) CreateRoute(c *fiber.Ctx) error {
	var rule model.RouteRule
	if err := c.BodyParser(&rule); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	rule.ID = uuid.New().String()

	db := database.GetDB()
	if err := db.Create(&rule).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 刷新 Profile
	h.profileManager.RefreshAll()

	return c.Status(http.StatusCreated).JSON(rule)
}

// UpdateRoute 更新 Route Rule
func (h *AdminHandler) UpdateRoute(c *fiber.Ctx) error {
	id := c.Params("id")

	var rule model.RouteRule
	if err := c.BodyParser(&rule); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	rule.ID = id

	db := database.GetDB()

	var existing model.RouteRule
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "route not found"})
	}

	if err := db.Save(&rule).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 刷新 Profile
	h.profileManager.RefreshAll()

	return c.JSON(rule)
}

// DeleteRoute 删除 Route Rule
func (h *AdminHandler) DeleteRoute(c *fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()
	if err := db.Delete(&model.RouteRule{}, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 刷新 Profile
	h.profileManager.RefreshAll()

	return c.SendStatus(http.StatusNoContent)
}

// ==================== 统计数据 ====================

// GetDashboardStats 获取仪表盘统计
func (h *AdminHandler) GetDashboardStats(c *fiber.Ctx) error {
	stats := h.stats.GetDashboardStats()

	// 添加活跃模型和供应商数量
	db := database.GetDB()
	var modelCount, providerCount int64
	db.Model(&model.Model{}).Where("enabled = ?", true).Count(&modelCount)
	db.Model(&model.Provider{}).Where("enabled = ?", true).Count(&providerCount)

	stats["active_models"] = modelCount
	stats["active_providers"] = providerCount

	return c.JSON(stats)
}

// GetProviderStats 获取 Provider 统计
func (h *AdminHandler) GetProviderStats(c *fiber.Ctx) error {
	id := c.Params("id")
	stats := h.stats.GetProviderStats(id)
	return c.JSON(stats)
}

// GetModelStats 获取 Model 统计
func (h *AdminHandler) GetModelStats(c *fiber.Ctx) error {
	name := c.Params("name")
	stats := h.stats.GetModelStats(name)
	return c.JSON(stats)
}

// ==================== 日志 ====================

// GetLogs 获取日志
func (h *AdminHandler) GetLogs(c *fiber.Ctx) error {
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

// ==================== 测试 ====================

// TestModel 测试模型
func (h *AdminHandler) TestModel(c *fiber.Ctx) error {
	var req struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	profile := h.profileManager.GetDefaultProfile()
	if profile == nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "no profile available"})
	}

	result, err := profile.TestModel(c.Context(), req.ProviderID, req.Model)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	return c.JSON(result)
}

// ==================== 设置管理 ====================

// SettingsRequest 设置请求结构
type SettingsRequest struct {
	Port           int    `json:"port"`
	Host           string `json:"host"`
	Language       string `json:"language"`
	EnableCORS     bool   `json:"enable_cors"`
	EnableStats    bool   `json:"enable_stats"`
	EnableFallback bool   `json:"enable_fallback"`
	AdminToken     string `json:"admin_token"`
	JWTSecret      string `json:"jwt_secret"`
	LogLevel       string `json:"log_level"`
	MaxRetries     int    `json:"max_retries"`
	DBPath         string `json:"db_path"`
}

// GetSettings 获取系统设置
func (h *AdminHandler) GetSettings(c *fiber.Ctx) error {
	cfg := config.Get()

	settings := fiber.Map{
		"port":            cfg.Port,
		"host":            cfg.Host,
		"language":        "zh-CN",
		"enable_cors":     cfg.EnableCORS,
		"enable_stats":    cfg.EnableStats,
		"enable_fallback": cfg.EnableFallback,
		"admin_token":     "",
		"jwt_secret":      "",
		"log_level":       cfg.LogLevel,
		"max_retries":     cfg.MaxRetries,
		"db_path":         cfg.DBPath,
	}

	return c.JSON(settings)
}

// UpdateSettings 更新系统设置
func (h *AdminHandler) UpdateSettings(c *fiber.Ctx) error {
	var req SettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	cfg := config.Get()

	if req.Port > 0 && req.Port <= 65535 {
		cfg.Port = req.Port
	}
	if req.Host != "" {
		cfg.Host = req.Host
	}
	if req.AdminToken != "" {
		cfg.AdminToken = req.AdminToken
	}
	if req.JWTSecret != "" {
		cfg.JWTSecret = req.JWTSecret
	}
	cfg.EnableCORS = req.EnableCORS
	cfg.EnableStats = req.EnableStats
	cfg.EnableFallback = req.EnableFallback
	if req.LogLevel != "" {
		cfg.LogLevel = req.LogLevel
	}
	if req.MaxRetries >= 0 {
		cfg.MaxRetries = req.MaxRetries
	}
	if req.DBPath != "" {
		cfg.DBPath = req.DBPath
	}

	response := fiber.Map{
		"port":            cfg.Port,
		"host":            cfg.Host,
		"language":        req.Language,
		"enable_cors":     cfg.EnableCORS,
		"enable_stats":    cfg.EnableStats,
		"enable_fallback": cfg.EnableFallback,
		"admin_token":     "",
		"jwt_secret":      "",
		"log_level":       cfg.LogLevel,
		"max_retries":     cfg.MaxRetries,
		"db_path":         cfg.DBPath,
		"message":         "Settings updated. Some changes may require server restart.",
	}

	return c.JSON(response)
}

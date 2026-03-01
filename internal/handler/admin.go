package handler

import (
	"net/http"
	"strconv"
	"strings"

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

	// 统计数据
	r.Get("/stats/dashboard", h.GetDashboardStats)
	r.Get("/stats/trend", h.GetTrendStats)
	r.Get("/stats/all", h.GetAllProviderModelStats)
	r.Get("/stats/provider/:id", h.GetProviderStats)
	r.Get("/stats/model/:name", h.GetModelStats)

	// 日志
	r.Get("/logs", h.GetLogs)
	r.Delete("/logs", h.ClearLogs)

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

	// Get existing profile first
	db := database.GetDB()
	var existingProfile model.Profile
	if err := db.First(&existingProfile, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// Parse only the fields we want to update
	var updates struct {
		Name                *string  `json:"name"`
		Path                *string  `json:"path"`
		Description         *string  `json:"description"`
		Enabled             *bool    `json:"enabled"`
		Priority            *int     `json:"priority"`
		EnableCompression   *bool    `json:"enable_compression"`
		CompressionStrategy *string  `json:"compression_strategy"`
		MaxContextWindow    *int     `json:"max_context_window"`
		ModelIDs            []string `json:"model_ids"`
		FallbackModels      []string `json:"fallback_models"`
	}
	if err := c.BodyParser(&updates); err != nil {
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
	if updates.Priority != nil {
		existingProfile.Priority = *updates.Priority
	}
	if updates.EnableCompression != nil {
		existingProfile.EnableCompression = *updates.EnableCompression
	}
	if updates.CompressionStrategy != nil {
		existingProfile.CompressionStrategy = *updates.CompressionStrategy
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

	if err := h.profileManager.UpdateProfile(&existingProfile); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(existingProfile)
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

	// 获取 Provider 类型以确定能力检测策略
	db := database.GetDB()
	var provider model.Provider
	if err := db.First(&provider, "id = ?", req.ProviderID).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "provider not found"})
	}

	// 基于模型名称和 Provider 类型检测能力
	supportsFunc := detectFunctionCapability(string(provider.Type), req.ModelName)
	supportsVision := detectVisionCapability(string(provider.Type), req.ModelName)

	return c.JSON(fiber.Map{
		"supports_function": supportsFunc,
		"supports_vision":   supportsVision,
		"message":          "Capabilities detected based on model name patterns",
	})
}

// detectFunctionCapability 检测模型是否支持函数调用
func detectFunctionCapability(providerType, modelName string) bool {
	// OpenAI 模型
	if providerType == "openai" || providerType == "azure" || providerType == "openai-compatible" {
		// GPT-4 系列支持函数调用
		if contains(modelName, "gpt-4") || contains(modelName, "gpt-3.5-turbo") || contains(modelName, "gpt-4o") {
			return true
		}
	}

	// Anthropic 模型
	if providerType == "anthropic" {
		// Claude 3 系列支持函数调用
		if contains(modelName, "claude-3") {
			return true
		}
	}

	// DeepSeek 模型
	if providerType == "deepseek" {
		// deepseek-chat 和 deepseek-coder 支持
		if contains(modelName, "deepseek-chat") || contains(modelName, "deepseek-coder") {
			return true
		}
	}

	return false
}

// detectVisionCapability 检测模型是否支持视觉
func detectVisionCapability(providerType, modelName string) bool {
	// OpenAI 模型 - 带 vision 或 v 的版本
	if providerType == "openai" || providerType == "azure" || providerType == "openai-compatible" {
		if contains(modelName, "vision") || contains(modelName, "-4o") || contains(modelName, "-4-turbo") {
			return true
		}
	}

	// Anthropic 模型 - Claude 3 系列都支持视觉
	if providerType == "anthropic" {
		if contains(modelName, "claude-3") {
			return true
		}
	}

	// DeepSeek - VL 模型支持
	if providerType == "deepseek" {
		if contains(modelName, "vl") || contains(modelName, "vision") {
			return true
		}
	}

	// Ollama - 多模态模型
	if providerType == "ollama" {
		if contains(modelName, "llava") || contains(modelName, "vision") || contains(modelName, "mm") {
			return true
		}
	}

	return false
}

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
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

// GetTrendStats 获取趋势统计（最近24小时）
func (h *AdminHandler) GetTrendStats(c *fiber.Ctx) error {
	stats := h.stats.GetTrendStats()
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

// GetAllProviderModelStats 获取所有供应商和模型的详细统计
func (h *AdminHandler) GetAllProviderModelStats(c *fiber.Ctx) error {
	stats := h.stats.GetAllProviderModelStats()
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

// ClearLogs 清空日志
func (h *AdminHandler) ClearLogs(c *fiber.Ctx) error {
	if err := h.stats.ClearLogs(); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(http.StatusNoContent).SendString("")
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

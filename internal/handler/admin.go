package handler

import (
	"net/http"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/service"
	"github.com/gemone/model-router/internal/utils"
	"github.com/gin-gonic/gin"
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
func (h *AdminHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Profile 管理
	r.GET("/profiles", h.ListProfiles)
	r.POST("/profiles", h.CreateProfile)
	r.GET("/profiles/:id", h.GetProfile)
	r.PUT("/profiles/:id", h.UpdateProfile)
	r.DELETE("/profiles/:id", h.DeleteProfile)

	// Provider 管理
	r.GET("/providers", h.ListProviders)
	r.POST("/providers", h.CreateProvider)
	r.GET("/providers/:id", h.GetProvider)
	r.PUT("/providers/:id", h.UpdateProvider)
	r.DELETE("/providers/:id", h.DeleteProvider)

	// Model 管理
	r.GET("/models", h.ListModels)
	r.POST("/models", h.CreateModel)
	r.GET("/models/:id", h.GetModel)
	r.PUT("/models/:id", h.UpdateModel)
	r.DELETE("/models/:id", h.DeleteModel)

	// Route 管理
	r.GET("/routes", h.ListRoutes)
	r.POST("/routes", h.CreateRoute)
	r.GET("/routes/:id", h.GetRoute)
	r.PUT("/routes/:id", h.UpdateRoute)
	r.DELETE("/routes/:id", h.DeleteRoute)

	// 统计数据
	r.GET("/stats/dashboard", h.GetDashboardStats)
	r.GET("/stats/provider/:id", h.GetProviderStats)
	r.GET("/stats/model/:name", h.GetModelStats)

	// 日志
	r.GET("/logs", h.GetLogs)

	// 测试
	r.POST("/test", h.TestModel)
	
	// 模型能力检测
	r.POST("/models/detect-capabilities", h.DetectModelCapabilities)

	// 设置管理
	r.GET("/settings", h.GetSettings)
	r.PUT("/settings", h.UpdateSettings)
}

// ==================== Profile 管理 ====================

// ListProfiles 列出所有 Profile
func (h *AdminHandler) ListProfiles(c *gin.Context) {
	profiles := h.profileManager.GetAllProfiles()
	c.JSON(http.StatusOK, profiles)
}

// GetProfile 获取单个 Profile
func (h *AdminHandler) GetProfile(c *gin.Context) {
	id := c.Param("id")
	profile := h.profileManager.GetProfileByID(id)
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}
	c.JSON(http.StatusOK, profile.Profile)
}

// CreateProfile 创建 Profile
func (h *AdminHandler) CreateProfile(c *gin.Context) {
	var profile model.Profile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	profile.ID = uuid.New().String()
	
	if err := h.profileManager.CreateProfile(&profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, profile)
}

// UpdateProfile 更新 Profile
func (h *AdminHandler) UpdateProfile(c *gin.Context) {
	id := c.Param("id")
	var profile model.Profile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	profile.ID = id
	
	if err := h.profileManager.UpdateProfile(&profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// DeleteProfile 删除 Profile
func (h *AdminHandler) DeleteProfile(c *gin.Context) {
	id := c.Param("id")
	if err := h.profileManager.DeleteProfile(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ==================== Provider 管理 ====================

// ListProviders 列出 Provider
func (h *AdminHandler) ListProviders(c *gin.Context) {
	db := database.GetDB()
	var providers []model.Provider
	if err := db.Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 不返回加密的API密钥
	for i := range providers {
		providers[i].APIKey = ""
		providers[i].APIKeyEnc = ""
	}
	
	c.JSON(http.StatusOK, providers)
}

// GetProvider 获取单个 Provider
func (h *AdminHandler) GetProvider(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()
	
	var provider model.Provider
	if err := db.First(&provider, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
	}
	
	// 不返回加密的API密钥
	provider.APIKey = ""
	provider.APIKeyEnc = ""
	
	c.JSON(http.StatusOK, provider)
}

// CreateProvider 创建 Provider
func (h *AdminHandler) CreateProvider(c *gin.Context) {
	var req struct {
		model.Provider
		APIKey string `json:"api_key"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	provider := req.Provider
	provider.ID = uuid.New().String()
	
	// 加密API密钥
	if req.APIKey != "" {
		encrypted, err := utils.Encrypt(req.APIKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt API key"})
			return
		}
		provider.APIKeyEnc = encrypted
	}
	
	db := database.GetDB()
	if err := db.Create(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	provider.APIKey = ""
	provider.APIKeyEnc = ""
	c.JSON(http.StatusCreated, provider)
}

// UpdateProvider 更新 Provider
func (h *AdminHandler) UpdateProvider(c *gin.Context) {
	id := c.Param("id")
	
	var req struct {
		model.Provider
		APIKey string `json:"api_key"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	db := database.GetDB()
	
	var provider model.Provider
	if err := db.First(&provider, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "provider not found"})
		return
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt API key"})
			return
		}
		provider.APIKeyEnc = encrypted
	}
	
	if err := db.Save(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	provider.APIKey = ""
	provider.APIKeyEnc = ""
	c.JSON(http.StatusOK, provider)
}

// DeleteProvider 删除 Provider
func (h *AdminHandler) DeleteProvider(c *gin.Context) {
	id := c.Param("id")
	
	db := database.GetDB()
	
	// 删除关联的模型
	db.Where("provider_id = ?", id).Delete(&model.Model{})
	
	// 删除 Provider
	if err := db.Delete(&model.Provider{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	c.JSON(http.StatusNoContent, nil)
}

// ==================== Model 管理 ====================

// ListModels 列出 Model
func (h *AdminHandler) ListModels(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, models)
}

// GetModel 获取单个 Model
func (h *AdminHandler) GetModel(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()
	
	var m model.Model
	if err := db.First(&m, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}
	
	c.JSON(http.StatusOK, m)
}

// CreateModel 创建 Model
func (h *AdminHandler) CreateModel(c *gin.Context) {
	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	m.ID = uuid.New().String()
	
	db := database.GetDB()
	if err := db.Create(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	c.JSON(http.StatusCreated, m)
}

// UpdateModel 更新 Model
func (h *AdminHandler) UpdateModel(c *gin.Context) {
	id := c.Param("id")
	
	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m.ID = id
	
	db := database.GetDB()
	
	var existing model.Model
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
		return
	}
	
	if err := db.Save(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	c.JSON(http.StatusOK, m)
}

// DeleteModel 删除 Model
func (h *AdminHandler) DeleteModel(c *gin.Context) {
	id := c.Param("id")
	
	db := database.GetDB()
	if err := db.Delete(&model.Model{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	c.JSON(http.StatusNoContent, nil)
}

// DetectModelCapabilities 检测模型能力
func (h *AdminHandler) DetectModelCapabilities(c *gin.Context) {
	var req struct {
		ProviderID string `json:"provider_id" binding:"required"`
		ModelName  string `json:"model_name" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// 这里可以实现实际的模型能力检测逻辑
	// 例如发送测试请求检测是否支持 function calling 和 vision
	c.JSON(http.StatusOK, gin.H{
		"supports_function": false,
		"supports_vision":   false,
		"message":           "Capabilities detection not implemented yet",
	})
}

// ==================== Route 管理 ====================

// ListRoutes 列出 Route Rules
func (h *AdminHandler) ListRoutes(c *gin.Context) {
	db := database.GetDB()
	var rules []model.RouteRule
	
	if err := db.Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, rules)
}

// GetRoute 获取单个 Route
func (h *AdminHandler) GetRoute(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()
	
	var rule model.RouteRule
	if err := db.First(&rule, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
		return
	}
	
	c.JSON(http.StatusOK, rule)
}

// CreateRoute 创建 Route Rule
func (h *AdminHandler) CreateRoute(c *gin.Context) {
	var rule model.RouteRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	rule.ID = uuid.New().String()
	
	db := database.GetDB()
	if err := db.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	c.JSON(http.StatusCreated, rule)
}

// UpdateRoute 更新 Route Rule
func (h *AdminHandler) UpdateRoute(c *gin.Context) {
	id := c.Param("id")
	
	var rule model.RouteRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.ID = id
	
	db := database.GetDB()
	
	var existing model.RouteRule
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
		return
	}
	
	if err := db.Save(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	c.JSON(http.StatusOK, rule)
}

// DeleteRoute 删除 Route Rule
func (h *AdminHandler) DeleteRoute(c *gin.Context) {
	id := c.Param("id")
	
	db := database.GetDB()
	if err := db.Delete(&model.RouteRule{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 刷新 Profile
	h.profileManager.RefreshAll()
	
	c.JSON(http.StatusNoContent, nil)
}

// ==================== 统计数据 ====================

// GetDashboardStats 获取仪表盘统计
func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	stats := h.stats.GetDashboardStats()
	
	// 添加活跃模型和供应商数量
	db := database.GetDB()
	var modelCount, providerCount int64
	db.Model(&model.Model{}).Where("enabled = ?", true).Count(&modelCount)
	db.Model(&model.Provider{}).Where("enabled = ?", true).Count(&providerCount)
	
	stats["active_models"] = modelCount
	stats["active_providers"] = providerCount
	
	c.JSON(http.StatusOK, stats)
}

// GetProviderStats 获取 Provider 统计
func (h *AdminHandler) GetProviderStats(c *gin.Context) {
	id := c.Param("id")
	stats := h.stats.GetProviderStats(id)
	c.JSON(http.StatusOK, stats)
}

// GetModelStats 获取 Model 统计
func (h *AdminHandler) GetModelStats(c *gin.Context) {
	name := c.Param("name")
	stats := h.stats.GetModelStats(name)
	c.JSON(http.StatusOK, stats)
}

// ==================== 日志 ====================

// GetLogs 获取日志
func (h *AdminHandler) GetLogs(c *gin.Context) {
	page := 1
	pageSize := 50
	
	// 解析分页参数
	if p := c.Query("page"); p != "" {
		// 简单的字符串转int，实际应该使用strconv.Atoi并处理错误
		// 这里简化处理
	}
	if ps := c.Query("page_size"); ps != "" {
		// 同上
	}
	
	logs, total := h.stats.GetRequestLogs(page, pageSize)
	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
	})
}

// ==================== 测试 ====================

// TestModel 测试模型
func (h *AdminHandler) TestModel(c *gin.Context) {
	var req struct {
		ProviderID string `json:"provider_id" binding:"required"`
		Model      string `json:"model" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile := h.profileManager.GetDefaultProfile()
	if profile == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no profile available"})
		return
	}

	result, err := profile.TestModel(c.Request.Context(), req.ProviderID, req.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
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
func (h *AdminHandler) GetSettings(c *gin.Context) {
	cfg := config.Get()

	settings := gin.H{
		"port":            cfg.Port,
		"host":            cfg.Host,
		"language":        "zh-CN", // 默认语言
		"enable_cors":     cfg.EnableCORS,
		"enable_stats":    cfg.EnableStats,
		"enable_fallback": cfg.EnableFallback,
		"admin_token":     "", // 不返回实际的 token
		"jwt_secret":      "", // 不返回实际的 secret
		"log_level":       cfg.LogLevel,
		"max_retries":     cfg.MaxRetries,
		"db_path":         cfg.DBPath,
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings 更新系统设置
func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	var req SettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 注意：这里只更新内存中的配置
	// 实际生产环境应该重启服务或更新环境变量
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

	// 返回更新后的设置（不返回敏感信息）
	response := gin.H{
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

	c.JSON(http.StatusOK, response)
}

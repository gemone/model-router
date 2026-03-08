package handler

import (
	"net/http"

	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RuleAdminHandler 规则管理处理器
type RuleAdminHandler struct{}

// NewRuleAdminHandler 创建规则管理处理器
func NewRuleAdminHandler() *RuleAdminHandler {
	return &RuleAdminHandler{}
}

// RegisterRoutes 注册路由
func (h *RuleAdminHandler) RegisterRoutes(app *fiber.App) {
	// 规则管理 API
	app.Get("/api/admin/rules", h.handleListRules)
	app.Get("/api/admin/rules/:id", h.handleGetRule)
	app.Get("/api/admin/profiles/:profileId/rules", h.handleListRulesByProfile)
	app.Post("/api/admin/rules", h.handleCreateRule)
	app.Put("/api/admin/rules/:id", h.handleUpdateRule)
	app.Delete("/api/admin/rules/:id", h.handleDeleteRule)
	app.Put("/api/admin/rules/:id/enable", h.handleEnableRule)
	app.Put("/api/admin/rules/:id/disable", h.handleDisableRule)
}

// handleListRules 获取所有规则
func (h *RuleAdminHandler) handleListRules(c *fiber.Ctx) error {
	db := database.GetDB()

	var rules []model.Rule
	if err := db.Order("priority desc").Find(&rules).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch rules",
		})
	}

	return c.JSON(rules)
}

// handleGetRule 获取单个规则
func (h *RuleAdminHandler) handleGetRule(c *fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()

	var rule model.Rule
	if err := db.First(&rule, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "Rule not found",
		})
	}

	return c.JSON(rule)
}

// handleListRulesByProfile 获取 Profile 的规则
func (h *RuleAdminHandler) handleListRulesByProfile(c *fiber.Ctx) error {
	profileID := c.Params("profileId")

	db := database.GetDB()

	var rules []model.Rule
	if err := db.Where("profile_id = ?", profileID).Order("priority desc").Find(&rules).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch rules",
		})
	}

	return c.JSON(rules)
}

// RuleCreateRequest 创建规则请求
type RuleCreateRequest struct {
	ProfileID   string              `json:"profile_id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Priority    int                 `json:"priority"`
	Enabled     bool                `json:"enabled"`
	Conditions  model.RuleConditions `json:"conditions"`
	Action      model.RuleAction     `json:"action"`
}

// handleCreateRule 创建规则
func (h *RuleAdminHandler) handleCreateRule(c *fiber.Ctx) error {
	var req RuleCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// 验证必填字段
	if req.Name == "" || req.ProfileID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Name and profile_id are required",
		})
	}

	// 检查 Profile 是否存在
	db := database.GetDB()
	var profile model.Profile
	if err := db.First(&profile, "id = ?", req.ProfileID).Error; err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Profile not found",
		})
	}

	// 创建规则
	rule := &model.Rule{
		ID:          uuid.New().String(),
		ProfileID:   req.ProfileID,
		Name:        req.Name,
		Description: req.Description,
		Priority:    req.Priority,
		Enabled:     req.Enabled,
		Conditions:  req.Conditions,
		Action:      req.Action,
	}

	if err := db.Create(rule).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create rule",
		})
	}

	return c.Status(http.StatusCreated).JSON(rule)
}

// handleUpdateRule 更新规则
func (h *RuleAdminHandler) handleUpdateRule(c *fiber.Ctx) error {
	id := c.Params("id")

	var req RuleCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	db := database.GetDB()

	var rule model.Rule
	if err := db.First(&rule, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "Rule not found",
		})
	}

	// 更新字段
	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	rule.Priority = req.Priority
	rule.Enabled = req.Enabled
	if req.Conditions != nil {
		rule.Conditions = req.Conditions
	}
	if req.Action.Type != "" {
		rule.Action = req.Action
	}

	if err := db.Save(&rule).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update rule",
		})
	}

	return c.JSON(rule)
}

// handleDeleteRule 删除规则
func (h *RuleAdminHandler) handleDeleteRule(c *fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()

	var rule model.Rule
	if err := db.First(&rule, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "Rule not found",
		})
	}

	if err := db.Delete(&rule).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete rule",
		})
	}

	return c.JSON(fiber.Map{"success": true})
}

// handleEnableRule 启用规则
func (h *RuleAdminHandler) handleEnableRule(c *fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()

	var rule model.Rule
	if err := db.First(&rule, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "Rule not found",
		})
	}

	rule.Enabled = true
	if err := db.Save(&rule).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to enable rule",
		})
	}

	return c.JSON(rule)
}

// handleDisableRule 禁用规则
func (h *RuleAdminHandler) handleDisableRule(c *fiber.Ctx) error {
	id := c.Params("id")

	db := database.GetDB()

	var rule model.Rule
	if err := db.First(&rule, "id = ?", id).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"error": "Rule not found",
		})
	}

	rule.Enabled = false
	if err := db.Save(&rule).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to disable rule",
		})
	}

	return c.JSON(rule)
}

// RuleTestRequest 规则测试请求
type RuleTestRequest struct {
	Input model.RuleEngineInput `json:"input"`
}

// RuleTestResponse 规则测试响应
type RuleTestResponse struct {
	Matched  bool              `json:"matched"`
	RuleID   string            `json:"rule_id,omitempty"`
	RuleName string            `json:"rule_name,omitempty"`
	Action   model.RuleAction  `json:"action,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

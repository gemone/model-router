package template

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"github.com/gemone/model-router/internal/model"
	"gorm.io/gorm"
)

// Service 提供模板管理功能
type Service struct {
	db        *gorm.DB
	cache     map[string]*model.PromptTemplate // 内存缓存
	cacheMu   sync.RWMutex
	parsedTpl map[string]*template.Template    // 解析后的模板缓存
}

// NewService 创建模板服务
func NewService(db *gorm.DB) *Service {
	s := &Service{
		db:        db,
		cache:     make(map[string]*model.PromptTemplate),
		parsedTpl: make(map[string]*template.Template),
	}
	return s
}

// InitDefaultTemplates 初始化默认模板到数据库
func (s *Service) InitDefaultTemplates() error {
	defaults := model.DefaultTemplates()
	
	for _, tpl := range defaults {
		var existing model.PromptTemplate
		err := s.db.Where("name = ? AND scope = ? AND profile_id = ?", 
			tpl.Name, tpl.Scope, "").First(&existing).Error
		
		if err == gorm.ErrRecordNotFound {
			// 创建默认模板
			if err := s.db.Create(&tpl).Error; err != nil {
				return fmt.Errorf("failed to create default template %s: %w", tpl.Name, err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to check template %s: %w", tpl.Name, err)
		}
		// 已存在则跳过，保留用户自定义
	}
	
	return nil
}

// GetTemplate 获取模板（优先从缓存，其次数据库）
func (s *Service) GetTemplate(name string, profileID string) (*model.PromptTemplate, error) {
	cacheKey := s.cacheKey(name, profileID)
	
	// 1. 先检查缓存
	s.cacheMu.RLock()
	if tpl, ok := s.cache[cacheKey]; ok && tpl.Enabled {
		s.cacheMu.RUnlock()
		return tpl, nil
	}
	s.cacheMu.RUnlock()
	
	// 2. 尝试从数据库获取 Profile 级别的模板
	if profileID != "" {
		var tpl model.PromptTemplate
		err := s.db.Where("name = ? AND scope = ? AND profile_id = ? AND enabled = ?", 
			name, model.TemplateScopeProfile, profileID, true).First(&tpl).Error
		if err == nil {
			s.cacheMu.Lock()
			s.cache[cacheKey] = &tpl
			s.cacheMu.Unlock()
			return &tpl, nil
		}
		// 如果找不到 Profile 级别的，继续查找全局模板
	}
	
	// 3. 获取全局模板
	var tpl model.PromptTemplate
	err := s.db.Where("name = ? AND scope = ? AND enabled = ?", 
		name, model.TemplateScopeGlobal, true).First(&tpl).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("template not found: %s", name)
		}
		return nil, err
	}
	
	// 缓存结果
	s.cacheMu.Lock()
	s.cache[cacheKey] = &tpl
	s.cacheMu.Unlock()
	
	return &tpl, nil
}

// Render 渲染模板
func (s *Service) Render(name string, profileID string, variables map[string]interface{}) (string, error) {
	tpl, err := s.GetTemplate(name, profileID)
	if err != nil {
		return "", err
	}
	
	return s.RenderTemplate(tpl, variables)
}

// validateTemplateContent validates template content for security
// Prevents template injection by blocking dangerous functions and limiting complexity
func validateTemplateContent(content string) error {
	// Check for potentially dangerous template constructs
	dangerousPatterns := []string{
		`{{.Env.`,
		`{{.System.`,
		`{{.Exec.`,
		`{{index .`,
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(content, pattern) {
			return fmt.Errorf("template contains forbidden pattern: %s", pattern)
		}
	}

	// Limit template complexity to prevent DoS
	const maxTemplateSize = 100000 // 100KB limit
	if len(content) > maxTemplateSize {
		return fmt.Errorf("template content too large (max %d bytes)", maxTemplateSize)
	}

	// Check for reasonable template syntax
	// Only allow safe template constructs
	if strings.Contains(content, "{{range") && !strings.Contains(content, "{{end") {
		return fmt.Errorf("template has unclosed range construct")
	}
	if strings.Contains(content, "{{if") && !strings.Contains(content, "{{end") {
		return fmt.Errorf("template has unclosed if construct")
	}

	return nil
}

// RenderTemplate 渲染指定模板
func (s *Service) RenderTemplate(tpl *model.PromptTemplate, variables map[string]interface{}) (string, error) {
	if tpl == nil {
		return "", fmt.Errorf("template is nil")
	}

	// Validate template content before parsing
	if err := validateTemplateContent(tpl.Content); err != nil {
		return "", fmt.Errorf("template validation failed: %w", err)
	}

	// 获取或解析模板
	cacheKey := fmt.Sprintf("%s:v%d", tpl.ID, tpl.Version)
	
	s.cacheMu.RLock()
	parsedTpl, ok := s.parsedTpl[cacheKey]
	s.cacheMu.RUnlock()
	
	if !ok {
		// 解析模板
		var err error
		parsedTpl, err = template.New(tpl.Name).Parse(tpl.Content)
		if err != nil {
			return "", fmt.Errorf("failed to parse template %s: %w", tpl.Name, err)
		}
		
		s.cacheMu.Lock()
		s.parsedTpl[cacheKey] = parsedTpl
		s.cacheMu.Unlock()
	}
	
	// 执行模板
	var buf bytes.Buffer
	if err := parsedTpl.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("failed to render template %s: %w", tpl.Name, err)
	}
	
	return buf.String(), nil
}

// RenderWithDefault 渲染模板，如果失败则返回默认值
func (s *Service) RenderWithDefault(name string, profileID string, variables map[string]interface{}, defaultValue string) string {
	result, err := s.Render(name, profileID, variables)
	if err != nil {
		return defaultValue
	}
	return result
}

// CreateOrUpdateTemplate 创建或更新模板
func (s *Service) CreateOrUpdateTemplate(tpl *model.PromptTemplate) error {
	if tpl.ID == "" {
		// 创建新模板
		return s.db.Create(tpl).Error
	}
	
	// 更新现有模板
	if err := s.db.Save(tpl).Error; err != nil {
		return err
	}
	
	// 清除缓存
	s.invalidateCache(tpl.Name, tpl.ProfileID)
	
	return nil
}

// DeleteTemplate 删除模板
func (s *Service) DeleteTemplate(id string) error {
	var tpl model.PromptTemplate
	if err := s.db.First(&tpl, "id = ?", id).Error; err != nil {
		return err
	}
	
	// 不允许删除默认模板
	if tpl.IsDefault {
		return fmt.Errorf("cannot delete default template")
	}
	
	if err := s.db.Delete(&tpl).Error; err != nil {
		return err
	}
	
	// 清除缓存
	s.invalidateCache(tpl.Name, tpl.ProfileID)
	
	return nil
}

// ListTemplates 列出模板
func (s *Service) ListTemplates(category string, scope string, profileID string) ([]model.PromptTemplate, error) {
	var templates []model.PromptTemplate
	query := s.db.Model(&model.PromptTemplate{})
	
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if scope != "" {
		query = query.Where("scope = ?", scope)
	}
	if profileID != "" {
		query = query.Where("profile_id = ?", profileID)
	}
	
	if err := query.Order("category, name").Find(&templates).Error; err != nil {
		return nil, err
	}
	
	return templates, nil
}

// GetTemplateByID 根据 ID 获取模板
func (s *Service) GetTemplateByID(id string) (*model.PromptTemplate, error) {
	var tpl model.PromptTemplate
	if err := s.db.First(&tpl, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &tpl, nil
}

// ResetToDefault 重置模板为默认值
func (s *Service) ResetToDefault(name string, profileID string) error {
	defaults := model.DefaultTemplates()
	
	for _, defaultTpl := range defaults {
		if defaultTpl.Name == name {
			// 删除自定义模板，恢复默认
			if profileID != "" {
				s.db.Where("name = ? AND profile_id = ?", name, profileID).Delete(&model.PromptTemplate{})
			} else {
				s.db.Where("name = ? AND scope = ?", name, model.TemplateScopeGlobal).Delete(&model.PromptTemplate{})
			}
			
			// 清除缓存
			s.invalidateCache(name, profileID)
			return nil
		}
	}
	
	return fmt.Errorf("default template not found: %s", name)
}

// invalidateCache 清除模板缓存
func (s *Service) invalidateCache(name string, profileID string) {
	cacheKey := s.cacheKey(name, profileID)
	
	s.cacheMu.Lock()
	delete(s.cache, cacheKey)
	s.cacheMu.Unlock()
}

// ClearCache 清除所有缓存
func (s *Service) ClearCache() {
	s.cacheMu.Lock()
	s.cache = make(map[string]*model.PromptTemplate)
	s.parsedTpl = make(map[string]*template.Template)
	s.cacheMu.Unlock()
}

// cacheKey 生成缓存键
func (s *Service) cacheKey(name string, profileID string) string {
	if profileID == "" {
		return fmt.Sprintf("global:%s", name)
	}
	return fmt.Sprintf("profile:%s:%s", profileID, name)
}

// RenderHelper 提供便捷的模板渲染方法
type RenderHelper struct {
	service   *Service
	profileID string
}

// NewRenderHelper 创建渲染助手
func (s *Service) NewRenderHelper(profileID string) *RenderHelper {
	return &RenderHelper{
		service:   s,
		profileID: profileID,
	}
}

// Render 使用助手的 profileID 渲染模板
func (h *RenderHelper) Render(name string, variables map[string]interface{}) (string, error) {
	return h.service.Render(name, h.profileID, variables)
}

// RenderWithDefault 渲染模板，失败返回默认值
func (h *RenderHelper) RenderWithDefault(name string, variables map[string]interface{}, defaultValue string) string {
	return h.service.RenderWithDefault(name, h.profileID, variables, defaultValue)
}

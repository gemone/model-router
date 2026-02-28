package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
)

// ProfileInstance Profile 实例
type ProfileInstance struct {
	sync.RWMutex
	Profile     *model.Profile
	adapters    map[string]adapter.Adapter // providerID -> adapter
	providerMap map[string]*model.Provider // providerID -> provider
	modelMap    map[string][]*model.Model  // modelName -> models
	routeRules  []model.RouteRule
	stats       *StatsCollector
}

// ProfileManager Profile 管理器
type ProfileManager struct {
	sync.RWMutex
	profiles       map[string]*ProfileInstance // path -> profile
	defaultProfile string
}

// RouteResult 路由结果
type RouteResult struct {
	Adapter      adapter.Adapter
	Model        *model.Model
	Provider     *model.Provider
	Profile      *model.Profile
	FallbackUsed bool
}

var (
	profileManager     *ProfileManager
	profileManagerOnce sync.Once
)

// GetProfileManager 获取 ProfileManager 单例
func GetProfileManager() *ProfileManager {
	profileManagerOnce.Do(func() {
		profileManager = &ProfileManager{
			profiles: make(map[string]*ProfileInstance),
		}
		profileManager.LoadFromDB()
	})
	return profileManager
}

// LoadFromDB 从数据库加载所有 Profile
func (pm *ProfileManager) LoadFromDB() error {
	pm.Lock()
	defer pm.Unlock()

	db := database.GetDB()

	// 加载所有 Profile
	var profiles []model.Profile
	if err := db.Find(&profiles).Error; err != nil {
		return err
	}

	// 如果没有 Profile，创建默认 Profile
	if len(profiles) == 0 {
		defaultProfile := &model.Profile{
			ID:       "default",
			Name:     "Default",
			Path:     "default",
			Enabled:  true,
			Priority: 0,
		}
		db.Create(defaultProfile)
		profiles = append(profiles, *defaultProfile)
	}

	for i := range profiles {
		p := &profiles[i]
		if p.Enabled {
			instance := &ProfileInstance{
				Profile:     p,
				adapters:    make(map[string]adapter.Adapter),
				providerMap: make(map[string]*model.Provider),
				modelMap:    make(map[string][]*model.Model),
				stats:       GetStatsCollector(),
			}
			instance.loadData()
			pm.profiles[p.Path] = instance

			// 记录默认 Profile（优先级最高的）
			if pm.defaultProfile == "" || p.Priority > pm.profiles[pm.defaultProfile].Profile.Priority {
				pm.defaultProfile = p.Path
			}
		}
	}

	return nil
}

// loadData 加载 Profile 数据
func (pi *ProfileInstance) loadData() error {
	db := database.GetDB()

	// 加载该 Profile 的路由规则
	db.Where("profile_id = ?", pi.Profile.ID).Find(&pi.routeRules)

	// 加载所有供应商及其模型
	var providers []model.Provider
	db.Preload("Models", "profile_id = ?", pi.Profile.ID).Find(&providers)

	for i := range providers {
		p := &providers[i]
		pi.providerMap[p.ID] = p

		// 创建适配器
		if adp := adapter.Create(p.Type); adp != nil {
			if err := adp.Init(p); err == nil {
				pi.adapters[p.ID] = adp
			}
		}

		// 建立模型映射（只添加属于该 Profile 的模型）
		for j := range p.Models {
			m := &p.Models[j]
			if m.Enabled && m.ProfileID == pi.Profile.ID {
				pi.modelMap[m.Name] = append(pi.modelMap[m.Name], m)
			}
		}
	}

	return nil
}

// GetProfile 通过路径获取 Profile
func (pm *ProfileManager) GetProfile(path string) *ProfileInstance {
	pm.RLock()
	defer pm.RUnlock()

	// 直接匹配
	if pi, ok := pm.profiles[path]; ok {
		return pi
	}

	// 尝试匹配前缀 /api/{profile}/...
	parts := strings.Split(path, "/")
	for i := len(parts); i > 0; i-- {
		checkPath := strings.Join(parts[:i], "/")
		if pi, ok := pm.profiles[checkPath]; ok {
			return pi
		}
	}

	// 返回默认 Profile
	if pi, ok := pm.profiles[pm.defaultProfile]; ok {
		return pi
	}

	return nil
}

// GetProfileByID 通过 ID 获取 Profile
func (pm *ProfileManager) GetProfileByID(id string) *ProfileInstance {
	pm.RLock()
	defer pm.RUnlock()

	for _, pi := range pm.profiles {
		if pi.Profile.ID == id {
			return pi
		}
	}
	return nil
}

// GetAllProfiles 获取所有 Profile
func (pm *ProfileManager) GetAllProfiles() []*model.Profile {
	pm.RLock()
	defer pm.RUnlock()

	profiles := make([]*model.Profile, 0, len(pm.profiles))
	for _, pi := range pm.profiles {
		profiles = append(profiles, pi.Profile)
	}
	return profiles
}

// GetDefaultProfile 获取默认 Profile
func (pm *ProfileManager) GetDefaultProfile() *ProfileInstance {
	pm.RLock()
	defer pm.RUnlock()

	if pi, ok := pm.profiles[pm.defaultProfile]; ok {
		return pi
	}
	return nil
}

// CreateProfile 创建新 Profile
func (pm *ProfileManager) CreateProfile(p *model.Profile) error {
	pm.Lock()
	defer pm.Unlock()

	// 验证 path 格式
	if !isValidPath(p.Path) {
		return fmt.Errorf("invalid profile path: %s", p.Path)
	}

	// 检查路径是否已存在
	if _, ok := pm.profiles[p.Path]; ok {
		return fmt.Errorf("profile path already exists: %s", p.Path)
	}

	db := database.GetDB()
	if err := db.Create(p).Error; err != nil {
		return err
	}

	instance := &ProfileInstance{
		Profile:     p,
		adapters:    make(map[string]adapter.Adapter),
		providerMap: make(map[string]*model.Provider),
		modelMap:    make(map[string][]*model.Model),
		stats:       GetStatsCollector(),
	}
	pm.profiles[p.Path] = instance

	return nil
}

// UpdateProfile 更新 Profile
func (pm *ProfileManager) UpdateProfile(p *model.Profile) error {
	pm.Lock()
	defer pm.Unlock()

	// 获取旧 profile 以检查 path 是否变更
	var oldProfile model.Profile
	db := database.GetDB()
	if err := db.First(&oldProfile, "id = ?", p.ID).Error; err != nil {
		return err
	}

	// 保存新数据
	if err := db.Save(p).Error; err != nil {
		return err
	}

	// 如果 path 变更，需要更新 map 的 key
	if oldProfile.Path != p.Path {
		if instance, ok := pm.profiles[oldProfile.Path]; ok {
			delete(pm.profiles, oldProfile.Path)
			pm.profiles[p.Path] = instance
		}
	}

	// 更新内存中的数据
	if instance, ok := pm.profiles[p.Path]; ok {
		instance.Profile = p
	}

	return nil
}

// DeleteProfile 删除 Profile
func (pm *ProfileManager) DeleteProfile(path string) error {
	pm.Lock()
	defer pm.Unlock()

	if path == pm.defaultProfile {
		return fmt.Errorf("cannot delete default profile")
	}

	instance, ok := pm.profiles[path]
	if !ok {
		return fmt.Errorf("profile not found: %s", path)
	}

	db := database.GetDB()
	if err := db.Delete(instance.Profile).Error; err != nil {
		return err
	}

	delete(pm.profiles, path)
	return nil
}

// Refresh 刷新指定 Profile
func (pm *ProfileManager) Refresh(path string) error {
	pm.Lock()
	defer pm.Unlock()

	instance, ok := pm.profiles[path]
	if !ok {
		return fmt.Errorf("profile not found: %s", path)
	}

	// 清空并重新加载
	instance.Lock()
	instance.adapters = make(map[string]adapter.Adapter)
	instance.providerMap = make(map[string]*model.Provider)
	instance.modelMap = make(map[string][]*model.Model)
	instance.routeRules = nil
	instance.Unlock()

	return instance.loadData()
}

// RefreshAll 刷新所有 Profile
func (pm *ProfileManager) RefreshAll() error {
	pm.Lock()
	defer pm.Unlock()

	// 清空现有
	pm.profiles = make(map[string]*ProfileInstance)

	return pm.LoadFromDB()
}

// ==================== ProfileInstance 方法 ====================

// Route 根据模型名称路由
func (pi *ProfileInstance) Route(ctx context.Context, modelName string) (*RouteResult, error) {
	pi.RLock()
	defer pi.RUnlock()

	// 1. 首先检查是否有匹配的路由规则
	for _, rule := range pi.routeRules {
		if !matchPattern(modelName, rule.ModelPattern) {
			continue
		}

		targetModel := pi.selectModelByStrategy(&rule, modelName)
		if targetModel != nil {
			provider, adapter := pi.getProviderAndAdapter(targetModel.ProviderID)
			if provider != nil && adapter != nil && provider.Enabled {
				return &RouteResult{
					Adapter:      adapter,
					Model:        targetModel,
					Provider:     provider,
					Profile:      pi.Profile,
					FallbackUsed: false,
				}, nil
			}
		}

		// 尝试 fallback
		if rule.FallbackEnabled && len(rule.FallbackModels) > 0 {
			for _, fallbackModelName := range rule.FallbackModels {
				if models, ok := pi.modelMap[fallbackModelName]; ok && len(models) > 0 {
					for _, m := range models {
						provider, adapter := pi.getProviderAndAdapter(m.ProviderID)
						if provider != nil && adapter != nil && provider.Enabled {
							return &RouteResult{
								Adapter:      adapter,
								Model:        m,
								Provider:     provider,
								Profile:      pi.Profile,
								FallbackUsed: true,
							}, nil
						}
					}
				}
			}
		}
	}

	// 2. 直接查找模型
	if models, ok := pi.modelMap[modelName]; ok {
		selected := pi.selectBestModel(models)
		if selected != nil {
			provider, adapter := pi.getProviderAndAdapter(selected.ProviderID)
			if provider != nil && adapter != nil && provider.Enabled {
				return &RouteResult{
					Adapter:      adapter,
					Model:        selected,
					Provider:     provider,
					Profile:      pi.Profile,
					FallbackUsed: false,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no available provider for model: %s in profile: %s", modelName, pi.Profile.Path)
}

// RouteWithFallback 带降级策略的路由
func (pi *ProfileInstance) RouteWithFallback(ctx context.Context, modelName string) (*RouteResult, error) {
	result, err := pi.Route(ctx, modelName)
	if err != nil {
		return nil, err
	}

	if pi.shouldFallback(result.Provider.ID, result.Model.Name) {
		fallbackResult := pi.tryFallback(modelName)
		if fallbackResult != nil {
			return fallbackResult, nil
		}
	}

	return result, nil
}

// selectModelByStrategy 根据策略选择模型
func (pi *ProfileInstance) selectModelByStrategy(rule *model.RouteRule, modelName string) *model.Model {
	switch rule.Strategy {
	case model.RouteStrategyPriority:
		return pi.selectByPriority(rule.TargetModels)
	case model.RouteStrategyWeighted:
		return pi.selectByWeight(rule.TargetModels)
	case model.RouteStrategyAuto:
		return pi.selectByHealth(rule.TargetModels)
	default:
		return pi.selectByPriority(rule.TargetModels)
	}
}

// selectByPriority 按优先级选择
func (pi *ProfileInstance) selectByPriority(modelNames []string) *model.Model {
	var bestModel *model.Model
	bestPriority := -1

	for _, name := range modelNames {
		if models, ok := pi.modelMap[name]; ok {
			for _, m := range models {
				provider := pi.providerMap[m.ProviderID]
				if provider != nil && provider.Priority > bestPriority {
					bestPriority = provider.Priority
					bestModel = m
				}
			}
		}
	}

	return bestModel
}

// selectByWeight 按权重选择
func (pi *ProfileInstance) selectByWeight(modelNames []string) *model.Model {
	type weightedModel struct {
		model  *model.Model
		weight int
	}

	var weightedModels []weightedModel
	totalWeight := 0

	for _, name := range modelNames {
		if models, ok := pi.modelMap[name]; ok {
			for _, m := range models {
				provider := pi.providerMap[m.ProviderID]
				if provider != nil {
					weightedModels = append(weightedModels, weightedModel{m, provider.Weight})
					totalWeight += provider.Weight
				}
			}
		}
	}

	if len(weightedModels) == 0 {
		return nil
	}

	randVal := time.Now().UnixNano() % int64(totalWeight)
	for _, wm := range weightedModels {
		randVal -= int64(wm.weight)
		if randVal < 0 {
			return wm.model
		}
	}

	return weightedModels[0].model
}

// selectByHealth 按健康度选择
func (pi *ProfileInstance) selectByHealth(modelNames []string) *model.Model {
	var bestModel *model.Model
	bestScore := -1.0

	for _, name := range modelNames {
		if models, ok := pi.modelMap[name]; ok {
			for _, m := range models {
				score := pi.stats.GetHealthScore(m.ProviderID, m.Name)
				if score > bestScore {
					bestScore = score
					bestModel = m
				}
			}
		}
	}

	return bestModel
}

// selectBestModel 选择最佳模型
func (pi *ProfileInstance) selectBestModel(models []*model.Model) *model.Model {
	if len(models) == 0 {
		return nil
	}
	if len(models) == 1 {
		return models[0]
	}

	var best *model.Model
	bestPriority := -1
	bestHealthScore := -1.0

	for _, m := range models {
		provider := pi.providerMap[m.ProviderID]
		if provider == nil {
			continue
		}

		healthScore := pi.stats.GetHealthScore(m.ProviderID, m.Name)

		if provider.Priority > bestPriority ||
			(provider.Priority == bestPriority && healthScore > bestHealthScore) {
			best = m
			bestPriority = provider.Priority
			bestHealthScore = healthScore
		}
	}

	return best
}

// tryFallback 尝试 fallback
func (pi *ProfileInstance) tryFallback(modelName string) *RouteResult {
	for _, rule := range pi.routeRules {
		if matchPattern(modelName, rule.ModelPattern) && rule.FallbackEnabled {
			for _, fallbackName := range rule.FallbackModels {
				if models, ok := pi.modelMap[fallbackName]; ok {
					for _, m := range models {
						provider, adapter := pi.getProviderAndAdapter(m.ProviderID)
						if provider != nil && adapter != nil && provider.Enabled {
							return &RouteResult{
								Adapter:      adapter,
								Model:        m,
								Provider:     provider,
								Profile:      pi.Profile,
								FallbackUsed: true,
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// shouldFallback 判断是否应当 fallback
func (pi *ProfileInstance) shouldFallback(providerID, modelName string) bool {
	cfg := config.Get()
	if !cfg.EnableFallback {
		return false
	}

	errorRate := pi.stats.GetErrorRate(providerID, modelName, time.Minute)
	if errorRate > 0.5 {
		return true
	}

	avgLatency := pi.stats.GetAvgLatency(providerID, modelName, time.Minute)
	if avgLatency > 10000 {
		return true
	}

	currentRPM := pi.stats.GetCurrentRPM(providerID, modelName)
	provider := pi.providerMap[providerID]
	if provider != nil && provider.RateLimit > 0 && currentRPM >= int64(provider.RateLimit) {
		return true
	}

	return false
}

// getProviderAndAdapter 获取供应商和适配器
func (pi *ProfileInstance) getProviderAndAdapter(providerID string) (*model.Provider, adapter.Adapter) {
	provider, ok := pi.providerMap[providerID]
	if !ok {
		return nil, nil
	}
	adp, ok := pi.adapters[providerID]
	if !ok {
		return nil, nil
	}
	return provider, adp
}

// GetModels 获取所有模型
func (pi *ProfileInstance) GetModels() []*model.Model {
	pi.RLock()
	defer pi.RUnlock()

	modelSet := make(map[string]*model.Model)
	for _, models := range pi.modelMap {
		for _, m := range models {
			modelSet[m.ID] = m
		}
	}

	result := make([]*model.Model, 0, len(modelSet))
	for _, m := range modelSet {
		result = append(result, m)
	}
	return result
}

// GetProviders 获取所有供应商
func (pi *ProfileInstance) GetProviders() []*model.Provider {
	pi.RLock()
	defer pi.RUnlock()

	result := make([]*model.Provider, 0, len(pi.providerMap))
	for _, p := range pi.providerMap {
		result = append(result, p)
	}
	return result
}

// TestModel 测试模型
func (pi *ProfileInstance) TestModel(ctx context.Context, providerID, modelName string) (*model.TestResult, error) {
	pi.RLock()
	provider, adp := pi.getProviderAndAdapter(providerID)
	pi.RUnlock()

	if provider == nil || adp == nil {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	pi.RLock()
	var targetModel *model.Model
	for _, m := range pi.modelMap[modelName] {
		if m.ProviderID == providerID {
			targetModel = m
			break
		}
	}
	pi.RUnlock()

	if targetModel == nil {
		return nil, fmt.Errorf("model not found: %s", modelName)
	}

	req := &model.ChatCompletionRequest{
		Model: modelName,
		Messages: []model.Message{
			{Role: "user", Content: "Hello, this is a test message. Please respond with 'OK'."},
		},
		MaxTokens: 50,
	}

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

	db := database.GetDB()
	db.Create(testResult)

	return testResult, nil
}

// 辅助函数

func isValidPath(path string) bool {
	if path == "" {
		return false
	}
	// 只允许字母、数字、连字符和下划线
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", path)
	return matched
}

func matchPattern(modelName, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		regex := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, `.*`)
		matched, _ := regexp.MatchString("^"+regex+"$", modelName)
		return matched
	}
	return modelName == pattern
}

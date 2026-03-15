package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/adapter"
	compressionpkg "github.com/gemone/model-router/internal/compression"
	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/middleware"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/repository"
	compressionservice "github.com/gemone/model-router/internal/service/compression"
)

// pkgRand is a package-level random source seeded once at initialization
// This ensures proper randomness across server restarts unlike the global rand
var pkgRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// ruleTargetModel 规则目标模型（内部使用）
type ruleTargetModel struct {
	model    *model.Model
	weight   int
	priority int
}

// ProfileInstance Profile 实例
type ProfileInstance struct {
	sync.RWMutex
	Profile     *model.Profile
	adapters    map[string]adapter.Adapter // providerID -> adapter
	providerMap map[string]*model.Provider // providerID -> provider
	modelMap    map[string][]*model.Model  // modelName -> models
	routeMap    map[string]*RouteInstance  // routeID -> route (NEW: route-based access)
	stats       *StatsCollector
	// DEPRECATED: Use compressionService instead (kept for backward compatibility during v3 refactoring)
	compressionPipeline *compressionpkg.CompressionPipeline
	// DEPRECATED: Use compressionService.GetSelector() instead (kept for backward compatibility)
	CompressionSelector *repository.CompressionGroupSelector
	// NEW: Independent compression service
	compressionService compressionservice.Service
	compositeModels    map[string]*model.CompositeAutoModel // compositeModelName -> composite model
	compositeService   *CompositeService                    // Composite service for routing
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
	return pm.loadFromDB_unlocked()
}

// loadFromDB_unlocked 内部方法，不加锁版本
func (pm *ProfileManager) loadFromDB_unlocked() error {
	db := database.GetDB()

	// 加载所有 Profile
	var profiles []model.Profile
	if err := db.Find(&profiles).Error; err != nil {
		return err
	}

	// 如果没有 Profile，创建默认 Profile
	if len(profiles) == 0 {
		defaultProfile := &model.Profile{
			ID:      "default",
			Name:    "Default",
			Path:    "default",
			Enabled: true,
		}
		db.Create(defaultProfile)
		profiles = append(profiles, *defaultProfile)
	}

	for i := range profiles {
		p := &profiles[i]
		if p.Enabled {
			instance := &ProfileInstance{
				Profile:             p,
				adapters:            make(map[string]adapter.Adapter),
				providerMap:         make(map[string]*model.Provider),
				modelMap:            make(map[string][]*model.Model),
				stats:               GetStatsCollector(),
				compressionPipeline: compressionpkg.NewPipeline(), // DEPRECATED: kept for backward compatibility
			}
			instance.loadData()
			instance.initCompression()
			pm.profiles[p.Path] = instance

			// 设置默认 Profile（第一个启用的 Profile）
			if pm.defaultProfile == "" {
				pm.defaultProfile = p.Path
			}
		}
	}

	return nil
}

// isValidUUID 检查字符串是否是有效的 UUID 格式
func isValidUUID(s string) bool {
	// UUID 格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	// 简单检查：长度为36，包含4个连字符
	if len(s) != 36 {
		return false
	}
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}
	// 检查各部分长度
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return false
	}
	return true
}

// loadData 加载 Profile 数据
func (pi *ProfileInstance) loadData() error {
	db := database.GetDB()

	// 加载所有供应商
	var providers []model.Provider
	db.Find(&providers)

	for i := range providers {
		p := &providers[i]
		pi.providerMap[p.ID] = p

		// 创建适配器
		if adp := adapter.Create(p.Type); adp != nil {
			if err := adp.Init(p); err == nil {
				pi.adapters[p.ID] = adp
			}
		}
	}

	// 根据 Profile.ModelIDs 加载模型 (直接访问)
	// ModelIDs 可以是模型 ID (UUID) 或模型名称
	if len(pi.Profile.ModelIDs) > 0 {
		for _, modelID := range pi.Profile.ModelIDs {
			var models []model.Model

			// 先尝试按 ID 查找（如果看起来像 UUID）
			if isValidUUID(modelID) {
				db.Where("id = ?", modelID).Find(&models)
			}

			// 如果按 ID 没找到，按名称查找
			if len(models) == 0 {
				db.Where("name = ? AND enabled = ?", modelID, true).Find(&models)
			}

			for j := range models {
				m := &models[j]
				pi.modelMap[m.Name] = append(pi.modelMap[m.Name], m)
			}
		}
	}

	// 根据 Profile.RouteIDs 加载路由 (通过路由访问)
	// RouteIDs 是路由 ID 列表，路由包含带权重和优先级的模型配置
	if len(pi.Profile.RouteIDs) > 0 {
		pi.routeMap = make(map[string]*RouteInstance)
		routeService := GetRouteService()
		for _, routeID := range pi.Profile.RouteIDs {
			if route := routeService.GetRoute(routeID); route != nil {
				pi.routeMap[routeID] = route
			}
		}
	}

	// 加载复合模型
	var compositeModels []model.CompositeAutoModel
	db.Where("profile_id = ? AND enabled = ?", pi.Profile.ID, true).Find(&compositeModels)
	pi.compositeModels = make(map[string]*model.CompositeAutoModel)
	for i := range compositeModels {
		cm := &compositeModels[i]
		pi.compositeModels[cm.Name] = cm
	}

	// 初始化复合模型服务
	if len(pi.compositeModels) > 0 {
		compositeService, err := NewCompositeModelService(pi.Profile.ID)
		if err != nil {
			fmt.Printf("Warning: failed to create composite service for profile %s: %v\n", pi.Profile.ID, err)
		} else {
			pi.compositeService = compositeService
		}
	}

	return nil
}

// initCompression 初始化压缩策略
func (pi *ProfileInstance) initCompression() {
	// Only initialize if compression is enabled for this profile
	if !pi.Profile.EnableCompression {
		return
	}

	// DEPRECATED: Initialize legacy compression pipeline for backward compatibility
	// This will be removed in Phase 4 of the refactoring
	var adapterForCompression adapter.Adapter
	for _, adp := range pi.adapters {
		adapterForCompression = adp
		break
	}

	if adapterForCompression == nil {
		return
	}

	// Register compression strategies based on profile config
	switch pi.Profile.CompressionStrategy {
	case "sliding_window", "hybrid", "":
		// Sliding window is always registered as it's the primary strategy
		// Wrap with NewLegacyStrategy for backward compatibility
		strategy := compressionpkg.NewLegacyStrategy(
			compressionpkg.NewSlidingWindowStrategy(adapterForCompression),
			adapterForCompression,
		)
		pi.compressionPipeline.Register(strategy)
	}

	// Initialize compression group selector if profile has compression groups configured
	var selector *repository.CompressionGroupSelector
	if len(pi.Profile.CompressionGroups) > 0 || pi.Profile.DefaultCompressionGroup != "" {
		var err error
		selector, err = repository.NewCompressionGroupSelector(
			pi.Profile.ID,
			GetStatsCollector(),
			nil, // GetCompressionMetrics() not needed for now
		)
		if err != nil {
			// Log warning but don't fail - compression will fall back to legacy mode
			fmt.Printf("Warning: failed to initialize compression group selector: %v\n", err)
		} else {
			pi.CompressionSelector = selector
		}
	}

	// NEW: Initialize independent compression service
	factory := compressionservice.NewFactory(nil) // Use default config
	pi.compressionService = factory.CreateService(pi.Profile, pi.adapters, selector)
}

// ApplyCompression 应用压缩策略到消息列表
// NEW SIGNATURE: Accepts session and compression group parameters for model selection
// REFACTORED: Now uses the independent compression service
func (pi *ProfileInstance) ApplyCompression(ctx context.Context, session *model.Session, maxTokens int, compressionGroup *string) ([]model.Message, *model.CompressionMetadata, error) {
	// Use the new compression service if available
	if pi.compressionService != nil {
		return pi.compressionService.Compress(ctx, pi.Profile, session, maxTokens, compressionGroup)
	}

	// Fallback to legacy implementation for backward compatibility during v3 refactoring
	if !pi.Profile.EnableCompression || pi.compressionPipeline == nil {
		// Return empty messages with empty metadata when compression is disabled
		return []model.Message{}, &model.CompressionMetadata{}, nil
	}

	// 1. Determine compression group using helper
	groupName := pi.getCompressionGroupName(compressionGroup)

	// 2. Create getAdapter function with fallback logic
	getAdapter := func(ctx context.Context) (adapter.Adapter, error) {
		if groupName == "" {
			// Legacy mode: return first available adapter
			for _, adp := range pi.adapters {
				return adp, nil
			}
			return nil, fmt.Errorf("no adapter available for compression")
		}
		// Group mode: use compression selector with fallback
		if pi.CompressionSelector != nil {
			adp, _, _, err := pi.CompressionSelector.SelectAdapter(ctx, groupName)
			if err == nil {
				return adp, nil
			}
			// Fall through to legacy adapter on error
		}
		// Fallback: return first available adapter
		for _, adp := range pi.adapters {
			return adp, nil
		}
		return nil, fmt.Errorf("no adapter available for compression")
	}

	// 3. Build strategy configs from profile settings
	configs := []compressionpkg.StrategyConfig{
		{
			Name:      pi.Profile.CompressionStrategy,
			MaxTokens: maxTokens,
			Weight:    100,
		},
	}

	// 4. Call compression pipeline
	result, err := pi.compressionPipeline.Compress(ctx, session, maxTokens, configs, getAdapter)
	if err != nil {
		return nil, nil, err
	}

	// 5. Populate CompressionMetadata
	metadata := &model.CompressionMetadata{
		GroupUsed:    groupName,
		FallbackUsed: groupName != "" && pi.CompressionSelector == nil,
		TokensAfter:  result.TotalTokens,
	}

	// Get tokens before from first strategy stat if available
	if len(result.Stats) > 0 {
		metadata.TokensBefore = result.Stats[0].InputTokens
	}

	// Calculate compression ratio
	if metadata.TokensBefore > 0 {
		metadata.CompressionRatio = float64(metadata.TokensAfter) / float64(metadata.TokensBefore)
	}

	return result.Messages, metadata, nil
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
		// compressionService will be initialized lazily when needed
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
	oldPath := oldProfile.Path
	newPath := p.Path

	if oldPath != newPath {
		if instance, ok := pm.profiles[oldPath]; ok {
			delete(pm.profiles, oldPath)
			pm.profiles[newPath] = instance
		}
	}

	// 更新内存中的数据 - 如果存在就更新，不存在就重新加载
	if instance, ok := pm.profiles[newPath]; ok {
		instance.Profile = p

		// 重新加载数据（ModelIDs 可能已变更）
		instance.Lock()
		instance.adapters = make(map[string]adapter.Adapter)
		instance.providerMap = make(map[string]*model.Provider)
		instance.modelMap = make(map[string][]*model.Model)
		instance.compositeModels = make(map[string]*model.CompositeAutoModel)
		if instance.compositeService != nil {
			instance.compositeService.Close()
			instance.compositeService = nil
		}
		instance.Unlock()

		if err := instance.loadData(); err != nil {
			// Note: Profile update was saved to DB but runtime state may be inconsistent.
			// The profile will be fully reloaded on next server restart.
			return fmt.Errorf("failed to reload profile data after update (profile saved to DB but runtime may be inconsistent): %w", err)
		}

		// Re-initialize compression pipeline with new settings
		instance.initCompression()
	} else {
		// Profile doesn't exist in memory, reload from DB
		return pm.loadProfile_unlocked(p)
	}

	return nil
}

// loadProfile_unlocked 加载单个 Profile (内部方法，不加锁)
func (pm *ProfileManager) loadProfile_unlocked(p *model.Profile) error {
	if !p.Enabled {
		return nil
	}

	instance := &ProfileInstance{
		Profile:             p,
		adapters:            make(map[string]adapter.Adapter),
		providerMap:         make(map[string]*model.Provider),
		modelMap:            make(map[string][]*model.Model),
		stats:               GetStatsCollector(),
		compressionPipeline: compressionpkg.NewPipeline(), // DEPRECATED: kept for backward compatibility
	}
	instance.loadData()
	instance.initCompression()

	pm.profiles[p.Path] = instance

	// 如果没有默认 Profile，设置为当前 Profile
	if pm.defaultProfile == "" {
		pm.defaultProfile = p.Path
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

// TestModel 通过模型名称测试模型（自动查找 provider）
func (pm *ProfileManager) TestModel(ctx context.Context, modelName string) (*model.TestResult, error) {
	// 1. 从数据库查询该模型及其 provider
	db := database.GetDB()
	var targetModel model.Model
	result := db.Where("name = ? AND enabled = ?", modelName, true).First(&targetModel)
	if result.Error != nil {
		return nil, fmt.Errorf("model not found: %s", modelName)
	}

	// 2. 查询 provider
	var provider model.Provider
	result = db.Where("id = ? AND enabled = ?", targetModel.ProviderID, true).First(&provider)
	if result.Error != nil {
		return nil, fmt.Errorf("provider not found for model: %s", modelName)
	}

	// 3. 创建适配器
	adp := adapter.Create(provider.Type)
	if adp == nil {
		return nil, fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
	if err := adp.Init(&provider); err != nil {
		return nil, fmt.Errorf("failed to init adapter: %v", err)
	}

	// 4. 构建请求
	actualModelName := targetModel.OriginalName
	if actualModelName == "" {
		actualModelName = modelName
	}

	req := &model.ChatCompletionRequest{
		Model: actualModelName,
		Messages: []model.Message{
			{Role: "user", Content: "Hello, this is a test message. Please respond with 'OK'."},
		},
		MaxTokens: 50,
	}

	// 5. 打印请求体
	endpoint := provider.BaseURL + provider.ChatPath
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	middleware.InfoLog("[TestModel] Request to provider: endpoint=%s model=%s\nBody: %s", endpoint, actualModelName, string(reqJSON))

	// 6. 发送请求
	start := time.Now()
	_, err := adp.ChatCompletion(ctx, req)
	latency := time.Since(start).Milliseconds()

	testResult := &model.TestResult{
		ProviderID: provider.ID,
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

	db.Create(testResult)

	return testResult, nil
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
	instance.compositeModels = make(map[string]*model.CompositeAutoModel)

	// Close old composite service if exists
	if instance.compositeService != nil {
		instance.compositeService.Close()
		instance.compositeService = nil
	}
	instance.Unlock()

	return instance.loadData()
}

// RefreshAll 刷新所有 Profile
func (pm *ProfileManager) RefreshAll() error {
	pm.Lock()
	defer pm.Unlock()

	// 清空现有
	pm.profiles = make(map[string]*ProfileInstance)

	return pm.loadFromDB_unlocked()
}

// ==================== ProfileInstance 方法 ====================

// Route 根据模型名称路由
func (pi *ProfileInstance) Route(ctx context.Context, modelName string) (*RouteResult, error) {
	pi.RLock()
	defer pi.RUnlock()

	// 1. 首先检查是否是复合模型
	if compositeModel, ok := pi.compositeModels[modelName]; ok && compositeModel.Enabled {
		if pi.compositeService != nil {
			return pi.compositeService.Route(ctx, pi, modelName)
		}
	}

	// 2. 直接查找模型 (通过 ModelIDs)
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

	// 3. 通过路由查找模型 (通过 RouteIDs，使用权重和优先级)
	if len(pi.routeMap) > 0 {
		result, err := pi.routeViaRoutes(ctx, modelName)
		if err == nil && result != nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("no available provider for model: %s in profile: %s", modelName, pi.Profile.Path)
}

// routeViaRoutes 通过路由查找模型 (使用路由的权重和优先级策略)
func (pi *ProfileInstance) routeViaRoutes(ctx context.Context, modelName string) (*RouteResult, error) {
	routeService := GetRouteService()

	for routeID, routeInstance := range pi.routeMap {
		// 检查路由中是否有匹配的模型
		for _, entry := range routeInstance.ModelEntries {
			if entry.Enabled && entry.Model != nil && entry.Model.Name == modelName {
				// 使用路由的策略选择最佳模型
				selectedEntry, err := routeService.SelectModelFromRoute(ctx, routeID)
				if err != nil {
					continue
				}

				if selectedEntry != nil && selectedEntry.Model != nil {
					provider, adapter := pi.getProviderAndAdapter(selectedEntry.Model.ProviderID)
					if provider != nil && adapter != nil && provider.Enabled {
						return &RouteResult{
							Adapter:      adapter,
							Model:        selectedEntry.Model,
							Provider:     provider,
							Profile:      pi.Profile,
							FallbackUsed: false,
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("model %s not found in any route", modelName)
}

// RouteByRouteID 根据路由 ID 从路由中选择模型（使用负载均衡策略）
// 用于规则动作类型为 route 的场景
func (pi *ProfileInstance) RouteByRouteID(ctx context.Context, routeID string) (*RouteResult, string, error) {
	pi.RLock()
	defer pi.RUnlock()

	// 1. 检查 profile 是否有这个路由
	routeInstance, ok := pi.routeMap[routeID]
	if !ok {
		return nil, "", fmt.Errorf("route %s not found in profile: %s", routeID, pi.Profile.Path)
	}

	// 2. 使用路由服务从路由中选择模型（带负载均衡）
	routeService := GetRouteService()
	selectedEntry, err := routeService.SelectModelFromRouteInstance(ctx, routeInstance)
	if err != nil {
		return nil, "", fmt.Errorf("failed to select model from route %s: %w", routeID, err)
	}

	if selectedEntry == nil || selectedEntry.Model == nil {
		return nil, "", fmt.Errorf("no available model in route: %s", routeID)
	}

	// 3. 获取 provider 和 adapter
	provider, adapter := pi.getProviderAndAdapter(selectedEntry.Model.ProviderID)
	if provider == nil || adapter == nil || !provider.Enabled {
		return nil, "", fmt.Errorf("provider not available for model: %s", selectedEntry.Model.Name)
	}

	return &RouteResult{
		Adapter:      adapter,
		Model:        selectedEntry.Model,
		Provider:     provider,
		Profile:      pi.Profile,
		FallbackUsed: false,
	}, selectedEntry.Model.Name, nil
}

// RouteByRuleTargets 根据规则的目标列表选择模型（支持负载均衡）
// 用于规则动作类型为 models 的场景
func (pi *ProfileInstance) RouteByRuleTargets(ctx context.Context, targets []model.RuleTarget, strategy string) (*RouteResult, string, error) {
	pi.RLock()
	defer pi.RUnlock()

	if len(targets) == 0 {
		return nil, "", fmt.Errorf("no targets specified")
	}

	// 加载目标模型
	var models []ruleTargetModel

	for i := range targets {
		// 设置默认值
		targets[i].Normalize()
		t := targets[i]
		if !t.Enabled {
			continue
		}
		// 从 modelMap 中查找模型
		if mList, ok := pi.modelMap[t.ModelID]; ok && len(mList) > 0 {
			// 选择第一个可用的模型实例
			for _, m := range mList {
				if m.Enabled {
					models = append(models, ruleTargetModel{
						model:    m,
						weight:   t.Weight,
						priority: t.Priority,
					})
					break
				}
			}
		}
	}

	if len(models) == 0 {
		return nil, "", fmt.Errorf("no available models found in targets")
	}

	// 根据策略选择模型
	var selected *model.Model
	switch strategy {
	case "priority":
		selected = pi.selectByPriorityFromTargets(models)
	case "weighted":
		selected = pi.selectByWeightFromTargets(models)
	case "random":
		selected = pi.selectByRandomFromTargets(models)
	case "auto", "":
		selected = pi.selectByAutoFromTargets(ctx, models)
	default:
		selected = pi.selectByAutoFromTargets(ctx, models)
	}

	if selected == nil {
		return nil, "", fmt.Errorf("no model selected")
	}

	// 获取 provider 和 adapter
	provider, adapter := pi.getProviderAndAdapter(selected.ProviderID)
	if provider == nil || adapter == nil || !provider.Enabled {
		return nil, "", fmt.Errorf("provider not available for model: %s", selected.Name)
	}

	return &RouteResult{
		Adapter:      adapter,
		Model:        selected,
		Provider:     provider,
		Profile:      pi.Profile,
		FallbackUsed: false,
	}, selected.Name, nil
}

// selectByPriorityFromTargets 按优先级选择模型（考虑健康度）
func (pi *ProfileInstance) selectByPriorityFromTargets(models []ruleTargetModel) *model.Model {
	var best *model.Model
	highestPriority := -1
	healthThreshold := 0.5 // 与 RouteService 保持一致的健康阈值

	// 首先尝试从健康的模型中选择
	for _, tm := range models {
		healthScore := pi.stats.GetHealthScore(tm.model.ProviderID, tm.model.Name)
		if healthScore < healthThreshold {
			continue // 跳过不健康的模型
		}
		if tm.priority > highestPriority {
			highestPriority = tm.priority
			best = tm.model
		}
	}

	// 如果没有健康的模型，降级选择优先级最高的
	if best == nil && len(models) > 0 {
		for _, tm := range models {
			if tm.priority > highestPriority {
				highestPriority = tm.priority
				best = tm.model
			}
		}
	}
	return best
}

// selectByWeightFromTargets 按权重随机选择
func (pi *ProfileInstance) selectByWeightFromTargets(models []ruleTargetModel) *model.Model {
	totalWeight := 0
	for _, tm := range models {
		totalWeight += tm.weight
	}

	if totalWeight <= 0 {
		return models[0].model
	}

	target := pkgRand.Intn(totalWeight)
	cumulative := 0
	for _, tm := range models {
		cumulative += tm.weight
		if target < cumulative {
			return tm.model
		}
	}
	return models[0].model
}

// selectByRandomFromTargets 随机选择
func (pi *ProfileInstance) selectByRandomFromTargets(models []ruleTargetModel) *model.Model {
	return models[pkgRand.Intn(len(models))].model
}

// selectByAutoFromTargets 自动选择（综合评分）
func (pi *ProfileInstance) selectByAutoFromTargets(ctx context.Context, models []ruleTargetModel) *model.Model {
	type scoredModel struct {
		model *model.Model
		score float64
	}
	var scored []scoredModel

	for _, tm := range models {
		score := 0.0
		// 健康度 (40%)
		healthScore := pi.stats.GetHealthScore(tm.model.ProviderID, tm.model.Name)
		score += healthScore * 0.4

		// 优先级 (30%)
		priorityScore := float64(tm.priority) * 10
		if priorityScore > 100 {
			priorityScore = 100
		}
		score += priorityScore * 0.3

		// 延迟 (20%)
		avgLatency := pi.stats.GetAvgLatency(tm.model.ProviderID, tm.model.Name, time.Minute)
		latencyScore := 100.0
		if avgLatency > 0 {
			latencyScore = max(0, 100-avgLatency/50.0)
		}
		score += latencyScore * 0.2

		// 权重 (10%)
		weightScore := float64(tm.weight)
		if weightScore > 100 {
			weightScore = 100
		}
		score += weightScore * 0.1

		scored = append(scored, scoredModel{model: tm.model, score: score})
	}

	// 选择得分最高的
	best := scored[0]
	for _, s := range scored[1:] {
		if s.score > best.score {
			best = s
		}
	}
	return best.model
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

// selectBestModel 选择最佳模型
func (pi *ProfileInstance) selectBestModel(models []*model.Model) *model.Model {
	if len(models) == 0 {
		return nil
	}
	if len(models) == 1 {
		return models[0]
	}

	var best *model.Model
	bestHealthScore := -1.0

	for _, m := range models {
		healthScore := pi.stats.GetHealthScore(m.ProviderID, m.Name)

		if healthScore > bestHealthScore {
			best = m
			bestHealthScore = healthScore
		}
	}

	return best
}

// tryFallback 尝试 fallback
func (pi *ProfileInstance) tryFallback(modelName string) *RouteResult {
	// Use Profile.FallbackModels for fallback
	for _, fallbackName := range pi.Profile.FallbackModels {
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
	return nil
}

// shouldFallback 判断是否应当 fallback
func (pi *ProfileInstance) shouldFallback(providerID, modelName string) bool {
	cfg := config.Get()
	if !cfg.GetEnableFallback() {
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

	// Check model-level rate limit first
	rateLimit := pi.getModelRateLimit(providerID, modelName)
	if rateLimit > 0 && currentRPM >= int64(rateLimit) {
		return true
	}

	return false
}

// getModelRateLimit 获取模型的速率限制
// 返回模型的 rate_limit（如果 > 0），否则返回 0（无限制）
func (pi *ProfileInstance) getModelRateLimit(providerID, modelName string) int {
	pi.RLock()
	defer pi.RUnlock()

	// Find the model in modelMap
	if models, ok := pi.modelMap[modelName]; ok {
		for _, m := range models {
			if m.ProviderID == providerID && m.RateLimit > 0 {
				return m.RateLimit
			}
		}
	}
	return 0
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

	// 直接从数据库查询该 provider 下的模型
	db := database.GetDB()
	var targetModel model.Model
	result := db.Where("name = ? AND provider_id = ? AND enabled = ?", modelName, providerID, true).First(&targetModel)
	if result.Error != nil {
		return nil, fmt.Errorf("model not found: %s", modelName)
	}

	// Use OriginalName for the actual API call, fallback to modelName if empty
	actualModelName := modelName
	if targetModel.OriginalName != "" {
		actualModelName = targetModel.OriginalName
	}

	req := &model.ChatCompletionRequest{
		Model: actualModelName,
		Messages: []model.Message{
			{Role: "user", Content: "Hello, this is a test message. Please respond with 'OK'."},
		},
		MaxTokens: 50,
	}

	// 打印发送到 provider 的请求体
	endpoint := provider.BaseURL + provider.ChatPath
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	middleware.InfoLog("[TestModel] Request to provider: endpoint=%s model=%s\nBody: %s", endpoint, actualModelName, string(reqJSON))

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

	db.Create(testResult)

	return testResult, nil
}

// GetAdapterForModel 获取指定模型的适配器
func (pi *ProfileInstance) GetAdapterForModel(modelName, providerID string) (adapter.Adapter, *model.Model, error) {
	pi.RLock()
	defer pi.RUnlock()

	// Look up models by name
	models, ok := pi.modelMap[modelName]
	if !ok || len(models) == 0 {
		return nil, nil, fmt.Errorf("model not found: %s", modelName)
	}

	// If provider specified, find that specific one
	if providerID != "" {
		for _, m := range models {
			if m.ProviderID == providerID {
				_, ok := pi.providerMap[m.ProviderID]
				if !ok {
					return nil, nil, fmt.Errorf("provider not found: %s", m.ProviderID)
				}
				adp, ok := pi.adapters[m.ProviderID]
				if !ok {
					return nil, nil, fmt.Errorf("adapter not found for provider: %s", m.ProviderID)
				}
				return adp, m, nil
			}
		}
		return nil, nil, fmt.Errorf("model '%s' not found for provider '%s'", modelName, providerID)
	}

	// No provider specified - use first available
	for _, m := range models {
		_, ok := pi.providerMap[m.ProviderID]
		if !ok {
			continue
		}
		adp, ok := pi.adapters[m.ProviderID]
		if ok {
			return adp, m, nil
		}
	}

	return nil, nil, fmt.Errorf("no adapter available for model: %s", modelName)
}

// GetCompositeModel returns the composite model definition by name
func (pi *ProfileInstance) GetCompositeModel(modelName string) (*model.CompositeAutoModel, bool) {
	pi.RLock()
	defer pi.RUnlock()

	composite, ok := pi.compositeModels[modelName]
	return composite, ok
}

// getCompressionGroupName determines which compression group to use
// Priority: API override > profile default > empty (legacy mode)
func (pi *ProfileInstance) getCompressionGroupName(apiGroup *string) string {
	if apiGroup != nil && *apiGroup != "" {
		return *apiGroup
	}
	return pi.Profile.DefaultCompressionGroup
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

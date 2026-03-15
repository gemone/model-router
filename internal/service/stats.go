package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
)

// StatsCollector 统计收集器
type StatsCollector struct {
	sync.RWMutex
	requestLogs []model.RequestLog
	hourlyStats map[string]*model.Stats // key: date_hour_provider_model
	quit        chan struct{}         // 用于优雅退出后台 goroutine
}

var (
	statsCollector     *StatsCollector
	statsCollectorOnce sync.Once
)

// GetStatsCollector 获取统计收集器单例
func GetStatsCollector() *StatsCollector {
	statsCollectorOnce.Do(func() {
		statsCollector = &StatsCollector{
			requestLogs: make([]model.RequestLog, 0),
			hourlyStats: make(map[string]*model.Stats),
			quit:        make(chan struct{}),
		}
		go statsCollector.flushLoop()
	})
	return statsCollector
}

// Stop 停止统计收集器，优雅关闭后台 goroutine
func (sc *StatsCollector) Stop() {
	if sc.quit != nil {
		close(sc.quit)
	}
}

// RecordRequest 记录请求
func (sc *StatsCollector) RecordRequest(log *model.RequestLog) {
	sc.Lock()
	sc.requestLogs = append(sc.requestLogs, *log)
	sc.Unlock()

	// 更新内存统计
	sc.updateHourlyStats(log)
}

// updateHourlyStats 更新每小时统计
func (sc *StatsCollector) updateHourlyStats(log *model.RequestLog) {
	sc.Lock()
	defer sc.Unlock()

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	hour := now.Hour()
	key := fmt.Sprintf("%s_%d_%s_%s", dateStr, hour, log.ProviderID, log.Model)

	stats, ok := sc.hourlyStats[key]
	if !ok {
		stats = &model.Stats{
			Date:       dateStr,
			Hour:       hour,
			ProviderID: log.ProviderID,
			Model:      log.Model,
		}
		sc.hourlyStats[key] = stats
	}

	stats.RequestCount++
	if log.Status == "success" {
		stats.SuccessCount++
	} else {
		stats.ErrorCount++
	}
	stats.TotalTokens += int64(log.TotalTokens)

	// 更新平均延迟
	if stats.RequestCount > 0 {
		stats.AvgLatency = (stats.AvgLatency*float64(stats.RequestCount-1) + float64(log.Latency)) / float64(stats.RequestCount)
	}
}

// GetHealthScore 获取健康评分 (0-100)
func (sc *StatsCollector) GetHealthScore(providerID, modelName string) float64 {
	sc.RLock()
	defer sc.RUnlock()

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	hour := now.Hour()
	key := fmt.Sprintf("%s_%d_%s_%s", dateStr, hour, providerID, modelName)

	stats, ok := sc.hourlyStats[key]
	if !ok {
		return 100.0 // 默认健康
	}

	if stats.RequestCount == 0 {
		return 100.0
	}

	// 计算成功率
	successRate := float64(stats.SuccessCount) / float64(stats.RequestCount)

	// 延迟因子 (假设 5000ms 为基准)
	latencyFactor := 1.0
	if stats.AvgLatency > 0 {
		latencyFactor = 5000.0 / (5000.0 + stats.AvgLatency)
		if latencyFactor > 1.0 {
			latencyFactor = 1.0
		}
	}

	// 综合健康分
	score := successRate * 100 * (0.7 + 0.3*latencyFactor)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// GetErrorRate 获取错误率
func (sc *StatsCollector) GetErrorRate(providerID, modelName string, duration time.Duration) float64 {
	sc.RLock()
	defer sc.RUnlock()

	cutoff := time.Now().Add(-duration)
	var total, errors int64

	for _, log := range sc.requestLogs {
		if log.CreatedAt.Before(cutoff) {
			continue
		}
		if log.ProviderID == providerID && log.Model == modelName {
			total++
			if log.Status != "success" {
				errors++
			}
		}
	}

	if total == 0 {
		return 0
	}
	return float64(errors) / float64(total)
}

// GetAvgLatency 获取平均延迟
func (sc *StatsCollector) GetAvgLatency(providerID, modelName string, duration time.Duration) float64 {
	sc.RLock()
	defer sc.RUnlock()

	cutoff := time.Now().Add(-duration)
	var totalLatency int64
	var count int64

	for _, log := range sc.requestLogs {
		if log.CreatedAt.Before(cutoff) {
			continue
		}
		if log.ProviderID == providerID && log.Model == modelName {
			totalLatency += log.Latency
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return float64(totalLatency) / float64(count)
}

// GetCurrentRPM 获取当前 RPM (最近一分钟的请求数)
func (sc *StatsCollector) GetCurrentRPM(providerID, modelName string) int64 {
	return sc.getRequestCount(providerID, modelName, time.Minute)
}

// getRequestCount 获取指定时间内的请求数
func (sc *StatsCollector) getRequestCount(providerID, modelName string, duration time.Duration) int64 {
	sc.RLock()
	defer sc.RUnlock()

	cutoff := time.Now().Add(-duration)
	var count int64

	for _, log := range sc.requestLogs {
		if log.CreatedAt.After(cutoff) && log.ProviderID == providerID && log.Model == modelName {
			count++
		}
	}

	return count
}

// GetProviderStats 获取供应商统计
func (sc *StatsCollector) GetProviderStats(providerID string) map[string]interface{} {
	sc.RLock()
	defer sc.RUnlock()

	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)
	cutoff1h := now.Add(-time.Hour)

	var totalRequests, successRequests, totalTokens int64
	var totalLatency int64
	modelStats := make(map[string]map[string]int64)

	for _, log := range sc.requestLogs {
		if log.ProviderID != providerID {
			continue
		}

		if log.CreatedAt.After(cutoff24h) {
			totalRequests++
			if log.Status == "success" {
				successRequests++
			}
			totalTokens += int64(log.TotalTokens)
			totalLatency += log.Latency

			if _, ok := modelStats[log.Model]; !ok {
				modelStats[log.Model] = make(map[string]int64)
			}
			modelStats[log.Model]["requests"]++
			if log.Status == "success" {
				modelStats[log.Model]["success"]++
			}
			modelStats[log.Model]["tokens"] += int64(log.TotalTokens)
		}
	}

	successRate := 0.0
	avgLatency := 0.0
	if totalRequests > 0 {
		successRate = float64(successRequests) / float64(totalRequests) * 100
		avgLatency = float64(totalLatency) / float64(totalRequests)
	}

	// 最近1小时 RPM
	rpm1h := int64(0)
	for _, log := range sc.requestLogs {
		if log.ProviderID == providerID && log.CreatedAt.After(cutoff1h) {
			rpm1h++
		}
	}

	return map[string]interface{}{
		"provider_id":        providerID,
		"total_requests_24h": totalRequests,
		"success_rate_24h":   successRate,
		"avg_latency_ms":     avgLatency,
		"total_tokens_24h":   totalTokens,
		"rpm_last_hour":      rpm1h,
		"model_stats":        modelStats,
	}
}

// GetModelStats 获取模型统计
func (sc *StatsCollector) GetModelStats(modelName string) map[string]interface{} {
	sc.RLock()
	defer sc.RUnlock()

	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)

	var totalRequests, successRequests, totalTokens int64
	var totalLatency int64
	providerStats := make(map[string]map[string]int64)

	for _, log := range sc.requestLogs {
		if log.Model != modelName {
			continue
		}

		if log.CreatedAt.After(cutoff24h) {
			totalRequests++
			if log.Status == "success" {
				successRequests++
			}
			totalTokens += int64(log.TotalTokens)
			totalLatency += log.Latency

			if _, ok := providerStats[log.ProviderID]; !ok {
				providerStats[log.ProviderID] = make(map[string]int64)
			}
			providerStats[log.ProviderID]["requests"]++
			if log.Status == "success" {
				providerStats[log.ProviderID]["success"]++
			}
			providerStats[log.ProviderID]["tokens"] += int64(log.TotalTokens)
		}
	}

	successRate := 0.0
	avgLatency := 0.0
	if totalRequests > 0 {
		successRate = float64(successRequests) / float64(totalRequests) * 100
		avgLatency = float64(totalLatency) / float64(totalRequests)
	}

	return map[string]interface{}{
		"model":              modelName,
		"total_requests_24h": totalRequests,
		"success_rate_24h":   successRate,
		"avg_latency_ms":     avgLatency,
		"total_tokens_24h":   totalTokens,
		"provider_stats":     providerStats,
	}
}

// GetDashboardStats 获取仪表盘统计（合并内存和数据库数据）
func (sc *StatsCollector) GetDashboardStats() map[string]interface{} {
	sc.RLock()
	defer sc.RUnlock()

	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)
	cutoff1h := now.Add(-time.Hour)
	dateStr := now.Format("2006-01-02")
	hour := now.Hour()

	var totalRequests, successRequests, totalTokens int64
	var totalLatency int64
	providerCounts := make(map[string]int64)
	modelCounts := make(map[string]int64)

	// 1. 先累加内存中的数据（最新的）
	for _, log := range sc.requestLogs {
		if log.CreatedAt.After(cutoff24h) {
			totalRequests++
			if log.Status == "success" {
				successRequests++
			}
			totalTokens += int64(log.TotalTokens)
			totalLatency += log.Latency
			providerCounts[log.ProviderID]++
			modelCounts[log.Model]++
		}
	}

	// 2. 从数据库中的每小时统计累加数据
	var dbStats []model.Stats
	db := database.GetDB()
	// 获取最近24小时的数据：今天的过去小时 + 昨天的剩余小时
	yesterdayStr := now.Add(-24 * time.Hour).Format("2006-01-02")
	db.Where("(date = ? AND hour <= ?) OR (date = ? AND hour > ?)",
		dateStr, hour, yesterdayStr, hour).Find(&dbStats)
	
	// 累加今天更早时段的数据
	for _, stat := range dbStats {
		// 跳过当前小时（已经在内存中）
		if stat.Date == dateStr && stat.Hour == hour {
			continue
		}
		totalRequests += stat.RequestCount
		successRequests += stat.SuccessCount
		totalTokens += stat.TotalTokens
		if stat.RequestCount > 0 {
			totalLatency += int64(stat.AvgLatency * float64(stat.RequestCount))
		}
		providerCounts[stat.ProviderID] += stat.RequestCount
		modelCounts[stat.Model] += stat.RequestCount
	}

	// 3. 从数据库中的 request_logs 表获取最近的数据（补充数据）
	var dbLogs []model.RequestLog
	db.Where("created_at >= ?", cutoff24h).Find(&dbLogs)
	
	// 去重：只添加不在内存中的记录
	memoryIDs := make(map[string]bool)
	for _, log := range sc.requestLogs {
		memoryIDs[log.RequestID] = true
	}
	
	for _, log := range dbLogs {
		if memoryIDs[log.RequestID] {
			continue // 已在内存中统计过
		}
		totalRequests++
		if log.Status == "success" {
			successRequests++
		}
		totalTokens += int64(log.TotalTokens)
		totalLatency += log.Latency
		providerCounts[log.ProviderID]++
		modelCounts[log.Model]++
	}

	successRate := 0.0
	avgLatency := 0.0
	if totalRequests > 0 {
		successRate = float64(successRequests) / float64(totalRequests) * 100
		avgLatency = float64(totalLatency) / float64(totalRequests)
	}

	// 最近1小时（内存 + 数据库）
	var requests1h int64
	for _, log := range sc.requestLogs {
		if log.CreatedAt.After(cutoff1h) {
			requests1h++
		}
	}
	// 从数据库补充最近1小时的数据
	for _, log := range dbLogs {
		if memoryIDs[log.RequestID] {
			continue
		}
		if log.CreatedAt.After(cutoff1h) {
			requests1h++
		}
	}

	return map[string]interface{}{
		"total_requests_24h": totalRequests,
		"requests_last_hour": requests1h,
		"success_rate":       successRate,
		"avg_latency_ms":     avgLatency,
		"total_tokens":       totalTokens,
		"top_providers":      providerCounts,
		"top_models":         modelCounts,
	}
}

// flushLoop 定期刷新到数据库
func (sc *StatsCollector) flushLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sc.flush()
		case <-sc.quit:
			// 优雅退出：最后刷新一次数据
			sc.flush()
			return
		}
	}
}

// flush 将统计刷新到数据库
func (sc *StatsCollector) flush() {
	sc.Lock()
	defer sc.Unlock()

	db := database.GetDB()

	// 保存请求日志
	if len(sc.requestLogs) > 0 {
		// 批量插入
		db.CreateInBatches(sc.requestLogs, 100)
		sc.requestLogs = sc.requestLogs[:0]
	}

	// 保存每小时统计
	for _, stats := range sc.hourlyStats {
		var existing model.Stats
		result := db.Where("date = ? AND hour = ? AND provider_id = ? AND model = ?",
			stats.Date, stats.Hour, stats.ProviderID, stats.Model).First(&existing)

		if result.Error == nil {
			// 更新
			existing.RequestCount = stats.RequestCount
			existing.SuccessCount = stats.SuccessCount
			existing.ErrorCount = stats.ErrorCount
			existing.TotalTokens = stats.TotalTokens
			existing.AvgLatency = stats.AvgLatency
			db.Save(&existing)
		} else {
			// 创建
			db.Create(stats)
		}
	}

	// 清理旧的内存统计（保留最近24小时）
	cutoff := time.Now().Add(-24 * time.Hour)
	for key, stats := range sc.hourlyStats {
		statsTime, _ := time.Parse("2006-01-02_15", fmt.Sprintf("%s_%d", stats.Date, stats.Hour))
		if statsTime.Before(cutoff) {
			delete(sc.hourlyStats, key)
		}
	}
}

// GetRequestLogs 获取请求日志（分页）
// 合并内存中的请求日志和数据库中的日志
func (sc *StatsCollector) GetRequestLogs(page, pageSize int) ([]model.RequestLog, int64) {
	sc.RLock()
	defer sc.RUnlock()

	// 获取数据库中的总数
	db := database.GetDB()
	var dbTotal int64
	db.Model(&model.RequestLog{}).Count(&dbTotal)

	// 合并内存中的日志和数据库中的日志
	allLogs := make([]model.RequestLog, 0, len(sc.requestLogs)+int(dbTotal))
	
	// 添加内存中的日志（倒序，最新的在前）
	for i := len(sc.requestLogs) - 1; i >= 0; i-- {
		allLogs = append(allLogs, sc.requestLogs[i])
	}
	
	// 如果内存中的日志不够，从数据库补充
	if len(allLogs) < page*pageSize {
		var dbLogs []model.RequestLog
		db.Order("created_at DESC").Limit(page * pageSize).Find(&dbLogs)
		
		// 去重：避免添加已在内存中的日志
		memoryIDs := make(map[string]bool)
		for _, log := range sc.requestLogs {
			memoryIDs[log.RequestID] = true
		}
		
		for _, log := range dbLogs {
			if !memoryIDs[log.RequestID] {
				allLogs = append(allLogs, log)
			}
		}
	}
	
	total := int64(len(allLogs))
	
	// 分页
	start := (page - 1) * pageSize
	if start > len(allLogs) {
		start = len(allLogs)
	}
	end := start + pageSize
	if end > len(allLogs) {
		end = len(allLogs)
	}
	
	return allLogs[start:end], total
}

// ClearLogs 清空所有请求日志
func (sc *StatsCollector) ClearLogs() error {
	db := database.GetDB()
	return db.Where("1 = 1").Delete(&model.RequestLog{}).Error
}

// GetAllProviderModelStats 获取所有供应商和模型的详细统计数据
func (sc *StatsCollector) GetAllProviderModelStats() map[string]interface{} {
	sc.RLock()
	defer sc.RUnlock()

	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)

	// 获取数据库中的所有供应商和模型
	db := database.GetDB()
	var providers []model.Provider
	var models []model.Model
	db.Find(&providers)
	db.Find(&models)

	// 构建供应商详细统计
	type ProviderStatDetail struct {
		ID          string  `json:"id"`
		Name        string  `json:"name"`
		Requests    int64   `json:"requests"`
		Tokens      int64   `json:"tokens"`
		AvgLatency  float64 `json:"avg_latency"`
		ErrorRate   float64 `json:"error_rate"`
	}

	type ModelStatDetail struct {
		Name       string  `json:"name"`
		Requests   int64   `json:"requests"`
		Tokens     int64   `json:"tokens"`
		AvgLatency float64 `json:"avg_latency"`
		ErrorRate  float64 `json:"error_rate"`
	}

	providerStatsMap := make(map[string]*ProviderStatDetail)
	modelStatsMap := make(map[string]*ModelStatDetail)

	// 初始化供应商统计结构
	for _, p := range providers {
		providerStatsMap[p.ID] = &ProviderStatDetail{
			ID:     p.ID,
			Name:   p.Name,
			Requests: 0,
			Tokens: 0,
			AvgLatency: 0,
			ErrorRate: 0,
		}
	}

	// 初始化模型统计结构（使用 original_name 作为 key）
	for _, m := range models {
		modelKey := m.OriginalName
		if modelKey == "" {
			modelKey = m.Name
		}
		modelStatsMap[modelKey] = &ModelStatDetail{
			Name:       m.Name,
			Requests:   0,
			Tokens:     0,
			AvgLatency: 0,
			ErrorRate:  0,
		}
	}

	// 用于统计的中间数据
	type providerAgg struct {
		requests     int64
		tokens       int64
		totalLatency int64
		errors       int64
	}

	type modelAgg struct {
		requests     int64
		tokens       int64
		totalLatency int64
		errors       int64
	}

	providerData := make(map[string]*providerAgg)
	modelData := make(map[string]*modelAgg)

	// 1. 处理内存中的请求数据
	for _, log := range sc.requestLogs {
		if log.CreatedAt.After(cutoff24h) {
			// 供应商统计
			if _, ok := providerData[log.ProviderID]; !ok {
				providerData[log.ProviderID] = &providerAgg{}
			}
			p := providerData[log.ProviderID]
			p.requests++
			p.tokens += int64(log.TotalTokens)
			p.totalLatency += log.Latency
			if log.Status != "success" {
				p.errors++
			}

			// 模型统计
			if _, ok := modelData[log.Model]; !ok {
				modelData[log.Model] = &modelAgg{}
			}
			m := modelData[log.Model]
			m.requests++
			m.tokens += int64(log.TotalTokens)
			m.totalLatency += log.Latency
			if log.Status != "success" {
				m.errors++
			}
		}
	}

	// 2. 从数据库 request_logs 表补充数据
	var dbLogs []model.RequestLog
	db.Where("created_at >= ?", cutoff24h).Find(&dbLogs)

	// 内存中的日志 ID，用于去重
	memoryIDs := make(map[string]bool)
	for _, log := range sc.requestLogs {
		memoryIDs[log.RequestID] = true
	}

	for _, log := range dbLogs {
		if memoryIDs[log.RequestID] {
			continue
		}

		// 供应商统计
		if _, ok := providerData[log.ProviderID]; !ok {
			providerData[log.ProviderID] = &providerAgg{}
		}
		p := providerData[log.ProviderID]
		p.requests++
		p.tokens += int64(log.TotalTokens)
		p.totalLatency += log.Latency
		if log.Status != "success" {
			p.errors++
		}

		// 模型统计
		if _, ok := modelData[log.Model]; !ok {
			modelData[log.Model] = &modelAgg{}
		}
		m := modelData[log.Model]
		m.requests++
		m.tokens += int64(log.TotalTokens)
		m.totalLatency += log.Latency
		if log.Status != "success" {
			m.errors++
		}
	}

	// 3. 将聚合数据转换为最终结果
	// 供应商统计
	providerStatsList := make([]*ProviderStatDetail, 0)
	for providerID, p := range providerData {
		if stat, ok := providerStatsMap[providerID]; ok {
			stat.Requests = p.requests
			stat.Tokens = p.tokens
			if p.requests > 0 {
				stat.AvgLatency = float64(p.totalLatency) / float64(p.requests)
				stat.ErrorRate = float64(p.errors) / float64(p.requests)
			}
			providerStatsList = append(providerStatsList, stat)
		}
	}

	// 模型统计
	modelStatsList := make([]*ModelStatDetail, 0)
	for modelKey, m := range modelData {
		if stat, ok := modelStatsMap[modelKey]; ok {
			stat.Requests = m.requests
			stat.Tokens = m.tokens
			if m.requests > 0 {
				stat.AvgLatency = float64(m.totalLatency) / float64(m.requests)
				stat.ErrorRate = float64(m.errors) / float64(m.requests)
			}
			modelStatsList = append(modelStatsList, stat)
		}
	}

	return map[string]interface{}{
		"providers": providerStatsList,
		"models":    modelStatsList,
	}
}

// GetTrendStats 获取趋势数据（最近24小时的每小时请求数）
func (sc *StatsCollector) GetTrendStats() map[string]interface{} {
	sc.RLock()
	defer sc.RUnlock()

	now := time.Now()
	hours := make([]string, 24)
	requests := make([]int64, 24)
	tokens := make([]int64, 24)

	// 初始化24小时时间标签
	for i := range 24 {
		h := now.Add(time.Duration(i-23) * time.Hour)
		hours[i] = fmt.Sprintf("%02d:00", h.Hour())
	}

	db := database.GetDB()
	cutoff := now.Add(-24 * time.Hour)

	// 1. 从 request_logs 表统计（主要数据源）
	var dbLogs []model.RequestLog
	db.Where("created_at >= ?", cutoff).Find(&dbLogs)

	// 按 hour bucket 分组统计
	for _, log := range dbLogs {
		// 计算日志时间相对于现在的小时差
		hoursDiff := int(now.Sub(log.CreatedAt).Hours())
		// 映射到数组索引：23小时前 -> index 0, 现在 -> index 23
		arrayIndex := 23 - hoursDiff
		if arrayIndex >= 0 && arrayIndex < 24 {
			requests[arrayIndex]++
			tokens[arrayIndex] += int64(log.TotalTokens)
		}
	}

	// 2. 从内存中的 hourlyStats 补充最新数据（未刷新到数据库的）
	for _, stat := range sc.hourlyStats {
		// 解析统计时间
		statTime, err := time.Parse("2006-01-02_15", fmt.Sprintf("%s_%02d", stat.Date, stat.Hour))
		if err != nil {
			continue
		}
		// 只处理24小时内的数据
		if statTime.Before(cutoff) {
			continue
		}
		hoursDiff := int(now.Sub(statTime).Hours())
		arrayIndex := 23 - hoursDiff
		if arrayIndex >= 0 && arrayIndex < 24 {
			// 内存中的数据是聚合的，直接使用
			requests[arrayIndex] += stat.RequestCount
			tokens[arrayIndex] += stat.TotalTokens
		}
	}

	return map[string]interface{}{
		"hours":    hours,
		"requests": requests,
		"tokens":   tokens,
	}
}

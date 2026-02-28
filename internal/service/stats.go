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
		}
		go statsCollector.flushLoop()
	})
	return statsCollector
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

// GetDashboardStats 获取仪表盘统计
func (sc *StatsCollector) GetDashboardStats() map[string]interface{} {
	sc.RLock()
	defer sc.RUnlock()

	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)
	cutoff1h := now.Add(-time.Hour)

	var totalRequests, successRequests, totalTokens int64
	var totalLatency int64
	providerCounts := make(map[string]int64)
	modelCounts := make(map[string]int64)

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

	successRate := 0.0
	avgLatency := 0.0
	if totalRequests > 0 {
		successRate = float64(successRequests) / float64(totalRequests) * 100
		avgLatency = float64(totalLatency) / float64(totalRequests)
	}

	// 最近1小时
	var requests1h int64
	for _, log := range sc.requestLogs {
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

	for range ticker.C {
		sc.flush()
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
func (sc *StatsCollector) GetRequestLogs(page, pageSize int) ([]model.RequestLog, int64) {
	db := database.GetDB()

	var total int64
	db.Model(&model.RequestLog{}).Count(&total)

	var logs []model.RequestLog
	db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	return logs, total
}

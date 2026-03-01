package service

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/model"
	"gorm.io/gorm"
)

const (
	// EMACalculate EMA 计算的平滑系数 (α = 2/(N+1), N=10 => α≈0.18)
	EMAAlpha = 0.18

	// SuccessWindowSize 滑动窗口大小
	SuccessWindowSize = 100

	// HealthScoreLatencyWeight 延迟权重
	HealthScoreLatencyWeight = 0.3
	// HealthScoreSuccessRateWeight 成功率权重
	HealthScoreSuccessRateWeight = 0.5
	// HealthScoreStabilityWeight 稳定性权重
	HealthScoreStabilityWeight = 0.2

	// PersistenceInterval 异步持久化间隔
	PersistenceInterval = 30 * time.Second

	// DefaultHealthScore 初始健康分数
	DefaultHealthScore = 100.0
)

// ModelMetrics 模型指标
type ModelMetrics struct {
	ModelID       string
	EMALatency    float64       // 指数移动平均延迟
	SuccessCount  int           // 成功次数
	FailureCount  int           // 失败次数
	SuccessWindow []bool        // 滑动窗口 (true=成功, false=失败)
	HealthScore   float64       // 健康分数 (0-100)
	LastUpdated   time.Time     // 最后更新时间
	Variance      float64       // 延迟方差 (用于稳定性评估)
	LastLatency   time.Duration // 最后一次延迟
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*ModelMetrics
	db      *gorm.DB
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(db *gorm.DB) *MetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())
	mc := &MetricsCollector{
		metrics: make(map[string]*ModelMetrics),
		db:      db,
		ctx:     ctx,
		cancel:  cancel,
	}

	// 启动异步持久化协程
	mc.wg.Add(1)
	go mc.persistenceLoop()

	return mc
}

// RecordRequest 记录请求指标
func (m *MetricsCollector) RecordRequest(modelID string, latency time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, exists := m.metrics[modelID]
	if !exists {
		metrics = &ModelMetrics{
			ModelID:       modelID,
			EMALatency:    float64(latency.Milliseconds()),
			HealthScore:   DefaultHealthScore,
			LastUpdated:   time.Now(),
			LastLatency:   latency,
			SuccessWindow: make([]bool, 0, SuccessWindowSize),
		}
		m.metrics[modelID] = metrics
	}

	// 更新 EMA 延迟
	latencyMs := float64(latency.Milliseconds())
	metrics.EMALatency = EMAAlpha*latencyMs + (1-EMAAlpha)*metrics.EMALatency

	// 更新方差 (用于稳定性评估)
	delta := latencyMs - metrics.EMALatency
	metrics.Variance = (1-EMAAlpha)*(metrics.Variance+delta*delta*EMAAlpha)

	// 更新滑动窗口成功率
	metrics.SuccessWindow = append(metrics.SuccessWindow, success)
	if len(metrics.SuccessWindow) > SuccessWindowSize {
		metrics.SuccessWindow = metrics.SuccessWindow[1:]
	}

	// 更新成功/失败计数
	if success {
		metrics.SuccessCount++
	} else {
		metrics.FailureCount++
	}

	metrics.LastLatency = latency
	metrics.LastUpdated = time.Now()

	// 更新健康分数
	m.updateHealthScoreLocked(modelID)
}

// GetMetrics 获取模型指标
func (m *MetricsCollector) GetMetrics(modelID string) *ModelMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, exists := m.metrics[modelID]; exists {
		// 返回副本避免外部修改
		copy := *metrics
		// 复制滑动窗口
		copy.SuccessWindow = make([]bool, len(metrics.SuccessWindow))
		copySlice(copy.SuccessWindow, metrics.SuccessWindow)
		return &copy
	}

	return nil
}

// GetEMALatency 获取 EMA 延迟
func (m *MetricsCollector) GetEMALatency(modelID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, exists := m.metrics[modelID]; exists {
		return metrics.EMALatency
	}
	return 0
}

// GetSuccessRate 获取成功率 (基于滑动窗口)
func (m *MetricsCollector) GetSuccessRate(modelID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, exists := m.metrics[modelID]; exists {
		if len(metrics.SuccessWindow) == 0 {
			return 1.0 // 无数据时默认100%
		}

		successCount := 0
		for _, success := range metrics.SuccessWindow {
			if success {
				successCount++
			}
		}
		return float64(successCount) / float64(len(metrics.SuccessWindow))
	}
	return 1.0
}

// GetHealthScore 获取健康分数
func (m *MetricsCollector) GetHealthScore(modelID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, exists := m.metrics[modelID]; exists {
		return metrics.HealthScore
	}
	return DefaultHealthScore
}

// UpdateHealthScore 更新健康分数
func (m *MetricsCollector) UpdateHealthScore(modelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateHealthScoreLocked(modelID)
}

// updateHealthScoreLocked 内部方法: 更新健康分数 (需要持有锁)
func (m *MetricsCollector) updateHealthScoreLocked(modelID string) {
	metrics, exists := m.metrics[modelID]
	if !exists {
		return
	}

	// 1. 延迟分数 (0-100, 延迟越低分数越高)
	// 基准: 1000ms = 0分, 0ms = 100分
	latencyScore := math.Max(0, 100-metrics.EMALatency/10)

	// 2. 成功率分数 (0-100)
	successRate := m.getSuccessRateLocked(metrics)
	successRateScore := successRate * 100

	// 3. 稳定性分数 (0-100, 方差越小分数越高)
	// 方差 0 = 100分, 方差 >= 10000 = 0分
	stabilityScore := math.Max(0, 100-math.Sqrt(metrics.Variance)/10)

	// 综合评分
	metrics.HealthScore = latencyScore*HealthScoreLatencyWeight +
		successRateScore*HealthScoreSuccessRateWeight +
		stabilityScore*HealthScoreStabilityWeight
}

// getSuccessRateLocked 内部方法: 获取成功率 (需要持有锁)
func (m *MetricsCollector) getSuccessRateLocked(metrics *ModelMetrics) float64 {
	if len(metrics.SuccessWindow) == 0 {
		return 1.0
	}

	successCount := 0
	for _, success := range metrics.SuccessWindow {
		if success {
			successCount++
		}
	}
	return float64(successCount) / float64(len(metrics.SuccessWindow))
}

// persistenceLoop 异步持久化循环
func (m *MetricsCollector) persistenceLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(PersistenceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.persistMetrics()
		case <-m.ctx.Done():
			// 关闭前持久化一次
			m.persistMetrics()
			return
		}
	}
}

// persistMetrics 持久化指标到数据库
func (m *MetricsCollector) persistMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	date := now.Format("2006-01-02")
	hour := now.Hour()

	for modelID, metrics := range m.metrics {
		// 只持久化有活动的指标
		if time.Since(metrics.LastUpdated) > PersistenceInterval {
			continue
		}

		// 创建或更新 Stats 记录
		var stats model.Stats
		err := m.db.Where("date = ? AND hour = ? AND model = ?", date, hour, modelID).
			First(&stats).Error

		if err == gorm.ErrRecordNotFound {
			// 创建新记录
			stats = model.Stats{
				Date:         date,
				Hour:         hour,
				Model:        modelID,
				RequestCount: 0,
				SuccessCount: int64(metrics.SuccessCount),
				ErrorCount:   int64(metrics.FailureCount),
				AvgLatency:   metrics.EMALatency,
				CreatedAt:    now,
				UpdatedAt:    now,
			}

			// 从滑动窗口计算请求数
			stats.RequestCount = int64(len(metrics.SuccessWindow))

			if err := m.db.Create(&stats).Error; err != nil {
				// 记录错误但继续处理其他指标
				continue
			}
		} else if err == nil {
			// 更新现有记录
			stats.RequestCount = int64(len(metrics.SuccessWindow))
			stats.SuccessCount = int64(metrics.SuccessCount)
			stats.ErrorCount = int64(metrics.FailureCount)
			stats.AvgLatency = metrics.EMALatency
			stats.UpdatedAt = now

			if err := m.db.Save(&stats).Error; err != nil {
				continue
			}
		}
	}
}

// GetAllMetrics 获取所有模型指标
func (m *MetricsCollector) GetAllMetrics() map[string]*ModelMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ModelMetrics, len(m.metrics))
	for modelID, metrics := range m.metrics {
		copy := *metrics
		copy.SuccessWindow = make([]bool, len(metrics.SuccessWindow))
		copySlice(copy.SuccessWindow, metrics.SuccessWindow)
		result[modelID] = &copy
	}
	return result
}

// ResetMetrics 重置指定模型的指标
func (m *MetricsCollector) ResetMetrics(modelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if metrics, exists := m.metrics[modelID]; exists {
		metrics.EMALatency = 0
		metrics.SuccessCount = 0
		metrics.FailureCount = 0
		metrics.SuccessWindow = make([]bool, 0, SuccessWindowSize)
		metrics.HealthScore = DefaultHealthScore
		metrics.Variance = 0
		metrics.LastUpdated = time.Now()
	}
}

// Close 关闭指标收集器
func (m *MetricsCollector) Close() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

// copySlice 辅助函数: 复制切片
func copySlice(dst, src []bool) {
	for i := range src {
		dst[i] = src[i]
	}
}

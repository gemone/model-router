package model

import (
	"time"
)

// ModelMetrics 模型性能指标
type ModelMetrics struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	ProviderID      string    `json:"provider_id" gorm:"index"`
	ModelName       string    `json:"model_name" gorm:"index"`

	// 性能指标
	HealthScore     float64   `json:"health_score" gorm:"default:100"` // 健康分数 0-100
	AvgLatencyMs    int64     `json:"avg_latency_ms"`                  // 平均延迟（毫秒）
	P95LatencyMs    int64     `json:"p95_latency_ms"`                  // P95 延迟
	P99LatencyMs    int64     `json:"p99_latency_ms"`                  // P99 延迟

	// 可靠性指标
	SuccessRate     float64   `json:"success_rate" gorm:"default:1"`   // 成功率 0-1
	ErrorCount      int64     `json:"error_count"`                     // 错误计数
	TimeoutCount    int64     `json:"timeout_count"`                   // 超时计数
	TotalRequests   int64     `json:"total_requests"`                  // 总请求数

	// 资源使用
	AvgPromptTokens int       `json:"avg_prompt_tokens"`              // 平均输入 token
	AvgOutputTokens int       `json:"avg_output_tokens"`              // 平均输出 token
	TotalTokens     int64     `json:"total_tokens"`                    // 总 token 数

	// 时间窗口
	WindowStart     time.Time `json:"window_start"`                   // 统计窗口开始时间
	WindowEnd       time.Time `json:"window_end"`                     // 统计窗口结束时间

	// 状态
	IsHealthy       bool      `json:"is_healthy" gorm:"default:true"` // 是否健康
	LastCheckAt     time.Time `json:"last_check_at"`                  // 最后检查时间

	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// IsAvailable 检查模型是否可用
func (m *ModelMetrics) IsAvailable() bool {
	if !m.IsHealthy {
		return false
	}
	// 健康分数阈值
	if m.HealthScore < 50 {
		return false
	}
	// 成功率阈值
	if m.SuccessRate < 0.5 && m.TotalRequests > 10 {
		return false
	}
	return true
}

// UpdateHealth 更新健康分数
func (m *ModelMetrics) UpdateHealth(success bool, latencyMs int64) {
	m.TotalRequests++
	if success {
		// Calculate success count from current success rate
		successCount := int64(float64(m.TotalRequests-1) * m.SuccessRate)
		successCount++
		m.SuccessRate = float64(successCount) / float64(m.TotalRequests)
	} else {
		m.ErrorCount++
		// Update success rate
		successCount := int64(float64(m.TotalRequests-1) * m.SuccessRate)
		m.SuccessRate = float64(successCount) / float64(m.TotalRequests)
	}

	// 更新平均延迟
	totalLatency := m.AvgLatencyMs*int64(m.TotalRequests-1) + latencyMs
	m.AvgLatencyMs = totalLatency / m.TotalRequests

	// 计算健康分数（基于成功率和延迟）
	// 成功率权重 70%，延迟权重 30%
	healthFromSuccess := m.SuccessRate * 70
	healthFromLatency := 30.0
	if latencyMs > 5000 {
		healthFromLatency = 10
	} else if latencyMs > 2000 {
		healthFromLatency = 20
	}
	m.HealthScore = healthFromSuccess + healthFromLatency

	// 更新健康状态
	m.IsHealthy = m.IsAvailable()
	m.LastCheckAt = time.Now()
}

// SuccessCount 成功计数器（内部使用）
type SuccessCount struct {
	count int64
}

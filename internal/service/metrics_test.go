package service

import (
	"testing"
	"time"

	"github.com/gemone/model-router/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate required tables
	if err := db.AutoMigrate(&model.Stats{}); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestNewMetricsCollector(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	if mc == nil {
		t.Fatal("Expected non-nil MetricsCollector")
	}

	if mc.metrics == nil {
		t.Fatal("Expected initialized metrics map")
	}
}

func TestRecordRequest(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	modelID := "test-provider:test-model"
	latency := 100 * time.Millisecond

	// Record successful request
	mc.RecordRequest(modelID, latency, true)

	metrics := mc.GetMetrics(modelID)
	if metrics == nil {
		t.Fatal("Expected metrics to be recorded")
	}

	if metrics.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount=1, got %d", metrics.SuccessCount)
	}

	if metrics.FailureCount != 0 {
		t.Errorf("Expected FailureCount=0, got %d", metrics.FailureCount)
	}

	if len(metrics.SuccessWindow) != 1 {
		t.Errorf("Expected SuccessWindow length=1, got %d", len(metrics.SuccessWindow))
	}

	if !metrics.SuccessWindow[0] {
		t.Error("Expected first window entry to be true (success)")
	}
}

func TestRecordFailure(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	modelID := "test-provider:test-model"
	latency := 200 * time.Millisecond

	// Record failed request
	mc.RecordRequest(modelID, latency, false)

	metrics := mc.GetMetrics(modelID)
	if metrics == nil {
		t.Fatal("Expected metrics to be recorded")
	}

	if metrics.SuccessCount != 0 {
		t.Errorf("Expected SuccessCount=0, got %d", metrics.SuccessCount)
	}

	if metrics.FailureCount != 1 {
		t.Errorf("Expected FailureCount=1, got %d", metrics.FailureCount)
	}

	if metrics.SuccessWindow[0] {
		t.Error("Expected first window entry to be false (failure)")
	}
}

func TestEMALatency(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	modelID := "test-provider:test-model"

	// Record multiple requests with different latencies
	latencies := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
	}

	for _, latency := range latencies {
		mc.RecordRequest(modelID, latency, true)
	}

	metrics := mc.GetMetrics(modelID)
	if metrics == nil {
		t.Fatal("Expected metrics to be recorded")
	}

	// EMA should be somewhere between min and max latencies
	minLatency := float64(100)
	maxLatency := float64(200)

	if metrics.EMALatency < minLatency || metrics.EMALatency > maxLatency {
		t.Errorf("Expected EMA latency between %.2f and %.2f, got %.2f",
			minLatency, maxLatency, metrics.EMALatency)
	}
}

func TestSuccessRate(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	modelID := "test-provider:test-model"

	// Record 5 successes and 3 failures
	for i := 0; i < 5; i++ {
		mc.RecordRequest(modelID, 100*time.Millisecond, true)
	}
	for i := 0; i < 3; i++ {
		mc.RecordRequest(modelID, 100*time.Millisecond, false)
	}

	successRate := mc.GetSuccessRate(modelID)
	expectedRate := 5.0 / 8.0

	if successRate != expectedRate {
		t.Errorf("Expected success rate %.2f, got %.2f", expectedRate, successRate)
	}
}

func TestHealthScore(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	modelID := "test-provider:test-model"

	// Record successful requests with low latency
	for i := 0; i < 10; i++ {
		mc.RecordRequest(modelID, 50*time.Millisecond, true)
	}

	metrics := mc.GetMetrics(modelID)
	if metrics == nil {
		t.Fatal("Expected metrics to be recorded")
	}

	// Health score should be high (close to 100) for low latency, high success rate
	if metrics.HealthScore < 80 {
		t.Errorf("Expected health score > 80 for low latency + high success, got %.2f", metrics.HealthScore)
	}

	// Now record some failures with high latency
	for i := 0; i < 5; i++ {
		mc.RecordRequest(modelID, 500*time.Millisecond, false)
	}

	metrics = mc.GetMetrics(modelID)
	if metrics == nil {
		t.Fatal("Expected metrics to be recorded")
	}

	// Health score should decrease
	if metrics.HealthScore > 90 {
		t.Errorf("Expected health score to decrease after failures, got %.2f", metrics.HealthScore)
	}
}

func TestSlidingWindow(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	modelID := "test-provider:test-model"

	// Fill window beyond size (SuccessWindowSize = 100)
	for i := 0; i < 150; i++ {
		success := i < 100 // First 100 are successes
		mc.RecordRequest(modelID, 100*time.Millisecond, success)
	}

	metrics := mc.GetMetrics(modelID)
	if metrics == nil {
		t.Fatal("Expected metrics to be recorded")
	}

	// Window should be capped at SuccessWindowSize
	if len(metrics.SuccessWindow) != SuccessWindowSize {
		t.Errorf("Expected window size %d, got %d", SuccessWindowSize, len(metrics.SuccessWindow))
	}

	// Last 50 should be failures, so success rate should be ~0.5
	successRate := mc.GetSuccessRate(modelID)
	if successRate < 0.4 || successRate > 0.6 {
		t.Errorf("Expected success rate ~0.5, got %.2f", successRate)
	}
}

func TestResetMetrics(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	modelID := "test-provider:test-model"

	// Record some data
	mc.RecordRequest(modelID, 100*time.Millisecond, true)
	mc.RecordRequest(modelID, 200*time.Millisecond, false)

	// Reset
	mc.ResetMetrics(modelID)

	metrics := mc.GetMetrics(modelID)
	if metrics == nil {
		t.Fatal("Expected metrics to still exist")
	}

	if metrics.SuccessCount != 0 {
		t.Errorf("Expected SuccessCount=0 after reset, got %d", metrics.SuccessCount)
	}

	if metrics.FailureCount != 0 {
		t.Errorf("Expected FailureCount=0 after reset, got %d", metrics.FailureCount)
	}

	if len(metrics.SuccessWindow) != 0 {
		t.Errorf("Expected empty SuccessWindow after reset, got %d entries", len(metrics.SuccessWindow))
	}

	if metrics.HealthScore != DefaultHealthScore {
		t.Errorf("Expected HealthScore=%.2f after reset, got %.2f", DefaultHealthScore, metrics.HealthScore)
	}
}

func TestGetAllMetrics(t *testing.T) {
	db := setupTestDB(t)
	mc := NewMetricsCollector(db)
	defer mc.Close()

	// Record metrics for multiple models
	models := []string{
		"provider1:model1",
		"provider1:model2",
		"provider2:model1",
	}

	for _, modelID := range models {
		mc.RecordRequest(modelID, 100*time.Millisecond, true)
	}

	allMetrics := mc.GetAllMetrics()
	if len(allMetrics) != len(models) {
		t.Errorf("Expected %d models, got %d", len(models), len(allMetrics))
	}

	for _, modelID := range models {
		if _, exists := allMetrics[modelID]; !exists {
			t.Errorf("Expected metrics for model %s", modelID)
		}
	}
}

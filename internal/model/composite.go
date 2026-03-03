package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// CompositeAutoModel 定义组合模型
type CompositeAutoModel struct {
	ID        string           `json:"id" gorm:"primaryKey;size:255"`
	ProfileID string           `json:"profile_id" gorm:"index:idx_composite_auto_model_profile;size:255"` // Deprecated: Will be removed in v4 when profile refactoring is complete
	Name      string           `json:"name" gorm:"index:idx_composite_auto_model_name;size:255"`
	Models    []ModelReference `json:"models" gorm:"serializer:json"`
	Priority  int              `json:"priority" gorm:"default:1"`
	Enabled   bool             `json:"enabled" gorm:"default:true;index:idx_composite_auto_model_enabled"`

	// Configuration
	HealthThreshold float64 `json:"health_threshold" gorm:"default:70.0"`
	FallbackPolicy  string  `json:"fallback_policy" gorm:"default:'same_model';size:50"`

	// Routing configuration
	Strategy      CompositeStrategy       `json:"strategy" gorm:"type:text"`
	RoutingRules  []CompositeRoutingRule  `json:"routing_rules" gorm:"type:text"` // JSON encoded

	// Backend models (ordered list for cascade/fallback)
	BackendModels []CompositeBackendModel `json:"backend_models" gorm:"type:text"` // JSON encoded

	// Aggregation config (for parallel mode)
	Aggregation   *CompositeAggregation    `json:"aggregation" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ModelReference represents a model with optional provider specification
type ModelReference struct {
	ModelName  string `json:"model_name" gorm:"size:255" yaml:"model_name"`
	ProviderID string `json:"provider_id,omitempty" gorm:"size:255" yaml:"provider_id,omitempty"`
}

// CompositeStrategy defines routing approach for composite models
type CompositeStrategy string

const (
	CompositeStrategyContent  CompositeStrategy = "content"  // Analyze content, route to best model
	CompositeStrategyRule     CompositeStrategy = "rule"     // Use rule-based matching
	CompositeStrategyParallel CompositeStrategy = "parallel" // Send to all, aggregate results
	CompositeStrategyCascade  CompositeStrategy = "cascade"  // Try models in order, return first success
)

// CompositeRoutingRule defines content-based routing rules
type CompositeRoutingRule struct {
	Condition    CompositeCondition `json:"condition"`
	TargetModels []string          `json:"target_models"`
	Weight       int               `json:"weight"`
}

// CompositeCondition defines when to apply a routing rule
type CompositeCondition struct {
	Type     string      `json:"type"`     // token_count, language, keyword, complexity_score
	Operator string      `json:"operator"` // lt, lte, gt, gte, eq, contains, regex
	Value    interface{} `json:"value"`    // Threshold or pattern
	Logic    string      `json:"logic,omitempty"` // AND, OR for combining
}

// CompositeBackendModel represents a physical model in the composite
type CompositeBackendModel struct {
	ModelName  string `json:"model_name"`
	ProviderID string `json:"provider_id"`
	Weight     int    `json:"weight"`
	TimeoutMs  int64  `json:"timeout_ms"` // JSON-safe milliseconds
}

// CompositeAggregation defines how to aggregate parallel responses
type CompositeAggregation struct {
	Method       AggregationMethod `json:"method"`
	WaitStrategy WaitStrategy      `json:"wait_strategy"`
	MinResponses int               `json:"min_responses"`
	MaxWaitMs    int64             `json:"max_wait_ms"` // JSON-safe milliseconds
	JudgeModel   string            `json:"judge_model,omitempty"`
}

// AggregationMethod defines how to aggregate responses
type AggregationMethod string

const (
	AggregationMethodFirst      AggregationMethod = "first"      // First response (supports streaming)
	AggregationMethodAverage    AggregationMethod = "average"    // For embeddings only
	AggregationMethodSynthesize AggregationMethod = "synthesize" // Judge model synthesis (no streaming)
)

// WaitStrategy defines when to return results
type WaitStrategy string

const (
	WaitStrategyAll WaitStrategy = "all" // Wait for all responses
	WaitStrategyAny WaitStrategy = "any" // First successful response
)

// Scan implements sql.Scanner for CompositeRoutingRule
func (c *CompositeRoutingRule) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, c)
}

// Value implements driver.Valuer for CompositeRoutingRule
func (c CompositeRoutingRule) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements sql.Scanner for CompositeBackendModel
func (c *CompositeBackendModel) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, c)
}

// Value implements driver.Valuer for CompositeBackendModel
func (c CompositeBackendModel) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements sql.Scanner for CompositeAggregation
func (c *CompositeAggregation) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, c)
}

// Value implements driver.Valuer for CompositeAggregation
func (c CompositeAggregation) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Validate validates the composite model configuration
func (m *CompositeAutoModel) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Strategy == "" {
		return fmt.Errorf("strategy is required")
	}
	if len(m.BackendModels) == 0 {
		return fmt.Errorf("at least one backend model is required")
	}
	if m.Strategy == CompositeStrategyParallel && m.Aggregation == nil {
		return fmt.Errorf("aggregation config required for parallel strategy")
	}
	return nil
}

package router

import (
	"testing"
	"time"

	"github.com/gemone/model-router/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestRuleEngine_Match(t *testing.T) {
	rules := []model.Rule{
		{
			ID:       "rule-1",
			Name:     "Image requests",
			Priority: 100,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeContent, Field: model.ContentFieldHasImage, Op: model.ConditionOpEQ, Value: "true"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "gpt-4-vision"},
		},
		{
			ID:       "rule-2",
			Name:     "Premium users",
			Priority: 90,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeHeader, Field: "X-User-Tier", Op: model.ConditionOpEQ, Value: "premium"},
			},
			Action: model.RuleAction{Type: model.ActionTypeRoute, Target: "premium-route"},
		},
		{
			ID:       "rule-3",
			Name:     "Business hours",
			Priority: 80,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeTime, Field: model.TimeFieldHour, Op: model.ConditionOpBetween, Value: "9-18"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "gpt-4"},
		},
	}

	engine := NewRuleEngine(rules)

	tests := []struct {
		name     string
		input    *model.RuleEngineInput
		expected bool
		ruleID   string
	}{
		{
			name: "Match image rule",
			input: &model.RuleEngineInput{
				HasImage:    true,
				Headers:     map[string]string{},
				CurrentTime: time.Now(),
			},
			expected: true,
			ruleID:   "rule-1",
		},
		{
			name: "Match premium user rule",
			input: &model.RuleEngineInput{
				HasImage: false,
				Headers: map[string]string{
					"X-User-Tier": "premium",
				},
				CurrentTime: time.Now(),
			},
			expected: true,
			ruleID:   "rule-2",
		},
		{
			name: "No match",
			input: &model.RuleEngineInput{
				HasImage:    false,
				Headers:     map[string]string{},
				CurrentTime: time.Date(2024, 1, 1, 5, 0, 0, 0, time.UTC), // Outside business hours (9-18)
			},
			expected: false,
			ruleID:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.input)
			assert.Equal(t, tt.expected, result.Matched)
			if tt.expected {
				assert.Equal(t, tt.ruleID, result.RuleID)
			}
		})
	}
}

func TestRuleEngine_HeaderConditions(t *testing.T) {
	rules := []model.Rule{
		{
			ID:       "header-exact",
			Name:     "Exact match",
			Priority: 100,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeHeader, Field: "X-API-Key", Op: model.ConditionOpEQ, Value: "secret123"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "premium-model"},
		},
		{
			ID:       "header-contains",
			Name:     "Contains match",
			Priority: 90,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeHeader, Field: "User-Agent", Op: model.ConditionOpContains, Value: "Mobile"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "mobile-model"},
		},
	}

	engine := NewRuleEngine(rules)

	tests := []struct {
		name     string
		headers  map[string]string
		expected string // expected target model
	}{
		{
			name:     "Exact match",
			headers:  map[string]string{"X-API-Key": "secret123"},
			expected: "premium-model",
		},
		{
			name:     "Contains match",
			headers:  map[string]string{"User-Agent": "Mozilla/5.0 Mobile Safari"},
			expected: "mobile-model",
		},
		{
			name:     "No match",
			headers:  map[string]string{"X-Other": "value"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &model.RuleEngineInput{
				Headers:     tt.headers,
				CurrentTime: time.Now(),
			}
			result := engine.Match(input)
			if tt.expected != "" {
				assert.True(t, result.Matched)
				assert.Equal(t, tt.expected, result.Action.Target)
			} else {
				assert.False(t, result.Matched)
			}
		})
	}
}

func TestRuleEngine_ContentConditions(t *testing.T) {
	rules := []model.Rule{
		{
			ID:       "long-text",
			Name:     "Long text",
			Priority: 100,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeContent, Field: model.ContentFieldTextLength, Op: model.ConditionOpGT, Value: "1000"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "long-context-model"},
		},
		{
			ID:       "many-messages",
			Name:     "Many messages",
			Priority: 90,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeContent, Field: model.ContentFieldMessageCount, Op: model.ConditionOpGTE, Value: "10"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "chat-model"},
		},
	}

	engine := NewRuleEngine(rules)

	tests := []struct {
		name          string
		textLength    int
		messageCount  int
		expectedMatch bool
	}{
		{
			name:          "Long text matches",
			textLength:    1500,
			messageCount:  1,
			expectedMatch: true,
		},
		{
			name:          "Many messages matches",
			textLength:    100,
			messageCount:  15,
			expectedMatch: true,
		},
		{
			name:          "No match",
			textLength:    500,
			messageCount:  5,
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &model.RuleEngineInput{
				TextLength:   tt.textLength,
				MessageCount: tt.messageCount,
				CurrentTime:  time.Now(),
			}
			result := engine.Match(input)
			assert.Equal(t, tt.expectedMatch, result.Matched)
		})
	}
}

func TestRuleEngine_TimeConditions(t *testing.T) {
	rules := []model.Rule{
		{
			ID:       "business-hours",
			Name:     "Business hours",
			Priority: 100,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeTime, Field: model.TimeFieldHour, Op: model.ConditionOpBetween, Value: "9-18"},
			},
			Action: model.RuleAction{Type: model.ActionTypeRoute, Target: "business-route"},
		},
	}

	engine := NewRuleEngine(rules)

	tests := []struct {
		name          string
		hour          int
		expectedMatch bool
	}{
		{
			name:          "Within business hours",
			hour:          14,
			expectedMatch: true,
		},
		{
			name:          "At start of business hours",
			hour:          9,
			expectedMatch: true,
		},
		{
			name:          "At end of business hours",
			hour:          18,
			expectedMatch: true,
		},
		{
			name:          "Outside business hours",
			hour:          20,
			expectedMatch: false,
		},
		{
			name:          "Early morning",
			hour:          5,
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &model.RuleEngineInput{
				CurrentTime: time.Date(2024, 1, 1, tt.hour, 0, 0, 0, time.UTC),
			}
			result := engine.Match(input)
			assert.Equal(t, tt.expectedMatch, result.Matched)
		})
	}
}

func TestRuleEngine_PriorityOrdering(t *testing.T) {
	// 低优先级的规则在前，高优先级在后
	rules := []model.Rule{
		{
			ID:       "low-priority",
			Name:     "Low Priority",
			Priority: 10,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeContent, Field: model.ContentFieldHasImage, Op: model.ConditionOpEQ, Value: "true"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "low-model"},
		},
		{
			ID:       "high-priority",
			Name:     "High Priority",
			Priority: 100,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeContent, Field: model.ContentFieldHasImage, Op: model.ConditionOpEQ, Value: "true"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "high-model"},
		},
	}

	engine := NewRuleEngine(rules)

	input := &model.RuleEngineInput{
		HasImage:    true,
		CurrentTime: time.Now(),
	}

	result := engine.Match(input)
	assert.True(t, result.Matched)
	// 应该匹配高优先级规则
	assert.Equal(t, "high-model", result.Action.Target)
}

func TestRuleEngine_DisabledRules(t *testing.T) {
	rules := []model.Rule{
		{
			ID:       "enabled-rule",
			Name:     "Enabled",
			Priority: 100,
			Enabled:  true,
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeContent, Field: model.ContentFieldHasImage, Op: model.ConditionOpEQ, Value: "true"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "enabled-model"},
		},
		{
			ID:       "disabled-rule",
			Name:     "Disabled",
			Priority: 90,
			Enabled:  false, // 禁用
			Conditions: model.RuleConditions{
				{Type: model.ConditionTypeContent, Field: model.ContentFieldHasImage, Op: model.ConditionOpEQ, Value: "true"},
			},
			Action: model.RuleAction{Type: model.ActionTypeModel, Target: "disabled-model"},
		},
	}

	engine := NewRuleEngine(rules)

	input := &model.RuleEngineInput{
		HasImage:    true,
		CurrentTime: time.Now(),
	}

	result := engine.Match(input)
	assert.True(t, result.Matched)
	// 应该匹配启用的规则
	assert.Equal(t, "enabled-model", result.Action.Target)
}

func TestRuleEngine_NoConditions(t *testing.T) {
	rules := []model.Rule{
		{
			ID:         "always-match",
			Name:       "Always Match",
			Priority:   1,
			Enabled:    true,
			Conditions: model.RuleConditions{}, // 无条件
			Action: model.RuleAction{
				Type:    model.ActionTypeAddHeader,
				Headers: map[string]string{"X-Custom": "value"},
			},
		},
	}

	engine := NewRuleEngine(rules)

	input := &model.RuleEngineInput{
		CurrentTime: time.Now(),
	}

	result := engine.Match(input)
	assert.True(t, result.Matched)
	assert.Equal(t, "always-match", result.RuleID)
}

func TestMergeHeaders(t *testing.T) {
	baseHeaders := map[string]string{
		"Content-Type": "application/json",
		"X-Base":       "base",
	}

	ruleHeaders := map[string]string{
		"X-Custom": "custom",
		"X-Base":   "overridden", // 应该覆盖基础请求头
	}

	// Test Add Header
	result := MergeHeaders(baseHeaders, ruleHeaders, model.ActionTypeAddHeader)
	assert.Equal(t, "application/json", result["Content-Type"])
	assert.Equal(t, "custom", result["X-Custom"])
	// Set 操作不应该覆盖已存在的
	assert.Equal(t, "base", result["X-Base"])

	// Test Set Header
	result = MergeHeaders(baseHeaders, ruleHeaders, model.ActionTypeSetHeader)
	assert.Equal(t, "application/json", result["Content-Type"])
	assert.Equal(t, "custom", result["X-Custom"])
	// Set 操作应该覆盖已存在的
	assert.Equal(t, "overridden", result["X-Base"])
}

func TestApplyBodyModifications(t *testing.T) {
	body := map[string]interface{}{
		"model": "gpt-4",
	}

	action := model.RuleAction{
		Type:   model.ActionTypeModifyBody,
		Target: "temperature",
		Value:  "0.5",
	}

	result := ApplyBodyModifications(body, action)
	assert.Equal(t, "gpt-4", result["model"])
	// 0.5 被解析为 float64(0.5)，不是 int

	// Test float value
	action.Value = "0.75"
	result = ApplyBodyModifications(body, action)
	assert.Equal(t, 0.75, result["temperature"])
}

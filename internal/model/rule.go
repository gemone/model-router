package model

import (
	"encoding/json"
	"time"
)

// Rule 定义路由规则
type Rule struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	ProfileID   string    `json:"profile_id" gorm:"index"` // 所属 Profile
	Name        string    `json:"name"`                    // 规则名称
	Description string    `json:"description"`             // 规则描述
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	Priority    int       `json:"priority" gorm:"default:0"` // 优先级，数值越高越优先

	// 匹配条件
	Conditions RuleConditions `json:"conditions" gorm:"serializer:json"`

	// 执行动作
	Action RuleAction `json:"action" gorm:"serializer:json"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RuleConditions 规则条件列表
type RuleConditions []RuleCondition

// RuleCondition 单个规则条件
type RuleCondition struct {
	Type  string `json:"type"`  // 条件类型: header/body_param/content/time/model
	Field string `json:"field"` // 字段名
	Op    string `json:"op"`    // 操作符: eq/neq/contains/regex/gt/lt/between/in
	Value string `json:"value"` // 比较值
}

// RuleAction 规则动作
type RuleAction struct {
	Type     string            `json:"type"`     // 动作类型: models/model/add_header/modify_body/add_param
	Target   string            `json:"target"`   // 目标模型（当 Type=model 时使用）
	Targets  []RuleTarget      `json:"targets"`  // 目标模型列表（当 Type=models 时使用，支持负载均衡）
	Strategy string            `json:"strategy"` // 负载均衡策略: auto/priority/weighted/random（当 Type=models 时使用）
	Value    string            `json:"value"`    // 修改值
	Headers  map[string]string `json:"headers"`  // 要添加的请求头
}

// RuleTarget 规则目标模型（带权重和优先级）
type RuleTarget struct {
	ModelID  string `json:"model_id"` // 模型ID或名称
	Weight   int    `json:"weight"`   // 权重（用于加权轮询），默认 100
	Priority int    `json:"priority"` // 优先级（数值越高越优先），默认 0
	Enabled  bool   `json:"enabled"`  // 是否启用，默认 true
}

// Normalize 设置默认值
func (t *RuleTarget) Normalize() {
	if t.Weight == 0 {
		t.Weight = 100
	}
	// Priority 默认为 0，无需设置
}

// Condition Type 常量
const (
	ConditionTypeHeader    = "header"     // HTTP 请求头
	ConditionTypeBodyParam = "body_param" // 请求体参数
	ConditionTypeContent   = "content"    // 内容特征
	ConditionTypeTime      = "time"       // 时间条件
	ConditionTypeModel     = "model"      // 模型名称模式
	ConditionTypeQuery     = "query"      // URL 查询参数
)

// Condition Op 常量
const (
	ConditionOpEQ       = "eq"       // 等于
	ConditionOpNEQ      = "neq"      // 不等于
	ConditionOpContains = "contains" // 包含
	ConditionOpRegex    = "regex"    // 正则匹配
	ConditionOpGT       = "gt"       // 大于
	ConditionOpLT       = "lt"       // 小于
	ConditionOpGTE      = "gte"      // 大于等于
	ConditionOpLTE      = "lte"      // 小于等于
	ConditionOpBetween  = "between"  // 范围 (格式: "min-max")
	ConditionOpIn       = "in"       // 在列表中 (逗号分隔)
	ConditionOpNotIn    = "not_in"   // 不在列表中
)

// Action Type 常量
const (
	ActionTypeModels     = "models"      // 负载均衡到多个模型
	ActionTypeModel      = "model"       // 使用指定模型
	ActionTypeAddHeader  = "add_header"  // 添加请求头
	ActionTypeSetHeader  = "set_header"  // 设置请求头
	ActionTypeModifyBody = "modify_body" // 修改请求体
	ActionTypeAddParam   = "add_param"   // 添加查询参数
	ActionTypeRoute      = "route"       // 路由到指定路由（兼容旧版）
)

// Content Field 常量 (用于 content 类型条件)
const (
	ContentFieldHasImage     = "has_image"     // 是否包含图片
	ContentFieldImageCount   = "image_count"   // 图片数量
	ContentFieldTextLength   = "text_length"   // 文本长度
	ContentFieldMessageCount = "message_count" // 消息数量
	ContentFieldHasFunction  = "has_function"  // 是否包含函数调用
	ContentFieldLanguage     = "language"      // 检测到的语言
)

// Time Field 常量 (用于 time 类型条件)
const (
	TimeFieldHour    = "hour"    // 小时 (0-23)
	TimeFieldWeekday = "weekday" // 星期 (0-6, 0=周日)
	TimeFieldMonth   = "month"   // 月份 (1-12)
)

// RuleMatchResult 规则匹配结果
type RuleMatchResult struct {
	Matched  bool              `json:"matched"`
	RuleID   string            `json:"rule_id"`
	RuleName string            `json:"rule_name"`
	Action   RuleAction        `json:"action"`
	Headers  map[string]string `json:"headers"` // 需要添加的请求头
}

// RuleEngineInput 规则引擎输入
type RuleEngineInput struct {
	Headers       map[string]string             `json:"headers"`
	QueryParams   map[string]string             `json:"query_params"`
	Body          map[string]interface{}        `json:"body"`
	Messages      []Message                     `json:"messages"`
	ModelName     string                        `json:"model_name"`
	HasImage      bool                          `json:"has_image"`
	ImageCount    int                           `json:"image_count"`
	TextLength    int                           `json:"text_length"`
	MessageCount  int                           `json:"message_count"`
	HasFunction   bool                          `json:"has_function"`
	CurrentTime   time.Time                     `json:"current_time"`
}

// NewRuleEngineInput 从请求创建规则引擎输入
func NewRuleEngineInput() *RuleEngineInput {
	return &RuleEngineInput{
		Headers:     make(map[string]string),
		QueryParams: make(map[string]string),
		Body:        make(map[string]interface{}),
		Messages:    make([]Message, 0),
		CurrentTime: time.Now(),
	}
}

// WithHeaders 设置请求头
func (r *RuleEngineInput) WithHeaders(headers map[string]string) *RuleEngineInput {
	r.Headers = headers
	return r
}

// WithBody 设置请求体
func (r *RuleEngineInput) WithBody(body map[string]interface{}) *RuleEngineInput {
	r.Body = body
	return r
}

// WithMessages 设置消息列表
func (r *RuleEngineInput) WithMessages(messages []Message) *RuleEngineInput {
	r.Messages = messages
	r.MessageCount = len(messages)

	// 分析消息内容
	var textLength int
	hasImage := false
	imageCount := 0

	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case string:
			textLength += len(content)
		case []interface{}:
			// 多模态内容
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					if partType, ok := partMap["type"].(string); ok {
						if partType == "image_url" {
							hasImage = true
							imageCount++
						} else if partType == "text" {
							if text, ok := partMap["text"].(string); ok {
								textLength += len(text)
							}
						}
					}
				}
			}
		}
	}

	r.TextLength = textLength
	r.HasImage = hasImage
	r.ImageCount = imageCount

	return r
}

// WithModelName 设置模型名称
func (r *RuleEngineInput) WithModelName(modelName string) *RuleEngineInput {
	r.ModelName = modelName
	return r
}

// Value 实现 driver.Valuer 接口
func (rc RuleConditions) DBValue() (interface{}, error) {
	return json.Marshal(rc)
}

// Scan 实现 sql.Scanner 接口
func (rc *RuleConditions) Scan(value interface{}) error {
	if value == nil {
		*rc = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, rc)
}

// Value 实现 driver.Valuer 接口
func (ra RuleAction) DBValue() (interface{}, error) {
	return json.Marshal(ra)
}

// Scan 实现 sql.Scanner 接口
func (ra *RuleAction) Scan(value interface{}) error {
	if value == nil {
		*ra = RuleAction{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, ra)
}

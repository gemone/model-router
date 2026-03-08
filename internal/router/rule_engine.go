package router

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/model"
)

// RuleEngine 规则引擎
type RuleEngine struct {
	rules []model.Rule
}

// NewRuleEngine 创建新的规则引擎
func NewRuleEngine(rules []model.Rule) *RuleEngine {
	// 按优先级排序（高优先级在前）
	sortedRules := make([]model.Rule, len(rules))
	copy(sortedRules, rules)

	// 简单的冒泡排序（规则数量通常不多）
	for i := 0; i < len(sortedRules); i++ {
		for j := i + 1; j < len(sortedRules); j++ {
			if sortedRules[j].Priority > sortedRules[i].Priority {
				sortedRules[i], sortedRules[j] = sortedRules[j], sortedRules[i]
			}
		}
	}

	return &RuleEngine{
		rules: sortedRules,
	}
}

// Match 执行规则匹配
func (e *RuleEngine) Match(input *model.RuleEngineInput) *model.RuleMatchResult {
	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		if e.matchesConditions(rule.Conditions, input) {
			headers := make(map[string]string)

			// 处理动作
			switch rule.Action.Type {
			case model.ActionTypeAddHeader, model.ActionTypeSetHeader:
				for k, v := range rule.Action.Headers {
					headers[k] = v
				}
			}

			return &model.RuleMatchResult{
				Matched:  true,
				RuleID:   rule.ID,
				RuleName: rule.Name,
				Action:   rule.Action,
				Headers:  headers,
			}
		}
	}

	return &model.RuleMatchResult{Matched: false}
}

// matchesConditions 检查所有条件是否匹配
func (e *RuleEngine) matchesConditions(conditions model.RuleConditions, input *model.RuleEngineInput) bool {
	if len(conditions) == 0 {
		return true // 无条件时总是匹配
	}

	for _, condition := range conditions {
		if !e.matchesCondition(&condition, input) {
			return false
		}
	}

	return true
}

// matchesCondition 检查单个条件
func (e *RuleEngine) matchesCondition(condition *model.RuleCondition, input *model.RuleEngineInput) bool {
	switch condition.Type {
	case model.ConditionTypeHeader:
		return e.matchesHeaderCondition(condition, input)
	case model.ConditionTypeBodyParam:
		return e.matchesBodyParamCondition(condition, input)
	case model.ConditionTypeContent:
		return e.matchesContentCondition(condition, input)
	case model.ConditionTypeTime:
		return e.matchesTimeCondition(condition, input)
	case model.ConditionTypeModel:
		return e.matchesModelCondition(condition, input)
	case model.ConditionTypeQuery:
		return e.matchesQueryCondition(condition, input)
	default:
		return false
	}
}

// matchesHeaderCondition 匹配请求头条件
func (e *RuleEngine) matchesHeaderCondition(condition *model.RuleCondition, input *model.RuleEngineInput) bool {
	headerValue, exists := input.Headers[condition.Field]
	if !exists {
		return false
	}

	return e.compareValues(headerValue, condition.Value, condition.Op)
}

// matchesBodyParamCondition 匹配请求体参数条件
func (e *RuleEngine) matchesBodyParamCondition(condition *model.RuleCondition, input *model.RuleEngineInput) bool {
	paramValue, exists := input.Body[condition.Field]
	if !exists {
		return false
	}

	valueStr := fmt.Sprintf("%v", paramValue)
	return e.compareValues(valueStr, condition.Value, condition.Op)
}

// matchesContentCondition 匹配内容特征条件
func (e *RuleEngine) matchesContentCondition(condition *model.RuleCondition, input *model.RuleEngineInput) bool {
	switch condition.Field {
	case model.ContentFieldHasImage:
		actualValue := strconv.FormatBool(input.HasImage)
		return e.compareValues(actualValue, condition.Value, condition.Op)

	case model.ContentFieldImageCount:
		actualValue := strconv.Itoa(input.ImageCount)
		return e.compareValues(actualValue, condition.Value, condition.Op)

	case model.ContentFieldTextLength:
		actualValue := strconv.Itoa(input.TextLength)
		return e.compareValues(actualValue, condition.Value, condition.Op)

	case model.ContentFieldMessageCount:
		actualValue := strconv.Itoa(input.MessageCount)
		return e.compareValues(actualValue, condition.Value, condition.Op)

	case model.ContentFieldHasFunction:
		actualValue := strconv.FormatBool(input.HasFunction)
		return e.compareValues(actualValue, condition.Value, condition.Op)

	case model.ContentFieldLanguage:
		// 简单语言检测逻辑
		lang := e.detectLanguage(input.Messages)
		return e.compareValues(lang, condition.Value, condition.Op)

	default:
		return false
	}
}

// matchesTimeCondition 匹配时间条件
func (e *RuleEngine) matchesTimeCondition(condition *model.RuleCondition, input *model.RuleEngineInput) bool {
	now := input.CurrentTime
	if now.IsZero() {
		now = time.Now()
	}

	switch condition.Field {
	case model.TimeFieldHour:
		hour := now.Hour()
		return e.compareNumericValues(float64(hour), condition.Value, condition.Op)

	case model.TimeFieldWeekday:
		weekday := int(now.Weekday())
		return e.compareNumericValues(float64(weekday), condition.Value, condition.Op)

	case model.TimeFieldMonth:
		month := int(now.Month())
		return e.compareNumericValues(float64(month), condition.Value, condition.Op)

	default:
		return false
	}
}

// matchesModelCondition 匹配模型名称条件
func (e *RuleEngine) matchesModelCondition(condition *model.RuleCondition, input *model.RuleEngineInput) bool {
	switch condition.Field {
	case "name":
		return e.compareValues(input.ModelName, condition.Value, condition.Op)
	default:
		return false
	}
}

// matchesQueryCondition 匹配查询参数条件
func (e *RuleEngine) matchesQueryCondition(condition *model.RuleCondition, input *model.RuleEngineInput) bool {
	paramValue, exists := input.QueryParams[condition.Field]
	if !exists {
		return false
	}

	return e.compareValues(paramValue, condition.Value, condition.Op)
}

// compareValues 比较字符串值
func (e *RuleEngine) compareValues(actual, expected, op string) bool {
	switch op {
	case model.ConditionOpEQ:
		return actual == expected
	case model.ConditionOpNEQ:
		return actual != expected
	case model.ConditionOpContains:
		return strings.Contains(actual, expected)
	case model.ConditionOpRegex:
		matched, err := regexp.MatchString(expected, actual)
		return err == nil && matched
	case model.ConditionOpIn:
		values := strings.Split(expected, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == actual {
				return true
			}
		}
		return false
	case model.ConditionOpNotIn:
		values := strings.Split(expected, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == actual {
				return false
			}
		}
		return true
	default:
		// 尝试数值比较
		actualNum, actualErr := strconv.ParseFloat(actual, 64)
		expectedNum, expectedErr := strconv.ParseFloat(expected, 64)
		if actualErr == nil && expectedErr == nil {
			return e.compareNumericValuesDirect(actualNum, expectedNum, op)
		}
		return false
	}
}

// compareNumericValues 比较数值（从字符串解析）
func (e *RuleEngine) compareNumericValues(actual float64, expected string, op string) bool {
	switch op {
	case model.ConditionOpBetween:
		parts := strings.Split(expected, "-")
		if len(parts) == 2 {
			min, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			max, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 == nil && err2 == nil {
				return actual >= min && actual <= max
			}
		}
		return false
	default:
		expectedNum, err := strconv.ParseFloat(expected, 64)
		if err != nil {
			return false
		}
		return e.compareNumericValuesDirect(actual, expectedNum, op)
	}
}

// compareNumericValuesDirect 直接比较数值
func (e *RuleEngine) compareNumericValuesDirect(actual, expected float64, op string) bool {
	switch op {
	case model.ConditionOpEQ, "":
		return actual == expected
	case model.ConditionOpNEQ:
		return actual != expected
	case model.ConditionOpGT:
		return actual > expected
	case model.ConditionOpLT:
		return actual < expected
	case model.ConditionOpGTE:
		return actual >= expected
	case model.ConditionOpLTE:
		return actual <= expected
	default:
		return false
	}
}

// detectLanguage 简单语言检测
func (e *RuleEngine) detectLanguage(messages []model.Message) string {
	if len(messages) == 0 {
		return "unknown"
	}

	// 获取最后一条用户消息
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			if content, ok := messages[i].Content.(string); ok {
				lastUserContent = content
				break
			}
		}
	}

	if lastUserContent == "" {
		return "unknown"
	}

	// 简单的启发式检测
	chineseCount := 0
	for _, r := range lastUserContent {
		if r >= '\u4e00' && r <= '\u9fff' {
			chineseCount++
		}
	}

	// 如果中文字符占比超过 10%，认为是中文
	if float64(chineseCount)/float64(len(lastUserContent)) > 0.1 {
		return "zh"
	}

	return "en"
}

// ApplyBodyModifications 应用请求体修改
func ApplyBodyModifications(body map[string]interface{}, action model.RuleAction) map[string]interface{} {
	if body == nil {
		body = make(map[string]interface{})
	}

	switch action.Type {
	case model.ActionTypeModifyBody:
		if action.Target != "" && action.Value != "" {
			// 尝试解析值为合适的类型
			if intVal, err := strconv.ParseInt(action.Value, 10, 64); err == nil {
				body[action.Target] = intVal
			} else if floatVal, err := strconv.ParseFloat(action.Value, 64); err == nil {
				body[action.Target] = floatVal
			} else if boolVal, err := strconv.ParseBool(action.Value); err == nil {
				body[action.Target] = boolVal
			} else {
				body[action.Target] = action.Value
			}
		}
	case model.ActionTypeAddParam:
		if action.Target != "" && action.Value != "" {
			body[action.Target] = action.Value
		}
	}

	return body
}

// MergeHeaders 合并请求头
func MergeHeaders(baseHeaders, ruleHeaders map[string]string, actionType string) map[string]string {
	result := make(map[string]string)

	// 复制基础请求头
	for k, v := range baseHeaders {
		result[k] = v
	}

	// 应用规则请求头
	for k, v := range ruleHeaders {
		switch actionType {
		case model.ActionTypeSetHeader:
			// 设置请求头（覆盖）
			result[k] = v
		case model.ActionTypeAddHeader:
			// 添加请求头（如果不存在）
			if _, exists := result[k]; !exists {
				result[k] = v
			}
		default:
			result[k] = v
		}
	}

	return result
}

package model

import (
	"time"
)

// TemplateCategory 定义模板类别
const (
	TemplateCategorySystem     = "system"     // 系统提示词
	TemplateCategoryUser       = "user"       // 用户提示词
	TemplateCategorySynthesis  = "synthesis"  // 合成相关
	TemplateCategorySummary    = "summary"    // 摘要相关
	TemplateCategoryCompression = "compression" // 压缩相关
)

// TemplateScope 定义模板作用域
const (
	TemplateScopeGlobal  = "global"  // 全局模板
	TemplateScopeProfile = "profile" // Profile 级别模板
)

// PromptTemplate 定义可自定义的提示词模板
type PromptTemplate struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex:idx_template_name_scope"`    // 模板名称，唯一标识
	Category    string    `json:"category" gorm:"index"`                              // 模板类别
	Scope       string    `json:"scope" gorm:"index;uniqueIndex:idx_template_name_scope"` // 作用域：global/profile
	ProfileID   string    `json:"profile_id" gorm:"index;uniqueIndex:idx_template_name_scope"` // Profile ID，全局模板为空
	Description string    `json:"description"`                                        // 模板描述
	Content     string    `json:"content" gorm:"type:text"`                          // 模板内容（Go Template 格式）
	Variables   []string  `json:"variables" gorm:"serializer:json"`                  // 模板变量列表
	Version     int       `json:"version" gorm:"default:1"`                          // 版本号
	IsDefault   bool      `json:"is_default" gorm:"default:false"`                   // 是否为系统默认
	Enabled     bool      `json:"enabled" gorm:"default:true"`                       // 是否启用
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TemplateVariable 定义模板变量元数据
type TemplateVariable struct {
	Name        string `json:"name"`        // 变量名
	Type        string `json:"type"`        // 类型：string, int, array, object
	Required    bool   `json:"required"`    // 是否必填
	Description string `json:"description"` // 变量描述
	Default     string `json:"default"`     // 默认值
}

// TemplateRenderRequest 模板渲染请求
type TemplateRenderRequest struct {
	TemplateName string                 `json:"template_name"` // 模板名称
	ProfileID    string                 `json:"profile_id"`    // Profile ID（可选）
	Variables    map[string]interface{} `json:"variables"`     // 变量值
}

// TemplateRenderResult 模板渲染结果
type TemplateRenderResult struct {
	Content string `json:"content"` // 渲染后的内容
	Version int    `json:"version"` // 使用的模板版本
}

// TemplateName 常量定义 - 所有可用的模板名称
const (
	// Synthesis 相关模板
	TemplateSynthesisSystem        = "synthesis_system"         // 合成系统提示词
	TemplateSynthesisUserPrompt    = "synthesis_user_prompt"    // 合成用户提示词
	
	// Summary 相关模板
	TemplateSummarySystem          = "summary_system"           // 摘要系统提示词
	TemplateSummaryUserPrompt      = "summary_user_prompt"      // 摘要用户提示词
	
	// Cascade Compression 相关模板
	TemplateCascadeExpertSystem    = "cascade_expert_system"    // 级联压缩专家系统提示词
	TemplateCascadeExpertPrompt    = "cascade_expert_prompt"    // 级联压缩专家优化提示词
	TemplateCascadeWorkerPrompt    = "cascade_worker_prompt"    // 级联压缩工作模型提示词
	
	// Sliding Window 相关模板
	TemplateSlidingWindowSystem    = "sliding_window_system"    // 滑动窗口系统提示词
	TemplateSlidingWindowUserPrompt = "sliding_window_user_prompt" // 滑动窗口用户提示词
)

// DefaultTemplates 返回所有默认模板定义
func DefaultTemplates() []PromptTemplate {
	return []PromptTemplate{
		// Synthesis 模板
		{
			Name:        TemplateSynthesisSystem,
			Category:    TemplateCategorySynthesis,
			Scope:       TemplateScopeGlobal,
			Description: "Synthesizer 系统提示词 - 用于将多个压缩块合成为统一上下文",
			Content: `You are an expert at synthesizing compressed conversation summaries into coherent, unified context. 
Preserve all critical information, decisions, and action items while eliminating redundancy.`,
			Variables: []string{},
			IsDefault: true,
			Enabled:   true,
		},
		{
			Name:        TemplateSynthesisUserPrompt,
			Category:    TemplateCategorySynthesis,
			Scope:       TemplateScopeGlobal,
			Description: "Synthesizer 用户提示词模板 - 用于构建合成请求",
			Content: `Synthesize the following compressed conversation chunks into a unified, coherent summary.

Requirements:
- Maximum output: {{.MaxOutputTokens}} tokens
- Preserve all critical information, decisions, and action items
- Eliminate redundancy across chunks
- Maintain chronological flow where relevant
- Organize by topic/theme where appropriate

Compressed chunks:

{{range .Chunks}}
[Chunk {{.Index}} - {{.Role}}]:
{{.Content}}

{{end}}
{{if .Truncated}}
[... additional context truncated ...]
{{end}}
Provide a comprehensive, unified synthesis.`,
			Variables: []string{"MaxOutputTokens", "Chunks", "Truncated"},
			IsDefault: true,
			Enabled:   true,
		},
		
		// Summary 模板
		{
			Name:        TemplateSummarySystem,
			Category:    TemplateCategorySummary,
			Scope:       TemplateScopeGlobal,
			Description: "摘要系统提示词 - 用于对话摘要",
			Content:     `You are a helpful assistant that summarizes conversations concisely while preserving important context, decisions, and action items.`,
			Variables:   []string{},
			IsDefault:   true,
			Enabled:     true,
		},
		{
			Name:        TemplateSummaryUserPrompt,
			Category:    TemplateCategorySummary,
			Scope:       TemplateScopeGlobal,
			Description: "摘要用户提示词模板 - 用于构建摘要请求",
			Content: `Summarize the following conversation history concisely (maximum {{.MaxOutputTokens}} tokens).

Focus on:
- Key topics discussed
- Important decisions made
- Action items or tasks
- Context relevant to ongoing conversation

Conversation:

{{range .Messages}}
[{{.Role}}]: {{.Content}}

{{end}}
{{if .Truncated}}
[... earlier conversation truncated ...]
{{end}}
Provide a concise summary that preserves essential context.`,
			Variables: []string{"MaxOutputTokens", "Messages", "Truncated"},
			IsDefault: true,
			Enabled:   true,
		},
		
		// Cascade Compression 模板
		{
			Name:        TemplateCascadeExpertSystem,
			Category:    TemplateCategoryCompression,
			Scope:       TemplateScopeGlobal,
			Description: "级联压缩专家系统提示词 - 用于优化对话上下文",
			Content: `You are an expert prompt engineer specializing in optimizing LLM conversations for better instruction following and output quality. 
Analyze the conversation and extract key information, reformulate instructions clearly, and organize context logically.`,
			Variables:   []string{},
			IsDefault:   true,
			Enabled:     true,
		},
		{
			Name:        TemplateCascadeExpertPrompt,
			Category:    TemplateCategoryCompression,
			Scope:       TemplateScopeGlobal,
			Description: "级联压缩专家优化提示词模板",
			Content: `# Conversation Optimization Task

Please analyze the following conversation and produce an optimized version that:
1. Preserves all critical information, decisions, and action items
2. Improves instruction clarity and structure
3. Eliminates redundancy while maintaining context
4. Organizes information logically for better LLM understanding

## Original Conversation

{{range .Messages}}
**[{{.Role}}]**: {{.Content}}

{{end}}
{{if .Truncated}}
[... Earlier conversation truncated for optimization ...]
{{end}}
## Optimization Output Format

Please provide the optimized context in the following structure:

### Optimized Context
[Provide a clear, well-structured summary with sections for:]
- Core task/objective
- Key decisions made
- Action items and requirements
- Important constraints or preferences
- Relevant context from earlier discussion`,
			Variables: []string{"Messages", "Truncated"},
			IsDefault: true,
			Enabled:   true,
		},
		{
			Name:        TemplateCascadeWorkerPrompt,
			Category:    TemplateCategoryCompression,
			Scope:       TemplateScopeGlobal,
			Description: "级联压缩工作模型提示词模板",
			Content: `=== Optimized Context ===
{{.OptimizedContext}}

{{if .LastUserMessage}}
=== Current Request ===
{{.LastUserMessage}}
{{end}}
=== Instructions ===
Based on the optimized context above, please provide a helpful and accurate response. 
The context contains all relevant information from the conversation history.`,
			Variables: []string{"OptimizedContext", "LastUserMessage"},
			IsDefault: true,
			Enabled:   true,
		},
	}
}

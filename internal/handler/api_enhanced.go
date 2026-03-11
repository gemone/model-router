package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/router"
	"github.com/gemone/model-router/internal/service"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// EnhancedAPIHandler 增强版 API 处理器
type EnhancedAPIHandler struct {
	*APIHandler
	profileManager *service.ProfileManager
}

// NewEnhancedAPIHandler 创建增强版 API 处理器
func NewEnhancedAPIHandler() *EnhancedAPIHandler {
	return &EnhancedAPIHandler{
		APIHandler:     NewAPIHandler(),
		profileManager: service.GetProfileManager(),
	}
}

// RegisterEnhancedRoutes 注册增强版路由
func (h *EnhancedAPIHandler) RegisterEnhancedRoutes(app *fiber.App) {
	// === OpenAI 格式 ===
	// 标准 OpenAI 端点
	app.Post("/api/:profile/v1/chat/completions", h.handleEnhancedChatCompletion)
	app.Post("/api/:profile/v1/embeddings", h.handleEnhancedEmbeddings)
	app.Get("/api/:profile/v1/models", h.handleEnhancedListModels)

	// 显式 OpenAI 格式
	app.Post("/api/openai/:profile/v1/chat/completions", h.handleEnhancedChatCompletion)
	app.Post("/api/openai/:profile/v1/embeddings", h.handleEnhancedEmbeddings)
	app.Get("/api/openai/:profile/v1/models", h.handleEnhancedListModels)

	// === Claude/Anthropic 格式 ===
	app.Post("/api/claude/:profile/v1/messages", h.handleClaudeFormat)
	app.Post("/api/anthropic/:profile/v1/messages", h.handleClaudeFormat)
	app.Get("/api/claude/:profile/v1/models", h.handleEnhancedListModels)
	app.Get("/api/anthropic/:profile/v1/models", h.handleEnhancedListModels)

	// === Ollama 格式 ===
	// Ollama Chat API
	app.Post("/api/ollama/:profile/api/chat", h.handleOllamaChat)
	// Ollama Generate API
	app.Post("/api/ollama/:profile/api/generate", h.handleOllamaGenerate)
	// Ollama List Models
	app.Get("/api/ollama/:profile/api/tags", h.handleOllamaListModels)
	// Ollama Embeddings
	app.Post("/api/ollama/:profile/api/embeddings", h.handleOllamaEmbeddings)
	// Ollama Pull Model (stub - returns 501)
	app.Post("/api/ollama/:profile/api/pull", h.handleOllamaNotImplemented)
	// Ollama Delete Model (stub - returns 501)
	app.Delete("/api/ollama/:profile/api/delete", h.handleOllamaNotImplemented)

	// === 简写格式 ===
	app.Post("/:profile/v1/chat/completions", h.handleEnhancedChatCompletion)
	app.Post("/:profile/v1/embeddings", h.handleEnhancedEmbeddings)
	app.Get("/:profile/v1/models", h.handleEnhancedListModels)

	// === 默认 Profile ===
	app.Post("/v1/chat/completions", h.handleDefaultEnhancedChatCompletion)
	app.Post("/v1/embeddings", h.handleDefaultEnhancedEmbeddings)
	app.Get("/v1/models", h.handleDefaultEnhancedListModels)
}

// ==================== 增强版 OpenAI 处理 ====================

func (h *EnhancedAPIHandler) handleEnhancedChatCompletion(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return h.processEnhancedChatCompletion(c, profilePath)
}

func (h *EnhancedAPIHandler) handleDefaultEnhancedChatCompletion(c fiber.Ctx) error {
	return h.processEnhancedChatCompletion(c, "")
}

func (h *EnhancedAPIHandler) processEnhancedChatCompletion(c fiber.Ctx, profilePath string) error {
	requestID := uuid.New().String()
	start := time.Now()

	// 获取 Profile
	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// 解析请求体
	var req model.ChatCompletionRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// 构建规则引擎输入
	ruleInput := buildRuleEngineInput(c, &req)

	// 应用规则（如果 Profile 启用了规则）
	ruleResult := h.applyRules(profile, ruleInput)
	if ruleResult.Matched {
		// 应用规则动作
		req = h.applyRuleActions(req, ruleResult)

		// 添加规则相关的请求头
		for k, v := range ruleResult.Headers {
			c.Set(k, v)
		}
	}

	// 路由到目标模型
	routeResult, err := profile.Route(c.Context(), req.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	// 使用 OriginalName 进行实际的 API 调用
	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		req.Model = routeResult.Model.OriginalName
	}

	// 添加自定义请求头
	customHeaders := h.buildCustomHeaders(profile, routeResult, ruleResult)
	for k, v := range customHeaders {
		c.Set(k, v)
	}

	// 执行请求
	if req.Stream {
		return h.handleStreamingResponse(c, routeResult, &req, requestID, start)
	}

	result, err := routeResult.Adapter.ChatCompletion(c.Context(), &req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

func (h *EnhancedAPIHandler) handleStreamingResponse(
	c fiber.Ctx,
	routeResult *service.RouteResult,
	req *model.ChatCompletionRequest,
	_ string,  // requestID - reserved for future logging
	_ time.Time, // start - reserved for future metrics
) error {
	c.Response().Header.SetContentType("text/event-stream")
	c.Response().Header.Set("Cache-Control", "no-cache")
	c.Response().Header.Set("Connection", "keep-alive")

	stream, err := routeResult.Adapter.ChatCompletionStream(c.Context(), req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for resp := range stream {
		data, _ := json.Marshal(resp)
		c.Write([]byte("data: "))
		c.Write(data)
		c.Write([]byte("\n\n"))
	}
	c.Write([]byte("data: [DONE]\n\n"))

	return nil
}

// ==================== Claude/Anthropic 格式处理 ====================

func (h *EnhancedAPIHandler) handleClaudeFormat(c fiber.Ctx) error {
	profilePath := c.Params("profile")

	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 获取 Profile
	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// 解析 Anthropic 请求
	var anthropicReq AnthropicRequest
	if err := c.Bind().Body(&anthropicReq); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body", "details": err.Error()})
	}

	// 转换为 OpenAI 格式
	openAIReq := ConvertAnthropicToOpenAI(&anthropicReq)

	// 构建规则引擎输入
	ruleInput := buildRuleEngineInput(c, openAIReq)
	ruleResult := h.applyRules(profile, ruleInput)
	if ruleResult.Matched {
		*openAIReq = h.applyRuleActions(*openAIReq, ruleResult)
	}

	// 路由并执行请求
	routeResult, err := profile.Route(c.Context(), openAIReq.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		openAIReq.Model = routeResult.Model.OriginalName
	}

	if openAIReq.Stream {
		return h.handleClaudeStreaming(c, routeResult, openAIReq)
	}

	result, err := routeResult.Adapter.ChatCompletion(c.Context(), openAIReq)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 转换回 Anthropic 格式
	anthropicResp := ConvertOpenAIToAnthropic(result)
	return c.JSON(anthropicResp)
}

// ==================== Ollama 格式处理 ====================

func (h *EnhancedAPIHandler) handleOllamaChat(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 获取 Profile
	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(ollamaError("profile not found"))
	}

	// 解析 Ollama 请求
	var ollamaReq OllamaChatRequest
	if err := c.Bind().Body(&ollamaReq); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ollamaError("invalid request body"))
	}

	// 转换为 OpenAI 格式
	openAIReq := convertOllamaChatToOpenAI(&ollamaReq)

	// 构建规则引擎输入
	ruleInput := buildRuleEngineInput(c, openAIReq)
	ruleResult := h.applyRules(profile, ruleInput)
	if ruleResult.Matched {
		*openAIReq = h.applyRuleActions(*openAIReq, ruleResult)
	}

	// 路由并执行请求
	routeResult, err := profile.Route(c.Context(), openAIReq.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(ollamaError(err.Error()))
	}

	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		openAIReq.Model = routeResult.Model.OriginalName
	}

	if ollamaReq.Stream {
		return h.handleOllamaStreaming(c, routeResult, openAIReq, ollamaReq.Model)
	}

	result, err := routeResult.Adapter.ChatCompletion(c.Context(), openAIReq)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ollamaError(err.Error()))
	}

	// 转换回 Ollama 格式
	ollamaResp := convertOpenAIToOllamaChat(result, ollamaReq.Model)
	return c.JSON(ollamaResp)
}

func (h *EnhancedAPIHandler) handleOllamaGenerate(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ollamaError(err.Error()))
	}

	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(ollamaError("profile not found"))
	}

	var ollamaReq OllamaGenerateRequest
	if err := c.Bind().Body(&ollamaReq); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ollamaError("invalid request body"))
	}

	// 转换为 OpenAI 格式（使用 messages 格式模拟）
	openAIReq := convertOllamaGenerateToOpenAI(&ollamaReq)

	ruleInput := buildRuleEngineInput(c, openAIReq)
	ruleResult := h.applyRules(profile, ruleInput)
	if ruleResult.Matched {
		*openAIReq = h.applyRuleActions(*openAIReq, ruleResult)
	}

	routeResult, err := profile.Route(c.Context(), openAIReq.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(ollamaError(err.Error()))
	}

	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		openAIReq.Model = routeResult.Model.OriginalName
	}

	if ollamaReq.Stream {
		return h.handleOllamaStreaming(c, routeResult, openAIReq, ollamaReq.Model)
	}

	result, err := routeResult.Adapter.ChatCompletion(c.Context(), openAIReq)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ollamaError(err.Error()))
	}

	ollamaResp := convertOpenAIToOllamaGenerate(result, ollamaReq.Model)
	return c.JSON(ollamaResp)
}

func (h *EnhancedAPIHandler) handleOllamaStreaming(
	c fiber.Ctx,
	routeResult *service.RouteResult,
	req *model.ChatCompletionRequest,
	modelName string,
) error {
	c.Response().Header.SetContentType("application/x-ndjson")

	stream, err := routeResult.Adapter.ChatCompletionStream(c.Context(), req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ollamaError(err.Error()))
	}

	for resp := range stream {
		ollamaChunk := convertOpenAIStreamToOllama(resp, modelName)
		data, _ := json.Marshal(ollamaChunk)
		c.Write(data)
		c.Write([]byte("\n"))
	}

	return nil
}

func (h *EnhancedAPIHandler) handleOllamaListModels(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ollamaError(err.Error()))
	}

	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(ollamaError("profile not found"))
	}

	models := profile.GetModels()
	ollamaResp := convertModelsToOllamaList(models)

	return c.JSON(ollamaResp)
}

func (h *EnhancedAPIHandler) handleOllamaEmbeddings(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ollamaError(err.Error()))
	}

	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(ollamaError("profile not found"))
	}

	var ollamaReq OllamaEmbeddingRequest
	if err := c.Bind().Body(&ollamaReq); err != nil {
		return c.Status(http.StatusBadRequest).JSON(ollamaError("invalid request body"))
	}

	// 转换为 OpenAI 格式
	openAIReq := &model.EmbeddingRequest{
		Model: ollamaReq.Model,
		Input: ollamaReq.Prompt,
	}

	routeResult, err := profile.Route(c.Context(), openAIReq.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(ollamaError(err.Error()))
	}

	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		openAIReq.Model = routeResult.Model.OriginalName
	}

	result, err := routeResult.Adapter.Embeddings(c.Context(), openAIReq)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ollamaError(err.Error()))
	}

	// 转换回 Ollama 格式
	ollamaResp := convertOpenAIEmbeddingToOllama(result)
	return c.JSON(ollamaResp)
}

func (h *EnhancedAPIHandler) handleOllamaNotImplemented(c fiber.Ctx) error {
	return c.Status(http.StatusNotImplemented).JSON(ollamaError("this endpoint is not implemented"))
}

// ==================== 辅助函数 ====================

func (h *EnhancedAPIHandler) handleEnhancedEmbeddings(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return h.processEnhancedEmbeddings(c, profilePath)
}

func (h *EnhancedAPIHandler) handleDefaultEnhancedEmbeddings(c fiber.Ctx) error {
	return h.processEnhancedEmbeddings(c, "")
}

func (h *EnhancedAPIHandler) processEnhancedEmbeddings(c fiber.Ctx, profilePath string) error {
	var req model.EmbeddingRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	routeResult, err := profile.Route(c.Context(), req.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		req.Model = routeResult.Model.OriginalName
	}

	result, err := routeResult.Adapter.Embeddings(c.Context(), &req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

func (h *EnhancedAPIHandler) handleEnhancedListModels(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return h.processEnhancedListModels(c, profilePath)
}

func (h *EnhancedAPIHandler) handleDefaultEnhancedListModels(c fiber.Ctx) error {
	return h.processEnhancedListModels(c, "")
}

func (h *EnhancedAPIHandler) processEnhancedListModels(c fiber.Ctx, profilePath string) error {
	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	models := profile.GetModels()
	modelInfos := make([]model.ModelInfo, 0, len(models))

	for _, m := range models {
		modelInfos = append(modelInfos, model.ModelInfo{
			ID:      m.Name,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: m.ProviderID,
		})
	}

	return c.JSON(model.ListModelsResponse{
		Object: "list",
		Data:   modelInfos,
	})
}

// ==================== 规则引擎集成 ====================

func (h *EnhancedAPIHandler) applyRules(profile *service.ProfileInstance, input *model.RuleEngineInput) *model.RuleMatchResult {
	// 如果 Profile 有规则配置，应用规则
	if profile.Profile == nil {
		return &model.RuleMatchResult{Matched: false}
	}

	// 从 Profile 的 Settings 中解析规则（临时方案）
	// 实际应该在 Profile 中添加 Rules 字段
	rules := h.loadRulesForProfile(profile.Profile.ID)
	if len(rules) == 0 {
		return &model.RuleMatchResult{Matched: false}
	}

	engine := router.NewRuleEngine(rules)
	return engine.Match(input)
}

func (h *EnhancedAPIHandler) loadRulesForProfile(_ string) []model.Rule {
	// TODO: 从数据库加载规则
	// 临时返回空规则列表
	return []model.Rule{}
}

func (h *EnhancedAPIHandler) applyRuleActions(req model.ChatCompletionRequest, result *model.RuleMatchResult) model.ChatCompletionRequest {
	if !result.Matched {
		return req
	}

	action := result.Action

	switch action.Type {
	case model.ActionTypeModel:
		// 使用规则指定的模型
		if action.Target != "" {
			req.Model = action.Target
		}
	case model.ActionTypeModifyBody:
		// 修改请求体参数
		// 这里可以添加 temperature, max_tokens 等参数的修改
	}

	return req
}

func (h *EnhancedAPIHandler) buildCustomHeaders(
	_ *service.ProfileInstance,
	routeResult *service.RouteResult,
	ruleResult *model.RuleMatchResult,
) map[string]string {
	headers := make(map[string]string)

	// 添加路由信息
	if routeResult.Model != nil {
		headers["X-Routed-Model"] = routeResult.Model.Name
		headers["X-Routed-Provider"] = routeResult.Provider.ID
	}

	// 添加规则信息
	if ruleResult.Matched {
		headers["X-Applied-Rule"] = ruleResult.RuleID
	}

	return headers
}

// ==================== 工具函数 ====================

func buildRuleEngineInput(c fiber.Ctx, req *model.ChatCompletionRequest) *model.RuleEngineInput {
	input := model.NewRuleEngineInput()

	// 收集请求头
	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})
	input.WithHeaders(headers)

	// 分析消息
	if len(req.Messages) > 0 {
		input.WithMessages(req.Messages)
	}

	// 设置模型名称
	input.WithModelName(req.Model)

	// 检查是否有函数调用
	if len(req.Tools) > 0 {
		input.HasFunction = true
	}

	// 解析请求体参数
	body := make(map[string]interface{})
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	input.WithBody(body)

	return input
}

func ollamaError(message string) fiber.Map {
	return fiber.Map{"error": message}
}

// ==================== Ollama 转换辅助函数 ====================

func convertOllamaChatToOpenAI(req *OllamaChatRequest) *model.ChatCompletionRequest {
	messages := make([]model.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		var content any
		content = msg.Content

		// 如果有图片，需要处理多模态内容
		if len(msg.Images) > 0 {
			contentParts := []any{
				map[string]any{"type": "text", "text": msg.Content},
			}
			for _, img := range msg.Images {
				contentParts = append(contentParts, map[string]any{
					"type": "image_url",
					"image_url": map[string]string{
						"url": fmt.Sprintf("data:image/jpeg;base64,%s", img),
					},
				})
			}
			content = contentParts
		}

		messages = append(messages, model.Message{
			Role:    msg.Role,
			Content: content,
		})
	}

	openAIReq := &model.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   req.Stream,
	}

	// 转换选项
	if req.Options.Temperature > 0 {
		temp := float32(req.Options.Temperature)
		openAIReq.Temperature = &temp
	}
	if req.Options.NumPredict > 0 {
		openAIReq.MaxTokens = req.Options.NumPredict
	}
	if req.Options.TopP > 0 {
		topP := float32(req.Options.TopP)
		openAIReq.TopP = &topP
	}

	return openAIReq
}

func convertOllamaGenerateToOpenAI(req *OllamaGenerateRequest) *model.ChatCompletionRequest {
	messages := []model.Message{
		{Role: "user", Content: req.Prompt},
	}

	if req.System != "" {
		messages = append([]model.Message{{Role: "system", Content: req.System}}, messages...)
	}

	openAIReq := &model.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   req.Stream,
	}

	if req.Options.Temperature > 0 {
		temp := float32(req.Options.Temperature)
		openAIReq.Temperature = &temp
	}
	if req.Options.NumPredict > 0 {
		openAIReq.MaxTokens = req.Options.NumPredict
	}

	return openAIReq
}

func convertOpenAIToOllamaChat(resp *model.ChatCompletionResponse, modelName string) *OllamaChatResponse {
	content := ""
	if len(resp.Choices) > 0 {
		if c, ok := resp.Choices[0].Message.Content.(string); ok {
			content = c
		}
	}

	return &OllamaChatResponse{
		Model: modelName,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Message: OllamaMessage{
			Role:    "assistant",
			Content: content,
		},
		Done:            true,
		PromptEvalCount: resp.Usage.PromptTokens,
		EvalCount:       resp.Usage.CompletionTokens,
	}
}

func convertOpenAIToOllamaGenerate(resp *model.ChatCompletionResponse, modelName string) *OllamaGenerateResponse {
	content := ""
	if len(resp.Choices) > 0 {
		if c, ok := resp.Choices[0].Message.Content.(string); ok {
			content = c
		}
	}

	return &OllamaGenerateResponse{
		Model:     modelName,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Response:  content,
		Done:      true,
		PromptEvalCount: resp.Usage.PromptTokens,
		EvalCount:       resp.Usage.CompletionTokens,
	}
}

func convertOpenAIStreamToOllama(resp *model.ChatCompletionStreamResponse, modelName string) *OllamaChatResponse {
	content := ""
	done := false
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Delta.Content
		if resp.Choices[0].FinishReason != nil && *resp.Choices[0].FinishReason != "" {
			done = true
		}
	}

	return &OllamaChatResponse{
		Model: modelName,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Message: OllamaMessage{
			Role:    "assistant",
			Content: content,
		},
		Done: done,
	}
}

func convertModelsToOllamaList(models []*model.Model) *OllamaListResponse {
	ollamaModels := make([]OllamaModelInfo, 0, len(models))
	for _, m := range models {
		ollamaModels = append(ollamaModels, OllamaModelInfo{
			Name:       m.Name,
			ModifiedAt: time.Now().UTC().Format(time.RFC3339),
			Size:       0,
		})
	}
	return &OllamaListResponse{Models: ollamaModels}
}

func convertOpenAIEmbeddingToOllama(resp *model.EmbeddingResponse) *OllamaEmbeddingResponse {
	if len(resp.Data) > 0 {
		return &OllamaEmbeddingResponse{
			Embedding: resp.Data[0].Embedding,
		}
	}
	return &OllamaEmbeddingResponse{Embedding: []float32{}}
}

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/middleware"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/service"
	"github.com/gemone/model-router/internal/tokenizer"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var (
	// validProfileNamePattern validates profile names: alphanumeric, underscore, dash, 1-64 chars
	validProfileNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

// APIHandler API 处理器
type APIHandler struct {
	profileManager *service.ProfileManager
	stats          *service.StatsCollector
	debug          bool
	// Shutdown context for graceful termination of background goroutines
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

// NewAPIHandler 创建 API 处理器
func NewAPIHandler() *APIHandler {
	cfg := config.Get()
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	return &APIHandler{
		profileManager: service.GetProfileManager(),
		stats:          service.GetStatsCollector(),
		debug:          cfg.GetLogLevel() == "debug",
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
	}
}

// Shutdown signals all background goroutines to stop
func (h *APIHandler) Shutdown() {
	if h.shutdownCancel != nil {
		h.shutdownCancel()
	}
}

// validateProfilePath validates that a profile path parameter is safe
func validateProfilePath(profilePath string) error {
	if profilePath == "" {
		return nil // Empty is valid (for default profile)
	}
	if len(profilePath) > 64 {
		return fmt.Errorf("profile name too long (max 64 characters)")
	}
	if !validProfileNamePattern.MatchString(profilePath) {
		return fmt.Errorf("profile name contains invalid characters (only alphanumeric, underscore, and dash allowed)")
	}
	return nil
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(app *fiber.App) {
	// Claude/Anthropic 格式 - 必须先注册，避免被通用路由匹配
	app.Post("/api/claude/:profile/v1/messages", h.handleChatCompletionWithFormat)
	app.Post("/api/anthropic/:profile/v1/messages", h.handleChatCompletionWithFormat)
	app.Get("/api/claude/:profile/v1/models", h.handleListModels)
	app.Get("/api/anthropic/:profile/v1/models", h.handleListModels)

	// 递进式格式 /api/{format}/{profile}/...
	app.Post("/api/openai/:profile/v1/chat/completions", h.handleChatCompletion)
	app.Post("/api/openai/:profile/v1/embeddings", h.handleEmbeddings)
	app.Get("/api/openai/:profile/v1/models", h.handleListModels)

	// OpenAI 兼容 API - 通用路由放在最后
	app.Post("/api/:profile/v1/chat/completions", h.handleChatCompletion)
	app.Post("/api/:profile/v1/embeddings", h.handleEmbeddings)
	app.Get("/api/:profile/v1/models", h.handleListModels)

	// 简写格式
	app.Post("/:profile/v1/chat/completions", h.handleChatCompletion)
	app.Post("/:profile/v1/embeddings", h.handleEmbeddings)
	app.Get("/:profile/v1/models", h.handleListModels)

	// 默认 profile（不带 profile 前缀）
	app.Post("/v1/chat/completions", h.handleDefaultChatCompletion)
	app.Post("/v1/embeddings", h.handleDefaultEmbeddings)
	app.Get("/v1/models", h.handleDefaultListModels)
}

// handleChatCompletion 处理聊天完成请求
func (h *APIHandler) handleChatCompletion(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return h.processChatCompletion(c, profilePath)
}

// handleDefaultChatCompletion 处理默认 Profile 的聊天完成
func (h *APIHandler) handleDefaultChatCompletion(c fiber.Ctx) error {
	return h.processChatCompletion(c, "")
}

// processChatCompletion 处理聊天完成
func (h *APIHandler) processChatCompletion(c fiber.Ctx, profilePath string) error {
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

	// 先路由，获取目标模型
	routeResult, err := profile.Route(c.Context(), req.Model)
	if err != nil {
		middleware.ErrorLog("Route failed: requestID=%s model=%s profile=%s error=%v",
			requestID, req.Model, profilePath, err)
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	// 使用 OriginalName 进行实际的 API 调用
	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		req.Model = routeResult.Model.OriginalName
	}

	// Check if this is a composite model and validate streaming support
	if compositeModel, ok := profile.GetCompositeModel(req.Model); ok && compositeModel.Enabled {
		// Streaming is not supported for parallel-synthesize mode
		if req.Stream && compositeModel.Strategy == model.CompositeStrategyParallel &&
			compositeModel.Aggregation != nil && compositeModel.Aggregation.Method == model.AggregationMethodSynthesize {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error":       "streaming is not supported for composite models with parallel-synthesize aggregation",
				"model":       req.Model,
				"strategy":    string(compositeModel.Strategy),
				"aggregation": string(compositeModel.Aggregation.Method),
			})
		}
	}

	// Validate compression group if provided
	if req.CompressionModelGroup != nil && *req.CompressionModelGroup != "" {
		if err := h.validateCompressionGroup(profile, *req.CompressionModelGroup); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error":   "invalid compression model group",
				"details": err.Error(),
				"hint":    "Valid groups can be queried via GET /api/admin/profiles/:id/compression-groups",
			})
		}
	}

	// 智能压缩决策
	var compressionMetadata *model.CompressionMetadata
	shouldCompress := h.shouldApplyCompression(profile.Profile, routeResult.Model, req.Messages)
	if shouldCompress {
		// Create session for compression (minimal required fields)
		session := &model.Session{
			ID:            requestID,
			ContextWindow: profile.Profile.MaxContextWindow,
		}
		compressedMessages, metadata, err := profile.ApplyCompression(c.Context(), session, profile.Profile.MaxContextWindow, req.CompressionModelGroup)
		if err != nil {
			// Log compression error but continue with original messages
			middleware.ErrorLog("Compression error: %v", err)
			c.Set("X-Compression-Status", "failed")
		} else {
			req.Messages = compressedMessages
			compressionMetadata = metadata
		}
	}

	// 执行请求
	var result *model.ChatCompletionResponse
	var streamErr error

	if req.Stream {
		// 流式处理
		c.Response().Header.SetContentType("text/event-stream")
		c.Response().Header.Set("Cache-Control", "no-cache")
		c.Response().Header.Set("Connection", "keep-alive")

		stream, chErr := routeResult.Adapter.ChatCompletionStream(c.Context(), &req)
		if chErr != nil {
			middleware.ErrorLog("ChatCompletionStream failed: requestID=%s model=%s profile=%s error=%v",
				requestID, req.Model, profilePath, chErr)
			go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), false, chErr.Error(), nil)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": chErr.Error()})
		}

		for resp := range stream {
			data, _ := json.Marshal(resp)
			c.Write([]byte("data: "))
			c.Write(data)
			c.Write([]byte("\n\n"))
		}
		c.Write([]byte("data: [DONE]\n\n"))
		go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), true, "", nil)
		return nil
	}

	result, streamErr = routeResult.Adapter.ChatCompletion(c.Context(), &req)
	if streamErr != nil {
		middleware.ErrorLog("ChatCompletion failed: requestID=%s model=%s profile=%s error=%v",
			requestID, req.Model, profilePath, streamErr)
		go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), false, streamErr.Error(), nil)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": streamErr.Error()})
	}

	// Include compression metadata if available
	if compressionMetadata != nil {
		result.Compression = compressionMetadata
	}

	go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), true, "", &result.Usage)
	return c.JSON(result)
}

// handleEmbeddings 处理嵌入请求
func (h *APIHandler) handleEmbeddings(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return h.processEmbeddings(c, profilePath)
}

// handleDefaultEmbeddings 处理默认 Profile 的嵌入
func (h *APIHandler) handleDefaultEmbeddings(c fiber.Ctx) error {
	return h.processEmbeddings(c, "")
}

// processEmbeddings 处理嵌入
func (h *APIHandler) processEmbeddings(c fiber.Ctx, profilePath string) error {
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
		middleware.ErrorLog("Embeddings Route failed: model=%s profile=%s error=%v",
			req.Model, profilePath, err)
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	// 使用 OriginalName 进行实际的 API 调用
	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		req.Model = routeResult.Model.OriginalName
	}

	result, err := routeResult.Adapter.Embeddings(c.Context(), &req)
	if err != nil {
		middleware.ErrorLog("Embeddings API call failed: model=%s profile=%s error=%v",
			req.Model, profilePath, err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// handleListModels 处理模型列表请求
func (h *APIHandler) handleListModels(c fiber.Ctx) error {
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return h.processListModels(c, profilePath)
}

// handleDefaultListModels 处理默认 Profile 的模型列表
func (h *APIHandler) handleDefaultListModels(c fiber.Ctx) error {
	return h.processListModels(c, "")
}

// processListModels 处理模型列表
func (h *APIHandler) processListModels(c fiber.Ctx, profilePath string) error {
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

// handleChatCompletionWithFormat 处理带格式参数的聊天完成请求
// 支持 /api/claude/:profile/v1/messages 和 /api/anthropic/:profile/v1/messages
func (h *APIHandler) handleChatCompletionWithFormat(c fiber.Ctx) error {
	// 从路径中提取格式类型
	apiFormat := GetAPIFormatFromPath(c.Path())
	profilePath := c.Params("profile")
	if err := validateProfilePath(profilePath); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// 对于 Anthropic/Claude 格式，需要转换请求
	if apiFormat == APIFormatAnthropic || apiFormat == APIFormatClaude {
		return h.processAnthropicRequest(c, profilePath)
	}

	// 默认使用 OpenAI 格式处理
	return h.processChatCompletion(c, profilePath)
}

// processAnthropicRequest 处理 Anthropic/Claude 格式请求
func (h *APIHandler) processAnthropicRequest(c *fiber.Ctx, profilePath string) error {
	requestID := uuid.New().String()
	start := time.Now()

	// 获取 Profile
	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// 读取 Anthropic 格式请求
	body := c.Body()
	if len(body) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "empty request body"})
	}

	var anthropicReq AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// 转换为 OpenAI 格式
	openAIReq := ConvertAnthropicToOpenAI(&anthropicReq)

	// 路由到目标模型
	routeResult, err := profile.Route(c.Context(), openAIReq.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	// 使用 OriginalName 进行实际的 API 调用
	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		openAIReq.Model = routeResult.Model.OriginalName
	}

	// 检查是否是复合模型并验证流式支持
	if compositeModel, ok := profile.GetCompositeModel(openAIReq.Model); ok && compositeModel.Enabled {
		if openAIReq.Stream && compositeModel.Strategy == model.CompositeStrategyParallel &&
			compositeModel.Aggregation != nil && compositeModel.Aggregation.Method == model.AggregationMethodSynthesize {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "streaming is not supported for composite models with parallel-synthesize aggregation",
				"model": openAIReq.Model,
			})
		}
	}

	// 验证压缩组（如果提供）
	if openAIReq.CompressionModelGroup != nil && *openAIReq.CompressionModelGroup != "" {
		if err := h.validateCompressionGroup(profile, *openAIReq.CompressionModelGroup); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error":   "invalid compression model group",
				"details": err.Error(),
			})
		}
	}

	// 智能压缩决策
	var compressionMetadata *model.CompressionMetadata
	shouldCompress := h.shouldApplyCompression(profile.Profile, routeResult.Model, openAIReq.Messages)
	if shouldCompress {
		originalCount := len(openAIReq.Messages)
		session := &model.Session{
			ID:            requestID,
			ContextWindow: profile.Profile.MaxContextWindow,
		}
		compressedMessages, metadata, err := profile.ApplyCompression(c.Context(), session, profile.Profile.MaxContextWindow, openAIReq.CompressionModelGroup)
		if err != nil {
			middleware.ErrorLog("Compression error: %v", err)
			c.Set("X-Compression-Status", "failed")
		} else {
			openAIReq.Messages = compressedMessages
			compressionMetadata = metadata
			if h.debug {
				fmt.Printf("Compression applied: %d -> %d messages\n", originalCount, len(compressedMessages))
			}
		}
	}

	// 执行请求
	var result *model.ChatCompletionResponse

	if openAIReq.Stream {
		// 流式处理 - Anthropic 格式的 SSE
		c.Response().Header.SetContentType("text/event-stream")
		c.Response().Header.Set("Cache-Control", "no-cache")
		c.Response().Header.Set("Connection", "keep-alive")

		stream, chErr := routeResult.Adapter.ChatCompletionStream(c.Context(), openAIReq)
		if chErr != nil {
			go h.recordRequestLog(requestID, routeResult, openAIReq.Model, time.Since(start), false, chErr.Error(), nil)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": chErr.Error()})
		}

		// 流式转换 OpenAI 格式响应到 Anthropic 格式
		for resp := range stream {
			// 这里需要将 OpenAI 流式响应转换为 Anthropic 格式
			// 简化处理：直接输出 OpenAI 格式（生产环境需要完整转换）
			data, _ := json.Marshal(resp)
			c.Write([]byte("data: "))
			c.Write(data)
			c.Write([]byte("\n\n"))
		}
		c.Write([]byte("data: [DONE]\n\n"))
		go h.recordRequestLog(requestID, routeResult, openAIReq.Model, time.Since(start), true, "", nil)
		return nil
	}

	result, err = routeResult.Adapter.ChatCompletion(c.Context(), openAIReq)
	if err != nil {
		go h.recordRequestLog(requestID, routeResult, openAIReq.Model, time.Since(start), false, err.Error(), nil)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 转换回 Anthropic 格式
	anthropicResp := ConvertOpenAIToAnthropic(result)

	// 包含压缩元数据（如果存在）
	if compressionMetadata != nil {
		// Anthropic 格式可能不支持自定义元数据字段，这里省略
		// 可以考虑在响应头中添加元数据信息
	}

	go h.recordRequestLog(requestID, routeResult, openAIReq.Model, time.Since(start), true, "", &result.Usage)
	return c.JSON(anthropicResp)
}

// recordRequestLog 记录请求日志
func (h *APIHandler) recordRequestLog(requestID string, routeResult *service.RouteResult, modelName string, latency time.Duration, success bool, errMsg string, usage *model.Usage) {
	if !config.Get().GetEnableStats() {
		return
	}

	// Check if shutdown has been signaled
	select {
	case <-h.shutdownCtx.Done():
		// Server is shutting down, skip logging to avoid blocking
		return
	default:
		// Continue processing
	}

	status := "success"
	if !success {
		status = "error"
	}

	requestLog := &model.RequestLog{
		RequestID:    requestID,
		Model:        modelName,
		ProviderID:   routeResult.Provider.ID,
		Status:       status,
		Latency:      latency.Milliseconds(),
		ErrorMessage: errMsg,
		CreatedAt:    time.Now(),
	}

	// 设置 token 使用量
	if usage != nil {
		requestLog.PromptTokens = usage.PromptTokens
		requestLog.CompletionTokens = usage.CompletionTokens
		requestLog.TotalTokens = usage.TotalTokens
	}

	h.stats.RecordRequest(requestLog)
}

// shouldApplyCompression 智能判断是否需要压缩
func (h *APIHandler) shouldApplyCompression(profile *model.Profile, targetModel *model.Model, messages []model.Message) bool {
	// 1. 检查 Profile 是否启用压缩
	if !profile.EnableCompression {
		return false
	}

	// 2. 检查模型是否标记为跳过压缩（如 1M+ 模型）
	if targetModel.SkipCompression {
		return false
	}

	// 3. 估算当前请求的 token 数量
	estimatedTokens := estimateMessagesTokens(messages)

	// 4. 根据压缩等级决策
	switch profile.CompressionLevel {
	case model.CompressionLevelSession:
		// session 模式：每次都检查是否超出模型原生上下文窗口
		return estimatedTokens > targetModel.ContextWindow
	case model.CompressionLevelThreshold:
		// threshold 模式：只在达到配置的阈值时压缩
		return profile.CompressionThreshold > 0 && estimatedTokens >= profile.CompressionThreshold
	default:
		// 默认行为：检查是否超出模型原生上下文窗口
		return estimatedTokens > targetModel.ContextWindow
	}
}

// estimateMessagesTokens 估算消息列表的 token 数量
func estimateMessagesTokens(messages []model.Message) int {
	return tokenizer.CountTokensForMessages(messages)
}

// validateCompressionGroup validates that a compression group exists and is usable
func (h *APIHandler) validateCompressionGroup(profile *service.ProfileInstance, groupName string) error {
	if groupName == "" {
		return fmt.Errorf("compression group name cannot be empty")
	}
	if profile.CompressionSelector == nil {
		return fmt.Errorf("compression groups not configured for this profile")
	}
	// Check if group exists (no side effects - doesn't create adapter)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !profile.CompressionSelector.GroupExists(ctx, groupName) {
		return fmt.Errorf("compression group not found: %s", groupName)
	}
	return nil
}

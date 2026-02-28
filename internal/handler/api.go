package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/proxy"
	"github.com/gemone/model-router/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// APIHandler API 处理器
type APIHandler struct {
	profileManager *service.ProfileManager
	stats          *service.StatsCollector
	proxy          *proxy.Proxy
	debug          bool
}

// NewAPIHandler 创建 API 处理器
func NewAPIHandler() *APIHandler {
	cfg := config.Get()
	return &APIHandler{
		profileManager: service.GetProfileManager(),
		stats:          service.GetStatsCollector(),
		proxy:          proxy.GetProxy(),
		debug:          cfg.LogLevel == "debug",
	}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(r *gin.Engine) {
	// 支持多种 URI 格式：
	// /api/{profile}/v1/...
	// /api/{profile}/claude/...
	// /{profile}/v1/...
	// /v1/... (使用默认 profile)

	// OpenAI 兼容 API
	r.POST("/api/:profile/v1/chat/completions", h.handleChatCompletion)
	r.POST("/api/:profile/v1/embeddings", h.handleEmbeddings)
	r.GET("/api/:profile/v1/models", h.handleListModels)

	// 递进式格式 /api/{format}/{profile}/...
	r.POST("/api/openai/:profile/v1/chat/completions", h.handleChatCompletion)
	r.POST("/api/openai/:profile/v1/embeddings", h.handleEmbeddings)
	r.GET("/api/openai/:profile/v1/models", h.handleListModels)

	// Claude 格式
	r.POST("/api/claude/:profile/v1/messages", h.handleClaudeMessages)
	r.GET("/api/claude/:profile/v1/models", h.handleListModels)

	// 简写格式
	r.POST("/:profile/v1/chat/completions", h.handleChatCompletion)
	r.POST("/:profile/v1/embeddings", h.handleEmbeddings)
	r.GET("/:profile/v1/models", h.handleListModels)

	// 默认 profile（不带 profile 前缀）
	r.POST("/v1/chat/completions", h.handleDefaultChatCompletion)
	r.POST("/v1/embeddings", h.handleDefaultEmbeddings)
	r.GET("/v1/models", h.handleDefaultListModels)
}

// handleChatCompletion 处理聊天完成请求
func (h *APIHandler) handleChatCompletion(c *gin.Context) {
	profilePath := c.Param("profile")
	h.processChatCompletion(c, profilePath)
}

// handleDefaultChatCompletion 处理默认 Profile 的聊天完成
func (h *APIHandler) handleDefaultChatCompletion(c *gin.Context) {
	h.processChatCompletion(c, "")
}

// processChatCompletion 处理聊天完成
func (h *APIHandler) processChatCompletion(c *gin.Context, profilePath string) {
	requestID := uuid.New().String()
	start := time.Now()

	// 获取 Profile
	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	// 检查是否可以直接透传（OpenAI Compatible 适配器）
	if h.canPassthrough(profile) && !h.debug {
		h.passthroughChatCompletion(c, profile, requestID, start)
		return
	}

	// 需要格式转换
	h.handleChatCompletionWithTransform(c, profile, requestID, start)
}

// canPassthrough 检查是否可以直接透传
func (h *APIHandler) canPassthrough(profile *service.ProfileInstance) bool {
	// 如果 profile 只有一个适配器且是 OpenAI 兼容类型，可以直接透传
	providers := profile.GetProviders()
	if len(providers) != 1 {
		return false
	}

	switch providers[0].Type {
	case model.ProviderOpenAI, model.ProviderDeepSeek, model.ProviderOpenAICompatible:
		return true
	default:
		return false
	}
}

// passthroughChatCompletion 直接透传聊天完成
func (h *APIHandler) passthroughChatCompletion(c *gin.Context, profile *service.ProfileInstance, requestID string, start time.Time) {
	// 获取模型名称
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var req model.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		// 如果解析失败，直接透传
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
		h.doPassthrough(c, profile, "/v1/chat/completions")
		return
	}

	// 路由到具体模型
	routeResult, err := profile.Route(c.Request.Context(), req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	// 重写请求中的模型名称为原始名称
	req.Model = routeResult.Model.OriginalName
	newBody, _ := json.Marshal(req)

	// 构建目标 URL
	targetURL := routeResult.Provider.BaseURL + "/v1/chat/completions"
	headers := routeResult.Adapter.GetRequestHeaders()

	// 设置 SSE headers（如果是流式请求）
	if req.Stream {
		c.Request.Body = io.NopCloser(strings.NewReader(string(newBody)))
		err := h.proxy.ProxyStream(c.Request.Context(), c.Writer, c.Request, targetURL, headers)
		if err != nil {
			// 流式响应开始后无法返回 JSON 错误
			return
		}
	} else {
		c.Request.Body = io.NopCloser(strings.NewReader(string(newBody)))
		err := h.proxy.ProxyRequest(c.Request.Context(), c.Writer, c.Request, targetURL, headers)
		if err != nil {
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}
	}

	// 异步记录统计
	go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), true, "")
}

// doPassthrough 直接透传
func (h *APIHandler) doPassthrough(c *gin.Context, profile *service.ProfileInstance, path string) {
	// 选择默认提供商
	providers := profile.GetProviders()
	if len(providers) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no providers available"})
		return
	}

	provider := providers[0]
	targetURL := provider.BaseURL + path

	// 这里需要获取适配器的 headers，但适配器未初始化
	// 使用基本 headers
	headers := map[string]string{
		"Authorization": "Bearer " + provider.APIKey,
	}

	err := h.proxy.ProxyRequest(c.Request.Context(), c.Writer, c.Request, targetURL, headers)
	if err != nil {
		if !c.Writer.Written() {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	}
}

// handleChatCompletionWithTransform 带格式转换的聊天完成
func (h *APIHandler) handleChatCompletionWithTransform(c *gin.Context, profile *service.ProfileInstance, requestID string, start time.Time) {
	var req model.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// 路由
	routeResult, err := profile.Route(c.Request.Context(), req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	// 执行请求
	var result *model.ChatCompletionResponse
	var stream <-chan *model.ChatCompletionStreamResponse
	var reqErr error

	if req.Stream {
		stream, reqErr = routeResult.Adapter.ChatCompletionStream(c.Request.Context(), &req)
	} else {
		result, reqErr = routeResult.Adapter.ChatCompletion(c.Request.Context(), &req)
	}

	if reqErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": reqErr.Error()})
		go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), false, reqErr.Error())
		return
	}

	// 响应
	if req.Stream {
		h.streamResponse(c, stream, routeResult.Model.Name)
	} else {
		c.JSON(http.StatusOK, result)
	}

	go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), true, "")
}

// streamResponse 流式响应
func (h *APIHandler) streamResponse(c *gin.Context, stream <-chan *model.ChatCompletionStreamResponse, modelName string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	for resp := range stream {
		data, _ := json.Marshal(resp)
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(data)
		c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
	}

	c.Writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

// handleEmbeddings 处理嵌入请求
func (h *APIHandler) handleEmbeddings(c *gin.Context) {
	profilePath := c.Param("profile")
	h.processEmbeddings(c, profilePath)
}

// handleDefaultEmbeddings 处理默认 Profile 的嵌入
func (h *APIHandler) handleDefaultEmbeddings(c *gin.Context) {
	h.processEmbeddings(c, "")
}

// processEmbeddings 处理嵌入
func (h *APIHandler) processEmbeddings(c *gin.Context, profilePath string) {
	var req model.EmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	routeResult, err := profile.Route(c.Request.Context(), req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	result, err := routeResult.Adapter.Embeddings(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// handleListModels 处理模型列表请求
func (h *APIHandler) handleListModels(c *gin.Context) {
	profilePath := c.Param("profile")
	h.processListModels(c, profilePath)
}

// handleDefaultListModels 处理默认 Profile 的模型列表
func (h *APIHandler) handleDefaultListModels(c *gin.Context) {
	h.processListModels(c, "")
}

// processListModels 处理模型列表
func (h *APIHandler) processListModels(c *gin.Context, profilePath string) {
	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
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

	c.JSON(http.StatusOK, model.ListModelsResponse{
		Object: "list",
		Data:   modelInfos,
	})
}

// handleClaudeMessages 处理 Claude 格式消息
func (h *APIHandler) handleClaudeMessages(c *gin.Context) {
	profilePath := c.Param("profile")

	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	// 读取 Claude 格式请求并转换为 OpenAI 格式
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var claudeReq claudeRequest
	if err := json.Unmarshal(body, &claudeReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// 转换为 OpenAI 格式
	openAIReq := convertClaudeToOpenAI(&claudeReq)

	// 路由
	routeResult, err := profile.Route(c.Request.Context(), openAIReq.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	// 执行请求
	result, err := routeResult.Adapter.ChatCompletion(c.Request.Context(), openAIReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换回 Claude 格式
	claudeResp := convertOpenAIToClaude(result)
	c.JSON(http.StatusOK, claudeResp)
}

// recordRequestLog 记录请求日志
func (h *APIHandler) recordRequestLog(requestID string, routeResult *service.RouteResult, modelName string, latency time.Duration, success bool, errMsg string) {
	if !config.Get().EnableStats {
		return
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
	}

	h.stats.RecordRequest(requestLog)
}

// Claude 请求/响应结构
type claudeRequest struct {
	Model     string          `json:"model"`
	Messages  []claudeMessage `json:"messages"`
	System    string          `json:"system,omitempty"`
	MaxTokens int             `json:"max_tokens,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Content    []claudeContent `json:"content"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason,omitempty"`
	Usage      claudeUsage     `json:"usage"`
}

type claudeContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func convertClaudeToOpenAI(req *claudeRequest) *model.ChatCompletionRequest {
	messages := make([]model.Message, 0, len(req.Messages)+1)

	// 添加 system 消息
	if req.System != "" {
		messages = append(messages, model.Message{
			Role:    "system",
			Content: req.System,
		})
	}

	// 添加对话消息
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, model.Message{
			Role:    role,
			Content: m.Content,
		})
	}

	return &model.ChatCompletionRequest{
		Model:     req.Model,
		Messages:  messages,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}
}

func convertOpenAIToClaude(resp *model.ChatCompletionResponse) *claudeResponse {
	if len(resp.Choices) == 0 {
		return &claudeResponse{
			ID:   resp.ID,
			Type: "message",
			Role: "assistant",
		}
	}

	choice := resp.Choices[0]
	content := ""
	if c, ok := choice.Message.Content.(string); ok {
		content = c
	}

	stopReason := "end_turn"
	switch choice.FinishReason {
	case "length":
		stopReason = "max_tokens"
	case "stop":
		stopReason = "end_turn"
	}

	return &claudeResponse{
		ID:   resp.ID,
		Type: "message",
		Role: "assistant",
		Content: []claudeContent{
			{Type: "text", Text: content},
		},
		Model:      resp.Model,
		StopReason: stopReason,
		Usage: claudeUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}

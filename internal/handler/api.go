package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// APIHandler API 处理器
type APIHandler struct {
	profileManager *service.ProfileManager
	stats          *service.StatsCollector
	debug          bool
}

// NewAPIHandler 创建 API 处理器
func NewAPIHandler() *APIHandler {
	cfg := config.Get()
	return &APIHandler{
		profileManager: service.GetProfileManager(),
		stats:          service.GetStatsCollector(),
		debug:          cfg.LogLevel == "debug",
	}
}

// RegisterRoutes 注册路由
func (h *APIHandler) RegisterRoutes(app *fiber.App) {
	// OpenAI 兼容 API
	app.Post("/api/:profile/v1/chat/completions", h.handleChatCompletion)
	app.Post("/api/:profile/v1/embeddings", h.handleEmbeddings)
	app.Get("/api/:profile/v1/models", h.handleListModels)

	// 递进式格式 /api/{format}/{profile}/...
	app.Post("/api/openai/:profile/v1/chat/completions", h.handleChatCompletion)
	app.Post("/api/openai/:profile/v1/embeddings", h.handleEmbeddings)
	app.Get("/api/openai/:profile/v1/models", h.handleListModels)

	// Claude 格式
	app.Post("/api/claude/:profile/v1/messages", h.handleClaudeMessages)
	app.Get("/api/claude/:profile/v1/models", h.handleListModels)

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
func (h *APIHandler) handleChatCompletion(c *fiber.Ctx) error {
	profilePath := c.Params("profile")
	return h.processChatCompletion(c, profilePath)
}

// handleDefaultChatCompletion 处理默认 Profile 的聊天完成
func (h *APIHandler) handleDefaultChatCompletion(c *fiber.Ctx) error {
	return h.processChatCompletion(c, "")
}

// processChatCompletion 处理聊天完成
func (h *APIHandler) processChatCompletion(c *fiber.Ctx, profilePath string) error {
	requestID := uuid.New().String()
	start := time.Now()

	// 获取 Profile
	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// 解析请求体
	var req model.ChatCompletionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// 路由
	routeResult, err := profile.Route(c.Context(), req.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
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
			go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), false, chErr.Error())
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": chErr.Error()})
		}

		for resp := range stream {
			data, _ := json.Marshal(resp)
			c.Write([]byte("data: "))
			c.Write(data)
			c.Write([]byte("\n\n"))
		}
		c.Write([]byte("data: [DONE]\n\n"))
		go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), true, "")
		return nil
	}

	result, streamErr = routeResult.Adapter.ChatCompletion(c.Context(), &req)
	if streamErr != nil {
		go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), false, streamErr.Error())
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": streamErr.Error()})
	}

	go h.recordRequestLog(requestID, routeResult, req.Model, time.Since(start), true, "")
	return c.JSON(result)
}

// handleEmbeddings 处理嵌入请求
func (h *APIHandler) handleEmbeddings(c *fiber.Ctx) error {
	profilePath := c.Params("profile")
	return h.processEmbeddings(c, profilePath)
}

// handleDefaultEmbeddings 处理默认 Profile 的嵌入
func (h *APIHandler) handleDefaultEmbeddings(c *fiber.Ctx) error {
	return h.processEmbeddings(c, "")
}

// processEmbeddings 处理嵌入
func (h *APIHandler) processEmbeddings(c *fiber.Ctx, profilePath string) error {
	var req model.EmbeddingRequest
	if err := c.BodyParser(&req); err != nil {
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

	result, err := routeResult.Adapter.Embeddings(c.Context(), &req)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// handleListModels 处理模型列表请求
func (h *APIHandler) handleListModels(c *fiber.Ctx) error {
	profilePath := c.Params("profile")
	return h.processListModels(c, profilePath)
}

// handleDefaultListModels 处理默认 Profile 的模型列表
func (h *APIHandler) handleDefaultListModels(c *fiber.Ctx) error {
	return h.processListModels(c, "")
}

// processListModels 处理模型列表
func (h *APIHandler) processListModels(c *fiber.Ctx, profilePath string) error {
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

// handleClaudeMessages 处理 Claude 格式消息
func (h *APIHandler) handleClaudeMessages(c *fiber.Ctx) error {
	profilePath := c.Params("profile")

	profile := h.profileManager.GetProfile(profilePath)
	if profile == nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "profile not found"})
	}

	// 读取 Claude 格式请求并转换为 OpenAI 格式
	body := c.Body()
	if len(body) == 0 {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "empty request body"})
	}

	var claudeReq claudeRequest
	if err := json.Unmarshal(body, &claudeReq); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// 转换为 OpenAI 格式
	openAIReq := convertClaudeToOpenAI(&claudeReq)

	// 路由
	routeResult, err := profile.Route(c.Context(), openAIReq.Model)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	}

	// 执行请求
	result, err := routeResult.Adapter.ChatCompletion(c.Context(), openAIReq)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// 转换回 Claude 格式
	claudeResp := convertOpenAIToClaude(result)
	return c.JSON(claudeResp)
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

// 兼容函数：透传到上游服务
func (h *APIHandler) proxyToUpstream(c *fiber.Ctx, targetURL string, body []byte) error {
	req, err := http.NewRequestWithContext(c.Context(), "POST", targetURL, strings.NewReader(string(body)))
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	req.Header = make(http.Header)
	c.Request().Header.VisitAll(func(key, value []byte) {
		req.Header.Add(string(key), string(value))
	})
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			c.Response().Header.Add(k, vv)
		}
	}
	c.Response().SetStatusCode(resp.StatusCode)

	io.Copy(c.Response().BodyWriter(), resp.Body)
	return nil
}

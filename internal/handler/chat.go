package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/middleware"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/router"
	"github.com/gemone/model-router/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ChatHandler handles OpenAI-compatible chat completion requests.
type ChatHandler struct {
	profileRouter *router.ProfileRouter
	stats         *service.StatsCollector
	enableStats   bool
}

// NewChatHandler creates a new chat handler instance.
func NewChatHandler(profileRouter *router.ProfileRouter) *ChatHandler {
	cfg := config.Get()
	return &ChatHandler{
		profileRouter: profileRouter,
		stats:         service.GetStatsCollector(),
		enableStats:   cfg.GetEnableStats(),
	}
}

// RegisterRoutes registers chat completion routes with the Fiber app.
func (h *ChatHandler) RegisterRoutes(app *fiber.App) {
	// Standard OpenAI-compatible endpoint
	app.Post("/v1/chat/completions", h.HandleChatCompletions)

	// Profile-based routing
	app.Post("/api/:profile/v1/chat/completions", h.HandleChatCompletions)

	// Format-prefixed routing
	app.Post("/api/openai/:profile/v1/chat/completions", h.HandleChatCompletions)

	// Shorthand format
	app.Post("/:profile/v1/chat/completions", h.HandleChatCompletions)
}

// HandleChatCompletions processes chat completion requests (streaming and non-streaming).
func (h *ChatHandler) HandleChatCompletions(c *fiber.Ctx) error {
	requestID := uuid.New().String()
	start := time.Now()

	// Parse request body using Sonic parser from internal/simdjson
	req, err := h.parseChatRequest(c.Body())
	if err != nil {
		return h.errorResponse(c, http.StatusBadRequest, "invalid_request_body", err.Error())
	}

	// Check if request contains image content
	hasImage := model.HasImageContent(req.Messages)

	// Route to appropriate provider/model
	routeResult, err := h.profileRouter.Route(c.Context(), req.Model, hasImage)
	if err != nil {
		return h.errorResponse(c, http.StatusServiceUnavailable, "no_route", err.Error())
	}

	// Execute request
	if req.Stream {
		return h.handleStreaming(c, req, routeResult, requestID, start)
	}

	return h.handleNonStreaming(c, req, routeResult, requestID, start)
}

// parseChatRequest parses the chat completion request body.
func (h *ChatHandler) parseChatRequest(body []byte) (*model.ChatCompletionRequest, error) {
	if len(body) == 0 {
		return nil, fiber.NewError(http.StatusBadRequest, "empty request body")
	}

	var req model.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fiber.NewError(http.StatusBadRequest, "failed to parse JSON: "+err.Error())
	}

	return &req, nil
}

// handleNonStreaming handles non-streaming chat completion requests.
func (h *ChatHandler) handleNonStreaming(
	c *fiber.Ctx,
	req *model.ChatCompletionRequest,
	routeResult *router.RouteResult,
	requestID string,
	start time.Time,
) error {
	// Use OriginalName for the actual API call if available
	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		req.Model = routeResult.Model.OriginalName
	}

	// Call adapter for response
	resp, err := routeResult.Adapter.ChatCompletion(c.Context(), req)
	if err != nil {
		middleware.ErrorLog("ChatCompletion failed: requestID=%s model=%s error=%v",
			requestID, req.Model, err)
		go h.recordRequest(requestID, routeResult, req.Model, time.Since(start), false, err.Error())
		return h.errorResponse(c, http.StatusInternalServerError, "provider_error", err.Error())
	}

	// Record successful request
	go h.recordRequest(requestID, routeResult, req.Model, time.Since(start), true, "")

	// Return OpenAI-compatible response
	c.Set("Content-Type", "application/json")
	return c.Status(http.StatusOK).JSON(resp)
}

// handleStreaming handles streaming chat completion requests with SSE.
func (h *ChatHandler) handleStreaming(
	c *fiber.Ctx,
	req *model.ChatCompletionRequest,
	routeResult *router.RouteResult,
	requestID string,
	start time.Time,
) error {
	// Use OriginalName for the actual API call if available
	if routeResult.Model != nil && routeResult.Model.OriginalName != "" {
		req.Model = routeResult.Model.OriginalName
	}

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	// Get stream channel from adapter
	streamChan, err := routeResult.Adapter.ChatCompletionStream(c.Context(), req)
	if err != nil {
		middleware.ErrorLog("ChatCompletionStream failed: requestID=%s model=%s error=%v",
			requestID, req.Model, err)
		go h.recordRequest(requestID, routeResult, req.Model, time.Since(start), false, err.Error())
		return h.errorResponse(c, http.StatusInternalServerError, "provider_error", err.Error())
	}

	// Stream responses
	flusher, ok := c.Context().Response.BodyWriter().(http.Flusher)
	if !ok {
		return h.errorResponse(c, http.StatusInternalServerError, "stream_error", "streaming not supported")
	}

	// Send each chunk
	for chunk := range streamChan {
		data, err := json.Marshal(chunk)
		if err != nil {
			break
		}

		c.Write([]byte("data: "))
		c.Write(data)
		c.Write([]byte("\n\n"))
		flusher.Flush()
	}

	// Send final [DONE] message
	c.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()

	// Record successful request
	go h.recordRequest(requestID, routeResult, req.Model, time.Since(start), true, "")

	return nil
}

// recordRequest records request statistics.
func (h *ChatHandler) recordRequest(
	requestID string,
	routeResult *router.RouteResult,
	modelName string,
	latency time.Duration,
	success bool,
	errorMsg string,
) {
	if !h.enableStats {
		return
	}

	status := "success"
	if !success {
		status = "error"
	}

	requestLog := &model.RequestLog{
		RequestID: requestID,
		Model:     modelName,
		ProviderID: routeResult.Provider.ID,
		Status:    status,
		Latency:   latency.Milliseconds(),
		ErrorMessage: errorMsg,
	}

	h.stats.RecordRequest(requestLog)
}

// errorResponse returns an OpenAI-compatible error response.
func (h *ChatHandler) errorResponse(c *fiber.Ctx, status int, errorCode, message string) error {
	resp := model.ErrorResponse{
		Error: model.APIError{
			Message: message,
			Type:    "invalid_request_error",
			Code:    errorCode,
		},
	}

	c.Set("Content-Type", "application/json")
	return c.Status(status).JSON(resp)
}

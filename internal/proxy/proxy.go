package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gemone/model-router/internal/config"
)

const (
	// MaxBodySize is the maximum allowed body size to prevent DoS attacks
	MaxBodySize = 10 * 1024 * 1024 // 10MB
)

// Proxy 高性能代理
type Proxy struct {
	httpClient *http.Client
	debug      bool
}

var (
	proxyInstance *Proxy
	proxyOnce     sync.Once
)

// GetProxy 获取代理实例
func GetProxy() *Proxy {
	proxyOnce.Do(func() {
		cfg := config.Get()
		proxyInstance = &Proxy{
			debug: cfg.GetLogLevel() == "debug",
			httpClient: &http.Client{
				Timeout: 30 * time.Second, // Default timeout as fallback, context can override
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
					IdleConnTimeout:     90 * time.Second,
					DisableCompression:  false,
				},
			},
		}
	})
	return proxyInstance
}

// ProxyRequest 代理请求（高性能透传）
func (p *Proxy) ProxyRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, targetURL string, headers map[string]string) error {
	start := time.Now()

	// 读取原始 body（仅用于 ContentLength 记录）
	var bodySize int64 = 0
	if r.Body != nil {
		// 只读取但不保存，直接转发
		bodySize = r.ContentLength
	}

	// 创建新请求
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, r.Body)
	if err != nil {
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	// 复制 headers（排除 hop-by-hop headers）
	copyHeaders(proxyReq.Header, r.Header)

	// 设置自定义 headers
	for k, v := range headers {
		proxyReq.Header.Set(k, v)
	}

	// 执行请求
	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		return fmt.Errorf("proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	// 复制响应 headers
	delHopHeaders(resp.Header)
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	// 流式复制响应 body（零拷贝）
	var written int64
	if p.debug {
		// Debug 模式：记录完整响应
		written, err = p.copyWithDebug(w, resp.Body, r.URL.Path)
	} else {
		// 生产模式：直接流式复制
		written, err = io.Copy(w, resp.Body)
	}

	if err != nil && err != io.EOF {
		// 记录错误但不返回，因为响应可能已经部分发送
		if p.debug {
			fmt.Printf("[Proxy] Error copying response: %v\n", err)
		}
	}

	// 记录指标（异步）
	if config.Get().GetEnableStats() {
		go p.recordMetrics(r.URL.Path, bodySize, written, resp.StatusCode, time.Since(start))
	}

	return nil
}

// ProxyStream 代理流式响应（SSE/WebSocket）
func (p *Proxy) ProxyStream(ctx context.Context, w http.ResponseWriter, r *http.Request, targetURL string, headers map[string]string) error {
	start := time.Now()

	// 创建新请求
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, r.Body)
	if err != nil {
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	// 复制 headers
	copyHeaders(proxyReq.Header, r.Header)
	for k, v := range headers {
		proxyReq.Header.Set(k, v)
	}

	// 确保接受流式响应
	proxyReq.Header.Set("Accept", "text/event-stream")
	proxyReq.Header.Set("Cache-Control", "no-cache")

	// 执行请求
	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		return fmt.Errorf("proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	// 设置流式响应 headers
	delHopHeaders(resp.Header)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 复制其他 headers
	for k, v := range resp.Header {
		if k != "Content-Length" && k != "Transfer-Encoding" {
			w.Header()[k] = v
		}
	}

	w.WriteHeader(resp.StatusCode)

	// 使用 flusher 确保实时流式传输
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// 流式复制
	buf := make([]byte, 32*1024) // 32KB buffer
	var totalBytes int64

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			flusher.Flush()
			totalBytes += int64(n)

			// Debug 模式记录
			if p.debug {
				p.logStreamChunk(r.URL.Path, buf[:n])
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// 记录指标
	if config.Get().GetEnableStats() {
		go p.recordMetrics(r.URL.Path, r.ContentLength, totalBytes, resp.StatusCode, time.Since(start))
	}

	return nil
}

// ProxyWithTransform 代理并转换请求/响应（用于格式转换）
func (p *Proxy) ProxyWithTransform(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	targetURL string,
	headers map[string]string,
	transformRequest func([]byte) ([]byte, error),
	transformResponse func([]byte) ([]byte, error),
) error {
	start := time.Now()

	// 读取请求 body - Use http.MaxBytesReader to enforce size limit before reading
	// This prevents memory exhaustion by rejecting requests that are too large
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		defer r.Body.Close()
		// http.MaxBytesReader returns a specific error message when limit exceeded
		if err.Error() == "http: request body too large" {
			return fmt.Errorf("request body exceeds maximum size of %d bytes", MaxBodySize)
		}
		return fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()

	// 转换请求
	if transformRequest != nil {
		body, err = transformRequest(body)
		if err != nil {
			return fmt.Errorf("failed to transform request: %w", err)
		}
	}

	// 创建新请求
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	// 复制 headers
	copyHeaders(proxyReq.Header, r.Header)
	for k, v := range headers {
		proxyReq.Header.Set(k, v)
	}
	proxyReq.Header.Set("Content-Length", strconv.Itoa(len(body)))

	// 执行请求
	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		return fmt.Errorf("proxy request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应 body - Use http.MaxBytesReader to enforce size limit before reading
	// This prevents memory exhaustion by rejecting responses that are too large
	resp.Body = http.MaxBytesReader(w, resp.Body, MaxBodySize)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// http.MaxBytesReader returns a specific error message when limit exceeded
		if err.Error() == "http: request body too large" || err.Error() == "http: response body too large" {
			return fmt.Errorf("response body exceeds maximum size of %d bytes", MaxBodySize)
		}
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// 转换响应
	if transformResponse != nil {
		respBody, err = transformResponse(respBody)
		if err != nil {
			return fmt.Errorf("failed to transform response: %w", err)
		}
	}

	// 写入响应
	delHopHeaders(resp.Header)
	copyHeaders(w.Header(), resp.Header)
	w.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)

	// 记录指标
	if config.Get().GetEnableStats() {
		go p.recordMetrics(r.URL.Path, int64(len(body)), int64(len(respBody)), resp.StatusCode, time.Since(start))
	}

	// Debug 记录
	if p.debug {
		p.logDebug(r.URL.Path, body, respBody)
	}

	return nil
}

// copyWithDebug 带调试信息的复制
func (p *Proxy) copyWithDebug(w io.Writer, r io.Reader, path string) (int64, error) {
	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)
	n, err := io.Copy(w, tee)

	// 记录响应元数据（不记录内容以防止信息泄露）
	data := buf.Bytes()
	fmt.Printf("[Proxy Debug] Path: %s, Response size: %d bytes\n", path, len(data))

	return n, err
}

// logStreamChunk 记录流式数据块元数据
func (p *Proxy) logStreamChunk(path string, data []byte) {
	// 只记录大小，不记录内容以防止信息泄露
	fmt.Printf("[Proxy Debug] Stream chunk size: %d bytes\n", len(data))
}

// logDebug 记录调试信息（只记录元数据，不记录内容）
func (p *Proxy) logDebug(path string, reqBody, respBody []byte) {
	fmt.Printf("[Proxy Debug] Path: %s, Request size: %d bytes, Response size: %d bytes\n",
		path, len(reqBody), len(respBody))
}

// recordMetrics 记录指标
func (p *Proxy) recordMetrics(path string, reqSize, respSize int64, statusCode int, duration time.Duration) {
	// 这里可以异步记录到 stats collector
	// 仅记录大小，不解析内容
}

// hop-by-hop headers 列表
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

// delHopHeaders 删除 hop-by-hop headers
func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

// copyHeaders 复制 headers
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		// 跳过 hop-by-hop headers
		if isHopHeader(k) {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// isHopHeader 检查是否是 hop-by-hop header
func isHopHeader(header string) bool {
	for _, h := range hopHeaders {
		if strings.EqualFold(h, header) {
			return true
		}
	}
	return false
}

// GetContentEncoding 获取内容编码
func GetContentEncoding(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	// gzip magic number: 0x1f 0x8b
	if data[0] == 0x1f && data[1] == 0x8b {
		return "gzip"
	}
	return ""
}

// DecompressIfNeeded 如果需要则解压
func DecompressIfNeeded(data []byte) ([]byte, error) {
	encoding := GetContentEncoding(data)
	if encoding == "gzip" {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	}
	return data, nil
}

// ProxyOptions 代理选项
type ProxyOptions struct {
	TargetURL         string
	Headers           map[string]string
	TransformRequest  func([]byte) ([]byte, error)
	TransformResponse func([]byte) ([]byte, error)
	Stream            bool
	Debug             bool
}

// ResponseInfo 响应信息（轻量级记录）
type ResponseInfo struct {
	Path       string
	StatusCode int
	ReqSize    int64
	RespSize   int64
	Duration   time.Duration
	Timestamp  time.Time
	Model      string // 如果能从路径解析
}

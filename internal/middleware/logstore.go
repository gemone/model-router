package middleware

import (
	"container/ring"
	"regexp"
	"strings"
	"sync"
	"time"
)

// LogEntry 结构化日志条目
type LogEntry struct {
	Timestamp  time.Time         `json:"timestamp"`
	Level      string            `json:"level"`
	RequestID  string            `json:"request_id,omitempty"`
	Method     string            `json:"method,omitempty"`
	Path       string            `json:"path,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
	Duration   time.Duration     `json:"duration,omitempty"`
	ClientIP   string            `json:"client_ip,omitempty"`
	Message    string            `json:"message"`
	RawLog     string            `json:"raw_log"`
	Tags       []string          `json:"tags,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// LogQuery 日志查询条件
type LogQuery struct {
	Level          string
	RequestID      string
	Keyword        string
	StartTime      *time.Time
	EndTime        *time.Time
	Page           int
	PageSize       int
	GroupByRequest bool // 按 request_id 分组
}

// LogResult 日志查询结果
type LogResult struct {
	Entries  []LogEntry `json:"entries"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	HasMore  bool       `json:"has_more"`
}

// RequestGroup 按请求分组的日志
type RequestGroup struct {
	RequestID  string        `json:"request_id"`
	Method     string        `json:"method"`
	Path       string        `json:"path"`
	StatusCode int           `json:"status_code"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
	LogCount   int           `json:"log_count"`
	Entries    []LogEntry    `json:"entries,omitempty"`
}

// LogStore 日志存储
type LogStore struct {
	mu      sync.RWMutex
	entries *ring.Ring // 环形缓冲区
	size    int        // 当前条目数量
	maxSize int        // 最大容量
	head    int        // 当前写入位置（绝对索引）

	// 索引
	requestIndex map[string][]int // request_id -> entry indices
	tagIndex     map[string][]int // tag -> entry indices
}

var (
	store     *LogStore
	storeOnce sync.Once
)

// GetLogStore 获取日志存储单例
func GetLogStore() *LogStore {
	storeOnce.Do(func() {
		store = &LogStore{
			entries:      ring.New(5000), // 最多保留 5000 条
			maxSize:      5000,
			head:         0,
			requestIndex: make(map[string][]int),
			tagIndex:     make(map[string][]int),
		}
	})
	return store
}

// Add 添加日志条目
func (s *LogStore) Add(entry LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 当前写入位置（在写入前记录，这是此条目的唯一标识）
	currentPos := s.head

	// 保存到环形缓冲区
	s.entries.Value = entry
	s.entries = s.entries.Next()

	// 清理过期的索引（当缓冲区即将覆盖时）
	if s.size >= s.maxSize {
		// 计算被覆盖的条目位置
		stalePos := currentPos - s.maxSize
		s.cleanupStaleIndex(stalePos)
	}

	// 更新计数器和头部位置
	if s.size < s.maxSize {
		s.size++
	}
	s.head++

	// 更新索引（使用当前绝对位置）
	if entry.RequestID != "" {
		s.requestIndex[entry.RequestID] = append(s.requestIndex[entry.RequestID], currentPos)
	}

	for _, tag := range entry.Tags {
		s.tagIndex[tag] = append(s.tagIndex[tag], currentPos)
	}
}

// cleanupStaleIndex 清理指定位置的过期索引
func (s *LogStore) cleanupStaleIndex(stalePos int) {
	// 清理 requestIndex 中的过期条目
	for reqID, positions := range s.requestIndex {
		newPositions := make([]int, 0, len(positions))
		for _, pos := range positions {
			if pos > stalePos {
				newPositions = append(newPositions, pos)
			}
		}
		if len(newPositions) == 0 {
			delete(s.requestIndex, reqID)
		} else if len(newPositions) < len(positions) {
			s.requestIndex[reqID] = newPositions
		}
	}

	// 清理 tagIndex 中的过期条目
	for tag, positions := range s.tagIndex {
		newPositions := make([]int, 0, len(positions))
		for _, pos := range positions {
			if pos > stalePos {
				newPositions = append(newPositions, pos)
			}
		}
		if len(newPositions) == 0 {
			delete(s.tagIndex, tag)
		} else if len(newPositions) < len(positions) {
			s.tagIndex[tag] = newPositions
		}
	}
}

// Query 查询日志
func (s *LogStore) Query(query LogQuery) LogResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []LogEntry

	// 如果按 request_id 查询，使用索引
	if query.RequestID != "" {
		indices := s.requestIndex[query.RequestID]
		for _, idx := range indices {
			if entry := s.getEntryByIndex(idx); entry != nil {
				entries = append(entries, *entry)
			}
		}
	} else {
		// 遍历所有条目
		s.entries.Do(func(v interface{}) {
			if v == nil {
				return
			}
			entry := v.(LogEntry)

			// 过滤条件
			if !s.matchQuery(entry, query) {
				return
			}

			entries = append(entries, entry)
		})
	}

	// 反转顺序（最新的在前）
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	total := len(entries)

	// 分页
	if query.PageSize <= 0 {
		query.PageSize = 50
	}
	if query.Page <= 0 {
		query.Page = 1
	}

	start := (query.Page - 1) * query.PageSize
	end := start + query.PageSize

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return LogResult{
		Entries:  entries[start:end],
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		HasMore:  end < total,
	}
}

// GetRequestGroups 获取按请求分组的日志
func (s *LogStore) GetRequestGroups(query LogQuery) []RequestGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groups := make(map[string]*RequestGroup)

	s.entries.Do(func(v interface{}) {
		if v == nil {
			return
		}
		entry := v.(LogEntry)

		if entry.RequestID == "" {
			return
		}

		// 过滤条件
		if !s.matchQuery(entry, query) {
			return
		}

		group, exists := groups[entry.RequestID]
		if !exists {
			group = &RequestGroup{
				RequestID: entry.RequestID,
				Method:    entry.Method,
				Path:      entry.Path,
				StartTime: entry.Timestamp,
				Entries:   []LogEntry{},
			}
			groups[entry.RequestID] = group
		}

		group.LogCount++
		group.Entries = append(group.Entries, entry)

		if entry.Timestamp.Before(group.StartTime) {
			group.StartTime = entry.Timestamp
		}
		if entry.Timestamp.After(group.EndTime) {
			group.EndTime = entry.Timestamp
		}
		if entry.StatusCode > 0 {
			group.StatusCode = entry.StatusCode
		}
	})

	// 计算持续时间并转换为切片
	result := make([]RequestGroup, 0, len(groups))
	for _, group := range groups {
		group.Duration = group.EndTime.Sub(group.StartTime)

		// 只保留概要，不返回所有 entries
		if !query.GroupByRequest {
			group.Entries = nil
		}

		result = append(result, *group)
	}

	// 按时间排序（最新的在前）
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetRequestLogs 获取特定请求的所有日志
func (s *LogStore) GetRequestLogs(requestID string) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []LogEntry
	indices := s.requestIndex[requestID]

	for _, idx := range indices {
		if entry := s.getEntryByIndex(idx); entry != nil {
			entries = append(entries, *entry)
		}
	}

	// 按时间排序
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries
}

// Clear 清空日志
func (s *LogStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = ring.New(s.maxSize)
	s.size = 0
	s.requestIndex = make(map[string][]int)
	s.tagIndex = make(map[string][]int)
}

// matchQuery 检查条目是否匹配查询条件
func (s *LogStore) matchQuery(entry LogEntry, query LogQuery) bool {
	// 级别过滤
	if query.Level != "" && !strings.EqualFold(entry.Level, query.Level) {
		return false
	}

	// 关键词过滤
	if query.Keyword != "" {
		keyword := strings.ToLower(query.Keyword)
		content := strings.ToLower(entry.Message + " " + entry.RawLog)
		if !strings.Contains(content, keyword) {
			return false
		}
	}

	// 时间范围过滤
	if query.StartTime != nil && entry.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && entry.Timestamp.After(*query.EndTime) {
		return false
	}

	return true
}

// getEntryByIndex 通过索引获取条目
func (s *LogStore) getEntryByIndex(index int) *LogEntry {
	if index < 0 || index >= s.head {
		return nil
	}

	// 计算此条目是否还在缓冲区中
	// 缓冲区保留最新的 maxSize 个条目
	oldestValidIndex := s.head - s.size
	if index < oldestValidIndex {
		return nil // 此条目已被覆盖
	}

	// 计算在环形缓冲区中的位置
	// head 指向下一个写入位置，当前条目在 head-1 处
	// 我们要找的条目在 index 位置，需要向后移动 (head - 1 - index) 步
	stepsBack := (s.head - 1 - index) % s.maxSize

	r := s.entries
	// s.entries 当前指向下一个写入位置，所以需要后退 stepsBack+1 步
	for i := 0; i <= stepsBack; i++ {
		r = r.Prev()
	}

	if v := r.Value; v != nil {
		entry := v.(LogEntry)
		return &entry
	}
	return nil
}

// ParseLogEntry 解析原始日志为结构化条目
func ParseLogEntry(rawLog string) LogEntry {
	entry := LogEntry{
		Timestamp: time.Now(),
		RawLog:    rawLog,
		Metadata:  make(map[string]string),
	}

	// 解析级别
	if strings.Contains(rawLog, "[ERROR]") {
		entry.Level = "error"
	} else if strings.Contains(rawLog, "[WARN]") {
		entry.Level = "warn"
	} else if strings.Contains(rawLog, "[DEBUG]") {
		entry.Level = "debug"
		entry.Tags = append(entry.Tags, "debug")
	} else {
		entry.Level = "info"
	}

	// 解析 HTTP 请求信息
	// 格式: [INFO]  [2026-03-03 19:32:28] GET /api/admin/routes 200 775.375µs - 127.0.0.1
	re := regexp.MustCompile(`\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\] (\w+) (\S+) (\d+) ([\d\.µms]+) - (\S+)`)
	if matches := re.FindStringSubmatch(rawLog); len(matches) >= 7 {
		entry.Method = matches[2]
		entry.Path = matches[3]
		entry.StatusCode = parseInt(matches[4])
		entry.ClientIP = matches[6]

		// 生成 request_id (基于时间和路径)
		entry.RequestID = generateRequestID(entry.Timestamp, entry.Method, entry.Path, entry.ClientIP)
	}

	// 解析适配器请求
	if strings.Contains(rawLog, "Adapter Request") {
		entry.Tags = append(entry.Tags, "adapter_request")
		// 提取 provider/model
		re := regexp.MustCompile(`Adapter Request \[(\w+)/(\w+)`)
		if matches := re.FindStringSubmatch(rawLog); len(matches) >= 3 {
			entry.Metadata["provider"] = matches[1]
			entry.Metadata["model"] = matches[2]
		}
	}

	// 解析适配器响应
	if strings.Contains(rawLog, "Adapter Response") {
		entry.Tags = append(entry.Tags, "adapter_response")
	}

	entry.Message = rawLog
	return entry
}

// AddRawLog 添加原始日志
func AddRawLog(rawLog string) {
	entry := ParseLogEntry(rawLog)
	GetLogStore().Add(entry)
}

// QueryLogs 查询日志（便捷函数）
func QueryLogs(level, keyword, requestID string, page, pageSize int) LogResult {
	return GetLogStore().Query(LogQuery{
		Level:     level,
		Keyword:   keyword,
		RequestID: requestID,
		Page:      page,
		PageSize:  pageSize,
	})
}

// GetRequestGroups 获取请求分组（便捷函数）
func GetRequestGroups(keyword string, limit int) []RequestGroup {
	return GetLogStore().GetRequestGroups(LogQuery{
		Keyword:  keyword,
		PageSize: limit,
	})
}

// 辅助函数
func parseInt(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func generateRequestID(t time.Time, method, path, ip string) string {
	// 简化的 request_id 生成
	return t.Format("20060102150405") + "-" + method + "-" + strings.ReplaceAll(path, "/", "-")
}

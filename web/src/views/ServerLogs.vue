<template>
  <div class="server-logs">
    <div class="page-header">
      <div class="header-actions">
        <!-- 视图切换 -->
        <el-radio-group v-model="viewMode" size="small" style="margin-right: 12px;">
          <el-radio-button label="list">{{ $t('serverLogs.listView', '列表') }}</el-radio-button>
          <el-radio-button label="group">{{ $t('serverLogs.groupView', '请求分组') }}</el-radio-button>
        </el-radio-group>

        <!-- 日志级别选择 -->
        <el-select v-model="filters.level" @change="handleSearch" clearable :placeholder="$t('serverLogs.allLevels', '所有级别')" style="width: 100px; margin-right: 12px;">
          <el-option label="Debug" value="debug" />
          <el-option label="Info" value="info" />
          <el-option label="Warn" value="warn" />
          <el-option label="Error" value="error" />
        </el-select>

        <el-button @click="refreshLogs" :loading="loading">
          <el-icon><Refresh /></el-icon>
          {{ $t('common.refresh', '刷新') }}
        </el-button>
        <el-button @click="clearDisplay">
          <el-icon><Delete /></el-icon>
          {{ $t('common.clear', '清空') }}
        </el-button>
      </div>
    </div>

    <!-- 搜索栏 -->
    <el-card class="filter-card" shadow="never">
      <el-form :inline="true" :model="filters" @submit.prevent="handleSearch">
        <el-form-item>
          <el-input
            v-model="filters.keyword"
            :placeholder="$t('serverLogs.searchPlaceholder', '搜索日志内容...')"
            clearable
            style="width: 300px;"
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item v-if="viewMode === 'list'">
          <el-input
            v-model="filters.requestId"
            :placeholder="$t('serverLogs.requestIdPlaceholder', 'Request ID')"
            clearable
            style="width: 200px;"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">
            {{ $t('common.search', '搜索') }}
          </el-button>
          <el-button @click="resetFilter">
            {{ $t('common.reset', '重置') }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 统计信息 -->
    <div class="stats-bar" v-if="viewMode === 'list'">
      <el-tag size="small">{{ $t('serverLogs.total', '总计') }}: {{ pagination.total }}</el-tag>
      <el-tag size="small" type="info" v-if="pagination.has_more">{{ $t('serverLogs.hasMore', '还有更多') }}</el-tag>
    </div>

    <!-- 列表视图 -->
    <div v-if="viewMode === 'list'" class="log-container" ref="logContainer" @scroll="handleScroll">
      <div v-if="logs.length === 0 && !loading" class="empty-logs">
        <el-empty :description="$t('serverLogs.noLogs', '暂无日志')" />
      </div>
      
      <!-- 虚拟列表 -->
      <div v-else class="log-list">
        <div
          v-for="log in logs"
          :key="log.timestamp + log.message"
          :class="['log-item', 'log-' + log.level]"
          @click="showLogDetail(log)"
        >
          <div class="log-header">
            <span class="log-time">{{ formatTime(log.timestamp) }}</span>
            <el-tag :type="getLevelType(log.level)" size="small" effect="plain">{{ log.level.toUpperCase() }}</el-tag>
            <span v-if="log.method" class="log-method">{{ log.method }}</span>
            <span v-if="log.path" class="log-path">{{ log.path }}</span>
            <span v-if="log.status_code" :class="['log-status', getStatusClass(log.status_code)]">{{ log.status_code }}</span>
            <span v-if="log.request_id" class="log-request-id" @click.stop="filterByRequestId(log.request_id)">
              <el-icon><Link /></el-icon>
              {{ truncateRequestId(log.request_id) }}
            </span>
          </div>
          <div class="log-message" v-html="highlightText(log.message)"></div>
        </div>
        
        <!-- 加载更多 -->
        <div v-if="loading" class="loading-more">
          <el-icon class="is-loading"><Loading /></el-icon>
          {{ $t('common.loading', '加载中...') }}
        </div>
        <div v-else-if="pagination.has_more" class="load-more">
          <el-button link @click="loadMore">{{ $t('serverLogs.loadMore', '加载更多') }}</el-button>
        </div>
      </div>
    </div>

    <!-- 请求分组视图 -->
    <div v-else class="group-container">
      <el-table :data="requestGroups" v-loading="loading" stripe>
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="request-logs">
              <div v-for="log in row.entries" :key="log.timestamp" :class="['request-log-item', 'log-' + log.level]">
                <span class="log-time">{{ formatTime(log.timestamp) }}</span>
                <el-tag :type="getLevelType(log.level)" size="small">{{ log.level }}</el-tag>
                <span class="log-message">{{ log.message }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="$t('serverLogs.requestId', 'Request ID')" min-width="200">
          <template #default="{ row }">
            <code class="request-id-code">{{ truncateRequestId(row.request_id, 20) }}</code>
          </template>
        </el-table-column>
        <el-table-column :label="$t('serverLogs.method', 'Method')" width="80">
          <template #default="{ row }">
            <span class="method-tag">{{ row.method }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="$t('serverLogs.path', 'Path')" min-width="200">
          <template #default="{ row }">
            {{ row.path }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('serverLogs.status', 'Status')" width="90">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status_code)" size="small">{{ row.status_code || '-' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="$t('serverLogs.duration', 'Duration')" width="100">
          <template #default="{ row }">
            {{ formatDuration(row.duration) }}
          </template>
        </el-table-column>
        <el-table-column :label="$t('serverLogs.logCount', '日志数')" width="80">
          <template #default="{ row }">
            <el-badge :value="row.log_count" />
          </template>
        </el-table-column>
        <el-table-column :label="$t('serverLogs.time', 'Time')" width="160">
          <template #default="{ row }">
            {{ formatTime(row.start_time) }}
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 日志详情对话框 -->
    <el-dialog v-model="detailVisible" :title="$t('serverLogs.logDetail', '日志详情')" width="700px">
      <el-descriptions :column="1" border v-if="currentLog">
        <el-descriptions-item :label="$t('serverLogs.timestamp', '时间')">
          {{ formatTime(currentLog.timestamp) }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('serverLogs.level', '级别')">
          <el-tag :type="getLevelType(currentLog.level)">{{ currentLog.level.toUpperCase() }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item v-if="currentLog.request_id" :label="$t('serverLogs.requestId', 'Request ID')">
          <code>{{ currentLog.request_id }}</code>
          <el-button link type="primary" size="small" @click="filterByRequestId(currentLog.request_id)">
            {{ $t('serverLogs.viewAll', '查看全部') }}
          </el-button>
        </el-descriptions-item>
        <el-descriptions-item v-if="currentLog.method" :label="$t('serverLogs.request', '请求')">
          {{ currentLog.method }} {{ currentLog.path }}
        </el-descriptions-item>
        <el-descriptions-item v-if="currentLog.status_code" :label="$t('serverLogs.status', '状态')">
          {{ currentLog.status_code }}
        </el-descriptions-item>
        <el-descriptions-item v-if="currentLog.client_ip" :label="$t('serverLogs.clientIp', '客户端')">
          {{ currentLog.client_ip }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('serverLogs.message', '消息')">
          <pre class="log-detail-content">{{ currentLog.message }}</pre>
        </el-descriptions-item>
        <el-descriptions-item v-if="currentLog.raw_log !== currentLog.message" :label="$t('serverLogs.rawLog', '原始日志')">
          <pre class="log-detail-content">{{ currentLog.raw_log }}</pre>
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import axios from 'axios'


const { t } = useI18n()

// 状态
const loading = ref(false)
const viewMode = ref('list') // 'list' | 'group'
const logs = ref([])
const requestGroups = ref([])
const detailVisible = ref(false)
const currentLog = ref(null)
const logContainer = ref(null)

// 过滤条件
const filters = reactive({
  level: '',
  keyword: '',
  requestId: ''
})

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 50,
  total: 0,
  has_more: false
})

// 自动刷新定时器
let refreshTimer = null

// 获取日志列表
async function fetchLogs(loadMore = false) {
  if (loading.value) return
  
  loading.value = true
  
  if (!loadMore) {
    pagination.page = 1
    logs.value = []
  }
  
  try {
    const params = new URLSearchParams({
      page: pagination.page.toString(),
      page_size: pagination.pageSize.toString()
    })
    
    if (filters.level) params.append('level', filters.level)
    if (filters.keyword) params.append('keyword', filters.keyword)
    if (filters.requestId) params.append('request_id', filters.requestId)
    
    const { data } = await axios.get(`/api/admin/server-logs?${params}`)
    
    if (loadMore) {
      logs.value.push(...(data.entries || []))
    } else {
      logs.value = data.entries || []
    }
    
    pagination.total = data.total || 0
    pagination.has_more = data.has_more || false
  } catch (e) {
    console.error('Failed to fetch logs:', e)
    ElMessage.error(t('serverLogs.fetchFailed', '获取日志失败'))
  } finally {
    loading.value = false
  }
}

// 获取请求分组
async function fetchRequestGroups() {
  if (loading.value) return
  
  loading.value = true
  
  try {
    const params = new URLSearchParams({
      group_by_request: 'true',
      page_size: '100'
    })
    
    if (filters.keyword) params.append('keyword', filters.keyword)
    
    const { data } = await axios.get(`/api/admin/server-logs?${params}`)
    requestGroups.value = data.groups || []
  } catch (e) {
    console.error('Failed to fetch groups:', e)
    ElMessage.error(t('serverLogs.fetchFailed', '获取请求分组失败'))
  } finally {
    loading.value = false
  }
}

// 加载更多
async function loadMore() {
  if (!pagination.has_more || loading.value) return
  pagination.page++
  await fetchLogs(true)
}

// 搜索
function handleSearch() {
  if (viewMode.value === 'list') {
    fetchLogs()
  } else {
    fetchRequestGroups()
  }
}

// 重置过滤
function resetFilter() {
  filters.level = ''
  filters.keyword = ''
  filters.requestId = ''
  handleSearch()
}

// 刷新
function refreshLogs() {
  handleSearch()
}

// 清空
async function clearDisplay() {
  try {
    await ElMessageBox.confirm(
      t('serverLogs.clearConfirm', '确定要清空日志吗？'),
      t('common.warning', '警告'),
      { type: 'warning' }
    )
    
    await axios.delete('/api/admin/server-logs')
    logs.value = []
    requestGroups.value = []
    pagination.total = 0
    pagination.has_more = false
    ElMessage.success(t('message.clearSuccess', '已清空'))
  } catch (e) {
    if (e !== 'cancel') {
      console.error('Failed to clear logs:', e)
    }
  }
}

// 显示日志详情
function showLogDetail(log) {
  currentLog.value = log
  detailVisible.value = true
}

// 按 Request ID 过滤
function filterByRequestId(requestId) {
  filters.requestId = requestId
  viewMode.value = 'list'
  detailVisible.value = false
  fetchLogs()
}

// 滚动加载
function handleScroll() {
  if (!logContainer.value || viewMode.value !== 'list') return
  
  const { scrollTop, scrollHeight, clientHeight } = logContainer.value
  const isNearBottom = scrollHeight - scrollTop - clientHeight < 100
  
  if (isNearBottom && pagination.has_more && !loading.value) {
    loadMore()
  }
}

// 格式化时间
function formatTime(time) {
  if (!time) return '-'
  const date = new Date(time)
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 格式化持续时间
function formatDuration(duration) {
  if (!duration) return '-'
  // duration 可能是字符串或数字（纳秒）
  const ns = parseInt(duration)
  if (ns < 1000) return ns + 'ns'
  if (ns < 1000000) return (ns / 1000).toFixed(2) + 'µs'
  if (ns < 1000000000) return (ns / 1000000).toFixed(2) + 'ms'
  return (ns / 1000000000).toFixed(2) + 's'
}

// 截断 Request ID
function truncateRequestId(id, length = 12) {
  if (!id) return ''
  if (id.length <= length) return id
  return id.substring(0, length) + '...'
}

// 高亮搜索文本
function highlightText(text) {
  if (!filters.keyword || !text) return text
  const keyword = filters.keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const regex = new RegExp(`(${keyword})`, 'gi')
  return text.replace(regex, '<mark>$1</mark>')
}

// 获取级别类型
function getLevelType(level) {
  switch (level) {
    case 'error': return 'danger'
    case 'warn': return 'warning'
    case 'debug': return 'info'
    default: return 'success'
  }
}

// 获取状态类型
function getStatusType(status) {
  if (status >= 500) return 'danger'
  if (status >= 400) return 'warning'
  if (status >= 300) return 'info'
  return 'success'
}

// 获取状态样式类
function getStatusClass(status) {
  if (status >= 500) return 'status-error'
  if (status >= 400) return 'status-warning'
  return 'status-success'
}

// 监听视图模式变化
watch(viewMode, (newMode) => {
  if (newMode === 'list') {
    fetchLogs()
  } else {
    fetchRequestGroups()
  }
})

import { watch } from 'vue'

// 生命周期
onMounted(() => {
  fetchLogs()
  // 每 10 秒自动刷新
  refreshTimer = setInterval(() => {
    if (viewMode.value === 'list' && !filters.requestId && !loading.value) {
      // 静默刷新，不重置列表
      fetchLogs()
    }
  }, 10000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<style scoped>
.server-logs {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.header-actions {
  display: flex;
  align-items: center;
}

.filter-card {
  margin-bottom: 12px;
}

.stats-bar {
  margin-bottom: 12px;
  padding: 8px 0;
}

/* 日志容器 */
.log-container {
  flex: 1;
  overflow-y: auto;
  background: #1e1e1e;
  border-radius: 8px;
  padding: 12px;
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
}

.empty-logs {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.log-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.log-item {
  padding: 8px 12px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.2s;
  border-left: 3px solid transparent;
}

.log-item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.log-error {
  border-left-color: #f56c6c;
  background: rgba(245, 108, 108, 0.05);
}

.log-warn {
  border-left-color: #e6a23c;
  background: rgba(230, 162, 60, 0.05);
}

.log-debug {
  border-left-color: #909399;
}

.log-info {
  border-left-color: #67c23a;
}

.log-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  flex-wrap: wrap;
}

.log-time {
  color: #858585;
  font-size: 11px;
  min-width: 70px;
}

.log-method {
  color: #9cdcfe;
  font-weight: bold;
}

.log-path {
  color: #d4d4d4;
}

.log-status {
  font-weight: bold;
}

.status-success {
  color: #67c23a;
}

.status-warning {
  color: #e6a23c;
}

.status-error {
  color: #f56c6c;
}

.log-request-id {
  color: #66d9ef;
  cursor: pointer;
  font-size: 11px;
  display: flex;
  align-items: center;
  gap: 4px;
}

.log-request-id:hover {
  text-decoration: underline;
}

.log-message {
  color: #d4d4d4;
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.5;
}

.log-message :deep(mark) {
  background: #ffd700;
  color: #000;
  padding: 0 2px;
  border-radius: 2px;
}

/* 加载更多 */
.loading-more, .load-more {
  text-align: center;
  padding: 16px;
  color: #909399;
}

/* 分组视图 */
.group-container {
  flex: 1;
  overflow-y: auto;
}

.request-id-code {
  font-family: monospace;
  font-size: 12px;
  color: #606266;
}

.method-tag {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 11px;
  font-weight: bold;
  background: #409eff;
  color: white;
}

.request-logs {
  padding: 12px;
  background: #f5f7fa;
  border-radius: 4px;
}

.request-log-item {
  padding: 6px 0;
  border-bottom: 1px solid #ebeef5;
  display: flex;
  align-items: center;
  gap: 8px;
}

.request-log-item:last-child {
  border-bottom: none;
}

/* 日志详情 */
.log-detail-content {
  margin: 0;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 4px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
  font-size: 12px;
  line-height: 1.6;
}
</style>

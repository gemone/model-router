<template>
  <div class="logs">
    <div class="page-header">
      <div class="header-actions">
        <el-button @click="exportLogs">
          <el-icon><Download /></el-icon>
          {{ $t('logs.exportLogs') }}
        </el-button>
        <el-button type="danger" @click="clearLogs">
          <el-icon><Delete /></el-icon>
          {{ $t('logs.clearLogs') }}
        </el-button>
      </div>
    </div>

    <!-- 筛选栏 -->
    <el-card class="filter-card" shadow="never">
      <el-form :inline="true" :model="filters">
        <el-form-item :label="$t('logs.model')">
          <el-input
            v-model="filters.model"
            :placeholder="$t('logs.model')"
            clearable
            @clear="handleSearch"
          />
        </el-form-item>
        <el-form-item :label="$t('logs.status')">
          <el-select v-model="filters.status" clearable :placeholder="$t('common.all')" @change="handleSearch">
            <el-option :label="$t('logs.success')" value="success" />
            <el-option :label="$t('logs.error')" value="error" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">
            <el-icon><Search /></el-icon>
            {{ $t('common.search') }}
          </el-button>
          <el-button @click="handleReset">
            <el-icon><RefreshLeft /></el-icon>
            {{ $t('common.reset') }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 日志表格 -->
    <el-table :data="displayLogs" v-loading="store.loading" stripe size="small">
      <el-table-column prop="request_id" label="ID" width="200" show-overflow-tooltip />
      <el-table-column prop="model" :label="$t('logs.model')" width="150" />
      <el-table-column prop="provider_id" :label="$t('logs.provider')" width="120" />
      <el-table-column :label="$t('logs.status')" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">
            {{ row.status === 'success' ? $t('logs.success') : $t('logs.error') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$t('logs.latency')" width="100">
        <template #default="{ row }">
          {{ row.latency }}ms
        </template>
      </el-table-column>
      <el-table-column :label="$t('logs.tokens')" width="120">
        <template #default="{ row }">
          {{ row.total_tokens || '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="client_ip" :label="$t('logs.clientIp')" width="120" />
      <el-table-column :label="$t('logs.timestamp')" width="180">
        <template #default="{ row }">
          {{ formatTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column :label="$t('common.actions')" width="100" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="viewDetail(row)">
            {{ $t('logs.viewDetail') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :page-sizes="[20, 50, 100, 200]"
        :total="pagination.total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="handleSizeChange"
        @current-change="handlePageChange"
      />
    </div>

    <!-- 详情对话框 -->
    <el-dialog
      v-model="detailVisible"
      :title="$t('logs.requestDetail')"
      width="800px"
    >
      <el-descriptions :column="2" border>
        <el-descriptions-item :span="2" :label="$t('logs.requestId')">
          {{ currentLog.request_id }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.model')">
          {{ currentLog.model }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.provider')">
          {{ currentLog.provider_id }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.status')">
          <el-tag :type="currentLog.status === 'success' ? 'success' : 'danger'" size="small">
            {{ currentLog.status === 'success' ? $t('logs.success') : $t('logs.error') }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.latency')">
          {{ currentLog.latency }}ms
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.promptTokens')">
          {{ currentLog.prompt_tokens || '-' }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.completionTokens')">
          {{ currentLog.completion_tokens || '-' }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.tokens')">
          {{ currentLog.total_tokens || '-' }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.clientIp')">
          {{ currentLog.client_ip || '-' }}
        </el-descriptions-item>
        <el-descriptions-item :label="$t('logs.timestamp')">
          {{ formatTime(currentLog.created_at) }}
        </el-descriptions-item>
        <el-descriptions-item v-if="currentLog.error_message" :span="2" :label="$t('common.error')">
          <span class="error-message">{{ currentLog.error_message }}</span>
        </el-descriptions-item>
      </el-descriptions>

      <div v-if="currentLog.request_body" class="json-section">
        <h4>{{ $t('logs.requestDetail') }}</h4>
        <el-input
          type="textarea"
          :model-value="formatJson(currentLog.request_body)"
          :rows="6"
          readonly
        />
      </div>

      <div v-if="currentLog.response_body" class="json-section">
        <h4>{{ $t('logs.responseDetail') }}</h4>
        <el-input
          type="textarea"
          :model-value="formatJson(currentLog.response_body)"
          :rows="6"
          readonly
        />
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'


const { t } = useI18n()
const store = useAppStore()

const filters = ref({
  model: '',
  status: '',
})

const pagination = ref({
  page: 1,
  pageSize: 50,
  total: 0,
})

const detailVisible = ref(false)
const currentLog = ref({})

// 过滤后的日志
const filteredLogs = computed(() => {
  let logs = [...store.logs]

  if (filters.value.model) {
    logs = logs.filter(log => log.model?.includes(filters.value.model))
  }

  if (filters.value.status) {
    logs = logs.filter(log => log.status === filters.value.status)
  }

  return logs
})

// 当前页显示的日志
const displayLogs = computed(() => {
  const start = (pagination.value.page - 1) * pagination.value.pageSize
  const end = start + pagination.value.pageSize
  pagination.value.total = filteredLogs.value.length
  return filteredLogs.value.slice(start, end)
})

function formatTime(time) {
  if (!time) return '-'
  return new Date(time).toLocaleString()
}

function formatJson(data) {
  try {
    if (typeof data === 'string') {
      return JSON.stringify(JSON.parse(data), null, 2)
    }
    return JSON.stringify(data, null, 2)
  } catch {
    return data || ''
  }
}

function viewDetail(log) {
  currentLog.value = log
  detailVisible.value = true
}

function handleSearch() {
  pagination.value.page = 1
}

function handleReset() {
  filters.value = {
    model: '',
    status: '',
  }
  pagination.value.page = 1
}

function handlePageChange(page) {
  pagination.value.page = page
}

function handleSizeChange(size) {
  pagination.value.pageSize = size
  pagination.value.page = 1
}

function exportLogs() {
  const data = filteredLogs.value.map(log => ({
    ID: log.request_id,
    Model: log.model,
    Provider: log.provider_id,
    Status: log.status,
    Latency: log.latency,
    Tokens: log.total_tokens,
    Time: formatTime(log.created_at),
  }))

  const csv = [
    Object.keys(data[0] || {}).join(','),
    ...data.map(row => Object.values(row).join(',')),
  ].join('\n')

  const blob = new Blob([csv], { type: 'text/csv' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `logs_${new Date().toISOString()}.csv`
  a.click()
  URL.revokeObjectURL(url)

  ElMessage.success(t('message.copySuccess'))
}

async function clearLogs() {
  ElMessageBox.confirm(t('message.confirmDelete'), 'Warning', { type: 'warning' })
    .then(async () => {
      await store.clearLogs()
      ElMessage.success(t('message.deleteSuccess'))
    })
    .catch(() => {})
}

onMounted(() => {
  store.fetchLogs()
})
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: #1f2937;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.filter-card {
  margin-bottom: 16px;
}

.pagination-wrapper {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

.json-section {
  margin-top: 20px;
}

.json-section h4 {
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 500;
  color: #606266;
}

.error-message {
  color: #f56c6c;
}
</style>

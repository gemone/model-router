<template>
  <div class="settings">
    <!-- 面包屑导航 -->
    <div class="breadcrumb-wrapper">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item :to="{ path: '/' }">Home</el-breadcrumb-item>
        <el-breadcrumb-item>{{ $t("settings.title") }}</el-breadcrumb-item>
      </el-breadcrumb>
    </div>

    <h2 class="page-title">{{ $t('settings.title') }}</h2>

    <!-- 系统信息卡片 -->
    <el-card class="info-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <span class="card-title">{{ $t('settings.systemInfo') }}</span>
          <el-button link @click="refreshSystemInfo">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>
      </template>
      <el-row :gutter="24">
        <el-col :xs="12" :sm="8" :md="6">
          <div class="info-item">
            <div class="info-label">{{ $t('settings.version') }}</div>
            <div class="info-value">{{ systemInfo.version }}</div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="6">
          <div class="info-item">
            <div class="info-label">{{ $t('settings.uptime') }}</div>
            <div class="info-value">{{ systemInfo.uptime }}</div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="6">
          <div class="info-item">
            <div class="info-label">{{ $t('settings.dbSize') }}</div>
            <div class="info-value">{{ systemInfo.dbSize }}</div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="6">
          <div class="info-item">
            <div class="info-label">{{ $t('settings.dbStatus') }}</div>
            <el-tag :type="systemInfo.dbHealthy ? 'success' : 'danger'" size="small">
              {{ systemInfo.dbHealthy ? $t('common.enabled') : $t('common.disabled') }}
            </el-tag>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <el-tabs type="border-card" v-model="activeTab" class="settings-tabs">
      <el-tab-pane :label="$t('settings.general')" name="general">
        <el-form :model="settings" label-width="160px" class="settings-form">
          <el-form-item :label="$t('settings.port')">
            <el-input-number v-model="settings.port" :min="1" :max="65535" />
            <div class="form-tip">{{ $t('settings.saveSettings') }} 后重启生效</div>
          </el-form-item>
          <el-form-item :label="$t('settings.host')">
            <el-input v-model="settings.host" />
          </el-form-item>
          <el-form-item :label="$t('settings.language')">
            <el-select v-model="settings.language" @change="changeLanguage">
              <el-option label="中文" value="zh-CN" />
              <el-option label="English" value="en-US" />
            </el-select>
          </el-form-item>
          <el-form-item :label="$t('settings.enableCors')">
            <el-switch v-model="settings.enable_cors" />
          </el-form-item>
          <el-form-item :label="$t('settings.enableStats')">
            <el-switch v-model="settings.enable_stats" />
          </el-form-item>
          <el-form-item :label="$t('settings.enableFallback')">
            <el-switch v-model="settings.enable_fallback" />
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="$t('settings.security')" name="security">
        <el-form :model="settings" label-width="160px" class="settings-form">
          <el-form-item :label="$t('settings.adminToken')">
            <el-input v-model="settings.admin_token" type="password" show-password :placeholder="adminTokenPlaceholder" />
            <div class="form-tip">{{ $t('settings.leaveEmptyToKeep') }}</div>
          </el-form-item>
          <el-form-item :label="$t('settings.jwtSecret')">
            <el-input v-model="settings.jwt_secret" type="password" show-password :placeholder="jwtSecretPlaceholder" />
            <div class="form-tip">{{ $t('settings.leaveEmptyToKeep') }}</div>
          </el-form-item>
          <el-form-item :label="$t('settings.generateSecret')">
            <el-button @click="generateSecret">
              <el-icon><RefreshRight /></el-icon>
              {{ $t('settings.generateNewSecret') }}
            </el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="$t('settings.advanced')" name="advanced">
        <el-form :model="settings" label-width="160px" class="settings-form">
          <el-form-item :label="$t('settings.logLevel')">
            <el-select v-model="settings.log_level">
              <el-option :label="$t('settings.logLevelDebug')" value="debug" />
              <el-option :label="$t('settings.logLevelInfo')" value="info" />
              <el-option :label="$t('settings.logLevelWarn')" value="warn" />
              <el-option :label="$t('settings.logLevelError')" value="error" />
            </el-select>
          </el-form-item>
          <el-form-item :label="$t('settings.maxRetries')">
            <el-input-number v-model="settings.max_retries" :min="0" :max="10" />
          </el-form-item>
          <el-form-item :label="$t('settings.dbPath')">
            <el-input v-model="settings.db_path" />
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 配置导入导出 -->
      <el-tab-pane :label="$t('settings.configManagement')" name="config">
        <div class="config-management">
          <el-alert
            :title="$t('settings.configBackupTip')"
            type="info"
            :closable="false"
            show-icon
            class="config-alert"
          />

          <div class="config-actions">
            <el-card shadow="hover">
              <template #header>
                <div class="card-title">{{ $t('settings.exportConfig') }}</div>
              </template>
              <p class="config-desc">{{ $t('settings.exportConfigDesc') }}</p>
              <el-button type="primary" @click="exportConfig" :icon="Download">
                {{ $t('settings.exportButton') }}
              </el-button>
            </el-card>

            <el-card shadow="hover">
              <template #header>
                <div class="card-title">{{ $t('settings.importConfig') }}</div>
              </template>
              <p class="config-desc">{{ $t('settings.importConfigDesc') }}</p>
              <el-upload
                ref="uploadRef"
                :auto-upload="false"
                :on-change="handleFileSelect"
                :show-file-list="false"
                accept=".json"
              >
                <el-button type="success" :icon="Upload">
                  {{ $t('settings.importButton') }}
                </el-button>
              </el-upload>
            </el-card>

            <el-card shadow="hover">
              <template #header>
                <div class="card-title">{{ $t('settings.resetConfig') }}</div>
              </template>
              <p class="config-desc">{{ $t('settings.resetConfigDesc') }}</p>
              <el-button type="danger" @click="confirmReset" :icon="Delete">
                {{ $t('settings.resetButton') }}
              </el-button>
            </el-card>
          </div>
        </div>
      </el-tab-pane>

      <!-- 压缩模型组配置 -->
      <el-tab-pane :label="$t('settings.compressionGroups')" name="compression">
        <div class="compression-section">
          <el-alert
            :title="$t('settings.compressionTip')"
            type="info"
            :closable="false"
            show-icon
            class="config-alert"
          />

          <div class="compression-list">
            <el-card
              v-for="group in compressionGroups"
              :key="group.id"
              class="compression-card"
              shadow="hover"
            >
              <template #header>
                <div class="card-header">
                  <span class="card-title">{{ group.name }}</span>
                  <el-tag size="small">{{ group.models?.length || 0 }} models</el-tag>
                </div>
              </template>
              <div class="compression-details">
                <div class="detail-item">
                  <span class="label">{{ $t('models.models') }}:</span>
                  <el-tag
                    v-for="model in group.models"
                    :key="model"
                    size="small"
                    class="model-tag"
                  >
                    {{ model }}
                  </el-tag>
                </div>
              </div>
            </el-card>

            <el-empty
              v-if="!compressionGroups.length"
              :description="$t('settings.noCompressionGroups')"
            />
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <div class="actions">
      <el-button @click="resetSettings">{{ $t('settings.resetDefaults') }}</el-button>
      <el-button type="primary" @click="saveSettings" :loading="saving">
        {{ $t('settings.saveSettings') }}
      </el-button>
    </div>

    <!-- 导入配置确认对话框 -->
    <el-dialog
      v-model="importDialogVisible"
      :title="$t('settings.confirmImport')"
      width="500px"
    >
      <div class="import-preview">
        <el-descriptions :column="2" border>
          <el-descriptions-item :label="$t('settings.configVersion')">
            {{ importPreview.version || 'N/A' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('settings.exportedAt')">
            {{ importPreview.exportedAt || 'N/A' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('profiles.title')">
            {{ importPreview.profilesCount || 0 }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('provider.title')">
            {{ importPreview.providersCount || 0 }}
          </el-descriptions-item>
        </el-descriptions>
      </div>
      <el-alert
        :title="$t('settings.importWarning')"
        type="warning"
        :closable="false"
        show-icon
        style="margin-top: 16px"
      />
      <template #footer>
        <el-button @click="importDialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmImport" :loading="importing">
          {{ $t('settings.confirmImportButton') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import {
  Download,
  Upload,
  Delete,
  Refresh,
  RefreshRight
} from '@element-plus/icons-vue'

const { t, locale } = useI18n()
const store = useAppStore()

const activeTab = ref('general')
const saving = ref(false)
const importing = ref(false)
const adminTokenPlaceholder = ref('••••••••')
const jwtSecretPlaceholder = ref('••••••••')
const importDialogVisible = ref(false)
const importPreview = ref({})
const importFile = ref(null)

// 系统信息
const systemInfo = ref({
  version: '1.0.0',
  uptime: '-',
  dbSize: '-',
  dbHealthy: true,
})

// 压缩模型组
const compressionGroups = ref([])

const defaultSettings = {
  port: 8080,
  host: '0.0.0.0',
  language: 'zh-CN',
  enable_cors: true,
  enable_stats: true,
  enable_fallback: true,
  admin_token: '',
  jwt_secret: '',
  log_level: 'info',
  max_retries: 3,
  db_path: '',
}

const settings = ref({ ...defaultSettings })

async function loadSettings() {
  try {
    const data = await store.fetchSettings()
    settings.value = { ...defaultSettings, ...data }
  } catch (e) {
    ElMessage.error(t('message.saveFailed'))
  }
}

async function refreshSystemInfo() {
  // 获取系统信息
  try {
    const startTime = Date.now()
    // 这里应该调用实际的 API
    // const info = await store.fetchSystemInfo()
    systemInfo.value = {
      version: '1.0.0',
      uptime: calculateUptime(startTime),
      dbSize: '0 MB',
      dbHealthy: true,
    }
  } catch (e) {
    console.error('Failed to fetch system info:', e)
  }
}

function calculateUptime(startTime) {
  // 模拟计算运行时间
  const uptime = Date.now() - startTime
  const seconds = Math.floor(uptime / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (days > 0) return `${days}d ${hours % 24}h`
  if (hours > 0) return `${hours}h ${minutes % 60}m`
  if (minutes > 0) return `${minutes}m ${seconds % 60}s`
  return `${seconds}s`
}

async function loadCompressionGroups() {
  try {
    // 这里应该调用实际的 API 获取压缩组列表
    // const groups = await store.fetchCompressionGroups()
    compressionGroups.value = []
  } catch (e) {
    console.error('Failed to fetch compression groups:', e)
  }
}

function changeLanguage(lang) {
  locale.value = lang
  localStorage.setItem('locale', lang)
  settings.value.language = lang
}

async function saveSettings() {
  saving.value = true
  try {
    // 如果是占位符，不发送密码字段
    const payload = { ...settings.value }
    if (payload.admin_token === adminTokenPlaceholder.value || !payload.admin_token) {
      delete payload.admin_token
    }
    if (payload.jwt_secret === jwtSecretPlaceholder.value || !payload.jwt_secret) {
      delete payload.jwt_secret
    }

    const result = await store.updateSettings(payload)
    ElMessage.success(result.message || t('message.saveSuccess'))
  } catch (e) {
    ElMessage.error(t('message.saveFailed'))
  } finally {
    saving.value = false
  }
}

function resetSettings() {
  settings.value = { ...defaultSettings }
  ElMessage.success(t('settings.resetDefaults'))
}

function generateSecret() {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*'
  let secret = ''
  for (let i = 0; i < 32; i++) {
    secret += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  settings.value.jwt_secret = secret
  ElMessage.success(t('settings.secretGenerated'))
}

// 配置导出
async function exportConfig() {
  try {
    const config = {
      version: '1.0.0',
      exportedAt: new Date().toISOString(),
      settings: settings.value,
      profiles: store.profiles,
      providers: store.providers,
      models: store.models,
    }

    const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `model-router-config-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)

    ElMessage.success(t('message.saveSuccess'))
  } catch (e) {
    ElMessage.error(t('message.saveFailed'))
  }
}

// 配置导入
function handleFileSelect(file) {
  importFile.value = file.raw
  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const config = JSON.parse(e.target.result)
      importPreview.value = {
        version: config.version,
        exportedAt: new Date(config.exportedAt).toLocaleString(),
        profilesCount: config.profiles?.length || 0,
        providersCount: config.providers?.length || 0,
      }
      importDialogVisible.value = true
    } catch (err) {
      ElMessage.error(t('settings.invalidConfigFile'))
    }
  }
  reader.readAsText(file.raw)
}

async function confirmImport() {
  if (!importFile.value) return

  importing.value = true
  try {
    const reader = new FileReader()
    reader.onload = async (e) => {
      try {
        const config = JSON.parse(e.target.result)
        // 这里应该调用实际的导入 API
        // await store.importConfig(config)
        ElMessage.success(t('settings.importSuccess'))
        importDialogVisible.value = false
        // 刷新数据
        await Promise.all([
          store.fetchProfiles(),
          store.fetchProviders(),
          store.fetchModels(),
        ])
      } catch (err) {
        ElMessage.error(t('settings.importFailed'))
      }
    }
    reader.readAsText(importFile.value)
  } finally {
    importing.value = false
  }
}

async function confirmReset() {
  ElMessageBox.confirm(
    t('settings.resetWarning'),
    t('settings.resetConfig'),
    {
      type: 'warning',
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
    }
  )
    .then(() => {
      settings.value = { ...defaultSettings }
      ElMessage.success(t('settings.resetSuccess'))
    })
    .catch(() => {})
}

onMounted(async () => {
  await loadSettings()
  await refreshSystemInfo()
  await loadCompressionGroups()
})
</script>

<style scoped>
.breadcrumb-wrapper {
  margin-bottom: 16px;
}

.page-title {
  margin-bottom: 24px;
  font-size: 20px;
  font-weight: 600;
  color: #1f2937;
}

.info-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-weight: 500;
  color: #1f2937;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 12px;
  color: #6b7280;
}

.info-value {
  font-size: 16px;
  font-weight: 500;
  color: #1f2937;
}

.settings-tabs {
  margin-bottom: 24px;
}

.settings-form {
  max-width: 600px;
  padding: 20px;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.config-management {
  padding: 20px;
}

.config-alert {
  margin-bottom: 20px;
}

.config-actions {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}

.config-desc {
  color: #6b7280;
  font-size: 14px;
  margin: 12px 0;
  min-height: 40px;
}

.compression-section {
  padding: 20px;
}

.compression-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.compression-card {
  margin-bottom: 0;
}

.compression-details {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  flex-wrap: wrap;
}

.detail-item .label {
  font-size: 13px;
  color: #606266;
  min-width: 60px;
}

.model-tag {
  margin: 2px;
}

.actions {
  margin-top: 24px;
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

.import-preview {
  margin-bottom: 16px;
}

/* Responsive */
@media (max-width: 768px) {
  .config-actions {
    grid-template-columns: 1fr;
  }

  .compression-list {
    grid-template-columns: 1fr;
  }
}
</style>

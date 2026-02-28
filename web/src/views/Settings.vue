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

    <el-tabs type="border-card" v-model="activeTab">
      <el-tab-pane :label="$t('settings.general')" name="general">
        <el-form :model="settings" label-width="160px" class="settings-form">
          <el-form-item :label="$t('settings.port')">
            <el-input-number v-model="settings.port" :min="1" :max="65535" />
            <div class="form-tip">{{ $t('settings.port') }} ({{ $t('settings.saveSettings') }} 后重启生效)</div>
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
            <div class="form-tip">留空则不修改</div>
          </el-form-item>
          <el-form-item :label="$t('settings.jwtSecret')">
            <el-input v-model="settings.jwt_secret" type="password" show-password :placeholder="jwtSecretPlaceholder" />
            <div class="form-tip">留空则不修改</div>
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
    </el-tabs>

    <div class="actions">
      <el-button @click="resetSettings">{{ $t('settings.resetDefaults') }}</el-button>
      <el-button type="primary" @click="saveSettings" :loading="saving">
        {{ $t('settings.saveSettings') }}
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'

const { t, locale } = useI18n()
const store = useAppStore()

const activeTab = ref('general')
const saving = ref(false)
const adminTokenPlaceholder = ref('••••••••')
const jwtSecretPlaceholder = ref('••••••••')

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

onMounted(() => {
  loadSettings()
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

.settings-form {
  max-width: 600px;
  padding: 20px;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.actions {
  margin-top: 24px;
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}
</style>

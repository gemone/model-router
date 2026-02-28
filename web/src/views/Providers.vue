<template>
  <div class="providers">
    <div class="page-header">
      <h2>{{ $t('provider.title') }}</h2>
      <el-button type="primary" @click="showAddDialog">
        <el-icon><Plus /></el-icon>
        {{ $t('provider.addProvider') }}
      </el-button>
    </div>

    <el-table :data="providers" v-loading="store.loading" stripe>
      <el-table-column prop="name" :label="$t('provider.name')" />
      <el-table-column prop="type" :label="$t('provider.type')" width="120">
        <template #default="{ row }">
          <el-tag size="small">{{ row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="base_url" :label="$t('provider.baseUrl')" show-overflow-tooltip />
      <el-table-column :label="$t('provider.healthStatus')" width="120">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'danger'" size="small">
            {{ row.enabled ? 'Healthy' : 'Offline' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$t('common.actions')" width="200">
        <template #default="{ row }">
          <el-button link type="primary" @click="testProvider(row)">
            {{ $t('provider.testConnection') }}
          </el-button>
          <el-button link type="primary" @click="editProvider(row)">
            {{ $t('common.edit') }}
          </el-button>
          <el-button link type="danger" @click="confirmDelete(row)">
            {{ $t('common.delete') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? $t('provider.editProvider') : $t('provider.addProvider')" width="600px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="140px">
        <el-form-item :label="$t('provider.name')" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="$t('provider.type')" prop="type">
          <el-select v-model="form.type" style="width: 100%">
            <el-option label="OpenAI" value="openai" />
            <el-option label="Anthropic" value="anthropic" />
            <el-option label="Azure OpenAI" value="azure" />
            <el-option label="DeepSeek" value="deepseek" />
            <el-option label="Ollama" value="ollama" />
            <el-option label="OpenAI Compatible" value="openai-compatible" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('provider.baseUrl')" prop="base_url">
          <el-input v-model="form.base_url" placeholder="https://api.example.com" />
        </el-form-item>
        <el-form-item :label="$t('provider.apiKey')" prop="api_key">
          <el-input v-model="form.api_key" type="password" show-password />
          <div class="form-tip">{{ $t('provider.apiKeyTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('provider.priority')">
          <el-slider v-model="form.priority" :max="10" show-stops />
        </el-form-item>
        <el-form-item :label="$t('provider.weight')">
          <el-input-number v-model="form.weight" :min="1" :max="100" />
          <div class="form-tip">{{ $t('provider.weightTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('provider.rateLimit')">
          <el-input-number v-model="form.rate_limit" :min="0" :step="10" />
          <span class="unit">RPM</span>
          <div class="form-tip">{{ $t('provider.rateLimitTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('common.status')">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveProvider">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const store = useAppStore()
const providers = computed(() => store.providers)

const dialogVisible = ref(false)
const isEdit = ref(false)
const formRef = ref()
const form = ref({
  name: '',
  type: 'openai',
  base_url: '',
  api_key: '',
  priority: 0,
  weight: 100,
  rate_limit: 0,
  enabled: true,
})

const rules = {
  name: [{ required: true, message: t('message.inputRequired'), trigger: 'blur' }],
  type: [{ required: true, message: t('message.selectRequired'), trigger: 'change' }],
  base_url: [{ required: true, message: t('message.inputRequired'), trigger: 'blur' }],
}

function showAddDialog() {
  isEdit.value = false
  form.value = { name: '', type: 'openai', base_url: '', api_key: '', priority: 0, weight: 100, rate_limit: 0, enabled: true }
  dialogVisible.value = true
}

function editProvider(provider) {
  isEdit.value = true
  form.value = { ...provider }
  dialogVisible.value = true
}

async function saveProvider() {
  await formRef.value.validate()
  try {
    if (isEdit.value) {
      await store.updateProvider(form.value.id, form.value)
    } else {
      await store.createProvider(form.value)
    }
    ElMessage.success(t('message.saveSuccess'))
    dialogVisible.value = false
  } catch (e) {
    ElMessage.error(t('message.saveFailed'))
  }
}

function confirmDelete(provider) {
  ElMessageBox.confirm(t('message.confirmDelete'), 'Warning', { type: 'warning' })
    .then(() => store.deleteProvider(provider.id))
    .then(() => ElMessage.success(t('message.deleteSuccess')))
    .catch(() => {})
}

function testProvider(provider) {
  // TODO: implement test
  ElMessage.info(`Testing ${provider.name}...`)
}

onMounted(() => store.fetchProviders())
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.unit {
  margin-left: 8px;
  color: #606266;
}
</style>

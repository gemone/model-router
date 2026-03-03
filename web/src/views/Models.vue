<template>
  <div class="models">
    <div class="page-header">
      <h2>{{ $t('model.title') }}</h2>
      <el-button type="primary" @click="showAddDialog">
        <el-icon><Plus /></el-icon>
        {{ $t('model.addModel') }}
      </el-button>
    </div>

    <el-table :data="models" v-loading="store.loading" stripe>
      <el-table-column prop="name" :label="$t('model.name')" />
      <el-table-column prop="original_name" :label="$t('model.originalName')" show-overflow-tooltip />
      <el-table-column :label="$t('model.capabilities')" width="200">
        <template #default="{ row }">
          <el-tag v-if="row.supports_func" size="small" type="success" class="cap-tag">Function</el-tag>
          <el-tag v-if="row.supports_vision" size="small" type="warning" class="cap-tag">Vision</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$t('common.status')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
            {{ row.enabled ? $t('common.enabled') : $t('common.disabled') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="$t('common.actions')" width="250">
        <template #default="{ row }">
          <el-button link type="primary" @click="testModel(row)">
            {{ $t('model.testModel') }}
          </el-button>
          <el-button link type="primary" @click="editModel(row)">
            {{ $t('common.edit') }}
          </el-button>
          <el-button link type="danger" @click="confirmDelete(row)">
            {{ $t('common.delete') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? $t('model.editModel') : $t('model.addModel')" width="600px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="160px">
        <el-form-item :label="$t('model.name')" prop="name">
          <el-input v-model="form.name" placeholder="gpt-4" />
        </el-form-item>
        <el-form-item :label="$t('model.originalName')" prop="original_name">
          <el-input v-model="form.original_name" placeholder="gpt-4-turbo-preview" />
          <div class="form-tip">{{ $t('model.originalNameTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('model.provider')" prop="provider_id">
          <el-select v-model="form.provider_id" style="width: 100%">
            <el-option
              v-for="p in store.providerOptions"
              :key="p.value"
              :label="p.label"
              :value="p.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('model.capabilities')">
          <el-checkbox v-model="form.supports_func">{{ $t('model.supportsFunc') }}</el-checkbox>
          <el-checkbox v-model="form.supports_vision">{{ $t('model.supportsVision') }}</el-checkbox>
        </el-form-item>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="$t('model.contextWindow')">
              <el-input-number v-model="form.context_window" :min="1024" :step="1024" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="$t('model.maxTokens')">
              <el-input-number v-model="form.max_tokens" :min="1" :step="1024" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="$t('model.inputPrice')">
              <el-input-number v-model="form.input_price" :min="0" :precision="4" :step="0.001" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="$t('model.outputPrice')">
              <el-input-number v-model="form.output_price" :min="0" :precision="4" :step="0.001" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <div class="form-tip price-tip">{{ $t('model.priceTip') }}</div>
        <el-form-item :label="$t('model.rateLimit')">
          <el-input-number v-model="form.rate_limit" :min="0" :step="10" style="width: 200px" />
          <div class="form-tip">{{ $t('model.rateLimitTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('common.status')">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveModel">{{ $t('common.save') }}</el-button>
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
const models = computed(() => store.models)

const dialogVisible = ref(false)
const isEdit = ref(false)
const formRef = ref()
const form = ref({
  name: '',
  original_name: '',
  provider_id: '',
  supports_func: false,
  supports_vision: false,
  context_window: 4096,
  max_tokens: 4096,
  input_price: 0,
  output_price: 0,
  rate_limit: 0,
  enabled: true,
})

const rules = {
  name: [{ required: true, message: t('message.inputRequired'), trigger: 'blur' }],
  provider_id: [{ required: true, message: t('message.selectRequired'), trigger: 'change' }],
}

function showAddDialog() {
  isEdit.value = false
  form.value = { name: '', original_name: '', provider_id: '', supports_func: false, supports_vision: false, context_window: 4096, max_tokens: 4096, input_price: 0, output_price: 0, rate_limit: 0, enabled: true }
  dialogVisible.value = true
}

function editModel(model) {
  isEdit.value = true
  form.value = { ...model }
  dialogVisible.value = true
}

async function saveModel() {
  await formRef.value.validate()
  try {
    if (isEdit.value) {
      await store.updateModel(form.value.id, form.value)
    } else {
      await store.createModel(form.value)
    }
    ElMessage.success(t('message.saveSuccess'))
    dialogVisible.value = false
  } catch (e) {
    ElMessage.error(t('message.saveFailed'))
  }
}

async function testModel(model) {
  try {
    const result = await store.testModel(model.provider_id, model.name)
    if (result.success) {
      ElMessage.success(t('model.testSuccess', { latency: result.latency }))
    } else {
      ElMessage.error(t('model.testFailed', { error: result.error }))
    }
  } catch (e) {
    ElMessage.error(t('model.testFailed', { error: e.message }))
  }
}

function confirmDelete(model) {
  ElMessageBox.confirm(t('message.confirmDelete'), 'Warning', { type: 'warning' })
    .then(() => store.deleteModel(model.id))
    .then(() => ElMessage.success(t('message.deleteSuccess')))
    .catch(() => {})
}

onMounted(() => {
  store.fetchProviders()
  store.fetchModels()
})
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.cap-tag {
  margin-right: 4px;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.price-tip {
  margin-left: 160px;
  margin-bottom: 16px;
}
</style>

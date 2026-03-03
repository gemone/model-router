<template>
  <div class="routes">
    <!-- 面包屑导航 -->
    <div class="breadcrumb-wrapper">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item :to="{ path: '/' }">Home</el-breadcrumb-item>
        <el-breadcrumb-item>{{ $t("route.title") }}</el-breadcrumb-item>
      </el-breadcrumb>
    </div>

    <div class="page-header">
      <h2>{{ $t('route.title') }}</h2>
      <el-button type="primary" @click="showAddDialog">
        <el-icon><Plus /></el-icon>
        {{ $t('route.addRule') }}
      </el-button>
    </div>

    <el-empty v-if="!routeRules || !routeRules.length" :description="$t('common.noData')" />

    <el-card v-for="rule in routeRules" :key="rule.id" class="rule-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <span class="rule-name">{{ rule.name }}</span>
          <div>
            <el-tag :type="rule.fallback_enabled ? 'success' : 'info'" size="small" class="mr-2">
              Fallback {{ rule.fallback_enabled ? 'ON' : 'OFF' }}
            </el-tag>
            <el-button link type="primary" @click="editRule(rule)">{{ $t('common.edit') }}</el-button>
            <el-button link type="danger" @click="confirmDelete(rule)">{{ $t('common.delete') }}</el-button>
          </div>
        </div>
      </template>
      <div class="rule-detail">
        <div class="detail-item">
          <span class="label">{{ $t('route.modelPattern') }}:</span>
          <code>{{ rule.model_pattern }}</code>
        </div>
        <div class="detail-item">
          <span class="label">{{ $t('route.strategy') }}:</span>
          <el-tag size="small">{{ getStrategyLabel(rule.strategy) }}</el-tag>
        </div>
        <div class="detail-item">
          <span class="label">{{ $t('route.contentType') }}:</span>
          <el-tag :type="getContentTypeTagType(rule.content_type)" size="small">{{ getContentTypeLabel(rule.content_type) }}</el-tag>
        </div>
        <div class="detail-item">
          <span class="label">{{ $t('route.targetModels') }}:</span>
          <el-tag v-for="m in rule.target_models" :key="m" size="small" class="mr-2">{{ m }}</el-tag>
        </div>
        <div v-if="rule.fallback_models?.length" class="detail-item">
          <span class="label">{{ $t('route.fallbackModels') }}:</span>
          <el-tag v-for="m in rule.fallback_models" :key="m" size="small" type="warning" class="mr-2">{{ m }}</el-tag>
        </div>
      </div>
    </el-card>

    <!-- Add/Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? $t('route.editRule') : $t('route.addRule')"
      width="600px"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="140px">
        <el-form-item :label="$t('common.name')" prop="name">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="$t('route.modelPattern')" prop="model_pattern">
          <el-input v-model="form.model_pattern" placeholder="gpt-*" />
          <div class="form-tip">{{ $t('route.modelPatternTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('route.strategy')" prop="strategy">
          <el-select v-model="form.strategy" style="width: 100%">
            <el-option :label="$t('route.strategyPriority')" value="priority" />
            <el-option :label="$t('route.strategyWeighted')" value="weighted" />
            <el-option :label="$t('route.strategyAuto')" value="auto" />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('route.contentType')" prop="content_type">
          <el-select v-model="form.content_type" style="width: 100%">
            <el-option :label="$t('route.contentTypeAll')" value="all" />
            <el-option :label="$t('route.contentTypeText')" value="text" />
            <el-option :label="$t('route.contentTypeImage')" value="image" />
          </el-select>
          <div class="form-tip">{{ $t('route.contentTypeTip') }}</div>
        </el-form-item>
        <el-form-item :label="$t('route.targetModels')" prop="target_models">
          <el-select
            v-model="form.target_models"
            multiple
            filterable
            allow-create
            style="width: 100%"
            :placeholder="$t('route.addTargetModel')"
          >
            <el-option
              v-for="m in store.models"
              :key="m.id"
              :label="m.name"
              :value="m.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="$t('route.fallbackEnabled')">
          <el-switch v-model="form.fallback_enabled" />
        </el-form-item>
        <el-form-item v-if="form.fallback_enabled" :label="$t('route.fallbackModels')">
          <el-select
            v-model="form.fallback_models"
            multiple
            filterable
            allow-create
            style="width: 100%"
            :placeholder="$t('route.addFallbackModel')"
          >
            <el-option
              v-for="m in store.models"
              :key="m.id"
              :label="m.name"
              :value="m.name"
            />
          </el-select>
          <div class="form-tip">{{ $t('route.fallbackTip') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveRule">{{ $t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, computed, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const store = useAppStore()
const routeRules = computed(() => store.routeRules)

const dialogVisible = ref(false)
const isEdit = ref(false)
const formRef = ref()
const form = ref({
  name: '',
  model_pattern: '',
  strategy: 'priority',
  content_type: 'all',
  target_models: [],
  fallback_models: [],
  fallback_enabled: true,
})

const rules = {
  name: [{ required: true, message: t('message.inputRequired'), trigger: 'blur' }],
  model_pattern: [{ required: true, message: t('message.inputRequired'), trigger: 'blur' }],
  strategy: [{ required: true, message: t('message.selectRequired'), trigger: 'change' }],
  target_models: [
    {
      type: 'array',
      required: true,
      message: 'Please select at least one target model',
      trigger: 'change',
    },
  ],
}

function getStrategyLabel(strategy) {
  const labels = {
    priority: t('route.strategyPriority'),
    weighted: t('route.strategyWeighted'),
    auto: t('route.strategyAuto'),
  }
  return labels[strategy] || strategy
}

function showAddDialog() {
  isEdit.value = false
  form.value = {
    name: '',
    model_pattern: '',
    strategy: 'priority',
    content_type: 'all',
    target_models: [],
    fallback_models: [],
    fallback_enabled: true,
  }
  dialogVisible.value = true
}

function getContentTypeLabel(contentType) {
  const labels = {
    all: t('route.contentTypeAll'),
    text: t('route.contentTypeText'),
    image: t('route.contentTypeImage'),
  }
  return labels[contentType] || contentType
}

function getContentTypeTagType(contentType) {
  const types = {
    all: '',
    text: 'info',
    image: 'warning',
  }
  return types[contentType] || ''
}

function editRule(rule) {
  isEdit.value = true
  form.value = {
    name: rule.name,
    model_pattern: rule.model_pattern,
    strategy: rule.strategy,
    content_type: rule.content_type || 'all',
    target_models: [...(rule.target_models || [])],
    fallback_models: [...(rule.fallback_models || [])],
    fallback_enabled: rule.fallback_enabled || false,
  }
  dialogVisible.value = true
}

async function saveRule() {
  await formRef.value.validate()
  try {
    if (isEdit.value) {
      // For edit, we need the rule ID
      const rule = routeRules.value.find(r => r.name === form.value.name)
      if (rule) {
        await store.updateRouteRule(rule.id, form.value)
      }
    } else {
      await store.createRouteRule(form.value)
    }
    ElMessage.success(t('message.saveSuccess'))
    dialogVisible.value = false
  } catch (e) {
    ElMessage.error(t('message.saveFailed'))
  }
}

function confirmDelete(rule) {
  ElMessageBox.confirm(t('message.confirmDelete'), 'Warning', { type: 'warning' })
    .then(() => store.deleteRouteRule(rule.id))
    .then(() => ElMessage.success(t('message.deleteSuccess')))
    .catch(() => {})
}

onMounted(() => {
  store.fetchRouteRules()
  store.fetchModels()
})
</script>

<style scoped>
.breadcrumb-wrapper {
  margin-bottom: 16px;
}

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

.rule-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.rule-name {
  font-weight: 500;
}

.rule-detail {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.detail-item {
  display: flex;
  align-items: center;
}

.detail-item .label {
  width: 100px;
  color: #606266;
}

.mr-2 {
  margin-right: 8px;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>

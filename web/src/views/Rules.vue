<template>
  <div class="rules-page">
    <div class="page-header">
      <el-button type="primary" @click="showAddDialog">
        <el-icon><Plus /></el-icon>
        {{ $t('rule.addRule') || '添加规则' }}
      </el-button>
    </div>

    <!-- 规则列表 -->
    <el-empty v-if="!rules || rules.length === 0" :description="$t('common.noData') || '暂无数据'" />

    <el-card v-for="rule in sortedRules" :key="rule.id" class="rule-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <div class="rule-info">
            <span class="rule-name">{{ rule.name }}</span>
            <el-tag :type="rule.enabled ? 'success' : 'info'" size="small" class="ml-2">
              {{ rule.enabled ? ($t('common.enabled') || '已启用') : ($t('common.disabled') || '已禁用') }}
            </el-tag>
            <el-tag type="warning" size="small" class="ml-2">
              {{ $t('rule.priority') || '优先级' }}: {{ rule.priority }}
            </el-tag>
          </div>
          <div class="rule-actions">
            <el-button link type="primary" size="small" @click="editRule(rule)">
              <el-icon><Edit /></el-icon>
              {{ $t('common.edit') }}
            </el-button>
            <el-button 
              link 
              :type="rule.enabled ? 'warning' : 'success'" 
              size="small" 
              @click="toggleRuleStatus(rule)"
            >
              {{ rule.enabled ? ($t('common.disable') || '禁用') : ($t('common.enable') || '启用') }}
            </el-button>
            <el-button link type="danger" size="small" @click="confirmDelete(rule)">
              <el-icon><Delete /></el-icon>
              {{ $t('common.delete') }}
            </el-button>
          </div>
        </div>
      </template>

      <div class="rule-content">
        <!-- 描述 -->
        <div v-if="rule.description" class="rule-description">
          {{ rule.description }}
        </div>

        <!-- 条件列表 -->
        <div class="section">
          <div class="section-title">
            <el-icon><Filter /></el-icon>
            {{ $t('rule.conditions') || '匹配条件' }}
            <el-tag size="small" type="info" class="ml-2">{{ rule.conditions?.length || 0 }}</el-tag>
          </div>
          <div v-if="rule.conditions && rule.conditions.length > 0" class="conditions-list">
            <div v-for="(condition, idx) in rule.conditions" :key="idx" class="condition-item">
              <el-tag size="small" :type="getConditionTypeTag(condition.type)">
                {{ getConditionTypeLabel(condition.type) }}
              </el-tag>
              <span class="condition-field">{{ condition.field }}</span>
              <el-tag size="small" effect="plain">{{ getOperatorLabel(condition.op) }}</el-tag>
              <code class="condition-value">{{ condition.value }}</code>
            </div>
          </div>
          <div v-else class="empty-text">{{ $t('rule.noConditions') || '无条件（总是匹配）' }}</div>
        </div>

        <!-- 动作 -->
        <div class="section">
          <div class="section-title">
            <el-icon><Pointer /></el-icon>
            {{ $t('rule.action') || '执行动作' }}
          </div>
          <div class="action-content">
            <el-tag size="small" :type="getActionTypeTag(rule.action?.type)">
              {{ getActionTypeLabel(rule.action?.type) }}
            </el-tag>
            <span v-if="rule.action?.target" class="action-target">
              → {{ rule.action.target }}
            </span>
            <div v-if="rule.action?.headers && Object.keys(rule.action.headers).length > 0" class="action-headers">
              <div v-for="(value, key) in rule.action.headers" :key="key" class="header-item">
                <code>{{ key }}: {{ value }}</code>
              </div>
            </div>
          </div>
        </div>

        <!-- Profile 关联 -->
        <div v-if="rule.profile_id" class="section">
          <div class="section-title">
            <el-icon><Connection /></el-icon>
            {{ $t('rule.profile') || '所属 Profile' }}
          </div>
          <div class="profile-tag">
            <el-tag size="small" type="info">
              <el-icon><Grid /></el-icon>
              {{ getProfileName(rule.profile_id) }}
            </el-tag>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 添加/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? ($t('rule.editRule') || '编辑规则') : ($t('rule.addRule') || '添加规则')"
      width="700px"
      class="rule-dialog"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="120px">
        <!-- 基本信息 -->
        <el-divider>{{ $t('rule.basicInfo') || '基本信息' }}</el-divider>
        
        <el-form-item :label="$t('rule.name') || '规则名称'" prop="name">
          <el-input v-model="form.name" :placeholder="$t('rule.namePlaceholder') || '输入规则名称'" />
        </el-form-item>

        <el-form-item :label="$t('rule.description') || '描述'">
          <el-input 
            v-model="form.description" 
            type="textarea" 
            :rows="2"
            :placeholder="$t('rule.descriptionPlaceholder') || '可选：描述此规则的作用'"
          />
        </el-form-item>

        <el-form-item :label="$t('rule.profile') || '所属 Profile'" prop="profile_id">
          <el-select v-model="form.profile_id" style="width: 100%" :disabled="isEdit">
            <el-option 
              v-for="profile in store.profiles" 
              :key="profile.id" 
              :label="profile.name" 
              :value="profile.id"
            />
          </el-select>
          <div class="form-tip">{{ $t('rule.profileTip') || '选择要应用此规则的 Profile' }}</div>
        </el-form-item>

        <el-form-item :label="$t('rule.priority') || '优先级'">
          <el-slider v-model="form.priority" :max="100" show-stops :step="10" />
          <div class="form-tip">{{ $t('rule.priorityTip') || '数值越高优先级越高，高优先级规则优先匹配' }}</div>
        </el-form-item>

        <el-form-item :label="$t('common.status') || '状态'">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <!-- 条件配置 -->
        <el-divider>{{ $t('rule.conditionsConfig') || '条件配置' }}</el-divider>

        <div v-for="(condition, index) in form.conditions" :key="index" class="condition-row">
          <el-row :gutter="10">
            <el-col :span="6">
              <el-select v-model="condition.type" :placeholder="$t('rule.conditionType') || '条件类型'">
                <el-option :label="$t('rule.conditionHeader') || 'Header'" value="header" />
                <el-option :label="$t('rule.conditionBodyParam') || 'Body 参数'" value="body_param" />
                <el-option :label="$t('rule.conditionQuery') || 'Query 参数'" value="query" />
                <el-option :label="$t('rule.conditionContent') || 'Content'" value="content" />
                <el-option :label="$t('rule.conditionTime') || 'Time'" value="time" />
                <el-option :label="$t('rule.conditionModel') || 'Model'" value="model" />
              </el-select>
            </el-col>
            <el-col :span="5">
              <el-select v-model="condition.field" :placeholder="$t('rule.field') || '字段'">
                <el-option 
                  v-for="field in getFieldOptions(condition.type)" 
                  :key="field.value" 
                  :label="field.label" 
                  :value="field.value"
                />
              </el-select>
            </el-col>
            <el-col :span="4">
              <el-select v-model="condition.op" :placeholder="$t('rule.operator') || '操作符'">
                <el-option label="=" value="eq" />
                <el-option label="!=" value="neq" />
                <el-option :label="$t('rule.opContains') || '包含'" value="contains" />
                <el-option :label="$t('rule.opRegex') || '正则'" value="regex" />
                <el-option label=">" value="gt" />
                <el-option label="<" value="lt" />
                <el-option label=">=" value="gte" />
                <el-option label="<=" value="lte" />
                <el-option :label="$t('rule.opBetween') || '范围'" value="between" />
                <el-option :label="$t('rule.opIn') || '在列表中'" value="in" />
                <el-option :label="$t('rule.opNotIn') || '不在列表中'" value="not_in" />
              </el-select>
            </el-col>
            <el-col :span="7">
              <el-input v-model="condition.value" :placeholder="$t('rule.value') || '值'" />
            </el-col>
            <el-col :span="2">
              <el-button type="danger" circle size="small" @click="removeCondition(index)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-col>
          </el-row>
        </div>

        <el-button type="primary" plain @click="addCondition" class="add-condition-btn">
          <el-icon><Plus /></el-icon>
          {{ $t('rule.addCondition') || '添加条件' }}
        </el-button>

        <!-- 动作配置 -->
        <el-divider>{{ $t('rule.actionConfig') || '动作配置' }}</el-divider>

        <el-form-item :label="$t('rule.actionType') || '动作类型'" prop="action.type">
          <el-select v-model="form.action.type" style="width: 100%">
            <el-option :label="$t('rule.actionRoute') || '路由到路由'" value="route" />
            <el-option :label="$t('rule.actionModel') || '使用模型'" value="model" />
            <el-option :label="$t('rule.actionAddHeader') || '添加请求头'" value="add_header" />
            <el-option :label="$t('rule.actionSetHeader') || '设置请求头'" value="set_header" />
            <el-option :label="$t('rule.actionModifyBody') || '修改请求体'" value="modify_body" />
            <el-option :label="$t('rule.actionAddParam') || '添加查询参数'" value="add_param" />
          </el-select>
        </el-form-item>

        <el-form-item 
          v-if="['route', 'model'].includes(form.action.type)" 
          :label="$t('rule.target') || '目标'"
        >
          <el-select v-if="form.action.type === 'route'" v-model="form.action.target" style="width: 100%">
            <el-option 
              v-for="route in store.routeRules" 
              :key="route.id" 
              :label="route.name" 
              :value="route.id"
            />
          </el-select>
          <el-select v-else-if="form.action.type === 'model'" v-model="form.action.target" style="width: 100%">
            <el-option 
              v-for="model in store.models" 
              :key="model.id" 
              :label="model.name" 
              :value="model.name"
            />
          </el-select>
        </el-form-item>

        <!-- 请求头配置 -->
        <template v-if="['add_header', 'set_header'].includes(form.action.type)">
          <el-form-item :label="$t('rule.headers') || '请求头'">
            <div v-for="(header, idx) in actionHeaders" :key="idx" class="header-row">
              <el-input v-model="header.key" :placeholder="'Header Key'" style="width: 200px" />
              <el-input v-model="header.value" :placeholder="'Header Value'" style="width: 200px; margin-left: 10px" />
              <el-button type="danger" circle size="small" @click="removeHeader(idx)" style="margin-left: 10px">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
            <el-button type="primary" plain size="small" @click="addHeader" class="mt-2">
              <el-icon><Plus /></el-icon>
              {{ $t('rule.addHeader') || '添加请求头' }}
            </el-button>
          </el-form-item>
        </template>

        <!-- 修改请求体 -->
        <template v-if="form.action.type === 'modify_body'">
          <el-form-item :label="$t('rule.targetField') || '目标字段'">
            <el-input v-model="form.action.target" placeholder="例如: temperature" />
          </el-form-item>
          <el-form-item :label="$t('rule.fieldValue') || '字段值'">
            <el-input v-model="form.action.value" placeholder="例如: 0.5" />
          </el-form-item>
        </template>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveRule">{{ $t('common.save') }}</el-button>
      </template>
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

// 规则列表
const rules = ref([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const formRef = ref()

// 表单数据
const form = ref({
  id: '',
  name: '',
  description: '',
  profile_id: '',
  priority: 50,
  enabled: true,
  conditions: [],
  action: {
    type: 'route',
    target: '',
    headers: {}
  }
})

// 动作请求头（用于编辑）
const actionHeaders = ref([])

// 排序后的规则（按优先级降序）
const sortedRules = computed(() => {
  return [...rules.value].sort((a, b) => b.priority - a.priority)
})

// 表单验证规则
const rulesValidation = {
  name: [{ required: true, message: t('message.inputRequired') || '请输入名称', trigger: 'blur' }],
  profile_id: [{ required: true, message: t('rule.selectProfile') || '请选择 Profile', trigger: 'change' }],
  'action.type': [{ required: true, message: t('rule.selectAction') || '请选择动作类型', trigger: 'change' }]
}

// 获取字段选项
function getFieldOptions(type) {
  const options = {
    header: [
      { label: 'X-User-Tier', value: 'X-User-Tier' },
      { label: 'User-Agent', value: 'User-Agent' },
      { label: 'X-API-Key', value: 'X-API-Key' },
      { label: 'Content-Type', value: 'Content-Type' },
      { label: t('rule.custom') || '自定义', value: 'custom' }
    ],
    body_param: [
      { label: t('rule.modelName') || '模型名称', value: 'model' },
      { label: t('rule.temperature') || '温度', value: 'temperature' },
      { label: t('rule.maxTokens') || '最大令牌数', value: 'max_tokens' },
      { label: t('rule.topP') || 'Top P', value: 'top_p' },
      { label: t('rule.custom') || '自定义', value: 'custom' }
    ],
    query: [
      { label: t('rule.custom') || '自定义', value: 'custom' }
    ],
    content: [
      { label: t('rule.hasImage') || '包含图片', value: 'has_image' },
      { label: t('rule.imageCount') || '图片数量', value: 'image_count' },
      { label: t('rule.textLength') || '文本长度', value: 'text_length' },
      { label: t('rule.messageCount') || '消息数量', value: 'message_count' },
      { label: t('rule.hasFunction') || '包含函数调用', value: 'has_function' },
      { label: t('rule.language') || '语言', value: 'language' }
    ],
    time: [
      { label: t('rule.hour') || '小时', value: 'hour' },
      { label: t('rule.weekday') || '星期', value: 'weekday' },
      { label: t('rule.month') || '月份', value: 'month' }
    ],
    model: [
      { label: t('rule.modelName') || '模型名称', value: 'name' }
    ]
  }
  return options[type] || []
}

// 获取条件类型标签
function getConditionTypeTag(type) {
  const tags = {
    header: 'primary',
    body_param: 'danger',
    query: 'warning',
    content: 'success',
    time: 'warning',
    model: 'info'
  }
  return tags[type] || ''
}

// 获取条件类型标签文本
function getConditionTypeLabel(type) {
  const labels = {
    header: 'Header',
    body_param: 'Body',
    query: 'Query',
    content: 'Content',
    time: 'Time',
    model: 'Model'
  }
  return labels[type] || type
}

// 获取操作符标签
function getOperatorLabel(op) {
  const labels = {
    eq: '=',
    neq: '!=',
    contains: '包含',
    regex: '正则',
    gt: '>',
    lt: '<',
    gte: '>=',
    lte: '<=',
    between: '范围',
    in: '在列表中',
    not_in: '不在列表中'
  }
  return labels[op] || op
}

// 获取动作类型标签
function getActionTypeTag(type) {
  const tags = {
    route: 'primary',
    model: 'success',
    add_header: 'warning',
    set_header: 'warning',
    modify_body: 'info',
    add_param: 'danger'
  }
  return tags[type] || ''
}

// 获取动作类型标签文本
function getActionTypeLabel(type) {
  const labels = {
    route: t('rule.actionRoute') || '路由到路由',
    model: t('rule.actionModel') || '使用模型',
    add_header: t('rule.actionAddHeader') || '添加请求头',
    set_header: t('rule.actionSetHeader') || '设置请求头',
    modify_body: t('rule.actionModifyBody') || '修改请求体',
    add_param: t('rule.actionAddParam') || '添加查询参数'
  }
  return labels[type] || type
}

// 获取 Profile 名称
function getProfileName(profileId) {
  const profile = store.profiles.find(p => p.id === profileId)
  return profile ? profile.name : profileId
}

// 添加条件
function addCondition() {
  form.value.conditions.push({
    type: 'header',
    field: '',
    op: 'eq',
    value: ''
  })
}

// 删除条件
function removeCondition(index) {
  form.value.conditions.splice(index, 1)
}

// 添加请求头
function addHeader() {
  actionHeaders.value.push({ key: '', value: '' })
}

// 删除请求头
function removeHeader(index) {
  actionHeaders.value.splice(index, 1)
}

// 显示添加对话框
function showAddDialog() {
  isEdit.value = false
  form.value = {
    name: '',
    description: '',
    profile_id: store.profiles[0]?.id || '',
    priority: 50,
    enabled: true,
    conditions: [],
    action: {
      type: 'route',
      target: '',
      headers: {}
    }
  }
  actionHeaders.value = []
  dialogVisible.value = true
}

// 编辑规则
function editRule(rule) {
  isEdit.value = true
  form.value = {
    id: rule.id,
    name: rule.name,
    description: rule.description || '',
    profile_id: rule.profile_id,
    priority: rule.priority || 50,
    enabled: rule.enabled !== false,
    conditions: rule.conditions ? JSON.parse(JSON.stringify(rule.conditions)) : [],
    action: rule.action ? JSON.parse(JSON.stringify(rule.action)) : { type: 'route', target: '' }
  }
  
  // 转换 headers 为数组形式
  if (form.value.action.headers) {
    actionHeaders.value = Object.entries(form.value.action.headers).map(([key, value]) => ({
      key,
      value
    }))
  } else {
    actionHeaders.value = []
  }
  
  dialogVisible.value = true
}

// 保存规则
async function saveRule() {
  await formRef.value.validate()
  
  try {
    // 转换 actionHeaders 为对象
    if (['add_header', 'set_header'].includes(form.value.action.type)) {
      form.value.action.headers = {}
      actionHeaders.value.forEach(h => {
        if (h.key && h.value) {
          form.value.action.headers[h.key] = h.value
        }
      })
    }
    
    if (isEdit.value) {
      await store.updateRule(form.value.id, form.value)
    } else {
      await store.createRule(form.value)
    }
    
    ElMessage.success(isEdit.value ? t('message.updateSuccess') : t('message.createSuccess'))
    dialogVisible.value = false
    await fetchRules()
  } catch (e) {
    console.error('Failed to save rule:', e)
    ElMessage.error(t('message.saveFailed'))
  }
}

// 切换规则状态
async function toggleRuleStatus(rule) {
  try {
    if (rule.enabled) {
      await store.disableRule(rule.id)
      ElMessage.success(t('rule.disabledSuccess') || '规则已禁用')
    } else {
      await store.enableRule(rule.id)
      ElMessage.success(t('rule.enabledSuccess') || '规则已启用')
    }
    await fetchRules()
  } catch (e) {
    ElMessage.error(t('message.operationFailed'))
  }
}

// 确认删除
function confirmDelete(rule) {
  ElMessageBox.confirm(
    t('message.confirmDelete') || '确定要删除此规则吗？',
    t('common.warning') || '警告',
    { type: 'warning' }
  )
    .then(() => deleteRule(rule.id))
    .catch(() => {})
}

// 删除规则
async function deleteRule(id) {
  try {
    await store.deleteRule(id)
    ElMessage.success(t('message.deleteSuccess'))
    await fetchRules()
  } catch (e) {
    ElMessage.error(t('message.deleteFailed'))
  }
}

// 获取规则列表
async function fetchRules() {
  try {
    const data = await store.fetchRules()
    rules.value = data || []
  } catch (e) {
    console.error('Failed to fetch rules:', e)
  }
}

onMounted(async () => {
  await Promise.all([
    store.fetchProfiles(),
    store.fetchModels(),
    store.fetchRouteRules()
  ])
  await fetchRules()
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

.rule-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.rule-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.rule-name {
  font-weight: 600;
  font-size: 16px;
}

.rule-actions {
  display: flex;
  gap: 8px;
}

.rule-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.rule-description {
  color: #6b7280;
  font-size: 14px;
  padding-bottom: 12px;
  border-bottom: 1px dashed #e5e7eb;
}

.section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  color: #374151;
  font-size: 14px;
}

.conditions-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.condition-item {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.condition-field {
  font-weight: 500;
  color: #4b5563;
}

.condition-value {
  background: #f3f4f6;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 13px;
}

.action-content {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.action-target {
  font-weight: 500;
  color: #4f46e5;
}

.action-headers {
  margin-top: 8px;
  padding: 8px;
  background: #f9fafb;
  border-radius: 4px;
  width: 100%;
}

.header-item {
  font-size: 13px;
  margin-bottom: 4px;
}

.empty-text {
  color: #9ca3af;
  font-size: 13px;
  font-style: italic;
}

.form-tip {
  font-size: 12px;
  color: #6b7280;
  margin-top: 4px;
}

.condition-row {
  margin-bottom: 12px;
  padding: 12px;
  background: #f9fafb;
  border-radius: 6px;
}

.header-row {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}

.add-condition-btn {
  margin-top: 8px;
}

.ml-2 {
  margin-left: 8px;
}

.mt-2 {
  margin-top: 8px;
}

.profile-tag {
  display: flex;
  align-items: center;
}
</style>

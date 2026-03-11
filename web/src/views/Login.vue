<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <div class="card-header">
          <el-icon class="logo-icon"><Connection /></el-icon>
          <h2>Model Router</h2>
        </div>
      </template>

      <el-form
        ref="loginFormRef"
        :model="loginForm"
        :rules="rules"
        label-width="0"
        @submit.prevent="handleLogin"
      >
        <el-form-item prop="password">
          <el-input
            v-model="loginForm.password"
            type="password"
            placeholder="请输入管理员密码"
            :prefix-icon="Lock"
            show-password
            :disabled="loading"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            :loading="loading"
            @click="handleLogin"
            style="width: 100%"
          >
            {{ loading ? '登录中...' : '登录' }}
          </el-button>
        </el-form-item>
      </el-form>

      <el-alert
        v-if="errorMessage"
        :title="errorMessage"
        type="error"
        :closable="false"
        style="margin-top: 16px"
      />

      <div class="login-footer">
        <p v-if="!authEnabled" class="warning-text">
          <el-icon><WarningFilled /></el-icon>
          管理员认证未配置，请设置 ADMIN_TOKEN 环境变量
        </p>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { Connection, Lock, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const router = useRouter()
const store = useAppStore()

const loginFormRef = ref()
const loginForm = ref({
  password: ''
})
const loading = ref(false)
const errorMessage = ref('')
const authEnabled = ref(true)

const rules = {
  password: [
    { required: true, message: '请输入管理员密码', trigger: 'blur' }
  ]
}

async function handleLogin() {
  try {
    await loginFormRef.value.validate()
  } catch {
    return
  }

  loading.value = true
  errorMessage.value = ''

  try {
    const success = await store.login(loginForm.value.password)
    if (success) {
      ElMessage.success('登录成功')
      await router.push('/dashboard')
    } else {
      errorMessage.value = '密码错误，请重试'
    }
  } catch (error) {
    if (error.response?.status === 401) {
      errorMessage.value = '密码错误，请重试'
    } else if (error.response?.data?.message) {
      errorMessage.value = error.response.data.message
    } else {
      errorMessage.value = '登录失败，请稍后重试'
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  // Check if auth is enabled
  authEnabled.value = await store.checkAuth()

  // If already authenticated, redirect to dashboard
  if (store.isAuthenticated) {
    router.push('/dashboard')
  }
})
</script>

<style scoped>
.login-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-card {
  width: 400px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.logo-icon {
  font-size: 32px;
  color: #409EFF;
}

.card-header h2 {
  margin: 0;
  color: #303133;
}

.login-footer {
  margin-top: 20px;
  text-align: center;
}

.warning-text {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #E6A23C;
  font-size: 14px;
  margin: 0;
}

.warning-text .el-icon {
  font-size: 18px;
}
</style>

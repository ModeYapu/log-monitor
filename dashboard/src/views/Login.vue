<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <h1>LogMonitor</h1>
        <p class="subtitle">日志监控与分析平台</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        @submit.prevent="handleLogin"
        class="login-form"
      >
        <el-form-item prop="username" label="用户名">
          <el-input
            v-model="form.username"
            placeholder="用户名"
            size="large"
            :prefix-icon="User"
            aria-label="用户名"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item prop="password" label="密码">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            size="large"
            :prefix-icon="Lock"
            show-password
            aria-label="密码"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            @click="handleLogin"
            class="login-button"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>


    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { authApi } from '../api'
import type { FormInstance, FormRules } from 'element-plus'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  username: '',
  password: ''
})

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    loading.value = true
    try {
      const { data } = await authApi.login(form)
      localStorage.setItem('logmon_token', data.token)
      localStorage.setItem('logmon_user', JSON.stringify(data.user))
      ElMessage.success('登录成功')
      router.push('/')
    } catch (error: any) {
      ElMessage.error(error.response?.data?.error || '登录失败')
    } finally {
      loading.value = false
    }
  })
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1e1b4b 0%, #0f172a 100%);
  padding: 20px;
}

.login-card {
  width: 100%;
  max-width: 400px;
  background: var(--color-bg-secondary);
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  padding: 40px;
}

.login-header {
  text-align: center;
  margin-bottom: 40px;
}

.login-header h1 {
  font-size: 32px;
  font-weight: 700;
  background: linear-gradient(135deg, #6366f1, #a855f7);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin: 0 0 12px 0;
}

.subtitle {
  color: var(--color-text-secondary);
  font-size: 14px;
  margin: 0;
}

.login-form {
  /* no extra margin */
}

.login-form :deep(.el-form-item) {
  margin-bottom: 24px;
}

.login-button {
  width: 100%;
}

.login-footer {
  text-align: center;
  padding-top: 24px;
  border-top: 1px solid var(--color-border);
}

.login-footer p {
  color: var(--color-text-secondary);
  font-size: 13px;
  margin: 0;
}

:deep(.el-input__wrapper) {
  background: var(--color-bg);
  box-shadow: none !important;
  border: 1px solid var(--color-border);
}

:deep(.el-input__wrapper:hover),
:deep(.el-input__wrapper.is-focus) {
  border-color: #6366f1 !important;
}

:deep(.el-input__inner) {
  color: var(--color-text);
}
</style>

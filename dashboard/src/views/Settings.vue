<template>
  <div class="settings-page">
    <h1 class="sr-only">系统设置</h1>
    <el-row :gutter="20">
      <el-col :span="16">
        <el-card>
          <template #header>
            <span>SDK 接入引导</span>
          </template>

          <el-form label-width="120px">
            <el-form-item label="1. 选择应用">
              <el-select v-model="selectedAppId" placeholder="选择或输入应用 ID" filterable allow-create>
                <el-option
                  v-for="app in apps"
                  :key="app.app_id"
                  :label="app.app_id"
                  :value="app.app_id"
                />
              </el-select>
            </el-form-item>

            <el-form-item label="2. 引入 SDK">
              <div class="code-block">
                <pre>&lt;script src="{{ sdkUrl }}"&gt;&lt;/script&gt;</pre>
                <el-button size="small" @click="copyToClipboard(sdkUrl)" class="copy-btn">
                  复制
                </el-button>
              </div>
            </el-form-item>

            <el-form-item label="3. 初始化 SDK">
              <div class="code-block">
                <pre>LogMonitor.init({
  dsn: '{{ collectorUrl }}/api/report',
  appId: '{{ selectedAppId || "your-app-id" }}',
  release: '1.0.0',
  autoCapture: true
})</pre>
                <el-button size="small" @click="copyInitCode" class="copy-btn">
                  复制
                </el-button>
              </div>
            </el-form-item>

            <el-form-item label="4. (可选) 手动上报">
              <div class="code-block">
                <pre>// 上报信息
LogMonitor.info('User logged in', { userId: '123' })

// 上报警告
LogMonitor.warn('API slow', { duration: 2000 })

// 上报错误
try {
  // some code
} catch (err) {
  LogMonitor.captureException(err)
}

// 自定义事件
LogMonitor.track('button_click', { button: 'submit' })</pre>
                <el-button size="small" @click="copyExampleCode" class="copy-btn">
                  复制
                </el-button>
              </div>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="mt-4">
          <template #header>
            <span>修改密码</span>
          </template>

          <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="120px">
            <el-form-item label="原密码" prop="old_password">
              <el-input
                v-model="passwordForm.old_password"
                type="password"
                show-password
                placeholder="请输入原密码"
              />
            </el-form-item>
            <el-form-item label="新密码" prop="new_password">
              <el-input
                v-model="passwordForm.new_password"
                type="password"
                show-password
                placeholder="请输入新密码（至少6位）"
              />
            </el-form-item>
            <el-form-item label="确认密码" prop="confirm_password">
              <el-input
                v-model="passwordForm.confirm_password"
                type="password"
                show-password
                placeholder="请再次输入新密码"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="changingPassword" @click="handleChangePassword">
                修改密码
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="mt-4">
          <template #header>
            <span>数据保留策略</span>
          </template>

          <el-form label-width="180px">
            <el-form-item label="数据保留天数">
              <el-input-number v-model="retentionDays" :min="7" :max="365" />
              <span class="ml-2 text-secondary">天</span>
              <el-button type="primary" size="small" @click="saveRetention" class="ml-4">
                保存
              </el-button>
            </el-form-item>
            <el-form-item>
              <el-alert
                title="注意：修改保留策略后将立即清理过期数据，该操作不可撤销"
                type="warning"
                :closable="false"
                show-icon
              />
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <el-col :span="8">
        <el-card>
          <template #header>
            <span>系统信息</span>
          </template>

          <div v-loading="loadingSystemInfo" class="system-info">
            <div class="info-item">
              <span class="info-label">Collector 状态</span>
              <el-tag :type="systemInfo.status === 'ok' ? 'success' : 'danger'">
                {{ systemInfo.status === 'ok' ? '运行中' : '离线' }}
              </el-tag>
            </div>
            <div class="info-item">
              <span class="info-label">版本号</span>
              <span class="info-value">v{{ systemInfo.version || '-' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">数据库大小</span>
              <span class="info-value">{{ formatBytes(systemInfo.dbSize) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">总事件数</span>
              <span class="info-value">{{ formatNumber(systemInfo.totalEvents) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">总录制数</span>
              <span class="info-value">{{ formatNumber(systemInfo.totalRecordings) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">应用数量</span>
              <span class="info-value">{{ apps.length }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">数据保留天数</span>
              <span class="info-value">{{ systemInfo.retentionDays }} 天</span>
            </div>
            <div class="info-item">
              <span class="info-label">上次清理时间</span>
              <span class="info-value">{{ systemInfo.lastCleanupTime ? formatTime(systemInfo.lastCleanupTime) : '从未清理' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">系统运行时间</span>
              <span class="info-value">{{ formatUptime(systemInfo.uptime) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">服务器时间</span>
              <span class="info-value">{{ formatTime(systemInfo.serverTime) }}</span>
            </div>
          </div>
        </el-card>

        <el-card class="mt-4">
          <template #header>
            <span>快速操作</span>
          </template>

          <div class="quick-actions">
            <el-button @click="testHealth" :loading="testingHealth">
              <el-icon><CircleCheck /></el-icon>
              健康检查
            </el-button>
            <el-button @click="refreshSystemInfo" :loading="loadingSystemInfo">
              <el-icon><Refresh /></el-icon>
              刷新信息
            </el-button>
            <el-button type="warning" @click="triggerCleanup" :loading="cleaningUp">
              <el-icon><Delete /></el-icon>
              立即清理
            </el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { CircleCheck, Refresh, Delete } from '@element-plus/icons-vue'
import { logApi, authApi, systemApi } from '../api'
import { formatNumber, formatTime } from '../utils/formatters'

const apps = ref<any[]>([])
const selectedAppId = ref('')
const retentionDays = ref(30)
const testingHealth = ref(false)
const changingPassword = ref(false)
const loadingSystemInfo = ref(false)
const cleaningUp = ref(false)

const passwordFormRef = ref<FormInstance>()
const passwordForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

const validateConfirmPassword = (rule: any, value: any, callback: any) => {
  if (value !== passwordForm.new_password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const passwordRules: FormRules = {
  old_password: [
    { required: true, message: '请输入原密码', trigger: 'blur' }
  ],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 个字符', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const sdkUrl = window.location.origin + '/sdk/logmonitor.min.js'
const collectorUrl = window.location.protocol + '//' + window.location.hostname + ':9200'

const systemInfo = ref({
  status: 'unknown',
  version: '',
  dbSize: 0,
  totalEvents: 0,
  totalRecordings: 0,
  retentionDays: 30,
  uptime: 0,
  serverTime: Date.now(),
  lastCleanupTime: 0
})

const fetchApps = async () => {
  try {
    const { data } = await logApi.getApps()
    apps.value = data
    if (apps.value.length > 0) {
      selectedAppId.value = apps.value[0].app_id
    }
  } catch (error) {
    console.error('Failed to fetch apps:', error)
  }
}

const refreshSystemInfo = async () => {
  loadingSystemInfo.value = true
  try {
    const { data } = await systemApi.getSystemInfo()
    systemInfo.value = data
  } catch (error) {
    console.error('Failed to fetch system info:', error)
  } finally {
    loadingSystemInfo.value = false
  }
}

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(i > 0 ? 2 : 0) + ' ' + sizes[i]
}

const formatUptime = (seconds: number): string => {
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时`
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  return `${days} 天 ${hours} 小时`
}

const triggerCleanup = async () => {
  cleaningUp.value = true
  try {
    const { data } = await systemApi.triggerCleanup()
    ElMessage.success(`清理完成: 删除了 ${data.eventsDeleted} 条事件、${data.recordingEventsDeleted} 条录制事件和 ${data.alertLogsDeleted} 条告警日志`)
    await refreshSystemInfo()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '清理失败')
  } finally {
    cleaningUp.value = false
  }
}

const testHealth = async () => {
  testingHealth.value = true
  try {
    const { data } = await logApi.health()
    if (data.status === 'ok') {
      ElMessage.success('Collector 运行正常')
    } else {
      ElMessage.error('Collector 状态异常')
    }
  } catch (error) {
    ElMessage.error('无法连接到 Collector')
  } finally {
    testingHealth.value = false
  }
}

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

const copyInitCode = () => {
  const code = `LogMonitor.init({
  dsn: '${collectorUrl}/api/report',
  appId: '${selectedAppId.value || "your-app-id"}',
  release: '1.0.0',
  autoCapture: true
})`
  copyToClipboard(code)
}

const copyExampleCode = () => {
  const code = `// 上报信息
LogMonitor.info('User logged in', { userId: '123' })

// 上报警告
LogMonitor.warn('API slow', { duration: 2000 })

// 上报错误
try {
  // some code
} catch (err) {
  LogMonitor.captureException(err)
}

// 自定义事件
LogMonitor.track('button_click', { button: 'submit' })`
  copyToClipboard(code)
}

const saveRetention = () => {
  ElMessage.info(`数据保留策略已设置为 ${retentionDays.value} 天（需要后端支持）`)
}

const handleChangePassword = async () => {
  if (!passwordFormRef.value) return

  await passwordFormRef.value.validate(async (valid) => {
    if (!valid) return

    changingPassword.value = true
    try {
      await authApi.changePassword({
        old_password: passwordForm.old_password,
        new_password: passwordForm.new_password
      })
      ElMessage.success('密码修改成功，请重新登录')
      // Clear password form
      Object.assign(passwordForm, {
        old_password: '',
        new_password: '',
        confirm_password: ''
      })
      // Logout and redirect to login
      setTimeout(() => {
        localStorage.removeItem('logmon_token')
        localStorage.removeItem('logmon_user')
        window.location.href = '/logmon/login'
      }, 1500)
    } catch (error: any) {
      ElMessage.error(error.response?.data?.error || '密码修改失败')
    } finally {
      changingPassword.value = false
    }
  })
}

onMounted(() => {
  fetchApps()
  refreshSystemInfo()
})
</script>

<style scoped>
.settings-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.code-block {
  position: relative;
  background: #0a0e27;
  border-radius: 6px;
  overflow: hidden;
}

.code-block pre {
  margin: 0;
  padding: 16px;
  font-size: 13px;
  color: #e0e6ed;
  overflow-x: auto;
}

.copy-btn {
  position: absolute;
  top: 8px;
  right: 8px;
}

.system-info {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 12px;
  border-bottom: 1px solid #2d3748;
}

.info-item:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.info-label {
  color: #94a3b8;
  font-size: 14px;
}

.info-value {
  color: #e0e6ed;
  font-size: 14px;
  font-family: 'Monaco', 'Menlo', monospace;
}

.quick-actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.quick-actions .el-button {
  width: 100%;
  justify-content: flex-start;
}

.ml-2 {
  margin-left: 8px;
}

.ml-4 {
  margin-left: 16px;
}

.mt-4 {
  margin-top: 20px;
}
</style>

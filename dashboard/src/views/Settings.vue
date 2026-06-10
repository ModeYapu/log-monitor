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

        <!-- Slice 4: Storage Overview Section -->
        <el-card class="mt-4">
          <template #header>
            <div class="flex justify-between items-center">
              <span>存储概览</span>
              <el-button size="small" @click="fetchStorageStats" :loading="loadingStorageStats">
                <el-icon><Refresh /></el-icon>
                刷新
              </el-button>
            </div>
          </template>

          <div v-loading="loadingStorageStats" class="storage-overview">
            <div class="stat-card">
              <div class="stat-label">数据库大小</div>
              <div class="stat-value">{{ formatBytes(storageStats.db_size_bytes) }}</div>
            </div>

            <div class="table-breakdown">
              <div class="section-title">数据表统计</div>
              <div v-for="table in storageStats.tables" :key="table.name" class="table-item">
                <span class="table-name">{{ formatTableName(table.name) }}</span>
                <span class="table-count">{{ formatNumber(table.row_count) }}</span>
                <span class="table-size">{{ formatBytes(table.size_estimate) }}</span>
              </div>
            </div>

            <div class="app-distribution" v-if="storageStats.apps.length > 0">
              <div class="section-title">应用事件分布</div>
              <div v-for="app in storageStats.apps" :key="app.app_id" class="app-item">
                <span class="app-name">{{ app.app_id }}</span>
                <span class="app-count">{{ formatNumber(app.event_count) }} 事件</span>
              </div>
            </div>
          </div>
        </el-card>

        <!-- Slice 4: Retention Policy Section -->
        <el-card class="mt-4">
          <template #header>
            <span>保留策略配置</span>
          </template>

          <el-form label-width="180px" v-loading="loadingRetentionPolicy">
            <el-form-item label="事件数据保留">
              <el-input-number v-model="retentionPolicy.events" :min="1" :max="365" />
              <span class="ml-2">天</span>
            </el-form-item>
            <el-form-item label="录制事件保留">
              <el-input-number v-model="retentionPolicy.recording_events" :min="1" :max="365" />
              <span class="ml-2">天</span>
            </el-form-item>
            <el-form-item label="截图数据保留">
              <el-input-number v-model="retentionPolicy.screenshots" :min="1" :max="365" />
              <span class="ml-2">天</span>
            </el-form-item>
            <el-form-item label="告警日志保留">
              <el-input-number v-model="retentionPolicy.alert_logs" :min="1" :max="365" />
              <span class="ml-2">天</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveRetentionPolicy" :loading="savingRetentionPolicy">
                保存策略
              </el-button>
              <el-button @click="resetRetentionPolicy">
                重置默认
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- Slice 4: Manual Cleanup Section -->
        <el-card class="mt-4">
          <template #header>
            <span>手动清理</span>
          </template>

          <div class="cleanup-section">
            <el-form label-width="180px">
              <el-form-item label="上次清理时间">
                <span>{{ systemInfo.lastCleanupTime ? formatTime(systemInfo.lastCleanupTime) : '从未清理' }}</span>
              </el-form-item>
              <el-form-item>
                <el-button type="warning" @click="confirmManualCleanup" :loading="cleaningUp">
                  <el-icon><Delete /></el-icon>
                  立即执行清理
                </el-button>
              </el-form-item>
            </el-form>

            <div v-if="cleanupResult" class="cleanup-result">
              <el-divider>清理结果</el-divider>
              <div class="result-item">
                <span class="result-label">删除事件数:</span>
                <span class="result-value">{{ formatNumber(cleanupResult.events_deleted) }}</span>
              </div>
              <div class="result-item">
                <span class="result-label">删除录制事件数:</span>
                <span class="result-value">{{ formatNumber(cleanupResult.recording_events_deleted) }}</span>
              </div>
              <div class="result-item">
                <span class="result-label">删除截图数:</span>
                <span class="result-value">{{ formatNumber(cleanupResult.screenshots_deleted) }}</span>
              </div>
              <div class="result-item">
                <span class="result-label">删除告警日志数:</span>
                <span class="result-value">{{ formatNumber(cleanupResult.alert_logs_deleted) }}</span>
              </div>
              <div class="result-item">
                <span class="result-label">释放空间:</span>
                <span class="result-value">{{ formatBytes(cleanupResult.freed_bytes) }}</span>
              </div>
            </div>
          </div>
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

    <!-- Slice 4: Manual Cleanup Confirmation Dialog -->
    <el-dialog
      v-model="showCleanupDialog"
      title="确认执行清理"
      width="500px"
      :close-on-click-modal="false"
    >
      <div class="cleanup-confirm">
        <el-alert
          title="警告：此操作将删除符合保留策略的旧数据，且不可恢复"
          type="warning"
          :closable="false"
          show-icon
          class="mb-4"
        />
        <p>即将执行清理操作，删除以下数据：</p>
        <ul>
          <li>超过 {{ retentionPolicy.events }} 天的事件数据</li>
          <li>超过 {{ retentionPolicy.recording_events }} 天的录制事件</li>
          <li>超过 {{ retentionPolicy.screenshots }} 天的截图数据</li>
          <li>超过 {{ retentionPolicy.alert_logs }} 天的告警日志</li>
        </ul>
        <p class="text-secondary mt-4">确定要继续吗？</p>
      </div>
      <template #footer>
        <el-button @click="showCleanupDialog = false">取消</el-button>
        <el-button type="warning" @click="executeManualCleanup" :loading="cleaningUp">
          确认清理
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { CircleCheck, Refresh, Delete } from '@element-plus/icons-vue'
import { logApi, authApi, systemApi, adminApi } from '../api'
import { formatNumber, formatTime } from '../utils/formatters'

const apps = ref<any[]>([])
const selectedAppId = ref('')
const retentionDays = ref(30)
const testingHealth = ref(false)
const changingPassword = ref(false)
const loadingSystemInfo = ref(false)
const cleaningUp = ref(false)

// Slice 4: Storage governance state
const loadingStorageStats = ref(false)
const storageStats = ref({
  db_size_bytes: 0,
  tables: [] as Array<{ name: string; row_count: number; size_estimate: number }>,
  apps: [] as Array<{ app_id: string; event_count: number }>
})

const loadingRetentionPolicy = ref(false)
const retentionPolicy = ref({
  events: 30,
  recording_events: 14,
  screenshots: 7,
  alert_logs: 30
})

const savingRetentionPolicy = ref(false)
const cleanupResult = ref<any>(null)
const showCleanupDialog = ref(false)

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

const formatTableName = (name: string): string => {
  const nameMap: Record<string, string> = {
    'events': '事件',
    'recording_events': '录制事件',
    'recordings': '录制会话',
    'alert_logs': '告警日志',
    'alert_rules': '告警规则',
    'system_meta': '系统元数据',
    'users': '用户'
  }
  return nameMap[name] || name
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

// Slice 4: Storage governance functions
const fetchStorageStats = async () => {
  loadingStorageStats.value = true
  try {
    const { data } = await adminApi.getStorageStats()
    storageStats.value = data
  } catch (error) {
    console.error('Failed to fetch storage stats:', error)
    ElMessage.error('获取存储统计失败')
  } finally {
    loadingStorageStats.value = false
  }
}

const fetchRetentionPolicy = async () => {
  loadingRetentionPolicy.value = true
  try {
    const { data } = await adminApi.getRetentionPolicy()
    retentionPolicy.value = data
  } catch (error) {
    console.error('Failed to fetch retention policy:', error)
    ElMessage.error('获取保留策略失败')
  } finally {
    loadingRetentionPolicy.value = false
  }
}

const saveRetentionPolicy = async () => {
  savingRetentionPolicy.value = true
  try {
    await adminApi.setRetentionPolicy(retentionPolicy.value)
    ElMessage.success('保留策略保存成功')
    await fetchStorageStats() // Refresh storage stats to show potential impact
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '保存保留策略失败')
  } finally {
    savingRetentionPolicy.value = false
  }
}

const resetRetentionPolicy = async () => {
  retentionPolicy.value = {
    events: 30,
    recording_events: 14,
    screenshots: 7,
    alert_logs: 30
  }
  await saveRetentionPolicy()
  ElMessage.info('保留策略已重置为默认值')
}

const confirmManualCleanup = () => {
  showCleanupDialog.value = true
}

const executeManualCleanup = async () => {
  cleaningUp.value = true
  showCleanupDialog.value = false
  try {
    const { data } = await adminApi.triggerManualCleanup()
    cleanupResult.value = data
    ElMessage.success('清理完成')
    await refreshSystemInfo()
    await fetchStorageStats()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '清理失败')
  } finally {
    cleaningUp.value = false
  }
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
  fetchStorageStats()
  fetchRetentionPolicy()
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

/* Slice 4: Storage governance styles */
.storage-overview {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.stat-card {
  background: #1a1f2e;
  border-radius: 8px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.stat-label {
  color: #94a3b8;
  font-size: 14px;
}

.stat-value {
  color: #e0e6ed;
  font-size: 24px;
  font-weight: bold;
  font-family: 'Monaco', 'Menlo', monospace;
}

.table-breakdown {
  background: #1a1f2e;
  border-radius: 8px;
  padding: 16px;
}

.section-title {
  color: #e0e6ed;
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 12px;
}

.table-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid #2d3748;
}

.table-item:last-child {
  border-bottom: none;
}

.table-name {
  color: #94a3b8;
  font-size: 14px;
  flex: 1;
}

.table-count {
  color: #e0e6ed;
  font-size: 14px;
  font-family: 'Monaco', 'Menlo', monospace;
  margin-right: 20px;
}

.table-size {
  color: #64748b;
  font-size: 12px;
  font-family: 'Monaco', 'Menlo', monospace;
}

.app-distribution {
  background: #1a1f2e;
  border-radius: 8px;
  padding: 16px;
}

.app-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid #2d3748;
}

.app-item:last-child {
  border-bottom: none;
}

.app-name {
  color: #94a3b8;
  font-size: 14px;
}

.app-count {
  color: #e0e6ed;
  font-size: 14px;
  font-family: 'Monaco', 'Menlo', monospace;
}

.cleanup-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.cleanup-result {
  background: #1a1f2e;
  border-radius: 8px;
  padding: 16px;
}

.result-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid #2d3748;
}

.result-item:last-child {
  border-bottom: none;
}

.result-label {
  color: #94a3b8;
  font-size: 14px;
}

.result-value {
  color: #e0e6ed;
  font-size: 14px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-weight: 600;
}

.text-secondary {
  color: #64748b;
}

.mb-4 {
  margin-bottom: 16px;
}

.flex {
  display: flex;
}

.justify-between {
  justify-content: space-between;
}

.items-center {
  align-items: center;
}
</style>

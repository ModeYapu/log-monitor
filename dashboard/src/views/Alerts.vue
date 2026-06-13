<template>
  <div class="alerts-page">
    <h1 class="sr-only">告警管理</h1>
    <el-row :gutter="20">
      <el-col :span="16">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>告警规则</span>
              <el-button type="primary" size="small" @click="showCreateDialog = true" :icon="Plus">
                新建规则
              </el-button>
            </div>
          </template>

          <el-table :data="alertRules" v-loading="loading" stripe @row-click="toggleExpand" class="alerts-table">
            <el-table-column type="expand">
              <template #default="{ row }">
                <div class="trigger-history-panel">
                  <div class="history-header">
                    <span class="history-title">触发历史（最近20条）</span>
                    <el-tag v-if="rowTriggerCount[row.id]" size="small" type="info">
                      累计触发 {{ rowTriggerCount[row.id] }} 次
                    </el-tag>
                  </div>
                  <div v-if="rowTriggerHistory[row.id] && rowTriggerHistory[row.id].length > 0" class="history-list">
                    <div v-for="log in rowTriggerHistory[row.id]" :key="log.id" class="history-item">
                      <div class="history-icon">
                        <el-icon><Bell /></el-icon>
                      </div>
                      <div class="history-content">
                        <div class="history-message">{{ log.message }}</div>
                        <div class="history-meta">
                          <span class="history-time">{{ formatDateTime(log.created_at) }}</span>
                          <el-tag size="small" type="success">已发送</el-tag>
                        </div>
                      </div>
                    </div>
                  </div>
                  <el-empty v-else description="暂无触发记录" :image-size="60" />
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="name" label="规则名称" width="180" />
            <el-table-column prop="condition_type" label="条件类型" width="110">
              <template #default="{ row }">
                <el-tag size="small">{{ getConditionTypeLabel(row.condition_type) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="condition_config" label="触发条件" min-width="180">
              <template #default="{ row }">
                {{ formatConditionConfig(row.condition_type, row.condition_config) }}
              </template>
            </el-table-column>
            <el-table-column prop="notify_type" label="通知方式" width="90">
              <template #default="{ row }">
                {{ getNotifyTypeLabel(row.notify_type) }}
              </template>
            </el-table-column>
            <el-table-column prop="last_triggered_at" label="最后触发" width="110">
              <template #default="{ row }">
                <span v-if="row.last_triggered_at" class="text-secondary">{{ formatRelativeTime(row.last_triggered_at) }}</span>
                <span v-else class="text-secondary">-</span>
              </template>
            </el-table-column>
            <el-table-column prop="trigger_count" label="触发次数" width="90" align="center">
              <template #default="{ row }">
                <el-badge :value="rowTriggerCount[row.id] || 0" type="danger" :max="999">
                  <el-button size="small" circle @click.stop="toggleExpand(row)">
                    <el-icon><DataAnalysis /></el-icon>
                  </el-button>
                </el-badge>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="90">
              <template #default="{ row }">
                <el-switch
                  v-model="row.enabled"
                  @change="toggleRule(row)"
                  :loading="row._toggling"
                  size="small"
                  @click.stop
                />
                <div v-if="isSilenced(row)" class="text-warning mt-1" style="font-size: 11px;">
                  <el-tooltip :content="`静默至 ${formatDateTime(row.silenced_until)}`" placement="top">
                    <span>已静默</span>
                  </el-tooltip>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="220" align="center">
              <template #default="{ row }">
                <el-button
                  type="primary"
                  size="small"
                  link
                  @click.stop="handleTestRule(row)"
                  :loading="row._testing"
                >
                  测试
                </el-button>
                <el-button
                  v-if="isSilenced(row)"
                  type="success"
                  size="small"
                  link
                  @click.stop="handleUnsilence(row)"
                  :loading="row._unsilencing"
                >
                  取消静默
                </el-button>
                <el-button
                  v-else
                  size="small"
                  link
                  @click.stop="handleSilence(row)"
                  :loading="row._silencing"
                >
                  静默
                </el-button>
                <el-button type="danger" size="small" link @click.stop="handleDelete(row)">
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>

      <el-col :span="8">
        <el-card>
          <template #header>
            <span>告警历史</span>
          </template>
          <div class="alert-logs">
            <div v-for="log in alertLogs" :key="log.id" class="alert-log-item">
              <div class="alert-log-icon">
                <el-icon><Bell /></el-icon>
              </div>
              <div class="alert-log-content">
                <div class="alert-log-message">{{ log.message }}</div>
                <div class="alert-log-time">{{ formatRelativeTime(log.created_at) }}</div>
              </div>
            </div>
            <el-empty v-if="alertLogs.length === 0" description="暂无告警历史" :image-size="80" />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Create Alert Dialog -->
    <el-dialog
      v-model="showCreateDialog"
      title="新建告警规则"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form :model="alertForm" :rules="alertFormRules" ref="alertFormRef" label-width="120px">
        <el-form-item label="应用" prop="app_id">
          <el-select v-model="alertForm.app_id" placeholder="选择应用" style="width: 100%">
            <el-option
              v-for="app in apps"
              :key="app.app_id"
              :label="app.app_id"
              :value="app.app_id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="规则名称" prop="name">
          <el-input v-model="alertForm.name" placeholder="输入规则名称（支持变量：{{appId}}, {{release}} 等）" />
        </el-form-item>

        <el-form-item label="消息模板">
          <el-input
            v-model="alertForm.message_template"
            type="textarea"
            :rows="3"
            placeholder="留空使用默认消息格式。可用变量：{{appId}}, {{release}}, {{page}}, {{device}}, {{userCount}}, {{errorCount}}, {{rate}}, {{timeRange}}, {{timestamp}}"
          />
          <div class="text-secondary mt-1" style="font-size: 12px;">
            可用变量：{{appId}} 应用ID, {{release}} 版本, {{page}} 页面, {{device}} 设备, {{userCount}} 影响用户数, {{errorCount}} 错误次数, {{rate}} 错误率, {{timeRange}} 时间范围, {{timestamp}} 触发时间
          </div>
        </el-form-item>

        <el-form-item label="条件类型" prop="condition_type">
          <el-select v-model="alertForm.condition_type" style="width: 100%" @change="handleConditionTypeChange">
            <el-option label="阈值告警" value="threshold" />
            <el-option label="错误率告警" value="rate" />
            <el-option label="新错误告警" value="new_error" />
          </el-select>
        </el-form-item>

        <template v-if="alertForm.condition_type === 'threshold'">
          <el-form-item label="日志级别" prop="condition_config.level">
            <el-select v-model="alertForm.condition_config.level">
              <el-option label="Error" value="error" />
              <el-option label="Warn" value="warn" />
            </el-select>
          </el-form-item>
          <el-form-item label="触发次数" prop="condition_config.count">
            <el-input-number v-model="alertForm.condition_config.count" :min="1" :max="1000" />
            <span class="ml-2 text-secondary">次</span>
          </el-form-item>
          <el-form-item label="时间窗口" prop="condition_config.windowMinutes">
            <el-input-number v-model="alertForm.condition_config.windowMinutes" :min="1" :max="1440" />
            <span class="ml-2 text-secondary">分钟</span>
          </el-form-item>
          <el-form-item label="聚合维度" prop="condition_config.aggregateBy">
            <el-select v-model="alertForm.condition_config.aggregateBy" placeholder="全局统计">
              <el-option label="全局统计" value="none" />
              <el-option label="按 Release" value="release" />
              <el-option label="按页面" value="page" />
              <el-option label="按浏览器" value="browser" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="alertForm.condition_config.aggregateBy === 'release'" label="Release 过滤" prop="condition_config.filterRelease">
            <el-input v-model="alertForm.condition_config.filterRelease" placeholder="留空监控所有 Release" />
          </el-form-item>
          <el-form-item v-if="alertForm.condition_config.aggregateBy === 'page'" label="页面过滤" prop="condition_config.filterPage">
            <el-input v-model="alertForm.condition_config.filterPage" placeholder="留空监控所有页面" />
          </el-form-item>
        </template>

        <template v-if="alertForm.condition_type === 'rate'">
          <el-form-item label="错误率阈值" prop="condition_config.rate">
            <el-input-number v-model="alertForm.condition_config.rate" :min="0.1" :max="100" :step="0.1" />
            <span class="ml-2 text-secondary">%</span>
          </el-form-item>
          <el-form-item label="最小样本数" prop="condition_config.minSamples">
            <el-input-number v-model="alertForm.condition_config.minSamples" :min="10" :max="10000" />
          </el-form-item>
          <el-form-item label="时间窗口" prop="condition_config.windowMinutes">
            <el-input-number v-model="alertForm.condition_config.windowMinutes" :min="1" :max="1440" />
            <span class="ml-2 text-secondary">分钟</span>
          </el-form-item>
          <el-form-item label="聚合维度" prop="condition_config.aggregateBy">
            <el-select v-model="alertForm.condition_config.aggregateBy" placeholder="全局统计">
              <el-option label="全局统计" value="none" />
              <el-option label="按 Release" value="release" />
              <el-option label="按页面" value="page" />
              <el-option label="按浏览器" value="browser" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="alertForm.condition_config.aggregateBy === 'release'" label="Release 过滤" prop="condition_config.filterRelease">
            <el-input v-model="alertForm.condition_config.filterRelease" placeholder="留空监控所有 Release" />
          </el-form-item>
          <el-form-item v-if="alertForm.condition_config.aggregateBy === 'page'" label="页面过滤" prop="condition_config.filterPage">
            <el-input v-model="alertForm.condition_config.filterPage" placeholder="留空监控所有页面" />
          </el-form-item>
        </template>

        <el-form-item label="通知方式" prop="notify_type">
          <el-select v-model="alertForm.notify_type" style="width: 100%" @change="handleNotifyTypeChange">
            <el-option label="飞书" value="feishu" />
            <el-option label="企业微信" value="wecom" />
            <el-option label="钉钉" value="dingtalk" />
            <el-option label="Telegram" value="telegram" />
            <el-option label="Webhook" value="webhook" />
            <el-option label="邮件" value="email" />
          </el-select>
        </el-form-item>

        <el-form-item v-if="alertForm.notify_type === 'feishu'" label="飞书 Webhook" prop="notify_config.url">
          <el-input v-model="alertForm.notify_config.url" placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..." />
        </el-form-item>

        <el-form-item v-if="alertForm.notify_type === 'wecom'" label="企业微信 Webhook" prop="notify_config.url">
          <el-input v-model="alertForm.notify_config.url" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..." />
        </el-form-item>

        <el-form-item v-if="alertForm.notify_type === 'dingtalk'" label="钉钉 Webhook" prop="notify_config.url">
          <el-input v-model="alertForm.notify_config.url" placeholder="https://oapi.dingtalk.com/robot/send?access_token=..." />
        </el-form-item>

        <template v-if="alertForm.notify_type === 'telegram'">
          <el-form-item label="Bot Token" prop="notify_config.bot_token">
            <el-input v-model="alertForm.notify_config.bot_token" placeholder="123456:ABC-DEF1234..." />
          </el-form-item>
          <el-form-item label="Chat ID" prop="notify_config.chat_id">
            <el-input v-model="alertForm.notify_config.chat_id" placeholder="-1001234567890" />
          </el-form-item>
        </template>

        <el-form-item v-if="alertForm.notify_type === 'webhook'" label="Webhook URL" prop="notify_config.url">
          <el-input v-model="alertForm.notify_config.url" placeholder="https://your-webhook-url" />
        </el-form-item>

        <el-form-item v-if="alertForm.notify_type === 'email'" label="邮箱地址" prop="notify_config.email">
          <el-input v-model="alertForm.notify_config.email" placeholder="admin@example.com" />
        </el-form-item>

        <el-form-item v-if="alertForm.notify_type" label="测试发送">
          <el-button @click="handleTestNotification" :loading="testing" size="small">
            发送测试消息
          </el-button>
          <span class="ml-2 text-secondary">验证通知配置是否正确</span>
        </el-form-item>

        <el-form-item label="冷却时间" prop="cooldown_minutes">
          <el-input-number v-model="alertForm.cooldown_minutes" :min="5" :max="1440" />
          <span class="ml-2 text-secondary">分钟</span>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleCreateAlert" :loading="creating">
          创建
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, Bell, DataAnalysis } from '@element-plus/icons-vue'
import { logApi } from '../api'
import { formatRelativeTime } from '../utils/formatters'
import type { AlertRule, AlertLog } from '../types'

const loading = ref(false)
const creating = ref(false)
const testing = ref(false)
const showCreateDialog = ref(false)
const alertFormRef = ref<FormInstance>()

const apps = ref<any[]>([])
const selectedAppId = ref('')
const alertRules = ref<any[]>([])
const alertLogs = ref<AlertLog[]>([])

// Store trigger history per rule
const rowTriggerHistory = ref<Record<number, AlertLog[]>>({})
const expandedRows = ref<Set<number>>(new Set())

// Calculate trigger count per rule from alert logs
const rowTriggerCount = computed(() => {
  const counts: Record<number, number> = {}
  alertLogs.value.forEach(log => {
    if (log.rule_id) {
      counts[log.rule_id] = (counts[log.rule_id] || 0) + 1
    }
  })
  return counts
})

const alertForm = reactive<AlertRule>({
  app_id: '',
  name: '',
  condition_type: 'threshold',
  condition_config: {
    level: 'error',
    count: 10,
    windowMinutes: 5,
    rate: 5,
    minSamples: 100,
    aggregateBy: 'none',
    filterRelease: '',
    filterPage: ''
  },
  notify_type: 'feishu',
  notify_config: {
    url: '',
    email: '',
    bot_token: '',
    chat_id: ''
  },
  enabled: true,
  cooldown_minutes: 30,
  message_template: ''
})

const alertFormRules: FormRules = {
  app_id: [{ required: true, message: '请选择应用', trigger: 'change' }],
  name: [{ required: true, message: '请输入规则名称', trigger: 'blur' }],
  condition_type: [{ required: true, message: '请选择条件类型', trigger: 'change' }],
  notify_type: [{ required: true, message: '请选择通知方式', trigger: 'change' }]
}

const getConditionTypeLabel = (type: string) => {
  const labels: Record<string, string> = {
    threshold: '阈值告警',
    rate: '错误率',
    new_error: '新错误'
  }
  return labels[type] || type
}

const getNotifyTypeLabel = (type: string) => {
  const labels: Record<string, string> = {
    feishu: '飞书',
    wecom: '企业微信',
    dingtalk: '钉钉',
    telegram: 'Telegram',
    webhook: 'Webhook',
    email: '邮件'
  }
  return labels[type] || type
}

const formatConditionConfig = (type: string, config: any) => {
  if (type === 'threshold') {
    return `${config.level} >= ${config.count}次/${config.windowMinutes}分钟`
  }
  if (type === 'rate') {
    return `错误率 >= ${config.rate}%, 样本 >= ${config.minSamples}`
  }
  if (type === 'new_error') {
    return '新出现的错误'
  }
  return '-'
}

const handleConditionTypeChange = (type: string) => {
  alertForm.condition_config = {
    level: 'error',
    count: 10,
    windowMinutes: 5,
    rate: 5,
    minSamples: 100
  }
}

const handleNotifyTypeChange = (type: string) => {
  alertForm.notify_config = { url: '', email: '', bot_token: '', chat_id: '' }
}

// Toggle expand for trigger history
const toggleExpand = (row: any) => {
  const ruleId = row.id
  if (!ruleId) return

  if (expandedRows.value.has(ruleId)) {
    expandedRows.value.delete(ruleId)
  } else {
    expandedRows.value.add(ruleId)
    // Load trigger history for this rule
    loadRuleTriggerHistory(ruleId)
  }
}

// Load trigger history for a specific rule
const loadRuleTriggerHistory = (ruleId: number) => {
  const history = alertLogs.value
    .filter(log => log.rule_id === ruleId)
    .sort((a, b) => b.created_at - a.created_at)
    .slice(0, 20)
  rowTriggerHistory.value[ruleId] = history
}

const fetchData = async () => {
  if (!selectedAppId.value) return

  loading.value = true
  try {
    const { data } = await logApi.getAlerts(selectedAppId.value)
    alertRules.value = (data.rules || []).map(r => ({
      ...r,
      _toggling: false,
      _testing: false,
      _silencing: false,
      _unsilencing: false
    }))
    alertLogs.value = data.logs || []

    // Pre-load trigger counts for all rules
    alertRules.value.forEach(rule => {
      if (rule.id) {
        loadRuleTriggerHistory(rule.id)
      }
    })
  } catch (error) {
    console.error('Failed to fetch alerts:', error)
  } finally {
    loading.value = false
  }
}

const fetchApps = async () => {
  try {
    const { data } = await logApi.getApps()
    apps.value = data
    if (apps.value.length > 0) {
      selectedAppId.value = apps.value[0].app_id
      fetchData()
    }
  } catch (error) {
    console.error('Failed to fetch apps:', error)
  }
}

const toggleRule = async (rule: any) => {
  rule._toggling = true
  try {
    await logApi.createAlert({
      ...rule,
      app_id: selectedAppId.value
    })
    ElMessage.success(rule.enabled ? '规则已启用' : '规则已禁用')
  } catch (error) {
    rule.enabled = !rule.enabled
    ElMessage.error('操作失败')
  } finally {
    rule._toggling = false
  }
}

const handleDelete = async (rule: any) => {
  try {
    await ElMessageBox.confirm('确定要删除这个告警规则吗？', '确认删除', {
      type: 'warning'
    })
    await logApi.deleteAlert(rule.id)
    ElMessage.success('删除成功')
    fetchData()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const isSilenced = (rule: any) => {
  return rule.silenced_until && rule.silenced_until > Date.now()
}

const handleSilence = async (rule: any) => {
  try {
    const { value } = await ElMessageBox.prompt(
      '请输入静默时长（分钟）',
      '静默告警规则',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        inputValue: '60',
        inputPattern: /^\d+$/,
        inputErrorMessage: '请输入有效的分钟数'
      }
    )

    rule._silencing = true
    try {
      await logApi.silenceAlert({
        id: rule.id,
        durationMinutes: parseInt(value)
      })
      ElMessage.success(`已静默 ${value} 分钟`)
      fetchData()
    } catch (error) {
      ElMessage.error('操作失败')
    } finally {
      rule._silencing = false
    }
  } catch (error) {
    // User cancelled
  }
}

const handleUnsilence = async (rule: any) => {
  rule._unsilencing = true
  try {
    await logApi.unsilenceAlert({ id: rule.id })
    ElMessage.success('已取消静默')
    fetchData()
  } catch (error) {
    ElMessage.error('操作失败')
  } finally {
    rule._unsilencing = false
  }
}

const formatDateTime = (timestamp: number) => {
  return new Date(timestamp).toLocaleString('zh-CN')
}

const handleCreateAlert = async () => {
  if (!alertFormRef.value) return

  try {
    await alertFormRef.value.validate()
    creating.value = true

    const ruleData = {
      ...alertForm,
      condition_config: JSON.stringify(alertForm.condition_config),
      notify_config: JSON.stringify(alertForm.notify_config)
    }

    await logApi.createAlert(ruleData)
    ElMessage.success('创建成功')
    showCreateDialog.value = false

    alertFormRef.value.resetFields()
    fetchData()
  } catch (error) {
    console.error('Failed to create alert:', error)
  } finally {
    creating.value = false
  }
}

const handleTestNotification = async () => {
  if (!alertForm.notify_type) {
    ElMessage.warning('请先选择通知方式')
    return
  }

  // 检查必需的配置字段
  if (alertForm.notify_type === 'feishu' || alertForm.notify_type === 'wecom' ||
      alertForm.notify_type === 'dingtalk' || alertForm.notify_type === 'webhook') {
    if (!alertForm.notify_config.url) {
      ElMessage.warning('请先填写 Webhook URL')
      return
    }
  }

  if (alertForm.notify_type === 'telegram') {
    if (!alertForm.notify_config.bot_token || !alertForm.notify_config.chat_id) {
      ElMessage.warning('请先填写 Bot Token 和 Chat ID')
      return
    }
  }

  if (alertForm.notify_type === 'email') {
    if (!alertForm.notify_config.email) {
      ElMessage.warning('请先填写邮箱地址')
      return
    }
  }

  testing.value = true
  try {
    const testData = {
      notify_type: alertForm.notify_type,
      notify_config: JSON.stringify(alertForm.notify_config),
      message: '这是一条来自 LogMonitor 的测试告警消息，如果您收到此消息，说明通知配置正确！'
    }

    await logApi.testAlert(testData)
    ElMessage.success('测试消息发送成功，请检查对应平台是否收到消息')
  } catch (error) {
    console.error('Failed to send test notification:', error)
    ElMessage.error('测试消息发送失败，请检查配置是否正确')
  } finally {
    testing.value = false
  }
}

// Test notification for an existing rule
const handleTestRule = async (rule: any) => {
  rule._testing = true
  try {
    const testData = {
      notify_type: rule.notify_type,
      notify_config: typeof rule.notify_config === 'string' ? rule.notify_config : JSON.stringify(rule.notify_config),
      message: `[测试] 规则 "${rule.name}" 的测试消息 - 如果您收到此消息，说明通知配置正确！`
    }

    await logApi.testAlert(testData)
    ElMessage.success('测试消息发送成功，请检查对应平台是否收到消息')
  } catch (error) {
    console.error('Failed to send test notification:', error)
    ElMessage.error('测试消息发送失败，请检查配置是否正确')
  } finally {
    rule._testing = false
  }
}

onMounted(() => {
  fetchApps()
})
</script>

<style scoped>
.alerts-page {
  padding: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.alerts-table {
  cursor: pointer;
}

.alerts-table :deep(.el-table__expanded-cell) {
  padding: 0;
}

.trigger-history-panel {
  padding: 16px 20px;
  background: #1a1e2e;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.history-title {
  font-size: 14px;
  font-weight: 500;
  color: #e0e6ed;
}

.history-list {
  max-height: 300px;
  overflow-y: auto;
}

.history-item {
  display: flex;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid #2d3748;
}

.history-item:last-child {
  border-bottom: none;
}

.history-icon {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.history-content {
  flex: 1;
  min-width: 0;
}

.history-message {
  color: #e0e6ed;
  font-size: 13px;
  margin-bottom: 4px;
  word-break: break-word;
}

.history-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.history-time {
  color: #94a3b8;
  font-size: 12px;
}

.alert-logs {
  max-height: 500px;
  overflow-y: auto;
}

.alert-log-item {
  display: flex;
  gap: 12px;
  padding: 12px 0;
  border-bottom: 1px solid #2d3748;
}

.alert-log-item:last-child {
  border-bottom: none;
}

.alert-log-icon {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.alert-log-content {
  flex: 1;
  min-width: 0;
}

.alert-log-message {
  color: #e0e6ed;
  font-size: 14px;
  margin-bottom: 4px;
  word-break: break-word;
}

.alert-log-time {
  color: #94a3b8;
  font-size: 12px;
}

.ml-2 {
  margin-left: 8px;
}

.mt-1 {
  margin-top: 4px;
}

.text-warning {
  color: #f59e0b;
}

.text-secondary {
  color: #94a3b8;
}
</style>

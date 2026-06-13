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

        <!-- Slice 4: Webhook Management Section -->
        <el-card class="mt-4">
          <template #header>
            <div class="flex justify-between items-center">
              <span>Webhook 管理</span>
              <el-button size="small" type="primary" @click="showCreateWebhookDialog">
                <el-icon><Plus /></el-icon>
                添加 Webhook
              </el-button>
            </div>
          </template>

          <div v-loading="loadingWebhooks" class="webhook-list">
            <el-table :data="webhooks" stripe>
              <el-table-column prop="name" label="名称" width="150" />
              <el-table-column prop="url" label="URL" min-width="200" show-overflow-tooltip />
              <el-table-column label="事件" width="180">
                <template #default="{ row }">
                  <el-tag v-for="event in row.events" :key="event" size="small" class="mr-1">
                    {{ formatEventName(event) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="80">
                <template #default="{ row }">
                  <el-switch v-model="row.enabled" @change="toggleWebhookEnabled(row)" />
                </template>
              </el-table-column>
              <el-table-column label="最后触发" width="160">
                <template #default="{ row }">
                  {{ row.last_triggered_at ? formatTime(row.last_triggered_at) : '从未触发' }}
                </template>
              </el-table-column>
              <el-table-column label="失败次数" width="80" prop="failure_count" />
              <el-table-column label="操作" width="150" fixed="right">
                <template #default="{ row }">
                  <el-button size="small" @click="testWebhook(row)">
                    <el-icon><Connection /></el-icon>
                    测试
                  </el-button>
                  <el-button size="small" type="danger" @click="confirmDeleteWebhook(row)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </template>
              </el-table-column>
            </el-table>

            <el-empty v-if="!loadingWebhooks && webhooks.length === 0" description="暂无 Webhook" />
          </div>
        </el-card>

        <!-- Slice 6: Source Map Management Section -->
        <el-card class="mt-4">
          <template #header>
            <div class="flex justify-between items-center">
              <span>Source Map 管理</span>
              <div class="flex gap-2">
                <el-select v-model="selectedAppId" placeholder="选择应用" size="small" style="width: 150px" @change="fetchSourceMaps">
                  <el-option
                    v-for="app in apps"
                    :key="app.app_id"
                    :label="app.app_id"
                    :value="app.app_id"
                  />
                </el-select>
                <el-button size="small" @click="fetchSourceMaps" :loading="loadingSourceMaps">
                  <el-icon><Refresh /></el-icon>
                  刷新
                </el-button>
                <el-button size="small" type="primary" @click="showUploadDialog">
                  <el-icon><Plus /></el-icon>
                  上传
                </el-button>
              </div>
            </div>
          </template>

          <div v-loading="loadingSourceMaps" class="sourcemap-list">
            <!-- Grouped by release -->
            <div v-for="(group, release) in groupedSourceMaps" :key="release" class="release-group">
              <div class="release-header">
                <div class="release-info">
                  <span class="release-name">{{ release }}</span>
                  <el-tag size="small" type="info">{{ group.files.length }} 个文件</el-tag>
                  <el-tag size="small" type="success">{{ formatBytes(group.totalSize) }}</el-tag>
                  <span class="release-time">{{ formatTime(group.uploadedAt) }}</span>
                </div>
                <el-button size="small" type="danger" @click="confirmDeleteRelease(release, group)">
                  <el-icon><Delete /></el-icon>
                  删除
                </el-button>
              </div>
              <div class="file-list">
                <div v-for="file in group.files" :key="file.id" class="file-item">
                  <span class="file-name">{{ file.originalUrl }}</span>
                  <span class="file-size">{{ formatBytes(file.fileSize) }}</span>
                </div>
              </div>
            </div>

            <el-empty v-if="!loadingSourceMaps && sourceMaps.length === 0" description="暂无 Source Map 文件" />
          </div>
        </el-card>

        <!-- Slice 6: Source Map Retention Policy -->
        <el-card class="mt-4">
          <template #header>
            <span>Source Map 保留策略</span>
          </template>

          <el-form label-width="180px">
            <el-form-item label="保留天数">
              <el-input-number v-model="sourcemapRetentionDays" :min="7" :max="365" />
              <span class="ml-2 text-secondary">天</span>
              <el-button type="primary" size="small" @click="saveSourcemapRetention" class="ml-4">
                保存
              </el-button>
            </el-form-item>
            <el-form-item>
              <el-alert
                title="注意：Source Map 文件清理需要后端支持，当前设置仅保存在本地"
                type="info"
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

    <!-- Slice 4: Webhook Create/Edit Dialog -->
    <el-dialog
      v-model="showWebhookDialog"
      :title="editingWebhook ? '编辑 Webhook' : '添加 Webhook'"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form ref="webhookFormRef" :model="webhookForm" :rules="webhookRules" label-width="120px">
        <el-form-item label="Webhook 名称" prop="name">
          <el-input v-model="webhookForm.name" placeholder="请输入 Webhook 名称" />
        </el-form-item>
        <el-form-item label="URL" prop="url">
          <el-input v-model="webhookForm.url" placeholder="https://example.com/webhook" />
        </el-form-item>
        <el-form-item label="Secret">
          <el-input v-model="webhookForm.secret" placeholder="留空自动生成" show-password />
        </el-form-item>
        <el-form-item label="监听事件" prop="events">
          <el-checkbox-group v-model="webhookForm.events">
            <el-checkbox label="issue.created">问题创建</el-checkbox>
            <el-checkbox label="issue.resolved">问题解决</el-checkbox>
            <el-checkbox label="issue.reopened">问题重新打开</el-checkbox>
            <el-checkbox label="alert.triggered">告警触发</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="webhookForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showWebhookDialog = false">取消</el-button>
        <el-button type="primary" @click="saveWebhook" :loading="savingWebhook">
          保存
        </el-button>
      </template>
    </el-dialog>

    <!-- Slice 4: Webhook Delete Confirmation Dialog -->
    <el-dialog
      v-model="showDeleteWebhookDialog"
      title="确认删除 Webhook"
      width="400px"
    >
      <p>确定要删除 Webhook "<strong>{{ deletingWebhook?.name }}</strong>" 吗？</p>
      <p class="text-secondary">此操作不可恢复。</p>
      <template #footer>
        <el-button @click="showDeleteWebhookDialog = false">取消</el-button>
        <el-button type="danger" @click="deleteWebhook" :loading="deletingWebhookInProgress">
          确认删除
        </el-button>
      </template>
    </el-dialog>

    <!-- Slice 6: Source Map Upload Dialog -->
    <el-dialog
      v-model="showUploadSourceMapDialog"
      title="上传 Source Map"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form label-width="120px">
        <el-form-item label="应用">
          <span>{{ selectedAppId || '请先选择应用' }}</span>
        </el-form-item>
        <el-form-item label="版本号 (Release)">
          <el-input v-model="uploadForm.release" placeholder="例如: 1.0.0" />
        </el-form-item>
        <el-form-item label="环境">
          <el-select v-model="uploadForm.env" placeholder="选择环境">
            <el-option label="生产" value="production" />
            <el-option label="预发布" value="staging" />
            <el-option label="开发" value="development" />
          </el-select>
        </el-form-item>
        <el-form-item label="选择文件">
          <input
            ref="fileInputRef"
            type="file"
            accept=".map,.json"
            multiple
            @change="handleFileSelect"
            style="display: none"
          />
          <el-button @click="triggerFileSelect">
            <el-icon><Plus /></el-icon>
            选择文件
          </el-button>
          <span class="ml-2 text-secondary text-sm">支持 .map 和 .json 文件</span>
        </el-form-item>
        <el-form-item v-if="uploadForm.files.length > 0">
          <div class="file-list-preview">
            <div v-for="(file, index) in uploadForm.files" :key="index" class="file-preview-item">
              <span>{{ file.name }}</span>
              <span class="text-secondary text-sm">{{ formatBytes(file.size) }}</span>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showUploadSourceMapDialog = false">取消</el-button>
        <el-button type="primary" @click="uploadSourceMaps" :loading="uploadingSourceMaps" :disabled="uploadForm.files.length === 0">
          上传
        </el-button>
      </template>
    </el-dialog>

    <!-- Slice 6: Source Map Delete Confirmation Dialog -->
    <el-dialog
      v-model="showDeleteSourceMapDialog"
      title="确认删除 Source Map"
      width="400px"
    >
      <p>确定要删除版本 "<strong>{{ deletingReleaseGroup?.release }}</strong>" 的所有 Source Map 文件吗？</p>
      <p class="text-secondary">此操作将删除 {{ deletingReleaseGroup?.files.length }} 个文件，且不可恢复。</p>
      <template #footer>
        <el-button @click="showDeleteSourceMapDialog = false">取消</el-button>
        <el-button type="danger" @click="deleteReleaseSourceMaps" :loading="deletingSourceMaps">
          确认删除
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { CircleCheck, Refresh, Delete, Plus, Connection } from '@element-plus/icons-vue'
import { logApi, authApi, systemApi, adminApi, sourcemapApi } from '../api'
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

// Slice 4: Webhook state
const webhooks = ref<any[]>([])
const loadingWebhooks = ref(false)
const showWebhookDialog = ref(false)
const showDeleteWebhookDialog = ref(false)
const editingWebhook = ref<any>(null)
const deletingWebhook = ref<any>(null)
const savingWebhook = ref(false)
const deletingWebhookInProgress = ref(false)
const webhookFormRef = ref<FormInstance>()

// Slice 6: Source Map state
const sourceMaps = ref<any[]>([])
const loadingSourceMaps = ref(false)
const showUploadSourceMapDialog = ref(false)
const showDeleteSourceMapDialog = ref(false)
const uploadingSourceMaps = ref(false)
const deletingSourceMaps = ref(false)
const deletingReleaseGroup = ref<any>(null)
const sourcemapRetentionDays = ref(30)
const fileInputRef = ref<HTMLInputElement | null>(null)

const uploadForm = reactive({
  release: '',
  env: 'production',
  files: [] as File[]
})

// Computed: Group source maps by release
const groupedSourceMaps = computed(() => {
  const groups: Record<string, { files: any[]; totalSize: number; uploadedAt: number }> = {}
  for (const sm of sourceMaps.value) {
    const key = sm.release
    if (!groups[key]) {
      groups[key] = {
        files: [],
        totalSize: 0,
        uploadedAt: sm.uploadedAt
      }
    }
    groups[key].files.push(sm)
    groups[key].totalSize += sm.fileSize
    // Use the latest upload time
    if (sm.uploadedAt > groups[key].uploadedAt) {
      groups[key].uploadedAt = sm.uploadedAt
    }
  }
  return groups
})

const webhookForm = reactive({
  name: '',
  url: '',
  secret: '',
  events: [] as string[],
  enabled: true
})

const webhookRules: FormRules = {
  name: [
    { required: true, message: '请输入 Webhook 名称', trigger: 'blur' }
  ],
  url: [
    { required: true, message: '请输入 Webhook URL', trigger: 'blur' },
    { type: 'url', message: '请输入有效的 URL', trigger: 'blur' }
  ],
  events: [
    { required: true, message: '请至少选择一个事件', trigger: 'change' }
  ]
}

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

// Slice 4: Webhook functions
const fetchWebhooks = async () => {
  loadingWebhooks.value = true
  try {
    const { data } = await adminApi.getWebhooks()
    webhooks.value = data
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '获取 Webhook 列表失败')
  } finally {
    loadingWebhooks.value = false
  }
}

const showCreateWebhookDialog = () => {
  editingWebhook.value = null
  Object.assign(webhookForm, {
    name: '',
    url: '',
    secret: '',
    events: [],
    enabled: true
  })
  showWebhookDialog.value = true
}

const showEditWebhookDialog = (webhook: any) => {
  editingWebhook.value = webhook
  Object.assign(webhookForm, {
    name: webhook.name,
    url: webhook.url,
    secret: webhook.secret,
    events: [...webhook.events],
    enabled: webhook.enabled
  })
  showWebhookDialog.value = true
}

const saveWebhook = async () => {
  if (!webhookFormRef.value) return

  await webhookFormRef.value.validate(async (valid) => {
    if (!valid) return

    savingWebhook.value = true
    try {
      if (editingWebhook.value) {
        // Update existing webhook
        await adminApi.updateWebhook(editingWebhook.value.id, webhookForm)
        ElMessage.success('Webhook 更新成功')
      } else {
        // Create new webhook
        await adminApi.createWebhook(webhookForm)
        ElMessage.success('Webhook 创建成功')
      }

      showWebhookDialog.value = false
      await fetchWebhooks()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.error || '保存 Webhook 失败')
    } finally {
      savingWebhook.value = false
    }
  })
}

const toggleWebhookEnabled = async (webhook: any) => {
  try {
    await adminApi.updateWebhook(webhook.id, { enabled: webhook.enabled })
    ElMessage.success(webhook.enabled ? 'Webhook 已启用' : 'Webhook 已禁用')
  } catch (error: any) {
    // Revert on error
    webhook.enabled = !webhook.enabled
    ElMessage.error(error.response?.data?.error || '更新 Webhook 状态失败')
  }
}

const testWebhook = async (webhook: any) => {
  try {
    await adminApi.testWebhook(webhook.id)
    ElMessage.success('测试 Webhook 发送成功')
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '测试 Webhook 发送失败')
  }
}

const confirmDeleteWebhook = (webhook: any) => {
  deletingWebhook.value = webhook
  showDeleteWebhookDialog.value = true
}

const deleteWebhook = async () => {
  if (!deletingWebhook.value) return

  deletingWebhookInProgress.value = true
  try {
    await adminApi.deleteWebhook(deletingWebhook.value.id)
    ElMessage.success('Webhook 删除成功')
    showDeleteWebhookDialog.value = false
    await fetchWebhooks()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '删除 Webhook 失败')
  } finally {
    deletingWebhookInProgress.value = false
  }
}

const formatEventName = (event: string) => {
  const eventNames: Record<string, string> = {
    'issue.created': '问题创建',
    'issue.resolved': '问题解决',
    'issue.reopened': '问题重新打开',
    'alert.triggered': '告警触发'
  }
  return eventNames[event] || event
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

// Slice 6: Source Map functions
const fetchSourceMaps = async () => {
  if (!selectedAppId.value) return

  loadingSourceMaps.value = true
  try {
    const { data } = await sourcemapApi.listSourceMaps(selectedAppId.value)
    sourceMaps.value = data.data || []
  } catch (error: any) {
    console.error('Failed to fetch source maps:', error)
    ElMessage.error('获取 Source Map 列表失败')
  } finally {
    loadingSourceMaps.value = false
  }
}

const showUploadDialog = () => {
  if (!selectedAppId.value) {
    ElMessage.warning('请先选择应用')
    return
  }
  // Reset form
  uploadForm.release = ''
  uploadForm.env = 'production'
  uploadForm.files = []
  showUploadSourceMapDialog.value = true
}

const triggerFileSelect = () => {
  fileInputRef.value?.click()
}

const handleFileSelect = (event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.files) {
    uploadForm.files = Array.from(target.files)
  }
}

const uploadSourceMaps = async () => {
  if (!selectedAppId.value) {
    ElMessage.warning('请先选择应用')
    return
  }
  if (!uploadForm.release) {
    ElMessage.warning('请输入版本号')
    return
  }
  if (uploadForm.files.length === 0) {
    ElMessage.warning('请选择文件')
    return
  }

  uploadingSourceMaps.value = true
  try {
    const formData = new FormData()
    formData.append('appId', selectedAppId.value)
    formData.append('release', uploadForm.release)
    formData.append('env', uploadForm.env)

    uploadForm.files.forEach(file => {
      formData.append('files', file)
    })

    const { data } = await sourcemapApi.uploadSourceMap(formData)

    if (data.success) {
      ElMessage.success(`成功上传 ${data.count} 个文件`)
      showUploadSourceMapDialog.value = false
      await fetchSourceMaps()
    } else {
      ElMessage.error('上传失败')
    }

    if (data.errors && data.errors.length > 0) {
      console.error('Upload errors:', data.errors)
    }
  } catch (error: any) {
    console.error('Failed to upload source maps:', error)
    ElMessage.error(error.response?.data?.error || '上传失败')
  } finally {
    uploadingSourceMaps.value = false
  }
}

const confirmDeleteRelease = (release: string, group: any) => {
  deletingReleaseGroup.value = { release, ...group }
  showDeleteSourceMapDialog.value = true
}

const deleteReleaseSourceMaps = async () => {
  if (!deletingReleaseGroup.value || !selectedAppId.value) return

  deletingSourceMaps.value = true
  try {
    await sourcemapApi.deleteReleaseSourceMaps(selectedAppId.value, deletingReleaseGroup.value.release)
    ElMessage.success('删除成功')
    showDeleteSourceMapDialog.value = false
    await fetchSourceMaps()
  } catch (error: any) {
    console.error('Failed to delete source maps:', error)
    ElMessage.error(error.response?.data?.error || '删除失败')
  } finally {
    deletingSourceMaps.value = false
  }
}

const saveSourcemapRetention = () => {
  localStorage.setItem('sourcemap_retention_days', String(sourcemapRetentionDays.value))
  ElMessage.success(`Source Map 保留天数已设置为 ${sourcemapRetentionDays.value} 天`)
}

onMounted(() => {
  fetchApps()
  refreshSystemInfo()
  fetchStorageStats()
  fetchRetentionPolicy()
  fetchWebhooks()

  // Load sourcemap retention from localStorage
  const savedRetention = localStorage.getItem('sourcemap_retention_days')
  if (savedRetention) {
    sourcemapRetentionDays.value = parseInt(savedRetention, 10)
  }
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

.gap-2 {
  gap: 8px;
}

/* Slice 6: Source Map styles */
.sourcemap-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.release-group {
  background: #1a1f2e;
  border-radius: 8px;
  padding: 16px;
}

.release-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 12px;
  border-bottom: 1px solid #2d3748;
  margin-bottom: 12px;
}

.release-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.release-name {
  color: #e0e6ed;
  font-size: 16px;
  font-weight: 600;
}

.release-time {
  color: #64748b;
  font-size: 12px;
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.file-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #121620;
  border-radius: 4px;
}

.file-name {
  color: #94a3b8;
  font-size: 13px;
}

.file-size {
  color: #64748b;
  font-size: 12px;
}

.file-list-preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 150px;
  overflow-y: auto;
}

.file-preview-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #1a1f2e;
  border-radius: 4px;
}

.text-sm {
  font-size: 12px;
}
</style>

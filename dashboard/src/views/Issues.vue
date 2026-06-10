<template>
  <div class="issues-page">
    <h1 class="sr-only">Issues</h1>

    <div class="page-header">
      <div class="header-title">
        <h2>Issues</h2>
        <span class="subtitle">Track and manage error issues</span>
      </div>
      <div class="header-actions">
        <el-select
          v-model="currentApp"
          placeholder="Select App"
          @change="handleAppChange"
          style="width: 200px"
        >
          <el-option
            v-for="app in apps"
            :key="app.app_id"
            :label="app.app_id"
            :value="app.app_id"
          />
        </el-select>
        <el-button @click="loadIssues" :loading="loading">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
    </div>

    <!-- Filters -->
    <div class="filters">
      <el-select v-model="filters.status" placeholder="Status" clearable @change="handleFilterChange">
        <el-option label="All" value="" />
        <el-option label="Open" value="open" />
        <el-option label="Resolved" value="resolved" />
        <el-option label="Ignored" value="ignored" />
        <el-option label="Muted" value="muted" />
      </el-select>

      <el-select v-model="filters.priority" placeholder="Priority" clearable @change="handleFilterChange">
        <el-option label="All" value="" />
        <el-option label="Critical" value="critical" />
        <el-option label="High" value="high" />
        <el-option label="Medium" value="medium" />
        <el-option label="Low" value="low" />
      </el-select>

      <el-input
        v-model="filters.search"
        placeholder="Search issues..."
        clearable
        @change="handleFilterChange"
        style="width: 250px"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>

      <el-select v-model="filters.sort" placeholder="Sort by" @change="handleFilterChange">
        <el-option label="Last Seen" value="last_seen" />
        <el-option label="Event Count" value="event_count" />
        <el-option label="Priority" value="priority" />
      </el-select>
    </div>

    <!-- Issues Table -->
    <el-table
      :data="issues"
      :loading="loading"
      stripe
      @row-click="handleRowClick"
      style="width: 100%"
    >
      <el-table-column prop="title" label="Title" min-width="300" show-overflow-tooltip>
        <template #default="{ row }">
          <div class="issue-title">
            <el-icon class="error-icon"><Warning /></el-icon>
            <span>{{ row.title }}</span>
          </div>
        </template>
      </el-table-column>

      <el-table-column prop="status" label="Status" width="120">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ formatStatus(row.status) }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column prop="priority" label="Priority" width="100">
        <template #default="{ row }">
          <el-badge :value="row.priority" :type="getPriorityType(row.priority)" class="priority-badge">
            <span>{{ formatPriority(row.priority) }}</span>
          </el-badge>
        </template>
      </el-table-column>

      <el-table-column prop="event_count" label="Events" width="100" align="right">
        <template #default="{ row }">
          <span class="count-number">{{ row.event_count.toLocaleString() }}</span>
        </template>
      </el-table-column>

      <el-table-column prop="user_count" label="Users" width="100" align="right">
        <template #default="{ row }">
          <span class="count-number">{{ row.user_count.toLocaleString() }}</span>
        </template>
      </el-table-column>

      <el-table-column prop="last_seen_at" label="Last Seen" width="180">
        <template #default="{ row }">
          {{ formatTimestamp(row.last_seen_at) }}
        </template>
      </el-table-column>

      <el-table-column label="Actions" width="180" fixed="right">
        <template #default="{ row }">
          <el-button-group size="small">
            <el-button
              v-if="row.status === 'open'"
              type="success"
              @click.stop="handleResolve(row)"
            >
              Resolve
            </el-button>
            <el-button
              v-if="row.status === 'open'"
              type="warning"
              @click.stop="handleIgnore(row)"
            >
              Ignore
            </el-button>
            <el-button
              v-if="row.status === 'resolved'"
              type="primary"
              @click.stop="handleReopen(row)"
            >
              Reopen
            </el-button>
          </el-button-group>
        </template>
      </el-table-column>
    </el-table>

    <!-- Pagination -->
    <div class="pagination">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="pagination.total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="handlePageSizeChange"
        @current-change="handlePageChange"
      />
    </div>

    <!-- Issue Detail Drawer -->
    <el-drawer
      v-model="drawerVisible"
      title="Issue Details"
      size="50%"
      :destroy-on-close="true"
    >
      <IssueDetail
        v-if="selectedIssue"
        :issue="selectedIssue"
        :events="issueEvents"
        :loading="detailLoading"
        @resolve="handleResolve"
        @ignore="handleIgnore"
        @reopen="handleReopen"
        @priority-change="handlePriorityChange"
      />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search, Warning } from '@element-plus/icons-vue'
import { logApi } from '../api'
import type { Issue, App } from '../types'
import IssueDetail from '../components/IssueDetail.vue'
import { formatTimestamp, formatPriority, formatStatus } from '../utils/formatters'

const currentApp = ref('')
const apps = ref<App[]>([])
const issues = ref<Issue[]>([])
const loading = ref(false)
const drawerVisible = ref(false)
const selectedIssue = ref<Issue | null>(null)
const issueEvents = ref<any[]>([])
const detailLoading = ref(false)

const filters = ref({
  status: '',
  priority: '',
  search: '',
  sort: 'last_seen'
})

const pagination = ref({
  page: 1,
  pageSize: 20,
  total: 0
})

// Load apps
const loadApps = async () => {
  try {
    const response = await logApi.getApps()
    apps.value = response.data
    if (apps.value.length > 0 && !currentApp.value) {
      currentApp.value = apps.value[0].app_id
      loadIssues()
    }
  } catch (error) {
    ElMessage.error('Failed to load applications')
  }
}

// Load issues
const loadIssues = async () => {
  if (!currentApp.value) return

  loading.value = true
  try {
    const response = await logApi.getIssues({
      app_id: currentApp.value,
      status: filters.value.status || undefined,
      priority: filters.value.priority || undefined,
      search: filters.value.search || undefined,
      sort: filters.value.sort,
      page: pagination.value.page,
      page_size: pagination.value.pageSize
    })

    issues.value = response.data.data
    pagination.value.total = response.data.total
  } catch (error) {
    ElMessage.error('Failed to load issues')
  } finally {
    loading.value = false
  }
}

// Load issue details
const loadIssueDetail = async (issue: Issue) => {
  detailLoading.value = true
  try {
    const response = await logApi.getIssue(issue.id)
    selectedIssue.value = response.data.issue
    issueEvents.value = response.data.recent_events
  } catch (error) {
    ElMessage.error('Failed to load issue details')
  } finally {
    detailLoading.value = false
  }
}

// Event handlers
const handleAppChange = () => {
  pagination.value.page = 1
  loadIssues()
}

const handleFilterChange = () => {
  pagination.value.page = 1
  loadIssues()
}

const handlePageChange = (page: number) => {
  pagination.value.page = page
  loadIssues()
}

const handlePageSizeChange = (size: number) => {
  pagination.value.pageSize = size
  pagination.value.page = 1
  loadIssues()
}

const handleRowClick = (row: Issue) => {
  selectedIssue.value = row
  drawerVisible.value = true
  loadIssueDetail(row)
}

const handleResolve = async (issue: Issue) => {
  try {
    await logApi.resolveIssue(issue.id)
    ElMessage.success('Issue resolved successfully')
    loadIssues()
    if (drawerVisible.value && selectedIssue.value?.id === issue.id) {
      loadIssueDetail(issue)
    }
  } catch (error) {
    ElMessage.error('Failed to resolve issue')
  }
}

const handleIgnore = async (issue: Issue) => {
  try {
    await logApi.ignoreIssue(issue.id)
    ElMessage.success('Issue ignored successfully')
    loadIssues()
    if (drawerVisible.value && selectedIssue.value?.id === issue.id) {
      loadIssueDetail(issue)
    }
  } catch (error) {
    ElMessage.error('Failed to ignore issue')
  }
}

const handleReopen = async (issue: Issue) => {
  try {
    await logApi.updateIssue(issue.id, { status: 'open' })
    ElMessage.success('Issue reopened successfully')
    loadIssues()
    if (drawerVisible.value && selectedIssue.value?.id === issue.id) {
      loadIssueDetail(issue)
    }
  } catch (error) {
    ElMessage.error('Failed to reopen issue')
  }
}

const handlePriorityChange = async (issue: Issue, priority: string) => {
  try {
    await logApi.updateIssue(issue.id, { priority })
    ElMessage.success('Priority updated successfully')
    loadIssues()
    if (drawerVisible.value && selectedIssue.value?.id === issue.id) {
      loadIssueDetail(issue)
    }
  } catch (error) {
    ElMessage.error('Failed to update priority')
  }
}

// Helper functions
const getStatusType = (status: string) => {
  const types: Record<string, any> = {
    open: 'danger',
    resolved: 'success',
    ignored: 'info',
    muted: 'warning'
  }
  return types[status] || 'info'
}

const getPriorityType = (priority: string) => {
  const types: Record<string, any> = {
    critical: 'danger',
    high: 'warning',
    medium: 'primary',
    low: 'info'
  }
  return types[priority] || 'info'
}

onMounted(() => {
  loadApps()
})
</script>

<style scoped>
.issues-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-title h2 {
  margin: 0;
  font-size: 24px;
  color: #303133;
}

.subtitle {
  color: #909399;
  font-size: 14px;
  margin-left: 10px;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.filters {
  display: flex;
  gap: 10px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.issue-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.error-icon {
  color: #F56C6C;
  font-size: 18px;
}

.count-number {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-weight: 600;
}

.priority-badge {
  font-weight: 600;
}

.pagination {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}
</style>
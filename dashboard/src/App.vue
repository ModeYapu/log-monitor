<template>
  <div v-if="isLoginPage">
    <router-view />
  </div>
  <el-container v-else class="app-container">
    <el-aside :width="sidebarCollapsed ? '64px' : '220px'" class="sidebar" :class="{ 'sidebar-light': isLight, 'sidebar-collapsed': sidebarCollapsed }">
      <div class="logo" :class="{ 'logo-collapsed': sidebarCollapsed }">
        <h2 v-if="!sidebarCollapsed">LogMonitor</h2>
        <h2 v-else class="logo-icon">LM</h2>
        <span v-if="!sidebarCollapsed" class="version">v1.0</span>
      </div>
      <el-menu
        :default-active="activeMenu"
        router
        :collapse="sidebarCollapsed"
        :background-color="isLight ? '#ffffff' : '#0a0e27'"
        :text-color="isLight ? '#606266' : '#94a3b8'"
        :active-text-color="'#6366f1'"
      >
        <el-tooltip content="概览" placement="right" :disabled="!sidebarCollapsed">
          <el-menu-item index="/">
            <el-icon><DataLine /></el-icon>
            <span>概览</span>
          </el-menu-item>
        </el-tooltip>
        <el-tooltip content="日志列表" placement="right" :disabled="!sidebarCollapsed">
          <el-menu-item index="/logs">
            <el-icon><Document /></el-icon>
            <span>日志列表</span>
          </el-menu-item>
        </el-tooltip>
        <el-tooltip content="性能分析" placement="right" :disabled="!sidebarCollapsed">
          <el-menu-item index="/performance">
            <el-icon><TrendCharts /></el-icon>
            <span>性能分析</span>
          </el-menu-item>
        </el-tooltip>
        <el-tooltip content="告警管理" placement="right" :disabled="!sidebarCollapsed">
          <el-menu-item index="/alerts">
            <el-icon><Bell /></el-icon>
            <span>告警管理</span>
          </el-menu-item>
        </el-tooltip>
        <el-tooltip content="实时会话" placement="right" :disabled="!sidebarCollapsed">
          <el-menu-item index="/live">
            <el-icon><VideoCamera /></el-icon>
            <span>实时会话</span>
          </el-menu-item>
        </el-tooltip>
        <el-tooltip content="录制回放" placement="right" :disabled="!sidebarCollapsed">
          <el-menu-item index="/recordings">
            <el-icon><Film /></el-icon>
            <span>录制回放</span>
          </el-menu-item>
        </el-tooltip>
        <el-tooltip content="系统设置" placement="right" :disabled="!sidebarCollapsed">
          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </el-menu-item>
        </el-tooltip>
        <el-tooltip content="用户管理" placement="right" :disabled="!sidebarCollapsed">
          <el-menu-item index="/users" v-if="isAdmin">
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
        </el-tooltip>
      </el-menu>
      <div class="sidebar-toggle" @click="toggleSidebar">
        <el-icon :size="16">
          <ArrowLeft v-if="!sidebarCollapsed" />
          <ArrowRight v-else />
        </el-icon>
      </div>
    </el-aside>

    <el-main class="main-content">
      <div class="app-header" :class="{ 'app-header-light': isLight }">
        <div class="header-left">
          <GlobalSearch />
          <el-select
            v-model="selectedAppId"
            placeholder="选择应用"
            filterable
            @change="handleAppChange"
            style="width: 300px"
          >
            <el-option
              v-for="app in apps"
              :key="app.app_id"
              :label="app.app_id"
              :value="app.app_id"
            >
              <span>{{ app.app_id }}</span>
              <span class="app-stats">({{ app.error_count }} errors)</span>
            </el-option>
          </el-select>
        </div>
        <div class="header-actions">
          <div class="user-info" v-if="currentUser">
            <span class="user-name">{{ currentUser.display_name || currentUser.username }}</span>
            <el-tag :type="currentUser.role === 'admin' ? 'danger' : 'primary'" size="small">
              {{ currentUser.role === 'admin' ? '管理员' : '用户' }}
            </el-tag>
          </div>
          <el-switch
            v-model="isDark"
            :active-action-icon="Moon"
            :inactive-action-icon="Sunny"
            @change="toggleTheme"
            style="--el-switch-on-color: #2d3748; --el-switch-off-color: #f59e0b"
          />
          <el-button @click="refreshData" :loading="loading" :icon="Refresh" circle />
          <el-button @click="handleLogout" :icon="SwitchButton" circle />
        </div>
      </div>

      <div class="page-content">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" :key="route.fullPath" />
          </transition>
        </router-view>
      </div>
    </el-main>
  </el-container>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Refresh, DataLine, Document, TrendCharts, Bell, Setting, VideoCamera, Film, Moon, Sunny, User, SwitchButton, ArrowLeft, ArrowRight } from '@element-plus/icons-vue'
import { logApi } from './api'
import type { App, UserInfo } from './types'
import GlobalSearch from './components/GlobalSearch.vue'

const route = useRoute()
const router = useRouter()

const selectedAppId = ref<string>('')
const apps = ref<App[]>([])
const loading = ref(false)
const currentUser = ref<UserInfo | null>(null)

const isLoginPage = computed(() => route.path === '/login')

const isAdmin = computed(() => currentUser.value?.role === 'admin')

// Theme
const isDark = ref(true)
const isLight = computed(() => !isDark.value)

// Sidebar collapse
const sidebarCollapsed = ref(false)
const initSidebar = () => {
  const saved = localStorage.getItem('logmon-sidebar-collapsed')
  if (saved === 'true') {
    sidebarCollapsed.value = true
  }
}
const toggleSidebar = () => {
  sidebarCollapsed.value = !sidebarCollapsed.value
  localStorage.setItem('logmon-sidebar-collapsed', String(sidebarCollapsed.value))
}

const initTheme = () => {
  const saved = localStorage.getItem('logmon-theme')

  if (saved === 'light') {
    isDark.value = false
    document.documentElement.setAttribute('data-theme', 'light')
    document.documentElement.classList.remove('dark')
  } else if (saved === 'dark') {
    isDark.value = true
    document.documentElement.setAttribute('data-theme', 'dark')
    document.documentElement.classList.add('dark')
  } else {
    // Check system preference on first visit
    const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
    if (prefersDark) {
      isDark.value = true
      document.documentElement.setAttribute('data-theme', 'dark')
      document.documentElement.classList.add('dark')
      // Save the preference
      localStorage.setItem('logmon-theme', 'dark')
    } else {
      isDark.value = false
      document.documentElement.setAttribute('data-theme', 'light')
      document.documentElement.classList.remove('dark')
      localStorage.setItem('logmon-theme', 'light')
    }
  }

  // Listen for system theme changes
  const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  mediaQuery.addEventListener('change', (e) => {
    // Only auto-switch if user hasn't set a preference
    if (!localStorage.getItem('logmon-theme')) {
      if (e.matches) {
        isDark.value = true
        document.documentElement.setAttribute('data-theme', 'dark')
        document.documentElement.classList.add('dark')
      } else {
        isDark.value = false
        document.documentElement.setAttribute('data-theme', 'light')
        document.documentElement.classList.remove('dark')
      }
    }
  })
}

const toggleTheme = (val: boolean) => {
  if (val) {
    document.documentElement.setAttribute('data-theme', 'dark')
    document.documentElement.classList.add('dark')
    localStorage.setItem('logmon-theme', 'dark')
  } else {
    document.documentElement.setAttribute('data-theme', 'light')
    document.documentElement.classList.remove('dark')
    localStorage.setItem('logmon-theme', 'light')
  }
}

const activeMenu = computed(() => route.path)

const fetchApps = async () => {
  try {
    const { data } = await logApi.getApps()
    apps.value = data
    if (apps.value.length > 0 && !selectedAppId.value) {
      selectedAppId.value = apps.value[0].app_id
    }
  } catch (error) {
    ElMessage.error('获取应用列表失败')
  }
}

const handleAppChange = (appId: string) => {
  const currentPath = route.path
  if (currentPath.startsWith('/logs') || currentPath.startsWith('/performance') || currentPath.startsWith('/alerts')) {
    router.replace({ path: currentPath, query: { appId } })
  }
}

const refreshData = () => {
  loading.value = true
  fetchApps().finally(() => {
    loading.value = false
    ElMessage.success('刷新成功')
  })
}

const getCurrentUser = () => {
  const userStr = localStorage.getItem('logmon_user')
  if (userStr) {
    try {
      currentUser.value = JSON.parse(userStr)
    } catch {
      currentUser.value = null
    }
  }
}

const handleLogout = () => {
  localStorage.removeItem('logmon_token')
  localStorage.removeItem('logmon_user')
  currentUser.value = null
  router.push('/login')
}

onMounted(() => {
  initTheme()
  initSidebar()
  getCurrentUser()
  fetchApps()
})
</script>

<style scoped>
.app-container {
  height: 100vh;
}

.sidebar {
  background: var(--color-bg);
  border-right: 1px solid var(--color-border);
  display: flex;
  flex-direction: column;
  transition: width 0.3s ease, background 0.3s ease, border-color 0.3s ease;
  overflow: hidden;
}

.sidebar-collapsed {
  overflow: visible;
}

.sidebar-light {
  background: var(--color-bg-secondary);
  border-right-color: var(--color-border);
}

.sidebar-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 12px;
  cursor: pointer;
  color: var(--color-text-secondary);
  border-top: 1px solid var(--color-border);
  transition: background 0.2s, color 0.2s;
}

.sidebar-toggle:hover {
  background: var(--color-bg-secondary);
  color: var(--color-text);
}

.logo {
  padding: 20px;
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.logo-collapsed {
  justify-content: center;
  padding: 20px 0;
}

.logo-icon {
  font-size: 18px;
  text-align: center;
  background: linear-gradient(135deg, #6366f1, #a855f7);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.logo h2 {
  color: var(--color-text);
  font-size: 24px;
  margin: 0;
  background: linear-gradient(135deg, #6366f1, #a855f7);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.version {
  color: var(--color-text-secondary);
  font-size: 12px;
}

.el-menu {
  border-right: none;
  flex: 1;
}

.el-menu-item {
  height: 48px;
  line-height: 48px;
}

.el-menu-item:hover {
  background: var(--color-bg-secondary) !important;
}

.el-menu-item.is-active {
  background: var(--color-bg-secondary) !important;
  border-right: 3px solid #6366f1;
}

.main-content {
  padding: 0;
  display: flex;
  flex-direction: column;
  background: var(--color-bg);
}

.app-header {
  padding: 16px 24px;
  background: var(--color-bg-secondary);
  border-bottom: 1px solid var(--color-border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
  flex: 1;
}

.app-header-light {
  background: var(--color-bg-secondary);
  border-bottom-color: var(--color-border);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  background: var(--color-bg);
  border-radius: 6px;
}

.user-name {
  color: var(--color-text);
  font-size: 14px;
  font-weight: 500;
}

.page-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.app-stats {
  color: var(--color-text-secondary);
  font-size: 12px;
  margin-left: 8px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>

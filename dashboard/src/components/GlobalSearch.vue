<template>
  <div class="global-search">
    <el-input
      v-model="searchQuery"
      placeholder="搜索错误消息、URL、标签、会话ID... (Ctrl+K)"
      :prefix-icon="Search"
      clearable
      @input="handleSearch"
      @focus="showResults = true"
      @blur="handleBlur"
      @keyup="handleKeydown"
      ref="searchInput"
    />

    <div v-if="showResults && (loading || results.length > 0)" class="search-results">
      <div v-if="loading" class="loading">
        <el-icon class="is-loading"><Loading /></el-icon>
        <span>搜索中...</span>
      </div>

      <div v-else-if="results.length > 0" class="results-content">
        <div v-for="category in categorizedResults" :key="category.type" class="result-category">
          <div class="category-header">{{ category.label }}</div>
          <div
            v-for="result in category.items"
            :key="result.id"
            class="result-item"
            @click="handleResultClick(result, category.type)"
          >
            <div class="result-main">
              <span class="result-title">{{ result.title }}</span>
              <span class="result-meta">{{ result.meta }}</span>
            </div>
            <div class="result-subtitle">{{ result.subtitle }}</div>
          </div>
        </div>
      </div>

      <div v-else class="no-results">
        <el-icon><DocumentRemove /></el-icon>
        <span>未找到相关结果</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Search, Loading, DocumentRemove } from '@element-plus/icons-vue'
import { logApi } from '../api'

const router = useRouter()
const route = useRoute()

const searchQuery = ref('')
const loading = ref(false)
const results = ref<any[]>([])
const showResults = ref(false)
const searchInput = ref()

interface SearchResult {
  id: string
  title: string
  subtitle: string
  meta: string
  type: 'error' | 'page' | 'session'
  data: any
}

const categorizedResults = computed(() => {
  const categories = [
    { type: 'error', label: '错误', items: [] as SearchResult[] },
    { type: 'page', label: '页面', items: [] as SearchResult[] },
    { type: 'session', label: '会话', items: [] as SearchResult[] }
  ]

  results.value.forEach(result => {
    const category = categories.find(c => c.type === result.type)
    if (category) {
      category.items.push(result)
    }
  })

  return categories.filter(c => c.items.length > 0)
})

let searchTimeout: number

const handleSearch = () => {
  clearTimeout(searchTimeout)

  if (!searchQuery.value || searchQuery.value.length < 2) {
    results.value = []
    return
  }

  loading.value = true
  showResults.value = true

  searchTimeout = window.setTimeout(() => {
    performSearch()
  }, 300)
}

const performSearch = async () => {
  try {
    const query = searchQuery.value.toLowerCase()
    const searchResults: SearchResult[] = []

    // Search errors
    const errorResponse = await logApi.query({
      appId: route.query.appId as string || '',
      keyword: query,
      page: 1,
      pageSize: 5
    })

    errorResponse.data.data.forEach((item: any) => {
      searchResults.push({
        id: `error-${item.id}`,
        title: item.message?.substring(0, 60) || '无消息',
        subtitle: item.url || '未知来源',
        meta: `${item.level?.toUpperCase() || ''} • ${formatTime(item.created_at)}`,
        type: 'error',
        data: item
      })
    })

    // Search sessions (by URL or session ID)
    if (query.match(/^[a-f0-9-]{36}$/i) || query.length > 3) {
      try {
        const sessionResponse = await logApi.query({
          appId: route.query.appId as string || '',
          keyword: query,
          page: 1,
          pageSize: 3
        })

        // Group by session
        const sessionGroups = new Map<string, any[]>()
        sessionResponse.data.data.forEach((item: any) => {
          if (item.session_id) {
            if (!sessionGroups.has(item.session_id)) {
              sessionGroups.set(item.session_id, [])
            }
            sessionGroups.get(item.session_id)!.push(item)
          }
        })

        sessionGroups.forEach((events, sessionId) => {
          const firstEvent = events[0]
          searchResults.push({
            id: `session-${sessionId}`,
            title: `会话 ${sessionId.substring(0, 8)}...`,
            subtitle: firstEvent.url || '未知来源',
            meta: `${events.length} 个事件`,
            type: 'session',
            data: { sessionId, events }
          })
        })
      } catch (error) {
        console.error('Session search failed:', error)
      }
    }

    // Search pages (unique URLs from errors)
    const pages = new Map<string, { count: number; lastSeen: number }>()
    const pageResponse = await logApi.query({
      appId: route.query.appId as string || '',
      keyword: query,
      page: 1,
      pageSize: 20
    })

    pageResponse.data.data.forEach((item: any) => {
      if (item.url) {
        const url = new URL(item.url, 'https://example.com')
        const pagePath = url.pathname
        if (!pages.has(pagePath)) {
          pages.set(pagePath, { count: 0, lastSeen: item.created_at })
        }
        pages.get(pagePath)!.count++
        pages.get(pagePath)!.lastSeen = Math.max(pages.get(pagePath)!.lastSeen, item.created_at)
      }
    })

    pages.forEach((stats, path) => {
      if (path.toLowerCase().includes(query) || stats.count > 0) {
        searchResults.push({
          id: `page-${path}`,
          title: path,
          subtitle: '页面',
          meta: `${stats.count} 个错误`,
          type: 'page',
          data: { path, ...stats }
        })
      }
    })

    results.value = searchResults.slice(0, 10)
  } catch (error) {
    console.error('Search failed:', error)
    ElMessage.error('搜索失败')
  } finally {
    loading.value = false
  }
}

const handleResultClick = (result: SearchResult, type: string) => {
  showResults.value = false
  searchQuery.value = ''

  switch (type) {
    case 'error':
      router.push({
        path: '/logs',
        query: {
          appId: route.query.appId as string || '',
          keyword: result.data.message?.substring(0, 50) || ''
        }
      })
      break

    case 'session':
      router.push({
        path: '/logs',
        query: {
          appId: route.query.appId as string || '',
          keyword: result.data.sessionId
        }
      })
      break

    case 'page':
      router.push({
        path: '/logs',
        query: {
          appId: route.query.appId as string || '',
          keyword: result.data.path
        }
      })
      break
  }
}

const handleBlur = () => {
  // Delay hiding results to allow click events to fire
  setTimeout(() => {
    showResults.value = false
  }, 200)
}

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    showResults.value = false
    searchInput.value?.blur()
  }
}

const formatTime = (timestamp: number) => {
  const date = new Date(timestamp)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  const diffHours = Math.floor(diffMs / 3600000)
  const diffDays = Math.floor(diffMs / 86400000)

  if (diffMins < 1) return '刚刚'
  if (diffMins < 60) return `${diffMins}分钟前`
  if (diffHours < 24) return `${diffHours}小时前`
  if (diffDays < 7) return `${diffDays}天前`

  return date.toLocaleDateString('zh-CN')
}

const handleKeyboardShortcut = (event: KeyboardEvent) => {
  if ((event.ctrlKey || event.metaKey) && event.key === 'k') {
    event.preventDefault()
    searchInput.value?.focus()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeyboardShortcut)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeyboardShortcut)
  clearTimeout(searchTimeout)
})
</script>

<style scoped>
.global-search {
  position: relative;
  width: 100%;
  max-width: 500px;
}

.search-results {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  margin-top: 8px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  max-height: 400px;
  overflow-y: auto;
  z-index: 1000;
}

.loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 20px;
  color: var(--el-text-color-secondary);
}

.results-content {
  padding: 8px 0;
}

.result-category {
  margin-bottom: 8px;
}

.category-header {
  padding: 8px 16px;
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.result-item {
  padding: 12px 16px;
  cursor: pointer;
  transition: background 0.2s;
}

.result-item:hover {
  background: var(--el-fill-color-light);
}

.result-main {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.result-title {
  font-weight: 500;
  color: var(--el-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}

.result-subtitle {
  font-size: 13px;
  color: var(--el-text-color-regular);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.no-results {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 20px;
  color: var(--el-text-color-secondary);
}

:deep(.el-input__wrapper) {
  border-radius: 8px;
}

:deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--el-border-color) inset;
}

:deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px var(--el-color-primary) inset;
}

html.dark .search-results {
  background: #1a1d2d;
  border-color: #2d3748;
}

html:not(.dark) .search-results {
  background: #ffffff;
  border-color: #e5e7eb;
}
</style>

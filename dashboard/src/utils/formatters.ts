import dayjs from 'dayjs'

export const formatTime = (timestamp: number, format = 'YYYY-MM-DD HH:mm:ss'): string => {
  return dayjs(timestamp).format(format)
}

export const formatRelativeTime = (timestamp: number | undefined | null): string => {
  if (timestamp == null) return '-'
  const now = Date.now()
  const diff = now - timestamp

  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`
  if (diff < 2592000000) return `${Math.floor(diff / 86400000)} 天前`

  return formatTime(timestamp)
}

export const formatNumber = (num: number | undefined | null): string => {
  if (num == null) return '0'
  if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`
  if (num >= 1000) return `${(num / 1000).toFixed(1)}K`
  return num.toString()
}

export const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

export const truncateMessage = (msg: string, maxLength = 100): string => {
  if (!msg) return ''
  if (msg.length <= maxLength) return msg
  return msg.substring(0, maxLength) + '...'
}

export const parseJSON = <T = any>(jsonStr: string, defaultValue: T = {} as T): T => {
  try {
    return JSON.parse(jsonStr || '{}')
  } catch {
    return defaultValue
  }
}

export const getLevelColor = (level: string): string => {
  const colors: Record<string, string> = {
    error: '#F56C6C',
    warn: '#E6A23C',
    info: '#409EFF',
    debug: '#909399'
  }
  return colors[level] || colors.info
}

export const getLevelTag = (level: string): string => {
  const tags: Record<string, string> = {
    error: 'danger',
    warn: 'warning',
    info: 'primary',
    debug: 'info'
  }
  return tags[level] || tags.info
}

export const formatDuration = (ms: number): string => {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  if (ms < 3600000) return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`
  return `${Math.floor(ms / 3600000)}h ${Math.floor((ms % 3600000) / 60000)}m`
}

export const formatTimestamp = (timestamp: number): string => {
  return formatTime(timestamp)
}

export const formatStatus = (status: string): string => {
  const statusMap: Record<string, string> = {
    open: 'Open',
    resolved: 'Resolved',
    ignored: 'Ignored',
    muted: 'Muted'
  }
  return statusMap[status] || status
}

export const formatPriority = (priority: string): string => {
  const priorityMap: Record<string, string> = {
    critical: 'Critical',
    high: 'High',
    medium: 'Medium',
    low: 'Low'
  }
  return priorityMap[priority] || priority
}

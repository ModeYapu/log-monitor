import { describe, it, expect } from 'vitest'
import {
  formatTime,
  formatRelativeTime,
  formatNumber,
  formatBytes,
  truncateMessage,
  parseJSON,
  getLevelColor,
  getLevelTag,
  formatDuration
} from '../../src/utils/formatters'

describe('formatters', () => {
  describe('formatTime', () => {
    it('formats timestamp to default datetime string', () => {
      const ts = new Date('2025-01-15T10:30:00').getTime()
      const result = formatTime(ts)
      expect(result).toBe('2025-01-15 10:30:00')
    })

    it('supports custom format', () => {
      const ts = new Date('2025-01-15T10:30:00').getTime()
      expect(formatTime(ts, 'YYYY/MM/DD')).toBe('2025/01/15')
    })
  })

  describe('formatRelativeTime', () => {
    it('returns "刚刚" for < 60s', () => {
      expect(formatRelativeTime(Date.now() - 30000)).toBe('刚刚')
    })

    it('returns minutes ago', () => {
      expect(formatRelativeTime(Date.now() - 5 * 60000)).toBe('5 分钟前')
    })

    it('returns hours ago', () => {
      expect(formatRelativeTime(Date.now() - 3 * 3600000)).toBe('3 小时前')
    })

    it('returns days ago', () => {
      expect(formatRelativeTime(Date.now() - 5 * 86400000)).toBe('5 天前')
    })

    it('returns formatted date for > 30 days', () => {
      const result = formatRelativeTime(Date.now() - 40 * 86400000)
      expect(result).toMatch(/\d{4}-\d{2}-\d{2}/)
    })
  })

  describe('formatNumber', () => {
    it('returns number as-is for < 1000', () => {
      expect(formatNumber(500)).toBe('500')
    })

    it('formats thousands with K', () => {
      expect(formatNumber(1500)).toBe('1.5K')
    })

    it('formats millions with M', () => {
      expect(formatNumber(2500000)).toBe('2.5M')
    })
  })

  describe('formatBytes', () => {
    it('returns 0 B for zero', () => {
      expect(formatBytes(0)).toBe('0 B')
    })

    it('formats bytes', () => {
      expect(formatBytes(500)).toBe('500.0 B')
    })

    it('formats KB', () => {
      expect(formatBytes(1024)).toBe('1.0 KB')
    })

    it('formats MB', () => {
      expect(formatBytes(1048576)).toBe('1.0 MB')
    })

    it('formats GB', () => {
      expect(formatBytes(1073741824)).toBe('1.0 GB')
    })
  })

  describe('truncateMessage', () => {
    it('returns empty for empty string', () => {
      expect(truncateMessage('')).toBe('')
    })

    it('keeps short messages', () => {
      expect(truncateMessage('hello')).toBe('hello')
    })

    it('truncates long messages', () => {
      const long = 'a'.repeat(200)
      const result = truncateMessage(long)
      expect(result.length).toBe(103) // 100 + '...'
      expect(result.endsWith('...')).toBe(true)
    })

    it('respects custom maxLength', () => {
      expect(truncateMessage('hello world', 5)).toBe('hello...')
    })
  })

  describe('parseJSON', () => {
    it('parses valid JSON', () => {
      expect(parseJSON('{"a":1}')).toEqual({ a: 1 })
    })

    it('returns default for invalid JSON', () => {
      expect(parseJSON('not json')).toEqual({})
    })

    it('returns custom default', () => {
      expect(parseJSON('bad', [])).toEqual([])
    })

    it('returns default for empty string', () => {
      expect(parseJSON('')).toEqual({})
    })
  })

  describe('getLevelColor', () => {
    it('returns correct colors for known levels', () => {
      expect(getLevelColor('error')).toBe('#F56C6C')
      expect(getLevelColor('warn')).toBe('#E6A23C')
      expect(getLevelColor('info')).toBe('#409EFF')
      expect(getLevelColor('debug')).toBe('#909399')
    })

    it('returns info color for unknown level', () => {
      expect(getLevelColor('custom')).toBe('#409EFF')
    })
  })

  describe('getLevelTag', () => {
    it('returns correct tag types', () => {
      expect(getLevelTag('error')).toBe('danger')
      expect(getLevelTag('warn')).toBe('warning')
      expect(getLevelTag('info')).toBe('primary')
      expect(getLevelTag('debug')).toBe('info')
    })
  })

  describe('formatDuration', () => {
    it('formats milliseconds', () => {
      expect(formatDuration(500)).toBe('500ms')
    })

    it('formats seconds', () => {
      expect(formatDuration(3500)).toBe('3.5s')
    })

    it('formats minutes and seconds', () => {
      expect(formatDuration(125000)).toBe('2m 5s')
    })

    it('formats hours and minutes', () => {
      expect(formatDuration(3900000)).toBe('1h 5m')
    })
  })
})

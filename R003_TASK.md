# Round R003: Web Vitals 采集补齐 + 性能分析 API

## 背景
LogMonitor P5 性能监控增强。SDK 已有 longtask/INP/TTFB/DNS/TCP/DOM 采集，但缺 FCP/LCP/CLS。
后端已有 Event 模型的 performance 字段，但缺少专门的性能分析 API。

## 你的任务

### 1. SDK 补齐 Web Vitals (`sdk/src/index.ts`)
在 `setupEnhancedPerformance()` 函数中添加：
- **FCP** (First Contentful Paint): PerformanceObserver entryTypes=['paint']
- **LCP** (Largest Contentful Paint): PerformanceObserver entryTypes=['largest-contentful-paint']
- **CLS** (Cumulative Layout Shift): PerformanceObserver entryTypes=['layout-shift'], 过滤 hadRecentInput=false
- 存入 collectedPerformance 对象

### 2. 创建 PerformanceMetric 模型 (`collector/model/performance.go`)
```go
type PerformanceMetric struct {
    ID        int64   `json:"id"`
    ProjectID int64   `json:"project_id"`
    AppID     string  `json:"app_id"`
    PageURL   string  `json:"page_url"`
    MetricName string `json:"metric_name"` // fcp/lcp/cls/inp/ttfb
    Value     float64 `json:"value"`
    Rating    string  `json:"rating"` // good/needs-improvement/poor
    Release   string  `json:"release"`
    UserID    string  `json:"user_id"`
    SessionID string  `json:"session_id"`
    UA        string  `json:"ua"`
    CreatedAt int64   `json:"created_at"`
}

// Web Vitals rating thresholds
// FCP: good<=1800ms, needs-improvement<=3000ms, poor>3000ms
// LCP: good<=2500ms, needs-improvement<=4000ms, poor>4000ms
// CLS: good<=0.1, needs-improvement<=0.25, poor>0.25
// INP: good<=200ms, needs-improvement<=500ms, poor>500ms
// TTFB: good<=800ms, needs-improvement<=1800ms, poor>1800ms
```

### 3. 创建 Performance Store (`collector/storage/performance-store.go`)
- `InsertPerformanceMetric(metric *PerformanceMetric) error`
- `GetPerformanceSummary(projectID int64, metricName string, period string) ([]*PagePerformanceSummary, error)`
  - 按 page_url 聚合：p50/p75/p95/count
- `GetPerformanceTrend(projectID int64, pageURL string, metricName string, days int) ([]*DailyMetric, error)`
  - 按天聚合的 P75 趋势
- `GetPerformanceComparison(projectID int64, metricName string, releaseA, releaseB string) ([]*ReleaseComparison, error)`
- SQLite 自动建表 `performance_metrics`
- 为已有 performance type events 提供 migration 入口

### 4. 创建 Performance Store 测试 (`collector/storage/performance-store_test.go`)
- 测试 Insert + Summary
- 测试 Trend
- 测试 Comparison
- 测试 Rating 计算

### 5. 创建 Performance Handler (`collector/handler/performance_handler.go`)
- `GET /api/query/performance/summary` — 按 page 聚合 Web Vitals P75
  - 参数：project_id, metric_name(fcp/lcp/cls/inp/ttfb), period(1d/7d/30d)
- `GET /api/query/performance/trend` — 时间趋势
  - 参数：project_id, page_url, metric_name, days
- `GET /api/query/performance/compare` — 版本对比
  - 参数：project_id, metric_name, release_a, release_b

## 验收标准
- `cd /home/coder/log-monitor/collector && go build ./...` exit 0
- `cd /home/coder/log-monitor/collector && go test ./storage/... ./handler/...` exit 0
- SDK 新增 FCP/LCP/CLS 采集
- PerformanceMetric 模型完整
- 三个查询 API 都实现
- SQLite 自动建表

## 重要
- 在 collector/storage/interfaces.go 中添加 PerformanceStore 接口
- 在 collector/storage/sqlite_store.go 中添加 PerformanceMetrics() 方法
- 在 collector/routes.go 中注册新路由
- 在 collector/main.go 中添加 EnsurePerformanceMetricsTable()
- SDK 修改只需在 setupEnhancedPerformance() 函数中追加，不要改其他部分
- Rating 使用 Web Vitals 官方阈值

完成后更新 /home/coder/log-monitor/.dev-loop/round-state.json：
- phase 改为 "completed"
- filesChanged 填写实际修改的文件
- verifyBuildPassed / verifySlicePassed 改为 true

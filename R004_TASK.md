# Round R004: 白屏检测 + 接口失败追踪 + 资源监控 + 回归检测

## 背景
R003 已完成 Web Vitals (FCP/LCP/CLS) + 性能分析 API。现在增强 SDK 采集能力和后端分析。

## 你的任务

### 1. SDK 白屏检测 (`sdk/src/index.ts`)
在 `setupEnhancedPerformance()` 函数或 init 中添加：
- DOMContentLoaded 后 3 秒检查 body 是否有可见内容
- 判断条件：`document.body.innerText.trim().length < 1 && document.body.children.length < 1`
- 如果检测到白屏，发送 `type: 'performance'` 事件，extra 中包含 `{blankPage: true, url, timestamp}`
- 用 `setTimeout` 延迟检查

### 2. SDK 接口失败追踪 (`sdk/src/index.ts`)
添加 XHR/fetch 拦截器：
- 拦截 `fetch`：包装 `window.fetch`，捕获 response.status >= 400 或网络异常
- 拦截 `XMLHttpRequest`：包装 `open/send`，捕获 status >= 400 或 error/timeout 事件
- 失败请求上报 `type: 'xhr'` 事件（已有此类型），包含 url、method、status、duration
- 只追踪失败请求（成功请求忽略，避免数据量过大）

### 3. SDK 资源加载异常 (`sdk/src/index.ts`)
在 `setupEnhancedPerformance()` 中添加：
- `PerformanceObserver entryTypes=['resource']`
- 捕获 `transferSize === 0 && decodedBodySize === 0` 的资源（加载失败）
- 或 `duration > 10000` 的超慢资源
- 上报 `type: 'performance'` 事件，extra 包含 `{resourceError: true, resourceUrl, resourceType, duration}`

### 4. 后端回归检测 API
在 `collector/storage/performance-store.go` 中添加：
- `DetectPerformanceRegressions(projectID int64, currentRelease, previousRelease string) ([]*PerformanceRegression, error)`
  - 对比两个 release 的 P75 值
  - 如果当前 release 的 P75 比前一个 release 恶化超过 20%，标记为回归

在 `collector/model/performance.go` 中添加：
```go
type PerformanceRegression struct {
    MetricName  string  `json:"metric_name"`
    PageURL     string  `json:"page_url"`
    PreviousP75 float64 `json:"previous_p75"`
    CurrentP75  float64 `json:"current_p75"`
    Change      float64 `json:"change"` // percentage change
    Severity    string  `json:"severity"` // minor(20-50%) / major(50-100%) / critical(>100%)
}
```

在 `collector/handler/performance_handler.go` 中添加：
- `GET /api/query/performance/regressions` — 参数：project_id, current_release, previous_release

### 5. 测试
在 `collector/storage/performance-store_test.go` 中添加回归检测测试。

## 验收标准
- `cd /home/coder/log-monitor/collector && go build ./...` exit 0
- `cd /home/coder/log-monitor/collector && go test ./storage/... ./handler/...` exit 0
- SDK 白屏检测实现
- SDK 接口失败追踪实现（fetch + XHR 拦截）
- SDK 资源加载异常监控实现
- 后端回归检测 API 实现

## 重要
- SDK 修改在现有代码结构中追加，不要重构
- 接口拦截要保证原始功能不受影响
- 资源监控要注意性能，不要记录所有资源，只记录异常的
- 回归检测的 SQL 要高效（使用已有的 performance_metrics 表）

完成后更新 /home/coder/log-monitor/.dev-loop/round-state.json：
- phase 改为 "completed"
- filesChanged 填写实际修改的文件
- verifyBuildPassed / verifySlicePassed 改为 true

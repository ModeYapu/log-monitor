# LogMonitor — 前端日志监控系统 SDD v1.0

## 1. 项目概述

轻量级、自托管、生产可用的前端日志监控系统。包含 JS SDK（嵌入前端应用自动采集）、收集服务（Go，高性能低内存）、以及 Web Dashboard（Vue3，实时可视化）。

**目标**：单台 4核/8GB 服务器可稳定运行，日均处理 100 万条日志，存储 30 天数据。

---

## 2. 架构

```
┌─────────────┐    HTTPS/POST     ┌──────────────────┐
│  前端应用    │ ──── SDK ────────→ │  Collector       │
│  (浏览器)   │    /api/report     │  (Go, :9200)     │
└─────────────┘                    │    ↓             │
                                   │  SQLite WAL      │
┌─────────────┐    HTTP/GET        │    ↓             │
│  Dashboard  │ ←──────────────── │  Query API       │
│  (Vue3)     │    /api/query      └──────────────────┘
│  (:9201)    │                          ↓
└─────────────┘                    ┌──────────────────┐
                                   │  Alerter         │
                                   │  (飞书/邮件)      │
                                   └──────────────────┘
```

---

## 3. 模块清单

### 3.1 SDK (`@logmonitor/sdk`)

**文件**：`sdk/src/index.ts`（构建后输出 `sdk/dist/logmonitor.min.js`，UMD 格式，< 10KB gzipped）

**功能**：
- 自动捕获 `window.onerror`、`window.onunhandledrejection`
- 自动捕获 `console.error`（可选）
- 自动采集性能指标（FCP、LCP、CLS、FID）via `PerformanceObserver`
- 自动采集用户信息（UA、屏幕尺寸、URL、页面停留时间）
- 批量上报：缓冲区满 10 条 或 每 5 秒自动发送（`navigator.sendBeacon` 或 `fetch` fallback）
- 支持手动上报：`LogMonitor.info()`、`LogMonitor.warn()`、`LogMonitor.error()`、`LogMonitor.track()`
- 支持自定义标签（tags）和附加数据（extra）
- 页面关闭时 `beforeunload` 刷出剩余缓冲
- SDK 初始化：`LogMonitor.init({ dsn: 'https://host/api/report', appId: 'xxx', release: 'v1.0' })`

**上报数据格式**（单条）：
```json
{
  "appId": "my-app",
  "release": "v1.0.0",
  "type": "error|performance|info|warn|track",
  "timestamp": 1717000000000,
  "message": "Uncaught TypeError: ...",
  "stack": "TypeError: ...\n  at ...",
  "url": "https://example.com/page",
  "line": 42,
  "col": 15,
  "level": "error",
  "tags": { "module": "payment" },
  "extra": { "orderId": "12345" },
  "ua": "Mozilla/5.0 ...",
  "screen": "1920x1080",
  "viewport": "1440x900",
  "performance": { "fcp": 1200, "lcp": 2500, "cls": 0.1 }
}
```

**批量上报格式**（POST body）：
```json
{
  "appId": "my-app",
  "release": "v1.0.0",
  "events": [ ... ]
}
```

### 3.2 Collector（Go 服务）

**目录**：`collector/`

**技术栈**：Go 1.22+，标准库 + SQLite（`modernc.org/sqlite`，纯 Go 无 CGO），Gin HTTP 框架

**端点**：

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/report` | 接收 SDK 上报（批量） |
| GET | `/api/query/logs` | 查询日志（分页、过滤） |
| GET | `/api/query/stats` | 统计数据（错误趋势、Top 错误等） |
| GET | `/api/query/apps` | 应用列表 |
| GET | `/api/query/alerts` | 告警列表 |
| POST | `/api/query/alerts` | 创建告警规则 |
| DELETE | `/api/query/alerts/:id` | 删除告警规则 |
| GET | `/api/health` | 健康检查 |

**数据库 Schema**（SQLite）：

```sql
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL,
    release TEXT DEFAULT '',
    type TEXT NOT NULL,          -- error|performance|info|warn|track
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    stack TEXT DEFAULT '',
    url TEXT DEFAULT '',
    line INTEGER DEFAULT 0,
    col INTEGER DEFAULT 0,
    tags TEXT DEFAULT '{}',      -- JSON
    extra TEXT DEFAULT '{}',     -- JSON
    ua TEXT DEFAULT '',
    screen TEXT DEFAULT '',
    viewport TEXT DEFAULT '',
    performance TEXT DEFAULT '{}', -- JSON
    ip TEXT DEFAULT '',
    created_at INTEGER NOT NULL  -- unix ms
);

CREATE INDEX IF NOT EXISTS idx_events_app_created ON events(app_id, created_at);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(app_id, type, created_at);
CREATE INDEX IF NOT EXISTS idx_events_level ON events(app_id, level, created_at);

CREATE TABLE IF NOT EXISTS alert_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL,
    name TEXT NOT NULL,
    condition_type TEXT NOT NULL,  -- threshold|rate|new_error
    condition_config TEXT NOT NULL, -- JSON: {"level":"error","count":10,"windowMinutes":5}
    notify_type TEXT NOT NULL,      -- webhook|feishu|email
    notify_config TEXT NOT NULL,    -- JSON: {"url":"https://...","email":"..."}
    enabled INTEGER DEFAULT 1,
    last_triggered_at INTEGER DEFAULT 0,
    cooldown_minutes INTEGER DEFAULT 30,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS alert_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL,
    app_id TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
```

**性能要求**：
- 写入：使用 channel 缓冲区（容量 10000），异步批量写入 SQLite，每 2 秒或缓冲满 500 条 flush
- 查询：使用 SQLite WAL 模式，读写不互斥
- 内存：< 100MB（正常负载）
- 日志保留：可配置，默认 30 天自动清理（每天凌晨清理一次）

**告警检查**：
- 每分钟检查一次告警规则
- threshold 类型：在时间窗口内错误数超过阈值
- rate 类型：错误率（错误数/总请求数）超过阈值
- new_error 类型：出现之前未出现过的错误 message
- 告警触发后通过飞书 webhook 通知

**配置文件**（`collector/config.yaml`）：
```yaml
server:
  port: 9200
  cors: true

database:
  path: ./data/logmonitor.db
  retention_days: 30

buffer:
  size: 10000
  flush_interval_ms: 2000
  flush_batch_size: 500

alert:
  check_interval_ms: 60000
```

### 3.3 Dashboard（Vue3 Web）

**目录**：`dashboard/`

**技术栈**：Vue 3 + TypeScript + Vite + Element Plus + ECharts

**页面**：

1. **概览页** (`/`)
   - 应用列表卡片（错误数、警告数、最近活跃时间）
   - 24h 错误趋势折线图（ECharts）
   - Top 5 错误排行
   - 性能指标概览（P95 FCP、LCP、CLS）

2. **日志列表页** (`/logs/:appId`)
   - 表格展示：时间、级别、类型、消息摘要、URL、浏览器
   - 过滤器：级别（error/warn/info）、类型、时间范围、关键词搜索
   - 点击展开详情：完整堆栈、标签、额外数据、性能数据
   - 分页（每页 50 条）
   - 实时更新：可选 WebSocket 或 10 秒轮询

3. **性能页** (`/performance/:appId`)
   - FCP/LCP/CLS/FID 趋势图
   - 性能分数分布（饼图）
   - 慢页面 Top 10

4. **告警页** (`/alerts/:appId`)
   - 告警规则列表（CRUD）
   - 告警历史记录
   - 创建规则表单：条件类型、阈值、通知方式

5. **设置页** (`/settings`)
   - 应用管理（添加/删除 appId）
   - SDK 接入引导代码
   - 数据保留策略配置

**通用**：
- 左侧导航栏
- 深色主题（与现有项目风格一致）
- 响应式布局

---

## 4. 项目结构

```
log-monitor/
├── README.md
├── sdk/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── src/
│       └── index.ts
├── collector/
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── config.yaml
│   ├── handler/
│   │   ├── report.go
│   │   └── query.go
│   ├── model/
│   │   └── event.go
│   ├── storage/
│   │   ├── sqlite.go
│   │   └── migrations.go
│   ├── alerter/
│   │   ├── checker.go
│   │   └── notifier.go
│   └── buffer/
│       └── writer.go
├── dashboard/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   └── src/
│       ├── main.ts
│       ├── App.vue
│       ├── router.ts
│       ├── api/
│       │   └── index.ts
│       ├── views/
│       │   ├── Overview.vue
│       │   ├── Logs.vue
│       │   ├── Performance.vue
│       │   ├── Alerts.vue
│       │   └── Settings.vue
│       ├── components/
│       │   ├── Layout.vue
│       │   ├── LogTable.vue
│       │   ├── LogDetail.vue
│       │   ├── ErrorChart.vue
│       │   ├── PerfChart.vue
│       │   └── AlertForm.vue
│       └── styles/
│           └── global.css
└── deploy/
    ├── collector.service
    └── dashboard.conf
```

---

## 5. 开发顺序

### R1：Collector 核心 + SDK 最小可用
1. Collector：Go 项目初始化 + SQLite 初始化 + `/api/report` 端点 + 缓冲写入
2. Collector：`/api/query/logs` 查询端点（分页+过滤）
3. SDK：错误捕获 + 批量上报
4. 集成测试：SDK → Collector → 查询验证

### R2：Collector 查询完善 + Dashboard 骨架
1. Collector：`/api/query/stats` 统计端点
2. Collector：`/api/query/apps` 应用列表
3. Dashboard：项目初始化 + 路由 + 布局 + 概览页
4. Dashboard：日志列表页（表格+过滤+详情展开）

### R3：性能页 + 告警系统
1. Collector：告警检查器 + 飞书通知
2. Collector：`/api/query/alerts` CRUD
3. Dashboard：性能页
4. Dashboard：告警页

### R4：生产化
1. Dashboard：设置页 + SDK 接入引导
2. Collector：数据保留清理
3. systemd 服务 + nginx 配置
4. 压力测试 + 验证

---

## 6. 非功能需求

- **可用性**：SDK CDN 可加载，一行代码接入
- **性能**：Collector 单实例处理 1000 req/s，p99 < 50ms
- **存储**：SQLite WAL 模式，30 天数据 < 2GB
- **安全**：Dashboard 访问需 Basic Auth（nginx 层），API 支持可选 token 验证
- **稳定性**：Collector 异常退出不丢数据（WAL + 缓冲落盘）

---

## 7. 技术约束

- Collector 必须纯 Go（无 CGO），使用 `modernc.org/sqlite`
- SDK 构建后 UMD 格式，< 10KB gzipped，不依赖第三方库
- Dashboard 构建后纯静态文件，由 nginx 托管
- 全部在 `/home/coder/log-monitor` 目录下开发

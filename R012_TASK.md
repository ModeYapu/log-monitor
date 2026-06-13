# R012: 告警规则引擎 + 通知渠道扩展 + 异常聚类

## 项目上下文
- Go 模块: `github.com/logmonitor/collector`，位于 `/home/coder/log-monitor/collector`
- 前端: `/home/coder/log-monitor/dashboard/index.html`（Vue3 + Element Plus SPA，~3000 行）
- 已有：`collector/alerter/notifier.go`（飞书/企微/钉钉/Telegram/webhook/email），`collector/alerter/checker.go`（阈值/速率/新错误检查），`collector/handler/alerts.go`（告警 CRUD API），`collector/handler/clusters.go`（聚类查询），`collector/storage/alert-store.go`
- 数据库：SQLite，`collector/storage/sqlite.go` 中 `DB` 结构体
- 路由注册：`collector/routes.go`
- 主入口：`collector/main.go`

## 任务

### 1. 告警规则引擎 (`collector/alerter/rule_engine.go` 新建)

创建 RuleEngine 结构体，实现：

```go
type Rule struct {
    ID              int64
    Name            string
    AppID           string
    Condition       RuleCondition
    Severity        string // critical, warning, info
    CooldownMinutes int
    Channels        []string // notification channel IDs
    Enabled         bool
}

type RuleCondition struct {
    Type       string                 // threshold, trend, missing
    Metric     string                 // error_rate, page_load, event_count, etc.
    Operator   string                 // >, <, >=, <=, ==
    Value      float64
    WindowMin  int                    // 时间窗口（分钟）
    TrendCount int                    // 趋势规则：连续 N 次
    Page       string                 // 页面过滤
    TrendDir   string                 // up, down
}
```

功能：
- **阈值规则**: error_rate > 5%, page_load > 3s, event_count > N
- **趋势规则**: 连续 N 次上升/下降（在窗口内对数据点做线性比较）
- **缺失规则**: 某页面 N 分钟无事件
- 规则 CRUD API: 
  - `GET /api/alerts/rules?appId=xxx` — 列出规则
  - `POST /api/alerts/rules` — 创建规则
  - `PUT /api/alerts/rules/{id}` — 更新规则
  - `DELETE /api/alerts/rules/{id}` — 删除规则（已有，保留兼容）
- 规则评估：每次新事件批量到达时异步评估匹配规则（用 goroutine + channel）
- 冷却机制：同一规则在 cooldown 分钟内不重复触发（已有逻辑保留并增强）

### 2. 通知渠道扩展 (`collector/alerter/channel_manager.go` 新建)

创建 ChannelManager 管理通知渠道：

```go
type NotifyChannel struct {
    ID      string
    Type    string // feishu, webhook, email, wecom, dingtalk, telegram
    Name    string
    Config  map[string]interface{}
    Enabled bool
}

type ChannelManager struct {
    channels map[string]*NotifyChannel
    notifier *Notifier
}
```

功能：
- 飞书 webhook 通知（含富文本卡片消息）— 复用已有 `SendFeishu`，增加 severity-based 卡片颜色
- 通用 webhook（POST JSON）— 已有
- 邮件通知（SMTP）— 已有
- **通知模板系统**: 不同严重级别(critical/warning/info)使用不同模板（颜色、格式、图标）
  - critical: 红色卡片 + 🔴
  - warning: 橙色卡片 + 🟡
  - info: 蓝色卡片 + 🔵
- API:
  - `POST /api/alerts/channels` — 创建/更新通知渠道配置
  - `GET /api/alerts/channels` — 列出所有渠道
  - `POST /api/alerts/test` — 发送测试通知（已有，保留增强）
- `ChannelManager.Send(channels []string, severity, title, message string)` 方法

### 3. 异常聚类 (`collector/alerter/clusterer.go` 新建)

创建 Clusterer 对连续错误事件自动聚类：

```go
type Cluster struct {
    ID            string
    Fingerprint   string
    Message       string // 代表性消息
    Count         int
    FirstSeen     int64
    LastSeen      int64
    AppID         string
    Severity      string
    SimilarClusters []string // 相似聚类 ID
}

type Clusterer struct {
    db       storage.EventStore
    clusters map[string]*Cluster // fingerprint -> cluster
    mu       sync.RWMutex
}
```

功能：
- 对错误事件按 error message 相似度自动聚类（用 Levenshtein 或简单 tokenize + Jaccard）
- 聚类后生成"异常组"，包含计数、首次/末次时间、代表性消息
- 自动合并相似度 > 0.8 的组
- 提供 `ProcessEvents(events []EventRecord)` 方法处理新事件
- API:
  - `GET /api/alerts/clusters?appId=xxx&limit=20` — 当前活跃异常组
  - `GET /api/alerts/clusters/{fingerprint}/events` — 聚类内事件（已有 `clusters.go`，保留）
- `Clusterer.Run()` 定期清理过期聚类（30分钟无新事件）

### 4. Dashboard 增强 (`dashboard/index.html`)

在已有 SPA 中新增 Alerts 页面（侧边栏菜单项 "告警管理"），包含：
- **规则管理** Tab：
  - 规则列表表格（名称、类型、严重级别、状态、最后触发时间、操作）
  - 新建/编辑规则对话框：条件类型下拉选择（threshold/trend/missing），指标下拉（error_rate/page_load/event_count），操作符下拉（>/<，等），值输入框，时间窗口输入框，冷却时间，严重级别选择
  - 启用/禁用规则开关
- **通知渠道** Tab：
  - 渠道列表（飞书/Webhook/邮件/企微/钉钉/Telegram）
  - 添加渠道对话框
  - 测试发送按钮
- **异常聚类** Tab：
  - 活跃聚类列表（指纹、消息预览、计数、首末次时间、严重级别）
  - 点击聚类查看详情（事件列表）

## 实现要求

1. **新建文件**: `rule_engine.go`, `channel_manager.go`, `clusterer.go` 在 `collector/alerter/` 目录
2. **新建 handler**: `collector/handler/rule_engine.go` 处理新 API 路由
3. **新建测试**: 
   - `rule_engine_test.go` — 测试规则评估（阈值、趋势、缺失）
   - `clusterer_test.go` — 测试聚类和合并逻辑
   - `channel_manager_test.go` — 测试渠道管理
4. **路由注册**: 在 `routes.go` 中注册新路由
5. **保持兼容**: 不破坏已有 alerts handler 和 clusters handler

## 验证（必须全部通过）
```bash
cd /home/coder/log-monitor/collector && go build ./...
cd /home/coder/log-monitor/collector && go test ./...
cd /home/coder/log-monitor/collector && go vet ./...
```

## 约束
- 不引入新的外部依赖，只用标准库 + 已有依赖
- 所有新文件都要有测试
- 代码风格保持与现有一致（log/slog，encoding/json，net/http）
- Dashboard 修改在 index.html 内完成（Vue3 + Element Plus CDN）
- 不要修改已有文件的核心逻辑，只做增量扩展

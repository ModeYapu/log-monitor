# Co-browsing 实时协作浏览 SDD v1.0

## 1. 目标

在前端日志监控系统 LogMonitor 基础上，增加**实时协作浏览**功能：
- 管理员可**实时看到**用户浏览器页面
- 管理员可**远程控制**用户页面（点击、输入、滚动）
- 支持多人同时观看同一会话
- 所有会话可**录制回放**

## 2. 架构

```
┌─────────────┐  rrweb record   ┌──────────────────┐
│  用户浏览器  │ ──WebSocket──→ │  Collector       │
│  (被控端)    │ ←──控制指令──── │  (Go, :9200)     │
└─────────────┘                  │  WS Hub          │
                                 └──────┬───────────┘
┌─────────────┐  rrweb replay          │
│  管理员     │ ←──实时DOM流───────────┘
│  Dashboard  │ ──发送控制指令──→ Collector → 用户浏览器
│  (控制端)   │
└─────────────┘
```

### 数据流

1. **用户端**：rrweb `record()` 捕获 DOM 变更 → WebSocket 发送到 Collector
2. **Collector**：WebSocket Hub 分发：
   - 存储到 SQLite（录制）
   - 转发给所有订阅该会话的管理员（实时）
   - 接收管理员控制指令 → 转发给用户端
3. **管理员端**：rrweb `replay()` 实时渲染 → 管理员操作 → 发送控制指令

## 3. 模块设计

### 3.1 SDK 扩展 (`@logmonitor/sdk`)

**新增功能**：

```javascript
LogMonitor.init({
  dsn: 'https://host/api/report',
  appId: 'my-app',
  cobrowse: true  // 启用实时协作
})

// 手动开启/关闭协作
LogMonitor.cobrowse.start()   // 开始录制+连接WS
LogMonitor.cobrowse.stop()    // 停止
LogMonitor.cobrowse.status    // 'connected' | 'disconnected' | 'controlling'
```

**rrweb 录制配置**：
- `recordInlineStyles: true` — 内联样式
- `maskAllInputs: false` — 不遮挡输入框（管理员需要看到）
- `maskTextSelector: null` — 不遮挡文字
- `sampling: { mouseInteraction: true, mouseMovement: true, input: true, scroll: true }`
- `emit` 频率：每 100ms 或 DOM 变更时发送

**远程控制接收**：
- 监听 WebSocket 控制指令
- 指令类型：
  - `click(x, y)` — 模拟点击坐标
  - `input(selector, value)` — 填写输入框
  - `scroll(x, y)` — 滚动到指定位置
  - `keydown(key)` — 模拟按键
  - `navigate(url)` — 页面跳转（需用户确认）

**安全**：
- 远程控制默认关闭，需要用户点击"允许协助"按钮
- 管理员每次控制操作，用户端显示蓝色高亮提示
- 用户随时可断开（快捷键 ESC 或点击断开按钮）
- 敏感字段（密码框）自动 mask，不传输

### 3.2 Collector 扩展（Go）

**新增 WebSocket 端点**：

| 路径 | 功能 |
|------|------|
| `WS /ws/cobrowse/:sessionId` | 用户端连接（上传录制流 + 接收控制指令）|
| `WS /ws/cobrowse/:sessionId/view` | 管理员连接（接收实时流 + 发送控制指令）|

**WebSocket Hub**：
```go
type SessionHub struct {
    sessionId   string
    userConn    *websocket.Conn      // 用户端（唯一）
    viewerConns map[*websocket.Conn]bool // 管理员端（可多个）
    events      []RecordingEvent     // 录制缓冲
    mu          sync.RWMutex
}
```

**消息格式**（WebSocket JSON）：

用户→服务器（录制数据）：
```json
{"type": "rrweb-event", "data": [{...rrweb event...}]}
{"type": "rrweb-full-snapshot", "data": {...full snapshot...}}
```

管理员→服务器（控制指令）：
```json
{"type": "control", "action": "click", "x": 100, "y": 200}
{"type": "control", "action": "input", "selector": "#email", "value": "test@test.com"}
{"type": "control", "action": "scroll", "x": 0, "y": 500}
{"type": "control", "action": "keydown", "key": "Enter"}
```

服务器→用户（转发控制）：
```json
{"type": "control", ...}
```

服务器→管理员（转发录制）：
```json
{"type": "rrweb-event", "data": [...]}
```

**录制存储**：
- 实时流同时写入 SQLite `recordings` 表
- 每个会话一个记录，full snapshot + 增量 events

```sql
CREATE TABLE IF NOT EXISTS recordings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL UNIQUE,
    app_id TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    event_count INTEGER DEFAULT 0,
    full_snapshot TEXT DEFAULT '',   -- JSON: 初始完整快照
    url TEXT DEFAULT '',             -- 页面 URL
    ua TEXT DEFAULT '',
    status TEXT DEFAULT 'recording', -- recording|completed|error
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS recording_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    seq INTEGER NOT NULL,            -- 事件序号
    timestamp INTEGER NOT NULL,
    event_data TEXT NOT NULL,        -- JSON: rrweb event
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_recording_events_session ON recording_events(session_id, seq);
```

**REST 端点（新增）**：

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/query/recordings` | 录制列表 |
| GET | `/api/query/recordings/:sessionId` | 录制详情（含所有事件）|
| GET | `/api/query/recordings/:sessionId/events` | 按时间范围获取事件 |
| DELETE | `/api/query/recordings/:sessionId` | 删除录制 |
| GET | `/api/query/live-sessions` | 当前在线会话列表 |

### 3.3 Dashboard 扩展（Vue3）

**新增页面：**

#### 实时会话页 (`/live`)
- 左侧：当前在线用户列表（实时更新）
  - 每个显示：appId、页面URL、时长、UA摘要
  - 绿色圆点表示在线
- 右侧：选中用户的实时画面（rrweb replay 沙箱）
  - iframe 沙箱渲染，隔离样式
  - 鼠标操作转发为控制指令
  - 顶部工具栏：
    - 🖱️ 控制模式（开关）— 开启后鼠标操作会转发
    - 📸 截图 — 保存当前画面
    - 🔗 打开原页面
    - 📊 查看该用户日志
    - ⏏️ 断开

#### 录制回放页 (`/recordings`)
- 表格：时间、应用、页面、时长、事件数
- 点击进入回放：
  - rrweb-player 播放器
  - 进度条、播放/暂停、倍速（0.5x/1x/2x/4x）
  - 事件时间线（可跳转到任意时刻）
  - 左侧显示同步的 console 日志和错误

#### 会话详情页增强 (`/logs/:appId`)
- 每条错误日志旁显示 "🎬 查看回放" 按钮
- 点击跳转到对应时间点的录制回放

### 3.4 用户端协助提示组件

SDK 自带一个浮动小组件：
- 管理员连接时：显示 "技术支持已连接" + 蓝色脉冲动画
- 管理员操作时：操作位置显示蓝色高亮
- 断开按钮（红色）：用户可随时断开
- 控制模式提示："技术支持正在操作您的页面"

**样式**：底部右侧浮动，不遮挡页面内容，可拖动

## 4. 依赖

### SDK
- `rrweb` — 录制+回放核心（~40KB gzip）
- 原生 `WebSocket` — 通信

### Collector
- `github.com/gorilla/websocket` — Go WebSocket
- 已有 SQLite 存储

### Dashboard
- `rrweb-player` — 回放播放器组件
- 原生 `WebSocket` — 实时流

## 5. 安全设计

1. **用户知情同意**：cobrowse 功能需要显式调用 `start()`，不能静默开启
2. **密码遮挡**：`input[type=password]` 自动 mask
3. **权限分离**：
   - 用户端连接：只能发送录制数据，接收控制指令
   - 管理员连接：只能接收录制数据，发送控制指令
4. **Token 验证**：管理员连接 WebSocket 需带 token
5. **用户可断开**：随时断开，服务端不保留连接
6. **敏感内容过滤**：可配置 CSS 选择器过滤（如 `.credit-card` 区域）

## 6. 性能

- rrweb 录制平均每秒 2-5 个事件，每个 ~200 字节 → 带宽 < 1KB/s
- WebSocket 心跳 30 秒
- 录制存储：1 小时会话 ≈ 1-5MB
- 支持同时 50+ 在线会话录制

## 7. 开发顺序

### C1：Collector WebSocket Hub + 存储
1. WebSocket Hub 实现（用户连接、管理员连接、消息分发）
2. 录制存储（recordings + recording_events 表）
3. REST 端点（录制列表/详情/删除、在线会话列表）
4. 控制指令转发

### C2：SDK rrweb 集成 + 控制接收
1. rrweb record 集成
2. WebSocket 连接管理
3. 控制指令接收+执行
4. 用户端协助提示 UI

### C3：Dashboard 实时观看 + 回放
1. 实时会话列表页
2. 实时观看器（rrweb replay + 控制面板）
3. 录制回放页（rrweb-player）
4. 日志页关联回放

### C4：安全+优化+部署
1. Token 验证
2. 敏感内容过滤
3. 性能优化（事件压缩、按需加载）
4. 构建部署

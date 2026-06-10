# SDD: LogMonitor P2 平台化演进

## 目标
将 LogMonitor 从"单项目监控工具"演进为"多项目可扩展监控平台"。

## Slice 1: Issue 概念 — 事件聚合与生命周期管理

### 背景
当前 events 表存储原始事件，错误通过 fingerprint 聚类显示（ClusterDetail），但没有 Issue 概念。
用户无法"标记一个错误为已解决"、"追踪某个问题的状态"。需要引入 Issue 作为事件的聚合层。

### 需要实现

#### 后端 (Go)
1. **新增 issues 表** (migrations):
```sql
CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fingerprint TEXT NOT NULL,
    app_id TEXT NOT NULL,
    title TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'error',  -- error|performance|resource
    status TEXT NOT NULL DEFAULT 'open',  -- open|resolved|ignored|muted
    priority TEXT NOT NULL DEFAULT 'medium',  -- low|medium|high|critical
    assignee TEXT DEFAULT '',
    first_seen_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    event_count INTEGER NOT NULL DEFAULT 0,
    user_count INTEGER NOT NULL DEFAULT 0,
    resolved_at INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_issues_fingerprint ON issues(app_id, fingerprint);
CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(app_id, status, updated_at DESC);
```

2. **Issue 自动生成/更新逻辑**:
   - 事件上报时，按 fingerprint 查找或创建 Issue
   - 更新 last_seen_at、event_count、user_count
   - 如果 Issue 已 resolved 但又出现新事件 → 自动 reopen

3. **Issue CRUD API**:
   - `GET /api/query/issues` — 列表（分页、筛选: status/priority/app/assignee、排序: last_seen/event_count）
   - `GET /api/query/issues/:id` — 详情 + 最近 20 条关联事件
   - `PUT /api/query/issues/:id` — 更新 status/priority/assignee
   - `POST /api/query/issues/:id/resolve` — 标记已解决
   - `POST /api/query/issues/:id/ignore` — 忽略（不再 reopen）

4. **Issue 统计 API**:
   - `GET /api/query/issues/stats` — 按状态/优先级分组计数 + 趋势

#### 前端 (Vue3)
1. **新增 Issues 页面** (`dashboard/src/views/Issues.vue`):
   - Issue 列表表格：标题、状态标签(color-coded)、优先级、事件数、用户数、最后出现时间
   - 筛选：状态 / 优先级 / 应用 / 指派给
   - 排序：最后出现 / 事件数 / 优先级
   - 快捷操作：Resolve / Ignore / Reopen

2. **Issue 详情抽屉**:
   - 标题 + 状态 + 优先级 + 指派
   - 趋势图（24h 事件频率）
   - 关联事件列表（最近 20 条，可展开看 stack）
   - 受影响用户 Top 列表
   - 操作按钮：Resolve / Ignore / 设置优先级

3. **路由注册**: `/issues` → Issues.vue，侧边栏加 Issues 菜单项

4. **Overview 集成**:
   - 在异常工作台区域显示 "Open Issues" 数量
   - 点击跳转 Issues 页面

### 验收
- Issue 自动从事件生成（按 fingerprint）
- Issues 页面可列表、筛选、排序
- 可以 Resolve/Ignore Issue
- 已解决的 Issue 再次出现会自动 reopen
- Go 编译 + Vite 构建通过

## Slice 2: 多租户与权限

### 需要实现

#### 后端
1. **Projects 表**:
```sql
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    api_key TEXT NOT NULL UNIQUE,
    retention_days INTEGER DEFAULT 30,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
```

2. **Project Members 表**:
```sql
CREATE TABLE IF NOT EXISTS project_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL DEFAULT 'viewer',  -- owner|developer|viewer
    created_at INTEGER NOT NULL
);
UNIQUE(project_id, user_id)
```

3. **项目级数据隔离**: 所有查询 API 加 project_id 过滤（app_id 映射到 project）

4. **项目管理 API**:
   - CRUD /api/admin/projects
   - 成员管理 /api/admin/projects/:id/members
   - API Key 管理

#### 前端
1. **项目切换器**: 顶部下拉菜单，切换当前项目
2. **项目管理页**: Settings 下方，仅 admin 可见
3. **成员管理**: 项目内成员列表 + 邀请

### 验收
- 多项目数据隔离
- 权限控制：owner/developer/viewer 可见不同操作

## Slice 3: 后端职责拆分准备

### 需要实现（架构重构，不改功能）
1. **引入 Worker 概念**: 后台任务（清理、告警检测、Issue 聚合）从主进程提取为独立 goroutine 组
2. **Storage 接口化**: 将 sqlite.go 中的具体查询逻辑抽到 interface 后面，为后续支持 PostgreSQL 做准备
3. **配置热重载**: retention policy 变更不需要重启

### 验收
- 现有功能不变，但代码结构更清晰
- 编译通过，测试通过

## Slice 4: OpenAPI 与 Webhook

### 需要实现
1. **OpenAPI 文档**: 自动生成 Swagger/OpenAPI spec（用 swaggo/swag）
2. **Webhook 订阅**: Issue 创建/状态变更时触发 webhook
3. **API Token 认证**: 除了 JWT，支持 API Token 访问（用于 CI/CD 集成）

### 验收
- Swagger UI 可访问
- Webhook 可配置和测试

## 执行顺序
1. Slice 1: Issue 概念（最高价值，核心产品差异化）
2. Slice 2: 多租户与权限（平台化基础）
3. Slice 3: 后端拆分（架构健康度）
4. Slice 4: OpenAPI/Webhook（生态扩展）

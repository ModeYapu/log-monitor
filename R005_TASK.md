# Round R005: API 职责分离 + 健康检查增强 + 优雅关闭

## 背景
LogMonitor P6 后端职责拆分。当前 routes.go 把所有路由混在一起，需要结构化分离。
Worker 框架已存在，但健康检查缺少深度信息，优雅关闭逻辑需要增强。

## 你的任务

### 1. 路由职责分离 (`collector/routes.go`)
重构 SetupRoutes，将路由分为清晰的组：

```go
// Write API — 接收 SDK 数据
// 路径前缀: /api/report, /api/events
// 中间件: rate limit (100 req/s), API Key auth, audit log, CORS
writeGroup := []Route{
    {"/api/report", reportHandler},
    {"/api/events", reportHandler},
    {"/api/report/screenshot", screenshotHandler},
}

// Read API — 查询数据
// 路径前缀: /api/query/*
// 中间件: JWT auth, project context, CORS
readGroup := []Route{
    {"/api/query/logs", queryHandler},
    {"/api/query/analytics", analyticsHandler},
    // ...performance, alerts, etc
}

// Admin API — 管理操作
// 路径前缀: /api/admin/*
// 中间件: JWT auth + admin role check, audit log
adminGroup := []Route{
    {"/api/admin/users", usersHandler},
    {"/api/admin/projects", projectsHandler},
    // ...
}
```

每个 group 用统一的方式注册中间件链。可以创建一个 `RouteGroup` helper struct。

**重要**：不要改变已有的 URL 路径！只改内部组织方式。所有现有路由必须保持兼容。

### 2. 健康检查增强 (`collector/handler/health.go`)
当前的 Health handler 只返回简单的 status。增强为：

```go
type HealthResponse struct {
    Status    string            `json:"status"` // "healthy" | "degraded" | "unhealthy"
    Timestamp int64             `json:"timestamp"`
    Uptime    int64             `json:"uptime_seconds"`
    Version   string            `json:"version"`
    Components map[string]ComponentHealth `json:"components"`
    System    SystemHealth      `json:"system"`
}

type ComponentHealth struct {
    Status  string `json:"status"` // "ok" | "error"
    Message string `json:"message,omitempty"`
}

type SystemHealth struct {
    Goroutines    int   `json:"goroutines"`
    MemoryAllocMB float64 `json:"memory_alloc_mb"`
    DBSizeMB      float64 `json:"db_size_mb"`
    WorkerCount   int   `json:"worker_count"`
}
```

- 检查 SQLite 连接（执行 `SELECT 1`）
- 读取 DB 文件大小
- runtime.NumGoroutine()
- runtime memory stats
- worker manager 的 worker 数量

### 3. 健康检查测试 (`collector/handler/health_test.go`)
- 测试返回的 JSON 包含 status、uptime、components
- 测试 DB 失败时返回 "degraded"

### 4. 优雅关闭改进 (`collector/main.go`)
当前 main.go 有基本的 signal handling。增强为：

```go
shutdown sequence:
1. 收到 SIGTERM/SIGINT
2. 停止接收新 HTTP 请求（http.Server.Shutdown，10s timeout）
3. Flush 写缓冲区（buffer.Writer.Flush）
4. 停止所有 workers（worker.Manager.Stop）
5. 关闭数据库连接
6. 退出
```

在 http.Server 创建时传入 shutdown timeout，用 context.WithTimeout。

### 5. Worker 状态 (`collector/worker/worker.go`)
在 Manager 上添加：
- `Status() []WorkerStatus` — 返回每个 worker 的名称和运行状态
- `WorkerStatus` struct: Name, Running bool, LastRunAt int64

## 验收标准
- `cd /home/coder/log-monitor/collector && go build ./...` exit 0
- `cd /home/coder/log-monitor/collector && go test ./handler/... ./worker/...` exit 0
- 路由分为 write/read/admin 三个 group
- 健康检查返回详细组件状态
- 优雅关闭按顺序 drain

## 重要
- 不要改变任何 URL 路径
- 不要删除任何已有功能
- 保持所有已有测试通过
- 先读取现有的 routes.go、main.go、health.go 完整代码再修改
- RouteGroup helper 可以作为 routes.go 中的内部类型

完成后更新 /home/coder/log-monitor/.dev-loop/round-state.json：
- phase 改为 "completed"
- filesChanged 填写实际修改的文件
- verifyBuildPassed / verifySlicePassed 改为 true

# Round R002: 审计日志系统

## 背景
LogMonitor P4 多租户开发。R001 已完成项目级数据隔离和角色权限。
现在需要实现审计日志：记录谁在什么时候对哪个项目做了什么操作。

## 你的任务

### 1. 创建 AuditLog 模型 (`collector/model/audit.go`)
```go
type AuditLog struct {
    ID        int64  `json:"id"`
    ProjectID int64  `json:"project_id"`
    UserID    int64  `json:"user_id"`
    Username  string `json:"username"`
    Action    string `json:"action"`     // create/update/delete/login/export
    Resource  string `json:"resource"`   // project/user/alert/issue/sourcemap
    ResourceID string `json:"resource_id"`
    Detail    string `json:"detail"`
    IP        string `json:"ip"`
    UserAgent string `json:"user_agent"`
    CreatedAt int64  `json:"created_at"`
}
```

### 2. 创建 Audit Store (`collector/storage/audit-store.go`)
- `InsertAuditLog(log *AuditLog) error`
- `QueryAuditLogs(filter AuditFilter) ([]*AuditLog, int, error)` — 支持分页、按 project_id/user_id/action 过滤
- SQLite 自动建表 `audit_logs`
- 参考已有的 event-store.go 和 issue-store.go 的模式

### 3. 创建 Audit Store 测试 (`collector/storage/audit-store_test.go`)
- 测试 Insert + Query
- 测试分页
- 测试过滤

### 4. 创建 Audit 中间件 (`collector/middleware/audit.go`)
- 自动记录写操作（POST/PUT/DELETE/PATCH）
- 从 context 提取 user_id, project_id
- 从 request 提取 action (method), resource (URL path), IP, User-Agent
- 异步写入（不阻塞请求）

### 5. 创建 Audit Handler (`collector/handler/audit_handler.go`)
- GET /api/admin/audit-logs — 查询审计日志（支持分页和过滤）
- 仅 admin 角色可访问

## 验收标准
- `cd /home/coder/log-monitor/collector && go build ./...` exit 0
- `cd /home/coder/log-monitor/collector && go test ./storage/... ./middleware/...` exit 0
- AuditLog 模型完整
- AuditMiddleware 自动记录写操作
- 查询 API 支持分页和过滤
- SQLite 自动建表

## 重要
- 先读取现有的 storage/sqlite.go 了解建表模式
- 参考 storage/event-store.go 的分页和过滤实现
- 在 collector/storage/interfaces.go 中添加 AuditStore 接口
- 在 collector/api/ 中注册新路由（如果存在路由注册文件的话）

完成后更新 /home/coder/log-monitor/.dev-loop/round-state.json：
- phase 改为 "completed"
- filesChanged 填写实际修改的文件
- verifyBuildPassed / verifySlicePassed 改为 true

# LogMonitor 深度架构 Review

**审查日期**: 2026-06-11
**代码规模**: Go 16,983 行 / 14 packages / 65 Go files / Vue ~10,000 行 / 21 Vue components
**最近提交**: `42332e5` fix: 修复路由双斜杠 + project-store SQL 列数不匹配

---

## 📊 架构评分 (v2)

| 维度 | 评分 | 变化 | 说明 |
|------|------|------|------|
| **模块化** | 6/10 | +1 | sqlite.go 已拆分，但 handler 层仍偏大 |
| **类型安全** | 5/10 | +1 | 新增 AnalyticsFilters/QueryParams，但 analytics.go 仍有 map[string]interface{} |
| **可测试性** | 3/10 | +1 | 有测试脚手架但 storage 测试死锁 |
| **代码组织** | 6/10 | +1 | storage 拆分后清晰多了 |
| **依赖管理** | 7/10 | → | Go modules 管理良好，依赖少 |
| **错误处理** | 5/10 | +1 | 新增 errors 包 + middleware/error_handler |
| **运行时安全** | 5/10 | NEW | 有死锁风险、无 graceful DB close |
| **综合** | **5.3/10** | **+0.8** | |

---

## 🔴 Critical (必须修)

### 1. 死锁：InsertEvents → CreateOrUpdateIssues 递归锁
**文件**: `storage/event-store.go:62` → `storage/issue-store.go:32`

```
InsertEvents() {
    db.mu.Lock()          // 获取写锁
    ... insert events ...
    db.CreateOrUpdateIssues(events)  // 又尝试 db.mu.Lock() → 💀 DEADLOCK
}
```

**影响**: 每次 event 插入时 cleanupOldData 恰好触发 → 系统完全卡死（测试已验证 30s timeout）

**修复方案**:
- 方案A: 将 `CreateOrUpdateIssues` 拆成不加锁的内部版本 `createOrUpdateIssuesInternal`（锁由调用方管理）
- 方案B: 将 `CreateOrUpdateIssues` 移到 `InsertEvents` 的锁外面，events 插入完成后释放锁再调

### 2. 互斥锁过度使用 — 单 RWMutex 管全局
**文件**: `storage/sqlite.go` 的 `sync.RWMutex mu`

整个 storage 层共用一把读写锁，所有读写操作都串行化：
- 37 处 Lock/RLock 调用
- 写操作（插入事件）阻塞所有读操作（查询）
- cleanupOldData 持锁时间可能很长（大批量 DELETE）

**影响**: 高并发下读写互相阻塞，无法达到"日均100万条"的设计目标

**修复方案**:
- 使用 SQLite WAL 模式 + 合理的事务隔离，依赖 SQLite 自身的并发控制
- 或改用 `sync.RWMutex` 按业务域拆分（events/issues/projects 各一把锁）
- 长时间批量操作（cleanup、analytics）用独立连接，不持有主锁

### 3. 数据库无 graceful close
**文件**: `storage/sqlite.go`

`Close()` 方法存在但 `main.go` 的 shutdown 流程中没有调用 `db.Close()`：
- HTTP server 优雅关闭了，但数据库连接直接丢弃
- `stopCh` channel 没有被关闭，cleanup goroutine 泄漏
- WAL 模式下不正确关闭可能导致数据损坏

---

## 🟠 High (应该修)

### 4. handler 层仍偏大
**文件**: `handler/cobrowse.go` (921行), `handler/query.go` (782行), `handler/projects.go` (608行)

cobrowse.go 混合了 WebSocket hub 管理、房间管理、权限校验、消息转发。一个文件承担了 3-4 个职责。

建议：
- `cobrowse.go` → `cobrowse_hub.go` + `cobrowse_room.go` + `cobrowse_handler.go`
- `query.go` → `query_parser.go` + `query_builder.go`

### 5. analytics.go 仍用 map[string]interface{} 返回
**文件**: `storage/analytics.go` (996行)

虽然新增了 `AnalyticsFilters` struct 做输入，输出仍大量使用 `map[string]interface{}`：
- 返回值无类型约束，前端字段依赖 string key
- 无法做编译时检查

建议：为每个 analytics 查询定义具体的返回 struct

### 6. main.go 488行 — 路由注册过于集中
所有路由（~50条）在 main.go 以 slice 方式注册，handler 依赖注入也在 main.go 里初始化。

建议：
- 路由注册移到 `handler/routes.go` 或 `router/router.go`
- handler 初始化用 wire 或简单工厂函数
- main.go 只做：parse config → init DB → init router → start server

### 7. Config 热重载无 debounce
**文件**: `config/watcher.go` (241行)

fsnotify 事件可能短时间内触发多次（编辑器保存、文件系统特性），当前直接 reload 可能导致：
- 配置不一致（两次 reload 之间部分生效）
- 资源泄漏（DB 连接等未正确关闭再创建）

建议加 500ms debounce

---

## 🟡 Medium (建议修)

### 8. 测试覆盖严重不足
只有 3 个测试文件（handler/health_test.go, storage/event-store_test.go, storage/issue-store_test.go），而且 storage 测试因死锁无法运行。

优先需要测试的包：
1. `storage/` — 核心数据层，SQL 逻辑必须测试
2. `handler/` — API 层，HTTP handler 正确性
3. `buffer/` — 批量写入，边界条件

### 9. Migration 管理粗糙
单文件 `migrations/001_init.sql` 包含所有表结构，后续 ALTER TABLE 通过直接 `db.Exec` 执行。

建议：
- 使用 golang-migrate 或 goose 等迁移工具
- 每次表结构变更一个 migration 文件
- 支持 up/down 回滚

### 10. 日志不统一
混用 `log/slog` 和 `fmt.Printf`，无结构化日志标准：
- 有些用 `slog.Info(msg, "key", value)` 
- 有些用 `fmt.Printf("[cleanup] ...")`
- 有些用 `log.Printf("[cleanup] ...")`

建议统一使用 `slog`，消除 `fmt.Printf` 和 `log.Printf`

### 11. 默认密码日志泄露
`main.go` 的 `seedAdminUser` 函数将默认密码打印到日志：
```go
slog.Info("Creating default admin user", "username", "admin", "password", password)
```
虽然标注了 "please change"，但日志可能被监控系统采集。建议只提示 "default credentials in config"。

---

## 🔵 Info (可选优化)

### 12. Webhook delivery 无持久化重试
`webhook/delivery.go` 的重试只在内存中（`attempts map[int64]int`），进程重启后丢失。

### 13. 缺少 OpenAPI spec
日志显示 `Failed to read OpenAPI spec: open api/openapi.yaml: no such file or directory`，handler/openapi.go (134行) 实现了 spec 服务但 spec 文件不存在。

### 14. 前端 dist 提交到 git
`dashboard/dist_new/` 的编译产物提交到了仓库，增加仓库体积。应加入 .gitignore，通过 CI/CD 构建。

---

## 🎯 修复路线图

### Phase 1: 紧急修复 (1-2天)
1. **修死锁**: InsertEvents 中 CreateOrUpdateIssues 改为内部方法，不重复加锁
2. **修 DB close**: main.go shutdown 流程加 `db.Close()`
3. **清理构建产物**: dist_new 加入 .gitignore

### Phase 2: 架构优化 (3-5天)
4. **拆分锁**: 按 domain 拆分 RWMutex，或改用 SQLite WAL + 事务控制
5. **拆分 handler**: cobrowse.go 和 query.go 拆文件
6. **路由提取**: main.go → routes.go
7. **统一日志**: 消除 fmt.Printf/log.Printf

### Phase 3: 质量保障 (1周)
8. **补测试**: storage 核心方法 80%+ 覆盖
9. **Migration 工具化**: 引入 goose/golang-migrate
10. **Analytics 类型化**: 定义返回 struct

### Phase 4: 功能增强
11. Webhook 持久化重试
12. OpenAPI spec 自动生成
13. Dashboard CI/CD 构建

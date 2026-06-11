# LogMonitor Architecture Code Review

**审查日期**: 2026-06-10
**代码规模**: Go 16,335 行 / 14 packages / Vue ~10,000 行

---

## 🔴 Critical (3)

### 1. storage/sqlite.go — 3242 行 God Object
60 个方法全部挂在 `*DB` 上，实现了 11 个 interface。
问题：
- 单个文件维护困难
- 任何表结构变更都可能影响整个文件
- 测试需要初始化整个 DB

建议：按 interface 拆分为独立文件 (event-store.go, issue-store.go, project-store.go, etc.)

### 2. 214 个 interface{} — 类型安全缺失
主要分布：
- storage/analytics.go: 大量 map[string]interface{} 返回值
- handler/query.go: 动态查询参数
- model 包：部分模型字段用 interface{}

建议：定义具体的 struct 替代 map[string]interface{}

### 3. 14 个包零测试
整个项目无任何测试文件。核心包 (storage, handler, alerter) 无测试覆盖。

---

## 🟠 High (3)

### 4. handler/cobrowse.go — 921 行
处理协同浏览功能，混合了 WebSocket 管理、房间管理、权限校验、消息转发。
建议拆分为 cobrowse_hub.go + cobrowse_room.go + cobrowse_handler.go

### 5. handler/query.go — 788 行
复杂查询处理，混合了参数解析、SQL 构建、结果格式化。
建议拆分为 query_parser.go + query_builder.go + query_executor.go

### 6. 缺少统一错误处理
300 处 log/slog 调用，无统一错误码或错误类型。
handler 层错误处理不统一，有的返回 JSON，有的直接 slog。

---

## 🟡 Medium (3)

### 7. storage/analytics.go — 1346 行
分析查询逻辑复杂，混合了多个维度的统计查询。
建议按维度拆分。

### 8. Config 热重载 (watcher.go 241行)
fsnotify 文件监听，无防抖 (debounce)，可能触发多次重载。

### 9. migrations/ 只有 1 个文件
所有表结构在一个 migration 文件里，应该按版本分文件。

---

## 📊 架构评分

| 维度 | 评分 | 说明 |
|------|------|------|
| **模块化** | 5/10 | 有 interface 但实现全在 sqlite.go |
| **类型安全** | 4/10 | 214 个 interface{} |
| **可测试性** | 2/10 | 零测试 |
| **代码组织** | 5/10 | handler 拆分了但每个偏大 |
| **依赖管理** | 7/10 | Go modules 管理好 |
| **错误处理** | 4/10 | 无统一错误码 |
| **综合** | **4.5/10** |

## 🎯 修复优先级

P1: 拆分 sqlite.go + 补核心测试
P2: 减少 interface{} + 统一错误处理
P3: 拆分大 handler 文件

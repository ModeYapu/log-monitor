# Round R009: 架构修复 — handler 层接口解耦 + Code Review 修复

## 背景
Code review 发现 handler 层 18 个文件直接依赖 *storage.DB 具体类型。
storage/interfaces.go 已定义 12 个 interface，但 handler 没有使用它们。
此外 handler/projects.go:583 有 JWT context extraction 的 TODO 未实现。

## 你的任务

### 1. 为 handler 层定义 Service 接口 (`handler/service.go`)
不要改已有的接口定义，而是定义一个组合了所有 handler 需要的 storage 方法的接口：

```go
package handler

// DashboardService 定义 dashboard handler 需要的存储能力
type DashboardService interface {
    storage.EventStore
    storage.IssueStore
    storage.AuditStore
    storage.PerformanceStore
    storage.AnalyticsStore
    storage.SystemStore
}
```

### 2. dashboard handler 改为接口依赖
修改 `handler/projects.go` 中 `projectsHandler` 的 JWT context extraction TODO（line 583）：
- 从 request context 中提取 JWT claims 中的 user_id
- 如果没有 JWT 中间件，从 API Key 中间件的 context 中获取
- 不要留 TODO

### 3. 保持向后兼容
- 不要改构造函数签名（NewXxxHandler 保持 *storage.DB 参数）
- *storage.DB 自然满足所有 handler 接口（鸭子类型）
- 只在新增 handler 时使用接口类型

### 4. 补充 alerter 和 worker 的测试
当前 `alerter` 和 `worker` 包没有测试文件。为 `worker/worker.go` 添加基础测试：
- 测试 Manager.RegisterWorker
- 测试 Manager.Status()
- 测试 Manager.Stop()

### 5. go vet 修复
修复 `go vet ./buffer/...` 的剩余警告（如果有）。

## 验收标准
- `go build ./...` exit 0
- `go test ./...` exit 0 — 全量测试
- `go vet ./...` exit 0 — 无警告
- handler/projects.go 中无 TODO

## 重要
- 这是一个重构轮，不要添加新功能
- 保持所有现有测试通过
- 小步修改，每步验证
- 如果某个改动会导致大量文件修改，跳过它（保持稳定优先）

完成后更新 /home/coder/log-monitor/.dev-loop/round-state.json：
- phase 改为 "completed"
- filesChanged 填写实际修改的文件
- verifyBuildPassed / verifySlicePassed 改为 true

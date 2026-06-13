# Round R006: OpenAPI 文档 + E2E 联动 + 事件类型细化

## 背景
P6 后端职责拆分已完成。现在做 ROADMAP 中的扩展生态项，让 LogMonitor 可以与 E2E Verifier 联动。

## 你的任务

### 1. 完善 OpenAPI 文档 (`collector/handler/openapi.go`)
先读取现有的 openapi.go 了解当前 spec 结构。然后：
- 为所有 `/api/query/*` 端点添加 OpenAPI 3.0 文档
- 为所有 `/api/admin/*` 端点添加文档
- 包含 P4/P5 新增的端点：audit-logs, performance/*, regressions
- 包含请求参数、响应格式的 schema 定义
- 生成一个完整的 JSON spec

### 2. E2E Verifier Webhook (`collector/webhook/e2e_verifier.go`)
创建一个专用的 webhook handler：
```go
// E2EVerifierHook 处理 E2E Verifier 的回调
type E2EVerifierHook struct {
    db *storage.DB
}

// HandleVerificationResult 接收 E2E 验证结果
// POST /api/webhooks/e2e-verifier
// Body: { "site": "travel-planner", "status": "pass|fail", "score": 8.2, 
//          "checks": [...], "release": "v1.2.3", "timestamp": ... }
// 功能：将验证结果存入 DB，关联到 release，验证失败时触发告警
```

在 storage 中创建 `verification_results` 表：
- id, project_id, site, release, status, score, checks_json, created_at

在 routes.go 中注册 `/api/webhooks/e2e-verifier` 路由（public，用 HMAC 验证或简单 API Key）。

### 3. 事件类型细化 (`collector/model/event.go`)
在 Event 结构中明确类型常量：
```go
const (
    EventTypeError       = "error"
    EventTypePerformance = "performance"
    EventTypeResource    = "resource"      // 资源加载异常
    EventTypeAPIError    = "api_error"     // 接口失败
    EventTypeUserAction  = "user_action"   // 用户行为
    EventTypeInfo        = "info"
    EventTypeWarn        = "warn"
    EventTypeTrack       = "track"
    EventTypeConsole     = "console"
    EventTypeXHR         = "xhr"
    EventBreadcrumb      = "breadcrumb"
)
```

### 4. Report Handler 增强 (`collector/handler/report.go`)
在事件处理时根据 type 分类：
- `resource` → 提取资源 URL、类型、失败原因，存入 resource_errors 表（或用 performance_metrics）
- `api_error` → 提取 API URL、status code、duration，用于接口失败分析
- `user_action` → 提取 action name、target、用于用户行为分析

### 5. Webhook Handler 更新 (`collector/handler/webhooks.go`)
注册 e2e-verifier webhook 端点。

## 验收标准
- `cd /home/coder/log-monitor/collector && go build ./...` exit 0
- `cd /home/coder/log-monitor/collector && go test ./handler/...` exit 0
- OpenAPI spec 包含所有端点
- E2E Verifier webhook 可以接收和存储验证结果
- Event Type 有明确的常量定义
- Report handler 能根据 type 分类处理

## 重要
- 先读取现有代码再修改
- OpenAPI 文档可以是用 Go string 生成的 JSON，不需要第三方库
- E2E Verifier webhook 用简单的 API Key 验证即可
- 不要引入新的外部依赖

完成后更新 /home/coder/log-monitor/.dev-loop/round-state.json：
- phase 改为 "completed"
- filesChanged 填写实际修改的文件
- verifyBuildPassed / verifySlicePassed 改为 true

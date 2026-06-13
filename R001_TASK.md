# Round R001: 项目级数据隔离 + 角色权限增强

## 背景
LogMonitor 已有 Project 模型、API Key 中间件、JWT 认证、User 模型、ProjectMember 模型。
现在需要实现真正的**项目级数据隔离**和**三级角色权限控制**。

## 你的任务

### 1. 增强 ProjectContext 中间件 (`collector/middleware/project.go`)
- 从请求中提取 project_id（通过 API Key 查询或 JWT claims）
- 将 project_id 注入 request context
- 将用户角色（owner/developer/viewer）也注入 context

### 2. 创建 Authorization 中间件 (`collector/middleware/authorization.go`)
- 三级角色：owner（全权限）、developer（GET+POST+PUT）、viewer（GET only）
- 检查 request method + 用户角色 → 允许或拒绝
- 管理员（admin role）绕过项目级限制

### 3. 创建 Authorization 测试 (`collector/middleware/authorization_test.go`)
- 测试 viewer 对 GET/POST/PUT/DELETE 的权限
- 测试 developer 对 GET/POST/PUT/DELETE 的权限
- 测试 owner 全权限
- 测试 admin 绕过

### 4. 修改查询处理器 (`collector/handler/query_handler.go` + `query_logs.go`)
- 从 context 获取 project_id
- 所有数据库查询添加 WHERE project_id = ? 条件

## 验收标准
- `cd /home/coder/log-monitor/collector && go build ./...` exit 0
- `cd /home/coder/log-monitor/collector && go test ./middleware/... ./handler/...` exit 0
- ProjectContext 中间件从 API Key 或 JWT 提取 project_id 注入 context
- Authorization 中间件检查角色：viewer 只能 GET，developer 可 POST/PUT，owner 全权限
- query_handler 和 query_logs 中的查询都按 project_id 过滤

## 重要
- 先读取现有的 middleware/auth.go, middleware/api_token.go, middleware/project.go, storage/types.go 了解已有代码
- 在已有代码基础上增强，不要重写
- 保持向后兼容
- 测试必须全绿

完成后更新 /home/coder/log-monitor/.dev-loop/round-state.json：
- phase 改为 "completed"
- filesChanged 填写实际修改的文件
- verifyBuildPassed / verifySlicePassed 改为 true

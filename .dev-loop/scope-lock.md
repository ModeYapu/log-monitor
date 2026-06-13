# Scope Lock — LogMonitor P4: 多租户与权限

## Original Requirement
实现 LogMonitor 的多租户与权限系统，支持多项目隔离、角色管理、API Token、审计日志。

## In Scope
- [ ] 项目(Project)数据模型 + CRUD API
- [ ] API Key / Token 认证中间件（替代当前的硬编码认证）
- [ ] 角色：admin / developer / readonly
- [ ] 项目级数据隔离（日志、告警、录屏、issue 按项目过滤）
- [ ] 审计日志（谁在什么时候做了什么操作）

## Explicitly Out of Scope
- P5 性能监控增强（Web Vitals 等）
- P6 后端职责拆分
- 前端用户管理 UI（先做后端 API）
- OAuth / SSO 外部认证集成
- 多数据库支持

## Locked At: 2026-06-13T14:01:00+08:00
## Locked By: Round R001

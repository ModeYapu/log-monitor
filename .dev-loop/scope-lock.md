# Scope Lock — LogMonitor P6: 后端职责拆分

## Original Requirement
将 Collector 的多职责拆分为独立模块：采集写入、查询聚合、回放存储、异步任务，提升扩展性和稳定性。

## In Scope
- [ ] 事件写入与查询分离（Write API vs Read API）
- [ ] 异步任务框架（告警检查、数据清理、聚合计算统一调度）
- [ ] 回放/录屏存储独立模块化
- [ ] 健康检查与优雅关闭增强

## Explicitly Out of Scope
- 微服务化（保持单进程，模块化拆分即可）
- 多实例部署
- 前端 Dashboard UI 变更

## Locked At: 2026-06-13T14:42:00+08:00
## Locked By: Round R005

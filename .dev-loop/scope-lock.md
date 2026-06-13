# Scope Lock — LogMonitor P5+: 增强性能监控

## Original Requirement
在 Web Vitals 基础上增加：白屏检测、长任务监控、接口失败追踪、资源加载异常监控、页面性能趋势图+回归提示

## In Scope
- [ ] 白屏检测（blank page detection）— SDK 端检测 + 后端记录
- [ ] 接口失败追踪（API failure tracking）— 捕获 XHR/fetch 错误
- [ ] 资源加载异常监控 — 捕获资源加载失败事件
- [ ] 性能回归检测 — 版本对比自动检测性能退化

## Explicitly Out of Scope
- 前端 Dashboard UI
- P6 后端职责拆分
- 前端用户管理 UI

## Locked At: 2026-06-13T14:35:00+08:00
## Locked By: Round R004

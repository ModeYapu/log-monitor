# Scope Lock — LogMonitor P5: 性能监控增强

## Original Requirement
实现 LogMonitor 的 Web Vitals 性能监控增强，包括 Core Web Vitals 指标采集、白屏检测、长任务监控、接口失败追踪、资源加载异常、页面性能趋势图和版本对比。

## In Scope
- [ ] Web Vitals 指标采集（FCP/LCP/CLS/INP/TTFB）
- [ ] Performance Event 模型和存储
- [ ] 性能数据查询 API（聚合+趋势+对比）
- [ ] SDK 采集 Web Vitals 数据
- [ ] 性能 Dashboard 后端 API（Top pages/performance trends/version comparison）

## Explicitly Out of Scope
- 前端 Dashboard UI（先做后端 API + SDK）
- 多浏览器兼容性测试
- P6 后端职责拆分
- AI 自动分析

## Locked At: 2026-06-13T14:24:00+08:00
## Locked By: Round R003

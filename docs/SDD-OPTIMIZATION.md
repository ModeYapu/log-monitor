# SDD: LogMonitor 项目优化

## 优化清单

### 1. 🔴 前端 UI/UX 优化
- **Overview 页面**: 24h 错误趋势图用 ECharts（已用），但 Top 错误只显示 message 没有 stack trace 预览
- **Logs 页面**: 没有日志详情抽屉（点击某条日志应展开显示 stack/extra/tags）
- **Performance 页面**: 图表没问题，但缺少指标评级说明（什么是"好"什么是"差"）
- **Recordings 页面**: 回放功能已有，但列表页缺少搜索/筛选
- **Settings 页面**: 只有 SDK 接入引导，缺少实际系统设置（数据保留天数、JWT 密钥修改、系统信息）
- **全局**: 没有 404 页面、没有 loading 骨架屏、错误提示用 ElMessage 不够优雅
- **全局**: 侧边栏没有折叠功能
- **全局**: 没有暗黑模式

### 2. 🔴 后端功能补全
- **JWT Secret**: 当前配置为空，应该自动生成或从配置读取
- **数据清理**: 配置了 retention_days: 30 但没有实现定时清理逻辑
- **性能指标 API**: Performance 页面用的 API 可能返回空数据，需要从 events 表的 performance 字段聚合
- **Alert 系统**: alert_rules 表为空，告警管理页面可能不完整
- **导出功能**: 没有 CSV/JSON 导出 API

### 3. 🟡 前端质量
- **TypeScript**: vue-tsc 编译有错误（之前跳过了），需要修复
- **组件拆分**: 大文件（Recordings.vue 599行）需要拆分
- **API 层**: 错误处理统一化（token 过期自动跳转 login）
- **响应式**: 移动端适配

### 4. 🟡 后端质量
- **数据库**: 111MB，recording_events 77906 条，需要定期清理策略
- **索引优化**: 已有基础索引，可以优化查询性能
- **日志**: 结构化日志（当前用 fmt.Println）
- **配置验证**: 启动时验证必填配置

### 5. 🟢 安全加固
- **CORS**: 当前 `cors: false` 但代码里有 corsMiddleware
- **Rate Limiting**: 登录接口没有限流
- **HTTPS**: 通过 nginx 代理，但 API 直接暴露在 :9200

## 执行计划

### Phase 1: 核心功能补全（优先级最高）
1. 实现数据保留清理定时任务（retention_days）
2. Logs 页面增加详情抽屉（stack/extra/tags）
3. Recordings 列表增加搜索/筛选
4. Settings 页面增加实际系统设置
5. 修复 vue-tsc 编译错误

### Phase 2: UI/UX 提升
6. 侧边栏折叠功能
7. 404 页面
8. API 层统一错误处理（401 自动跳转）
9. Performance 指标评级说明

### Phase 3: 质量 & 安全
10. 结构化日志（slog 替代 fmt.Println）
11. 登录限流
12. 组件拆分

## 使用 Claude Code 执行
按 Phase 顺序让 Claude Code 实现，每个 Phase 独立验证。

# SDD: LogMonitor P1 产品化增强

## 目标
将 LogMonitor 从"团队工具"提升为"可持续运营的监控平台"。聚焦 4 个方向，按优先级排序。

## Slice 1: 性能监控增强 — Web Vitals 仪表盘升级

### 当前状态
- Performance.vue 已有 FCP/LCP/CLS 基础卡片和趋势图
- 但缺少 INP 和 TTFB 指标
- 指标评级说明不直观
- 缺少页面级性能对比和"最近变差"回归提示

### 需要实现
1. **后端 API 增强** (`collector/handler/query.go`)
   - 新增 `GET /api/query/performance/summary` — 返回 Web Vitals P75 + 评级(good/needs-improvement/poor)
   - 新增 `GET /api/query/performance/trend` — 按时间粒度(1h/6h/1d)返回指标趋势
   - 新增 `GET /api/query/performance/pages` — 页面级性能排名 + 与上一周期对比
   - 新增 `GET /api/query/performance/regression` — 检测最近变差的页面/指标

2. **前端 Performance.vue 升级**
   - 5 个 Web Vitals 指标卡片: FCP / LCP / CLS / INP / TTFB
   - 每个卡片显示 P75 值 + 彩色评级(green/yellow/red) + 评级阈值说明
   - 趋势图增加 INP/TTFB
   - 新增"页面性能排名"表格：按 LCP P75 排序，显示与前一天对比箭头
   - 新增"最近变差"告警区域：检测哪些页面指标恶化超过 20%

3. **Web Vitals 评级标准**
   ```
   FCP: good ≤ 1.8s, needs-improvement ≤ 3.0s, poor > 3.0s
   LCP: good ≤ 2.5s, needs-improvement ≤ 4.0s, poor > 4.0s
   CLS: good ≤ 0.1, needs-improvement ≤ 0.25, poor > 0.25
   INP: good ≤ 200ms, needs-improvement ≤ 500ms, poor > 500ms
   TTFB: good ≤ 800ms, needs-improvement ≤ 1800ms, poor > 1800ms
   ```

### 验收
- Performance 页面展示 5 个 Web Vitals + 评级
- 页面性能排名表格有数据
- 回归检测能展示变差页面
- Go 编译通过 + Vite 构建通过

## Slice 2: Overview 首页改造 — 异常工作台

### 当前状态
- Overview 页面显示统计卡片 + 24h 错误趋势 + Top 统计
- 功能偏"数据罗列"，不够"排障入口"

### 需要实现
1. **异常工作台区域**（置于统计卡片下方）
   - "需要关注"卡片列表：最近 1h 新出现的错误（带 NEW 标签）
   - 最近触发告警列表（最近 5 条，可点击跳转）
   - 最近活跃 sessions（带 quick replay 按钮）

2. **快捷操作区**
   - 全局搜索框（搜索错误消息、URL、session ID）
   - 快捷筛选按钮：今天错误 / 昨天对比 / 本周 Top

3. **统计卡片增强**
   - 增加"vs 昨日"对比箭头（↑ 红色 / ↓ 绿色）
   - 增加用户影响数卡片

### 验收
- Overview 能展示"需要关注"的新错误
- 告警触发记录可见
- 统计卡片有日环比对比

## Slice 3: Dashboard 产品体验增强

### 需要实现
1. **全局搜索**
   - 顶部搜索栏，搜索错误消息、URL、tag、session ID
   - 搜索结果分类展示（错误/页面/session）
   - 跳转到对应详情

2. **保存的视图 (Saved Views)**
   - Logs 页面支持保存当前筛选条件为"视图"
   - 视图列表在侧边栏显示
   - 视图存储在 localStorage

3. **侧边栏增强**
   - 折叠/展开功能
   - 折叠时显示图标 + tooltip
   - 记住折叠状态

4. **暗黑模式**
   - Element Plus 暗黑主题切换
   - 顶部工具栏切换按钮
   - 偏好持久化到 localStorage

### 验收
- 全局搜索可用
- Saved Views 可创建/切换/删除
- 侧边栏可折叠
- 暗黑模式可切换且图表适配

## Slice 4: 数据生命周期治理

### 需要实现
1. **后端存储治理 API**
   - `GET /api/admin/storage/stats` — 返回各表行数、大小、按 app 分组统计
   - `POST /api/admin/retention/policy` — 按数据类型配置保留天数（events/recording_events/screenshots）
   - `GET /api/admin/retention/policy` — 获取当前保留策略
   - `GET /api/admin/tasks` — 后台任务状态（清理任务进度）

2. **Settings 页面治理区**
   - 存储使用概览（数据库大小 + 各表占比）
   - 按数据类型设置保留天数（事件/录制/截图）
   - 手动清理按钮 + 确认对话框
   - 后台任务状态显示

### 验收
- Storage stats API 返回正确数据
- 保留策略可配置
- Settings 页面显示存储概览

## 执行顺序
1. Slice 1: Web Vitals 升级（最高价值，前端监控核心能力）
2. Slice 2: Overview 异常工作台（日常使用入口）
3. Slice 3: Dashboard 产品体验（全局搜索/视图/暗黑模式）
4. Slice 4: 数据治理（运营必需）

## 技术约束
- 后端: Go，使用现有 storage 接口
- 前端: Vue 3 + Element Plus + ECharts
- 数据库: SQLite，performance 字段为 JSON TEXT
- 编译: `cd collector && go build` / `cd dashboard && npx vite build`
- GOPROXY=https://goproxy.cn

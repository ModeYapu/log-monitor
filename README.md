# LogMonitor - 前端日志监控系统

轻量级、自托管、生产可用的前端日志监控系统。

## 项目结构

```
log-monitor/
├── collector/      # Go 收集服务
├── sdk/            # TypeScript SDK
├── dashboard/      # Vue3 Dashboard (待开发)
└── test.html       # 集成测试页面
```

## 快速开始

### 1. 启动 Collector

```bash
cd collector
go run main.go
```

Collector 将在 `http://localhost:9200` 启动。

### 2. 构建 SDK

```bash
cd sdk
npm install
npm run build
```

### 3. 测试集成

在浏览器中打开 `test.html`，或访问 `http://localhost:9200/test.html`（如果通过静态文件服务）。

### 4. 在前端应用中使用 SDK

```html
<script src="/sdk/dist/logmonitor.min.js"></script>
<script>
  LogMonitor.init({
    dsn: 'http://localhost:9200/api/report',
    appId: 'my-app',
    release: '1.0.0',
  });

  // 自动捕获错误
  // 手动上报
  LogMonitor.info('User logged in');
  LogMonitor.error('Payment failed', { orderId: '12345' });
</script>
```

## API 端点

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/api/report` | 接收 SDK 上报 |
| GET | `/api/query/logs` | 查询日志 |
| GET | `/api/query/stats` | 统计数据 |
| GET | `/api/query/apps` | 应用列表 |
| GET | `/api/health` | 健康检查 |

## 开发进度

- [x] R1: Collector 核心 + SDK 最小可用
  - [x] Go 项目初始化
  - [x] SQLite 初始化
  - [x] /api/report 端点
  - [x] 缓冲批量写入
  - [x] /api/query/logs 查询端点
  - [x] SDK 错误捕获 + 批量上报
  - [x] 集成测试页面
- [ ] R2: Collector 查询完善 + Dashboard 骨架
- [ ] R3: 性能页 + 告警系统
- [ ] R4: 生产化

## 路线图

- 产品路线图见 `docs/ROADMAP.md`

## 配置

Collector 配置文件 `collector/config.yaml`:

```yaml
server:
  port: 9200
  cors: true

database:
  path: ./data/logmonitor.db
  retention_days: 30

buffer:
  size: 10000
  flush_interval_ms: 2000
  flush_batch_size: 500
```

# LogMonitor - 开发状态

## 当前进度: R1 完成 ✅

### R1: Collector 核心 + SDK 最小可用
- ✅ Collector: Go 项目初始化 + SQLite 初始化
- ✅ Collector: `/api/report` 端点 + 缓冲写入
- ✅ Collector: `/api/query/logs` 查询端点（分页+过滤）
- ✅ SDK: 错误捕获 + 批量上报
- ✅ 集成测试: API 测试脚本 + 测试页面

### R2: Collector 查询完善 + Dashboard 骨架
- ⏳ 待开始

### R3: 性能页 + 告警系统
- ⏳ 待开始

### R4: 生产化
- ⏳ 待开始

## 快速测试

```bash
# 1. 启动 Collector
cd collector
export GOPROXY=https://goproxy.cn
go mod tidy
go run main.go

# 2. 测试 API
python3 test_api.py

# 3. 浏览器测试
# 打开 test.html
```

## 项目结构

```
log-monitor/
├── collector/          # Go 收集服务 (R1 完成)
│   ├── main.go
│   ├── handler/        # HTTP 处理器
│   ├── model/          # 数据模型
│   ├── storage/        # SQLite 存储
│   ├── buffer/         # 缓冲写入
│   └── config/         # 配置
├── sdk/                # TypeScript SDK (R1 完成)
│   └── src/index.ts
├── dashboard/          # Vue3 Dashboard (待开发 R2)
├── test.html           # 测试页面
├── test_api.py         # API 测试脚本
└── README.md
```

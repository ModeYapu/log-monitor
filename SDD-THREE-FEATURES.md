# SDD: 三大功能 + 数据库解耦 + 缓存评估

## 项目位置
- 源码：`/home/coder/log-monitor/`
- Collector（Go）：`collector/`
- SDK（TypeScript）：`sdk/src/`
- Dashboard（Vue3）：`dashboard/src/`

## 当前架构问题
- `storage.DB` 是一个巨大的具体类型（1765行 sqlite.go），所有 SQL 直接写在方法里
- handler/buffer/alerter 全部依赖 `*storage.DB` 具体类型
- 无法切换到 MySQL/PostgreSQL 等其他数据库

## 任务分 5 步，按顺序执行

---

### Step 0: 数据库抽象层（最高优先级，先做这个）

**目标**：引入 Repository 接口，将 SQLite 实现与业务逻辑解耦。

1. **`collector/storage/interfaces.go`（新文件）**：

定义以下接口（从现有 DB 方法提取）：

```go
package storage

// EventRepository handles event CRUD
type EventRepository interface {
    InsertEvents(events []EventRecord) error
    QueryEvents(query QueryParams) (*QueryResult, error)
    GetStats(appID string) (*Stats, error)
    GetApps() ([]AppStats, error)
    GetTopN(appID, topType, orderBy string, limit int, filters map[string]interface{}) (*TopNResult, error)
    GetErrorClusters(appID, errorMessage string, threshold float64, limit int) ([]ErrorCluster, error)
    GetSessionEvents(sessionID string, limit int) ([]EventRecord, error)
    GetSessionErrorCount(sessionID string) (int64, error)
}

// AlertRepository handles alert rules and logs
type AlertRepository interface {
    CreateAlertRule(rule AlertRule) (int64, error)
    GetAlertRules(appID string) ([]AlertRule, error)
    GetAllAlertRules() ([]AlertRule, error)
    UpdateAlertRuleLastTriggered(id int64, timestamp int64) error
    DeleteAlertRule(id int64) error
    SilenceAlertRule(id int64, until int64) error
    UnsilenceAlertRule(id int64) error
    CreateAlertLog(log AlertLog) error
    GetAlertLogs(appID string, limit int) ([]AlertLog, error)
}

// RecordingRepository handles session recordings
type RecordingRepository interface {
    CreateRecording(recording RecordingInfo) (int64, error)
    GetRecording(sessionID string) (*RecordingInfo, error)
    GetRecordings(limit, offset int, filters map[string]interface{}) ([]RecordingInfo, error)
    AddRecordingEvent(sessionID string, seq int, timestamp int64, eventData []byte) error
    GetRecordingEvents(sessionID string, limit, offset int) ([]EventEventData, error)
    DeleteRecording(sessionID string) error
    UpdateRecording(sessionID string, endTime int64, durationMs int64, eventCount int, status string) error
    GetRecordingStats(sessionID string) (interface{}, error)
}

// SourceMapRepository handles source map storage
type SourceMapRepository interface {
    CreateSourceMap(record SourceMapRecord) (int64, error)
    GetSourceMap(appID, release, env, buildID string) (*SourceMapRecord, error)
    GetSourceMapByBuildID(buildID string) (*SourceMapRecord, error)
    ListSourceMaps(appID string, limit int) ([]SourceMapRecord, error)
    DeleteSourceMap(id int64) error
}

// UserRepository handles user management
type UserRepository interface {
    // 参照 storage/users.go 中的方法签名
}

// Store 组合接口 — 所有 handler 依赖这个
type Store interface {
    Events() EventRepository
    Alerts() AlertRepository
    Recordings() RecordingRepository
    SourceMaps() SourceMapRepository
    Users() UserRepository
    Close() error
}
```

2. **`collector/storage/sqlite_store.go`（新文件）**：

```go
package storage

// SQLiteStore implements Store interface
type SQLiteStore struct {
    db *DB
}

func NewSQLiteStore(cfg Config) (*SQLiteStore, error) {
    db, err := NewDB(cfg)
    if err != nil {
        return nil, err
    }
    return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Events() EventRepository { return s.db }
func (s *SQLiteStore) Alerts() AlertRepository { return s.db }
func (s *SQLiteStore) Recordings() RecordingRepository { return s.db }
func (s *SQLiteStore) SourceMaps() SourceMapRepository { return s.db }
func (s *SQLiteStore) Users() UserRepository { return s.db }
func (s *SQLiteStore) Close() error { return s.db.Close() }
```

3. **渐进式迁移**（不要一次改完所有文件）：
   - `buffer/writer.go`：`db *storage.DB` → `db storage.EventRepository`
   - `handler/*.go`：逐步从 `*storage.DB` 改为具体 Repository 接口
   - `alerter/checker.go`：`db *storage.DB` → `db storage.AlertRepository`
   - `main.go`：创建 `SQLiteStore` 传给各 handler

4. **编译验证**：确保每改一个文件都能 `go build` 通过

---

### Step 1: 错误聚合/指纹（Error Fingerprinting）

5. **`collector/fingerprint/fingerprint.go`（新文件）**：

```go
package fingerprint

import (
    "crypto/sha256"
    "fmt"
    "regexp"
    "strings"
)

// Generate creates a fingerprint from error stack and message
func Generate(stack, message string) string {
    if stack != "" {
        return stackFingerprint(stack)
    }
    return messageFingerprint(message)
}

// stackFingerprint extracts function+file from each frame, ignoring line numbers
func stackFingerprint(stack string) string {
    // 匹配 "at functionName (file:line:col)" 或 "at file:line:col"
    re := regexp.MustCompile(`at\s+(?:\S+\s+)?\(?(.+?):\d+:\d+\)?`)
    matches := re.FindAllStringSubmatch(stack, -1)
    var frames []string
    for _, m := range matches {
        if len(m) > 1 {
            frames = append(frames, m[1])
        }
    }
    if len(frames) == 0 {
        return messageFingerprint(stack)
    }
    h := sha256.Sum256([]byte(strings.Join(frames, "|")))
    return fmt.Sprintf("%x", h[:16]) // 前16字节，32个hex字符
}

// messageFingerprint normalizes dynamic values then hashes
func messageFingerprint(message string) string {
    normalized := normalizeMessage(message)
    h := sha256.Sum256([]byte(normalized))
    return fmt.Sprintf("%x", h[:16])
}

// normalizeMessage replaces dynamic values with placeholders
func normalizeMessage(msg string) string {
    // UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    re := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
    msg = re.ReplaceAllString(msg, "{uuid}")

    // Hex: 0x...
    re = regexp.MustCompile(`0x[0-9a-fA-F]+`)
    msg = re.ReplaceAllString(msg, "{hex}")

    // Numbers (but not in obvious version strings like "1.0.0")
    re = regexp.MustCompile(`\b\d{3,}\b`)
    msg = re.ReplaceAllString(msg, "{n}")

    // URLs with dynamic paths
    re = regexp.MustCompile(`/\d+/`)
    msg = re.ReplaceAllString(msg, "/{id}/")

    return strings.TrimSpace(msg)
}
```

6. **storage/sqlite.go 迁移**：
   - 添加 `ALTER TABLE events ADD COLUMN fingerprint TEXT DEFAULT ''`
   - 添加 `CREATE INDEX IF NOT EXISTS idx_events_fingerprint ON events(app_id, fingerprint)`
   - `InsertEvents` 时写入 fingerprint 字段

7. **`collector/handler/clusters.go`（新文件）**：
   - `GET /api/query/clusters?appId=&startTime=&endTime=&limit=`
   - `GET /api/query/clusters/{fingerprint}/events?page=&pageSize=`

8. **main.go 注册路由**

9. **dashboard/src/views/Logs.vue**：聚合视图切换

---

### Step 2: Breadcrumbs 面包屑

10. **storage/sqlite.go 迁移**：`ALTER TABLE events ADD COLUMN breadcrumbs TEXT DEFAULT '[]'`

11. **model/breadcrumb.go（新文件）**：Breadcrumb 结构体

12. **SDK `sdk/src/breadcrumbs.ts`（新文件）**：
    - BreadcrumbCollector 类，环形缓冲区 50 条
    - 自动捕获：click、navigation、fetch/XHR、console
    - 隐私过滤：URL 中 token/session 参数替换为 [filtered]
    - `sdk/src/index.ts` 初始化时启动，上报错误时附带

---

### Step 3: Release Health

13. **model/release.go（新文件）**：ReleaseHealth、SessionStats 结构体

14. **storage 新增查询方法**：
    - `GetReleaseHealth(appID, startTime, endTime)` — 按 release 分组统计崩溃率
    - `GetSessionStats(appID, startTime, endTime)` — 总体统计

15. **handler/health.go（新文件）**：
    - `GET /api/query/release-health?appId=&startTime=&endTime=`
    - `GET /api/query/session-stats?appId=&startTime=&endTime=`

16. **SDK**：初始化上报 session_start，beforeunload 上报 session_end

17. **dashboard Overview.vue**：Release Health 卡片区域

---

### Step 4: 缓存评估与可选 Redis 集成

**评估结论（写入代码注释）**：

对于 LogMonitor 当前规模（单机部署、SQLite），**不需要引入 Redis**。理由：

1. **SQLite 本身就是缓存层**：WAL 模式下读性能足够（单机几千 QPS）
2. **写入走 buffer**：已有内存缓冲批量写入，不需要额外写缓存
3. **聚合查询可加内存缓存**：在 Go 层用 `sync.Map` + TTL 做轻量缓存即可
4. **Redis 引入的代价**：额外依赖、运维复杂度、内存开销，对小规模场景是 over-engineering

**轻量缓存方案**（如果未来需要）：

```go
// collector/storage/cache.go
type QueryCache struct {
    mu    sync.RWMutex
    items map[string]*cacheItem
    ttl   time.Duration
}

type cacheItem struct {
    value     interface{}
    expireAt  time.Time
}

func (c *QueryCache) Get(key string) (interface{}, bool) { ... }
func (c *QueryCache) Set(key string, value interface{}) { ... }
```

适用场景：
- Dashboard 首页统计（10s TTL）
- Error Clusters 聚合结果（30s TTL）
- Release Health 数据（60s TTL）

**当以下条件满足时再考虑 Redis**：
- 日志量 > 10GB/天
- 多 Collector 实例部署
- 需要跨实例共享缓存
- 需要分布式锁（如告警去重）

**实现方式**：在 interfaces.go 中添加 `CacheRepository` 接口：
```go
type CacheRepository interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{}, ttl time.Duration) error
    Delete(key string) error
}
```
先实现内存版本 `MemoryCache`，未来可无缝切换到 Redis 实现。

---

## 技术约束

1. **不要引入新依赖**（crypto/sha256 是标准库）
2. **SQLite 兼容**：所有 SQL 兼容 modernc.org/sqlite
3. **向后兼容**：新字段都有 DEFAULT，用 ALTER TABLE 迁移
4. **编译验证**：每步完成后 `cd collector && go build -buildvcs=false ./...`
5. **Go 代理**：`export GOPROXY=https://goproxy.cn,direct`
6. **SDK 编译**：`cd sdk && npm run build`
7. **Dashboard 编译**：`cd dashboard && npm run build`
8. **渐进式重构**：不要一次改 50 个文件，每改几个文件就编译验证

## 最终验证
```bash
cd /home/coder/log-monitor/collector && go build -buildvcs=false ./... && go test ./...
cd /home/coder/log-monitor/sdk && npm run build
cd /home/coder/log-monitor/dashboard && npm run build
```

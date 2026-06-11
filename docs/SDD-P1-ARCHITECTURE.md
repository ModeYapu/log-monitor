# SDD: LogMonitor P1 Architecture Fixes

## Slice 1: Split sqlite.go (3242→~300 lines + 8 files)

Split storage/sqlite.go by interface:
- storage/event-store.go — EventStore implementation
- storage/issue-store.go — IssueStore implementation  
- storage/project-store.go — ProjectStore implementation
- storage/alert-store.go — AlertStore implementation
- storage/analytics-store.go — AnalyticsStore implementation (from analytics.go too)
- storage/system-store.go — SystemStore implementation
- storage/recording-store.go — RecordingRepository implementation
- storage/sourcemap-store.go — SourceMapRepository implementation
- storage/user-store.go — UserRepository implementation
- storage/sqlite.go — DB struct, Init(), Close(), migrations (shared infra only)

## Slice 2: Add Core Tests

- storage/storage_test.go — DB init, migrations, basic CRUD
- storage/event-store_test.go — EventStore interface tests
- storage/issue-store_test.go — IssueStore interface tests
- handler/health_test.go — Health endpoint
- handler/query_test.go — Query parser + builder
- alerter/checker_test.go — Alert rule matching

## Slice 3: Replace interface{} with typed structs

- Replace map[string]interface{} in analytics queries with specific result structs
- Replace interface{} in query handler with typed params
- Define AnalysisResult, QueryParams, etc.

## Slice 4: Unified Error Handling

- Create errors/errors.go with typed errors (NotFoundError, ValidationError, etc.)
- Create middleware/error-handler.go for consistent JSON error responses
- Update handler layer to use typed errors

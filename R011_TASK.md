# R011: Session Tracking + Replay Timeline + User Journey

## Context
This is a Go + SQLite log monitoring platform. Key files:
- `collector/routes.go` — central route registration (readGroup = JWT-authed APIs)
- `collector/handler/cobrowse_recording.go` — existing RecordingHandler with routes at `/api/query/recordings`
- `collector/handler/cobrowse_hub.go` — CoBrowseHub, CoBrowseDB interface
- `collector/handler/cobrowse_session.go` — SessionHub for live sessions
- `collector/handler/query_handler.go` — QueryHandler with QueryLogs etc.
- `collector/storage/recording-store.go` — RecordingInfo, RecordingEventData, CRUD ops
- `collector/storage/event-store.go` — GetSessionEvents, GetSessionErrorCount already exist
- `collector/storage/records.go` — EventRecord struct
- `collector/storage/interfaces.go` — EventStore interface
- `collector/storage/sqlite.go` — DB struct with `conn *sql.DB`
- `dashboard/index.html` — Vue 3 + Element Plus SPA (~2500 lines)
- Go module: `github.com/logmonitor/collector`, Go 1.25, uses `modernc.org/sqlite`

## Task 1: Replay Timeline Enhancement

### 1A. Add timeline extraction to `collector/storage/recording-store.go`

Add a `TimelineEvent` struct:
```go
type TimelineEvent struct {
    Seq       int    `json:"seq"`
    Timestamp int64  `json:"timestamp"`
    Offset    int64  `json:"offsetMs"`  // ms from session start
    Type      string `json:"type"`       // e.g. "click", "scroll", "input", "navigation", "error", "custom"
    Label     string `json:"label"`      // human-readable description
    Data      string `json:"data"`       // raw event data JSON
}
```

Add method `GetRecordingTimeline(sessionID string) ([]TimelineEvent, error)` that:
- Queries recording_events for the session
- Parses each event's `event_data` JSON to extract rrweb event type (types 2=DomContentLoaded, 4=Meta, 5=Load, 6=FullSnapshot, 7=IncrementalSnapshot, etc.)
- For incremental snapshots, look at data.source to classify: 0=mutation, 1=mouseMove, 2=mouseInteraction, 3=scroll, 4=viewportResize, 5=input, 6=mediaInteraction, etc.
- For mouseInteraction (source=2), data.type: 0=mouseup/left click, etc. → map to "click"
- Returns timeline events sorted by timestamp
- Computes OffsetMs relative to first event

### 1B. Add timeline API endpoint in `collector/handler/cobrowse_recording.go`

In `getRecordingWithRouting`, add routing for `/api/query/recordings/:id/timeline`:
- Calls `db.GetRecordingTimeline(sessionID)`
- Returns JSON `{"sessionId": "...", "timeline": [...]}`

Also update `RecordingHandler.RegisterRoutes` if needed — the existing catch-all routing in `getRecordingWithRouting` should handle it, just add the timeline case.

### 1C. Add `CoBrowseDB` interface method

In `collector/handler/cobrowse_hub.go`, add to `CoBrowseDB` interface:
```go
GetRecordingTimeline(sessionID string) ([]storage.TimelineEvent, error)
```

## Task 2: User Session Tracking (new file: `collector/handler/session_handler.go`)

Create a `SessionHandler` struct with `db *storage.DB`:

### 2A. GET /api/query/sessions — Session list with filters

Query params: `app_id`, `user_id`, `start_from`, `start_to`, `limit` (default 50), `offset`

Implementation:
- Query `events` table: `SELECT session_id, app_id, user_id, MIN(created_at) as start_time, MAX(created_at) as end_time, COUNT(*) as event_count, SUM(CASE WHEN level='error' THEN 1 ELSE 0 END) as error_count, MAX(url) as last_url FROM events WHERE session_id != '' GROUP BY session_id`
- Apply filters using WHERE clauses
- Order by start_time DESC
- Return paginated results

Response shape:
```json
{
  "data": [
    {
      "sessionId": "abc123",
      "appId": "my-app",
      "userId": "user-1",
      "startTime": 1718000000000,
      "endTime": 1718000060000,
      "durationMs": 60000,
      "eventCount": 145,
      "errorCount": 3,
      "lastUrl": "https://example.com/page"
    }
  ],
  "total": 42,
  "limit": 50,
  "offset": 0
}
```

### 2B. GET /api/query/sessions/:id — Session detail

Returns:
```json
{
  "session": { /* same fields as list item + recording info if available */ },
  "events": [ /* EventRecord array — first 200 events */ ],
  "stats": {
    "errorCount": 3,
    "warningCount": 5,
    "pageCount": 4,
    "topPages": ["https://...", ...],
    "eventTypes": {"error": 3, "info": 100, ...}
  }
}
```

### 2C. GET /api/query/sessions/:id/journey — User journey

Returns page transition data for SVG flow diagram:
```json
{
  "sessionId": "abc123",
  "nodes": [
    {"id": 0, "url": "https://example.com/", "label": "/", "firstVisit": 1718000000000, "visitCount": 1},
    {"id": 1, "url": "https://example.com/login", "label": "/login", "firstVisit": 1718000010000, "visitCount": 1}
  ],
  "edges": [
    {"from": 0, "to": 1, "count": 1, "timestamp": 1718000010000}
  ],
  "errors": [
    {"timestamp": 1718000030000, "message": "TypeError: ...", "url": "https://example.com/login"}
  ]
}
```

Build this by querying events for the session ordered by created_at, extracting URL changes and errors.

### 2D. Register routes

In `collector/routes.go`, in the `readGroup` section (JWT-authed), add:
```go
// Session tracking
sessionHandler := handler.NewSessionHandler(rc.DB)
readGroup.HandleFunc("GET /api/query/sessions", sessionHandler.ListSessions)
readGroup.HandleFunc("GET /api/query/sessions/", sessionHandler.GetSessionDetail)
```

**IMPORTANT**: The existing `RecordingHandler.RegisterRoutes` already registers `GET /api/query/sessions/` for `getSessionEvents`. You must update the routing so that:
- `GET /api/query/sessions` (no trailing ID) → SessionHandler.ListSessions
- `GET /api/query/sessions/:id` → SessionHandler.GetSessionDetail (which internally routes to journey/events/etc.)
- `GET /api/query/sessions/:id/journey` → SessionHandler.GetSessionJourney

Since Go's `http.ServeMux` with Go 1.22+ pattern routing can have conflicts, the cleanest approach is to REMOVE the `getSessionEvents` route registration from `RecordingHandler.RegisterRoutes` and fold it into the new `SessionHandler`. The `SessionHandler.GetSessionDetail` will route sub-paths (journey, events, etc.).

Actually, since `RecordingHandler.RegisterRoutes` registers on the root mux and `readGroup` also registers on the root mux with JWT middleware, you need to consolidate. The simplest approach:

1. Remove the `GET /api/query/sessions/` registration from `RecordingHandler.RegisterRoutes`
2. Have `SessionHandler` handle all `/api/query/sessions` routes in the readGroup
3. In `SessionHandler`, route internally: if path after `/api/query/sessions/` ends with `/journey`, call journey handler; if ends with `/events`, call events handler (reusing same logic as old getSessionEvents); otherwise return session detail.

### 2E. Add storage methods to `collector/storage/event-store.go`

Add these methods to `DB`:
```go
func (db *DB) GetSessionList(filters map[string]interface{}, limit, offset int) ([]SessionSummary, error)
func (db *DB) GetSessionListCount(filters map[string]interface{}) (int64, error)
func (db *DB) GetSessionJourney(sessionID string) ([]EventRecord, error) // returns ordered events for journey building
```

Add `SessionSummary` struct to `records.go`:
```go
type SessionSummary struct {
    SessionID  string `json:"sessionId"`
    AppID      string `json:"appId"`
    UserID     string `json:"userId"`
    StartTime  int64  `json:"startTime"`
    EndTime    int64  `json:"endTime"`
    DurationMs int64  `json:"durationMs"`
    EventCount int64  `json:"eventCount"`
    ErrorCount int64  `json:"errorCount"`
    LastURL    string `json:"lastUrl"`
}
```

## Task 3: Dashboard Frontend Enhancement (`dashboard/index.html`)

### 3A. Add a new "Sessions" nav item/tab

Add a navigation entry for "用户会话" (User Sessions) in the sidebar/nav.

### 3B. Sessions List View

- Table showing session list with columns: Session ID (truncated), App, User, Duration, Events, Errors, Last URL, Start Time
- Filter bar: App selector (dropdown), User ID input, date range picker
- Pagination controls
- Click a row to open detail drawer

### 3C. Session Detail Drawer

A slide-out panel (el-drawer) showing:
- Session metadata (session ID, app, user, duration, event/error counts)
- Tabs: "事件列表" (Events), "用户旅程" (Journey), "回放" (Replay)
- Events tab: scrollable list of events with timestamp, type, level, message
- Journey tab: SVG flow diagram showing page transitions, with error markers
- Replay tab: embed the recording player (if recording exists) with:
  - Timeline scrubber showing event markers (color-coded: click=blue, scroll=gray, input=green, error=red)
  - Speed controls (1x, 2x, 4x, 8x buttons)
  - "Jump to error" button

### 3D. Journey SVG

Simple SVG visualization:
- Nodes = unique pages (circles with page label)
- Edges = transitions (arrows between pages)
- Red pulse on nodes where errors occurred
- Use Vue reactive SVG (not a charting library)

### 3E. API Integration

Add JavaScript functions:
- `fetchSessions(params)` — calls GET /api/query/sessions
- `fetchSessionDetail(id)` — calls GET /api/query/sessions/:id
- `fetchSessionJourney(id)` — calls GET /api/query/sessions/:id/journey
- `fetchTimeline(id)` — calls GET /api/query/recordings/:id/timeline

Use the existing `apiRequest` or `fetch` pattern already in the file.

## Build & Test Requirements

**CRITICAL**: After all changes:
```bash
cd /home/coder/log-monitor/collector && go build ./...
cd /home/coder/log-monitor/collector && go test ./...
```

Both MUST exit 0.

## Code Style
- Follow existing patterns in the codebase
- Use `encoding/json` for JSON (no external libs)
- Use `log/slog` for logging
- Use `fmt.Errorf` with `%w` for error wrapping
- DB methods check `db.closed.Load()` first
- HTTP handlers set `Content-Type: application/json` and use `json.NewEncoder(w).Encode()`
- Route handlers in session_handler.go should NOT need admin auth (they're in readGroup which has JWT auth)

## Files to Create/Modify
1. **MODIFY** `collector/storage/recording-store.go` — add TimelineEvent, GetRecordingTimeline
2. **MODIFY** `collector/storage/event-store.go` — add GetSessionList, GetSessionListCount, GetSessionJourney
3. **MODIFY** `collector/storage/records.go` — add SessionSummary
4. **MODIFY** `collector/handler/cobrowse_hub.go` — add GetRecordingTimeline to CoBrowseDB interface
5. **MODIFY** `collector/handler/cobrowse_recording.go` — add timeline route in getRecordingWithRouting, remove sessions/ route from RegisterRoutes
6. **CREATE** `collector/handler/session_handler.go` — SessionHandler with ListSessions, GetSessionDetail (routes sub-paths), GetSessionJourney
7. **MODIFY** `collector/routes.go` — register session handler routes in readGroup
8. **MODIFY** `dashboard/index.html` — add Sessions page, detail drawer, journey SVG, timeline-enhanced replay

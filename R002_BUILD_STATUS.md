# R002 — Build & Fix Verification Status

**Round:** R002
**Date:** 2026-06-16
**Module root:** `collector/` (`module github.com/logmonitor/collector`, Go 1.25)

## Build verification

```
cd collector && go build ./...
```

| Command        | Result   |
|----------------|----------|
| `go build ./...` | **exit 0 ✅** |
| `go vet ./...`   | **exit 0 ✅** |

> `go build ./...` must be run from the **`collector/`** directory (the module
> root). From the repository root `./...` matches no packages because there is no
> `go.mod` at that level.

## Issue summary

| ID | Issue | Status |
|----|-------|--------|
| P2-ERROR-HANDLING | Swallowed `RowsAffected`/`LastInsertId` errors | **Fixed** |
| P2-RACE-RECORDING | Unbounded `SessionHub.events` growth | **Verified present** |
| P2-SOURCEMAP-PATH | Path traversal on user-supplied path segments | **Fixed (hardened)** |
| P3-METRICS | CORS preflight / OPTIONS handling | **Verified present** |
| P3-GRACEFUL-SHUTDOWN | Graceful shutdown on SIGINT/SIGTERM | **Verified present** |

---

## P2-ERROR-HANDLING — Fixed

**Audit scope:** `collector/storage/sqlite.go`, `collector/storage/recording-store.go`,
`collector/storage/verification-store.go` (and `collector/storage/sourcemap.go`,
which shares the same pattern).

Every place that discarded the error from `RowsAffected()` / `LastInsertId()` with
`, _ :=` **while using the returned value in logic** now logs the error via
`slog.Warn` and proceeds with the (zero) default. The preceding SQL statement
already succeeded, so these counts/IDs are best-effort — failing the whole call on
a meta-query error would be wrong, but silently swallowing it hid real problems.

| File | Line(s) | Call | Used for |
|------|---------|------|----------|
| `storage/sqlite.go` | 325–329 | `DeleteEventsBefore` → `RowsAffected` | vacuum decision |
| `storage/sqlite.go` | 406–408 | `DeleteRecordingsBefore` batch → `RowsAffected` | `totalDeleted` |
| `storage/sqlite.go` | 422–424 | `DeleteRecordingsBefore` → `RowsAffected` | `recordingCount` |
| `storage/sqlite.go` | 498–500 | `cleanupOldData` events → `RowsAffected` | `result.DeletedEvents` |
| `storage/sqlite.go` | 516–518 | `cleanupOldData` orphan events → `RowsAffected` | `result.TotalFilesFreed` |
| `storage/sqlite.go` | 533–535 | `cleanupOldData` alert logs → `RowsAffected` | `result.TotalFilesFreed` |
| `storage/verification-store.go` | 69–73 | `StoreVerificationResult` → `LastInsertId` | `result.ID` |
| `storage/sourcemap.go` | 250–254 | `DeleteSourceMapsByRelease` → `RowsAffected` | returned count |

**Already correct (no change needed):**
- `storage/recording-store.go:68` — `return result.LastInsertId()` returns the error directly.
- `storage/sourcemap.go:73` — `return result.LastInsertId()` returns the error directly.

---

## P2-RACE-RECORDING — Verified present

`SessionHub.events` is trimmed so it cannot grow past `maxEvents` (10000).
The cap is declared in `handler/cobrowse_handler.go:102` (`maxEvents: 10000`)
and enforced in `handler/cobrowse_session.go` after each rrweb-event batch:

```go
// handler/cobrowse_session.go:178
if len(hub.events) > hub.maxEvents {
    removed := len(hub.events) - hub.maxEvents
    hub.events = hub.events[removed:]   // keep last maxEvents
}
```

This matches the required fix: when `len(events) > maxEvents`, trim from the front
and keep the last `maxEvents`.

> **Note / known follow-up (not changed in R002, out of task scope):** the trim
> keeps the *last* `maxEvents` entries, so for a session that exceeds 10000 events
> the full snapshot currently held at `events[0]` can be evicted. A new viewer
> joining such a long session would then receive an incremental event as the
> "snapshot" (`HandleViewerConnection`), which would break replay. Preserving
> `events[0]` (snapshot) and trimming only incrementals would be the strictly
> correct improvement; left documented here rather than introduced to avoid
> diverging from the specified "keep last maxEvents" behavior.

---

## P2-SOURCEMAP-PATH — Fixed (hardened)

All file paths built from user input (`appId`, `release`, `buildID`, `filename`)
are passed through `sanitizePathSegment` (`storage/sourcemap.go:426`), which keeps
only `[A-Za-z0-9-_.]` and replaces everything else (including `/`, `\`, `..`)
with `_`. This is applied to every `filepath.Join` that uses a user-controlled
segment, so no input can escape `baseDir`.

| Site | Segments sanitized |
|------|--------------------|
| `Save` (`storage/sourcemap.go:276–288`) | appID, release, buildID |
| `GetPath` (`storage/sourcemap.go:295–301`) | appID, release, buildID |
| `GetSourceMap` (`storage/sourcemap.go:355–380`) | appID, **filename** |
| `ListSourceMaps` (`storage/sourcemap.go:388–404`) | appID, release (prefix) |

**R002 change:** `GetSourceMap` previously joined the raw `filename` in its
`.map`/`.json` branch (it was gated by a `ReadDir` base-name match, so traversal
was already blocked, but the segment was not explicitly sanitized). It now
sanitizes `filename` into `safeFilename` before matching and joining
(`storage/sourcemap.go:358, 370–371, 376, 380`) — explicit defense-in-depth that
satisfies "add a safe-path-segment check for all `path.Join` calls using
user-supplied values."

The HTTP layer (`handler/sourcemap.go`) never builds filesystem paths directly:
it routes user input through `SourceMapStorage.Save/Get/GetByPath` (which
sanitize) or through parameterized DB queries (`repo.*`), and file reads use
`record.FilePath` taken from the DB (stored sanitized at upload time).

---

## P3-METRICS (CORS / OPTIONS) — Verified present

CORS preflight is handled **once, in middleware**, and applied to the entire mux:

- `middleware/cors.go:51` — `if r.Method == http.MethodOptions { w.WriteHeader(http.StatusOK); return }`
  short-circuits preflight **before** any handler (and before Go 1.22 method-pattern
  routing would 404/405 on an unmatched `OPTIONS`).
- `main.go:336` — `handlerWithCORS := corsMiddleware.Handler(mux)` wraps the whole
  mux, so every route (including cobrowse/recording routes registered at
  `main.go:327/331`) inherits CORS + OPTIONS handling.

CORS headers (`Allow-Origin`, `Allow-Methods`, `Allow-Headers`, `Max-Age`) are set
unconditionally beforehand (`cors.go:45–48`). No per-handler OPTIONS handling is
required or duplicated.

---

## P3-GRACEFUL-SHUTDOWN — Verified present

`main.go` implements a full ordered shutdown on `SIGINT`/`SIGTERM`:

1. **Signal capture** — `main.go:357–359`: `signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)`.
2. **HTTP server drain** — `main.go:366–373`: `shutdownCtx` (10s timeout) →
   `server.Shutdown(shutdownCtx)`, which stops accepting new connections and lets
   in-flight handlers finish.
3. **Buffer flush** — `main.go:377`: `writer.Close()` persists buffered events.
4. **Background workers** — `main.go:385`: `workerManager.Stop()` (cleanup,
   issue aggregator, alert checker).
5. **Database** — `main.go:393–400`: `store.Close()` + `db.Close()`.
6. **WebSocket + remaining goroutines** — `main.go:404/407/410/413`:
   `webhookManager.Stop()`, `configWatcher.Stop()`, `cobrowseHub.Close()` (closes
   every live `SessionHub`, which closes all user/viewer WS connections and their
   ping goroutines via `SessionHub.close()`), `retryWorker.Stop()`.

`CoBrowseHub.Close()` (`handler/cobrowse_hub.go:57`) iterates all sessions and
calls `SessionHub.close()`, which closes user + viewer sockets and signals stop
channels, so WS goroutines are cleaned up.

---

## Test notes

`go build ./...` and `go vet ./...` pass. One storage test fails on `main`
**independently of these changes** — confirmed by `git stash` + re-run on the
original code:

```
--- FAIL: TestGetStatsComparison (analytics_test.go:642)
  sql: Scan error on column index 1, name "error_count":
  converting NULL to int64 is unsupported
```

This is a pre-existing NULL-handling bug in the analytics stats query, unrelated
to the R002 error-handling/path changes (which touch `RowsAffected`/`LastInsertId`
meta-calls and path sanitization, not the stats SELECT). Out of scope for this
round; flagged for follow-up.

## Files changed in R002

- `collector/storage/sqlite.go`
- `collector/storage/verification-store.go`
- `collector/storage/sourcemap.go`

`collector/handler/cobrowse_session.go`, `collector/handler/cobrowse_handler.go`,
`collector/middleware/cors.go`, and `collector/main.go` were audited and found to
already satisfy P2-RACE-RECORDING, P3-METRICS, and P3-GRACEFUL-SHUTDOWN; no
edits required.

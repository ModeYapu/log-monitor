# R010: Dashboard Frontend Enhancement + SDK Optimization + Webhook Tests

You are working on LogMonitor, a self-hosted frontend log monitoring system. This is the final polish round.

## Project Structure
- `dashboard/index.html` — Vue 3 SPA using Element Plus (CDN), ~1943 lines
- `sdk/src/index.ts` — TypeScript SDK for frontend monitoring
- `collector/` — Go backend with handlers, storage, webhook support
- `collector/webhook/e2e_verifier.go` — E2E verifier webhook handler (already implemented)

---

## Task 1: Dashboard Frontend Enhancement (`dashboard/index.html`)

The dashboard is a Vue 3 SPA loaded from CDN (Element Plus 2.8.7, Vue 3). Enhance it with:

### 1a. Real-time Event Stream on Dashboard Home
- Add a "Real-time Events" panel on the dashboard home page
- Poll the events API every 5 seconds (GET `/api/events?limit=20`) to fetch recent events
- Display events in a scrollable list showing: timestamp, type (with color-coded badge), level, message (truncated)
- Add a "Live" indicator with a pulsing green dot
- Auto-scroll to top when new events arrive
- Handle errors gracefully (show "Connection lost" state)

### 1b. Dark Mode Toggle
- Add a dark/light theme toggle button in the header (use Element Plus's `html.dark` class approach)
- Add Element Plus dark mode CSS: `<link rel="stylesheet" href="https://unpkg.com/element-plus@2.8.7/dist/index-dark.css">`
- Add `html.dark` class toggling on `<html>` element
- Store preference in localStorage (`logmonitor-theme`)
- Apply dark theme styles to the dashboard layout (sidebar, content area, cards)
- Use CSS variables for smooth theme switching

### 1c. Performance Page with Chart.js
- Add Chart.js from CDN: `<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js"></script>`
- On the Performance page, add 3 line charts showing 7-day trends for:
  - FCP (First Contentful Paint) in ms
  - LCP (Largest Contentful Paint) in ms  
  - CLS (Cumulative Layout Shift) score
- Fetch data from `/api/performance?days=7` (if API doesn't exist, generate realistic mock data as fallback)
- Charts should be responsive and have tooltips

### 1d. Audit Log Page Enhancement
- On the existing audit log page, enhance the display:
  - Add user avatar circles (colored circles with first letter of username)
  - Display logs in a timeline view (vertical line with entries)
  - Color-code by action type (create=green, update=blue, delete=red, login=orange)

---

## Task 2: SDK Optimization (`sdk/src/index.ts`)

Add these features to the existing SDK:

### 2a. `beforeSend` Hook
- Add `beforeSend?: (event: LogEvent) => LogEvent | null` to `LogMonitorConfig`
- In `addEvent()`, before adding to buffer, call `beforeSend` if configured
- If it returns `null`, drop the event entirely
- If it returns a modified event, use that
- This allows users to filter/modify events before sending

### 2b. Performance Event Sampling
- The existing `sampleRate` applies to ALL events. Add a separate `performanceSampleRate`:
  - Add `performanceSampleRate?: number` to config (default: same as sampleRate)
  - Only apply this rate to events of type `'performance'`
  - Regular `sampleRate` continues to apply to all other event types
  - Example: `sampleRate: 1, performanceSampleRate: 0.1` sends all errors but only 10% of performance events

### 2c. `setUser(userId, traits)` API
- Add exported function `setUser(userId: string, traits?: Record<string, any>): void`
- Store user info in internal state
- Attach `userId` and `userTraits` to all outgoing events' tags
- Add `clearUser(): void` to clear user context
- Add to the UMD global export

### 2d. Error Stack Cleanup
- Add a `cleanStack(stack: string): string` function
- Remove `node_modules` paths (e.g., `webpack:///./node_modules/...` → `[internal]`)
- Remove webpack:/// prefixes
- Remove query strings from file URLs (e.g., `?t=12345`)
- Truncate to max 3000 chars
- Apply automatically in `captureException()` and error handler

---

## Task 3: Webhook E2E Verifier Tests (`collector/webhook/e2e_verifier_test.go`)

Create comprehensive tests for the E2E verifier webhook:

- Test `HandleVerificationResult`:
  - Valid POST with pass status → 200, result stored
  - Valid POST with fail status → 200, alert triggered
  - Missing site field → 400
  - Invalid status → 400
  - Wrong API key → 401
  - Wrong HTTP method → 405
  - Invalid JSON → 400

- Test `GetVerificationResults`:
  - Valid query returns results
  - Limit parameter works
  - Wrong method → 405

Use mock implementations for `VerificationResultStore` and `VerificationAlerter` interfaces. Use `net/http/httptest` for HTTP testing.

---

## Validation Steps

After making all changes:

1. `cd /home/coder/log-monitor/collector && go build ./...` must succeed (exit 0)
2. `cd /home/coder/log-monitor/collector && go test ./...` must succeed (exit 0)
3. The dashboard `index.html` should be valid HTML with all new features integrated

## Important Notes
- Keep the existing CDN-based approach (no build step for dashboard)
- Maintain backward compatibility with existing SDK API
- All new config options should have sensible defaults
- Write clean, well-commented code
- Keep the single-file approach for dashboard/index.html

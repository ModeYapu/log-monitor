# R008: Dashboard Frontend SPA (Vue 3 CDN + Element Plus)

## Objective
Create a fully functional `dashboard/index.html` as a Vue 3 CDN + Element Plus single-page application. Also wire up static file serving in the collector so the dashboard is accessible at `http://localhost:8080/`.

## Important Context
- The existing `dashboard/` directory has a Vite-based Vue project, but we need a **standalone CDN-based** `index.html` that requires NO build step.
- **Create a new file `dashboard/index.html`** (overwrite the existing one) that is completely self-contained.
- **Do NOT modify any backend Go code except `collector/routes.go`** (or wherever static serving needs to be added) to serve the dashboard directory as static files.

## Dashboard Requirements

### Layout
- Top navigation bar with LogMonitor branding + user menu
- Left sidebar with navigation: Dashboard / Events / Issues / Alerts / Performance / Audit Logs / Settings
- Main content area that switches based on selected nav item
- Responsive design (sidebar collapses on mobile)

### Pages

#### 1. Dashboard (Overview)
- 4 stat cards: Today's Events, Unresolved Issues, Active Alerts, Active Projects
- Fetch data from `/api/query/analytics/overview` (fallback to `/api/query/stats` if not available)
- Recent events list (last 10 events from `/api/query/logs?limit=10`)

#### 2. Events Page
- Full-width data table using Element Plus `<el-table>`
- Columns: timestamp, type, severity (color-coded tag), project, message, URL
- Filters: search box (message text), type select, severity select, project select, date range picker
- Pagination (Element Plus `<el-pagination>`)
- API: `GET /api/query/logs` with query params

#### 3. Issues Page
- Issue list table: title, status (tag), priority (tag), assignee, created date, last seen
- Click a row to expand issue detail (use `<el-drawer>` or `<el-dialog>`)
- Status transition buttons: Open → Resolved, Open → Ignored
- API: `GET /api/query/issues`, `PUT /api/query/issues/{id}` with `{status: "resolved"}` etc.
- Also support `POST /api/query/issues/{id}?action=resolve`

#### 4. Alerts Page
- Alert list table: name, condition, status, created date
- Create alert dialog (name, condition expression, notification channel)
- Delete alert button
- API: `GET /api/query/alerts`, `POST /api/query/alerts`, `DELETE /api/query/alerts/{id}`

#### 5. Performance Page
- Web Vitals overview cards: FCP, LCP, CLS, INP, TTFB (show P75 values)
- Page performance table: URL, FCP, LCP, CLS, INP, TTFB, sample count
- Trend chart using Chart.js (CDN): show LCP trend over time
- API: `GET /api/query/performance/summary`, `GET /api/query/performance/trend`

#### 6. Audit Logs Page
- Audit log table: timestamp, user, action, resource, IP address
- Filters: user search, action type, date range
- API: `GET /api/admin/audit-logs`

#### 7. Settings Page
- Tabs: Projects / API Keys / Members
- **Projects tab**: project list table + create project dialog (name, description)
- **API Keys tab**: show project API keys (masked), regenerate button
- **Members tab**: member list with role tags, add member dialog
- APIs: `GET /api/admin/projects`, `POST /api/admin/projects`, `GET /api/admin/projects/members`, etc.

### Technical Requirements
- **Vue 3** from CDN: `https://unpkg.com/vue@3/dist/vue.global.prod.js`
- **Element Plus** from CDN: `https://unpkg.com/element-plus` (CSS + JS)
- **Element Plus Icons** from CDN: `https://unpkg.com/@element-plus/icons-vue`
- **Chart.js** from CDN for the performance trend chart
- **Day.js** from CDN for date formatting
- All API calls use `fetch()`. Include a helper function with error handling and loading states.
- API base URL is empty (same-origin). All API paths start with `/api/`.
- Handle 401 errors by redirecting to a login prompt.
- Use Element Plus notifications (`ElNotification`) for success/error feedback.
- Use a reactive store (Vue reactive) for global state (current page, loading, user info).

### Styling
- Use Element Plus default theme (blue primary)
- Clean, modern dashboard look
- Cards with subtle shadows
- Responsive grid for stat cards

## Static File Serving

Check `collector/routes.go` and `collector/main.go`. If the collector does NOT already serve static files from the dashboard directory, add a route to serve them.

In `collector/routes.go`, add BEFORE the API routes (or at the end of SetupRoutes):
```go
// Serve dashboard static files
dashboardFS := http.FileServer(http.Dir("../dashboard"))
mux.Handle("/", dashboardFS)
```

Or if the working directory might be different, use a configurable path. The key requirement: visiting `http://localhost:8080/` should serve `dashboard/index.html`.

**Important**: Make sure the static file route uses `/` pattern and is registered last (after all `/api/` routes), so API routes take precedence.

Also ensure that the `/` route doesn't interfere with the existing routes. The Go `http.ServeMux` pattern `/` matches all unmatched URLs, so it works as a catch-all for the SPA.

## Verification Steps
1. Verify `dashboard/index.html` exists and is a complete HTML file (has DOCTYPE, head, body, Vue app)
2. Verify it includes Vue 3, Element Plus, Chart.js CDN links
3. Verify it has all 7 pages implemented
4. Verify static serving is configured in collector routes
5. Run `cd /home/coder/log-monitor/collector && go build ./...` — must exit 0
6. Run `cd /home/coder/log-monitor && go build ./...` — must exit 0

## Constraints
- Do NOT create multiple files. Everything in ONE `dashboard/index.html`.
- Do NOT modify any backend handler logic.
- ONLY modify `collector/routes.go` (or `collector/main.go`) for static file serving.
- Do NOT install npm packages or run any build commands.
- The HTML file should be fully functional by just opening it in a browser (with API calls going to the backend).

## Commit
After verification passes:
```bash
cd /home/coder/log-monitor
git add -A
git commit -m "feat: Vue 3 dashboard SPA (R008)"
git push origin main
```

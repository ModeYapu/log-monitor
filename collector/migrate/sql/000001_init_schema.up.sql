-- Initial schema: core tables
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL,
    release TEXT DEFAULT '',
    env TEXT DEFAULT '',
    build_id TEXT DEFAULT '',
    user_id TEXT DEFAULT '',
    session_id TEXT DEFAULT '',
    type TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    stack TEXT DEFAULT '',
    url TEXT DEFAULT '',
    line INTEGER DEFAULT 0,
    col INTEGER DEFAULT 0,
    tags TEXT DEFAULT '{}',
    extra TEXT DEFAULT '{}',
    ua TEXT DEFAULT '',
    screen TEXT DEFAULT '',
    viewport TEXT DEFAULT '',
    performance TEXT DEFAULT '{}',
    ip TEXT DEFAULT '',
    fingerprint TEXT DEFAULT '',
    breadcrumbs TEXT DEFAULT '[]',
    project_id INTEGER DEFAULT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_app_created ON events(app_id, created_at);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(app_id, type, created_at);
CREATE INDEX IF NOT EXISTS idx_events_level ON events(app_id, level, created_at);
CREATE INDEX IF NOT EXISTS idx_events_appid ON events(app_id);
CREATE INDEX IF NOT EXISTS idx_events_level_only ON events(level);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_release ON events(app_id, release);
CREATE INDEX IF NOT EXISTS idx_events_env ON events(app_id, env);
CREATE INDEX IF NOT EXISTS idx_events_session_id ON events(session_id);
CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id);
CREATE INDEX IF NOT EXISTS idx_events_fingerprint ON events(app_id, fingerprint);

CREATE TABLE IF NOT EXISTS alert_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL,
    name TEXT NOT NULL,
    condition_type TEXT NOT NULL,
    condition_config TEXT NOT NULL,
    notify_type TEXT NOT NULL,
    notify_config TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    last_triggered_at INTEGER DEFAULT 0,
    cooldown_minutes INTEGER DEFAULT 30,
    silenced_until INTEGER DEFAULT 0,
    fingerprint TEXT DEFAULT '',
    message_template TEXT DEFAULT '',
    project_id INTEGER DEFAULT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS alert_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL,
    app_id TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_logs_created ON alert_logs(created_at);

CREATE TABLE IF NOT EXISTS system_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fingerprint TEXT NOT NULL,
    app_id TEXT NOT NULL,
    title TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'error',
    status TEXT NOT NULL DEFAULT 'open',
    priority TEXT NOT NULL DEFAULT 'medium',
    assignee TEXT DEFAULT '',
    first_seen_at INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    event_count INTEGER NOT NULL DEFAULT 0,
    user_count INTEGER NOT NULL DEFAULT 0,
    resolved_at INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    project_id INTEGER DEFAULT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_issues_fingerprint ON issues(app_id, fingerprint);
CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(app_id, status, updated_at DESC);

-- Multi-tenant project tables
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    api_key TEXT NOT NULL UNIQUE,
    retention_days INTEGER DEFAULT 30,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS project_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    role TEXT NOT NULL DEFAULT 'viewer',
    created_at INTEGER NOT NULL,
    UNIQUE(project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_project ON project_members(project_id);
CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);

-- Webhook delivery queue for persistent retry (Task 5)
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    webhook_id INTEGER NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    next_retry_at INTEGER NOT NULL,
    last_error TEXT DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status, next_retry_at);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);

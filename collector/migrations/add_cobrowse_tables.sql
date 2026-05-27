-- Migration: Add cobrowsing recording tables
-- Run this to add the tables manually if auto-migration fails

CREATE TABLE IF NOT EXISTS recordings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL UNIQUE,
	app_id TEXT NOT NULL,
	start_time INTEGER NOT NULL,
	end_time INTEGER DEFAULT 0,
	duration_ms INTEGER DEFAULT 0,
	event_count INTEGER DEFAULT 0,
	full_snapshot TEXT DEFAULT '',
	url TEXT DEFAULT '',
	ua TEXT DEFAULT '',
	status TEXT DEFAULT 'recording',
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS recording_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	seq INTEGER NOT NULL,
	timestamp INTEGER NOT NULL,
	event_data TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_recording_events_session ON recording_events(session_id, seq);

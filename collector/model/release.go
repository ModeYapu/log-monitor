package model

import "time"

// ReleaseHealth represents health metrics for a specific release
type ReleaseHealth struct {
	Release      string    `json:"release"`
	Env          string    `json:"env"`
	Version      string    `json:"version,omitempty"`
	TotalSessions int64    `json:"totalSessions"`
	CrashSessions int64    `json:"crashSessions"`
	CrashFreeRate float64   `json:"crashFreeRate"`
	ErrorCount   int64     `json:"errorCount"`
	FirstSeen    time.Time `json:"firstSeen"`
	LastSeen     time.Time `json:"lastSeen"`
	AdoptionRate float64   `json:"adoptionRate"` // Percentage of total sessions
}

// SessionStats represents overall session statistics
type SessionStats struct {
	TotalSessions int64     `json:"totalSessions"`
	CrashSessions int64     `json:"crashSessions"`
	CrashFreeRate float64   `json:"crashFreeRate"`
	ErrorCount    int64     `json:"errorCount"`
	AvgSessionDuration float64 `json:"avgSessionDuration"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
}

// ReleaseTrend represents release health over time
type ReleaseTrend struct {
	Release      string    `json:"release"`
	Env          string    `json:"env"`
	Timestamp    time.Time `json:"timestamp"`
	CrashRate    float64   `json:"crashRate"`
	SessionCount int64     `json:"sessionCount"`
}

package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// VerificationResult represents an E2E verification result from E2E Verifier
type VerificationResult struct {
	ID        int64         `json:"id"`
	ProjectID int64         `json:"project_id"` // Optional project association
	Site      string        `json:"site"`       // e.g., "travel-planner"
	Release   string        `json:"release"`    // e.g., "v1.2.3"
	Status    string        `json:"status"`     // "pass" or "fail"
	Score     float64       `json:"score"`      // Overall score (0-10)
	Checks    []CheckResult `json:"checks"`    // Individual check results
	Timestamp int64         `json:"timestamp"`  // Unix milliseconds
	CreatedAt int64         `json:"created_at"`
}

// CheckResult represents a single check within a verification
type CheckResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`   // "pass" or "fail"
	Message  string `json:"message"`
	Duration int64  `json:"duration"` // Duration in milliseconds
}

// StoreVerificationResult stores a verification result in the database
func (db *DB) StoreVerificationResult(result *VerificationResult) error {
	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	// Ensure table exists
	if err := db.ensureVerificationResultsTable(); err != nil {
		return fmt.Errorf("failed to ensure verification_results table: %w", err)
	}

	// Convert checks to JSON
	checksJSON, err := json.Marshal(result.Checks)
	if err != nil {
		return fmt.Errorf("failed to marshal checks: %w", err)
	}

	now := time.Now().UnixMilli()
	if result.CreatedAt == 0 {
		result.CreatedAt = now
	}

	query := `
		INSERT INTO verification_results (
			project_id, site, release, status, score, checks_json,
			timestamp, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	insertResult, err := db.conn.Exec(query,
		result.ProjectID, result.Site, result.Release, result.Status,
		result.Score, string(checksJSON), result.Timestamp, result.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert verification result: %w", err)
	}

	id, _ := insertResult.LastInsertId()
	result.ID = id

	return nil
}

// GetVerificationResults retrieves verification results for a site
func (db *DB) GetVerificationResults(site string, limit int) ([]VerificationResult, error) {
	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 100 {
		limit = 10
	}

	// Ensure table exists
	if err := db.ensureVerificationResultsTable(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, project_id, site, release, status, score, checks_json,
		       timestamp, created_at
		FROM verification_results
		WHERE site = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := db.conn.Query(query, site, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query verification results: %w", err)
	}
	defer rows.Close()

	var results []VerificationResult
	for rows.Next() {
		var v VerificationResult
		var checksJSON string
		var projectID sql.NullInt64

		err := rows.Scan(
			&v.ID, &projectID, &v.Site, &v.Release, &v.Status,
			&v.Score, &checksJSON, &v.Timestamp, &v.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan verification result: %w", err)
		}

		if projectID.Valid {
			v.ProjectID = projectID.Int64
		}

		// Parse checks JSON
		if checksJSON != "" {
			if err := json.Unmarshal([]byte(checksJSON), &v.Checks); err != nil {
				slog.Warn("Failed to parse checks JSON", "error", err)
				v.Checks = []CheckResult{}
			}
		}

		results = append(results, v)
	}

	return results, nil
}

// GetLatestVerificationResult gets the most recent verification result for a site/release
func (db *DB) GetLatestVerificationResult(site, release string) (*VerificationResult, error) {
	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Ensure table exists
	if err := db.ensureVerificationResultsTable(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, project_id, site, release, status, score, checks_json,
		       timestamp, created_at
		FROM verification_results
		WHERE site = ? AND release = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var v VerificationResult
	var checksJSON string
	var projectID sql.NullInt64

	err := db.conn.QueryRow(query, site, release).Scan(
		&v.ID, &projectID, &v.Site, &v.Release, &v.Status,
		&v.Score, &checksJSON, &v.Timestamp, &v.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query latest verification result: %w", err)
	}

	if projectID.Valid {
		v.ProjectID = projectID.Int64
	}

	// Parse checks JSON
	if checksJSON != "" {
		if err := json.Unmarshal([]byte(checksJSON), &v.Checks); err != nil {
			slog.Warn("Failed to parse checks JSON", "error", err)
			v.Checks = []CheckResult{}
		}
	}

	return &v, nil
}

// GetFailedVerifications returns failed verifications for alerting
func (db *DB) GetFailedVerifications(since int64) ([]VerificationResult, error) {
	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	if since == 0 {
		since = time.Now().Add(-24 * time.Hour).UnixMilli()
	}

	// Ensure table exists
	if err := db.ensureVerificationResultsTable(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, project_id, site, release, status, score, checks_json,
		       timestamp, created_at
		FROM verification_results
		WHERE status = 'fail' AND created_at >= ?
		ORDER BY created_at DESC
	`

	rows, err := db.conn.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query failed verifications: %w", err)
	}
	defer rows.Close()

	var results []VerificationResult
	for rows.Next() {
		var v VerificationResult
		var checksJSON string
		var projectID sql.NullInt64

		err := rows.Scan(
			&v.ID, &projectID, &v.Site, &v.Release, &v.Status,
			&v.Score, &checksJSON, &v.Timestamp, &v.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan failed verification: %w", err)
		}

		if projectID.Valid {
			v.ProjectID = projectID.Int64
		}

		// Parse checks JSON
		if checksJSON != "" {
			if err := json.Unmarshal([]byte(checksJSON), &v.Checks); err != nil {
				slog.Warn("Failed to parse checks JSON", "error", err)
				v.Checks = []CheckResult{}
			}
		}

		results = append(results, v)
	}

	return results, nil
}

// ensureVerificationResultsTable creates the verification_results table if it doesn't exist
func (db *DB) ensureVerificationResultsTable() error {
	schema := `
		CREATE TABLE IF NOT EXISTS verification_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER DEFAULT NULL,
			site TEXT NOT NULL,
			release TEXT NOT NULL,
			status TEXT NOT NULL,
			score REAL DEFAULT 0,
			checks_json TEXT DEFAULT '{}',
			timestamp INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_verification_results_site ON verification_results(site, timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_verification_results_release ON verification_results(release, timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_verification_results_status ON verification_results(status, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_verification_results_project ON verification_results(project_id);
	`

	_, err := db.conn.Exec(schema)
	return err
}

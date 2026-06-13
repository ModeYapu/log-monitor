package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SourceMapRecord represents a source map database record
type SourceMapRecord struct {
	ID          int64
	AppID       string
	Release     string
	Env         string
	BuildID     string
	FilePath    string
	OriginalURL string
	FileSize    int64
	UploadedAt  int64
}

// EnsureSourceMapsTable creates the source_maps table if it doesn't exist
func (db *DB) EnsureSourceMapsTable() error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	schema := `
	CREATE TABLE IF NOT EXISTS source_maps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id TEXT NOT NULL,
		release TEXT NOT NULL,
		env TEXT NOT NULL DEFAULT 'production',
		build_id TEXT NOT NULL,
		file_path TEXT NOT NULL,
		original_url TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		uploaded_at INTEGER NOT NULL
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_sourcemaps_app_release_build ON source_maps(app_id, release, env, build_id);
	CREATE INDEX IF NOT EXISTS idx_sourcemaps_app_id ON source_maps(app_id);
	CREATE INDEX IF NOT EXISTS idx_sourcemaps_build_id ON source_maps(build_id);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// CreateSourceMap creates a new source map record
func (db *DB) CreateSourceMap(record SourceMapRecord) (int64, error) {

	if db.closed.Load() {
		return 0, fmt.Errorf("database is closed")
	}

	result, err := db.conn.Exec(`
		INSERT INTO source_maps (app_id, release, env, build_id, file_path, original_url, file_size, uploaded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, record.AppID, record.Release, record.Env, record.BuildID, record.FilePath,
		record.OriginalURL, record.FileSize, record.UploadedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create source map: %w", err)
	}

	return result.LastInsertId()
}

// GetSourceMap retrieves a source map by app_id, release, env, and build_id
func (db *DB) GetSourceMap(appID, release, env, buildID string) (*SourceMapRecord, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	var record SourceMapRecord
	err := db.conn.QueryRow(`
		SELECT id, app_id, release, env, build_id, file_path, original_url, file_size, uploaded_at
		FROM source_maps
		WHERE app_id = ? AND release = ? AND env = ? AND build_id = ?
	`, appID, release, env, buildID).Scan(
		&record.ID, &record.AppID, &record.Release, &record.Env, &record.BuildID,
		&record.FilePath, &record.OriginalURL, &record.FileSize, &record.UploadedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get source map: %w", err)
	}

	return &record, nil
}

// GetSourceMapByBuildID retrieves a source map by build_id only
func (db *DB) GetSourceMapByBuildID(buildID string) (*SourceMapRecord, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	var record SourceMapRecord
	err := db.conn.QueryRow(`
		SELECT id, app_id, release, env, build_id, file_path, original_url, file_size, uploaded_at
		FROM source_maps
		WHERE build_id = ?
	`, buildID).Scan(
		&record.ID, &record.AppID, &record.Release, &record.Env, &record.BuildID,
		&record.FilePath, &record.OriginalURL, &record.FileSize, &record.UploadedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get source map by build_id: %w", err)
	}

	return &record, nil
}

// ListSourceMaps retrieves all source maps for an app
func (db *DB) ListSourceMaps(appID string, limit int) ([]SourceMapRecord, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	rows, err := db.conn.Query(`
		SELECT id, app_id, release, env, build_id, file_path, original_url, file_size, uploaded_at
		FROM source_maps
		WHERE app_id = ?
		ORDER BY uploaded_at DESC
		LIMIT ?
	`, appID, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to list source maps: %w", err)
	}
	defer rows.Close()

	var records []SourceMapRecord
	for rows.Next() {
		var record SourceMapRecord
		err := rows.Scan(
			&record.ID, &record.AppID, &record.Release, &record.Env, &record.BuildID,
			&record.FilePath, &record.OriginalURL, &record.FileSize, &record.UploadedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan source map: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// DeleteSourceMap deletes a source map by ID
func (db *DB) DeleteSourceMap(id int64) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	// First get the file path to delete the file
	var filePath string
	err := db.conn.QueryRow("SELECT file_path FROM source_maps WHERE id = ?", id).Scan(&filePath)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("source map not found")
		}
		return fmt.Errorf("failed to get source map file path: %w", err)
	}

	// Delete the database record
	_, err = db.conn.Exec("DELETE FROM source_maps WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete source map: %w", err)
	}

	// Delete the file
	if filePath != "" {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			// Log but don't fail if file deletion fails
			slog.Info("[sourcemap] Failed to delete file %s: %v\n", filePath, err)
		}
	}

	return nil
}

// SourceMapStorage handles source map file storage
type SourceMapStorage struct {
	baseDir string
	mu      sync.RWMutex
}

// NewSourceMapStorage creates a new source map storage
func NewSourceMapStorage(baseDir string) (*SourceMapStorage, error) {
	dir := filepath.Join(baseDir, "sourcemaps")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sourcemaps directory: %w", err)
	}

	return &SourceMapStorage{baseDir: dir}, nil
}

// Save saves a source map file and returns the file path and size
func (s *SourceMapStorage) Save(appID, release, buildID string, content []byte) (string, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create subdirectory for app
	appDir := filepath.Join(s.baseDir, sanitizePathSegment(appID))
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create app directory: %w", err)
	}

	// Generate filename: {release}-{buildID}.map
	filename := sanitizePathSegment(release) + "-" + sanitizePathSegment(buildID) + ".map"
	filePath := filepath.Join(appDir, filename)

	// Write file
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return "", 0, fmt.Errorf("failed to write source map file: %w", err)
	}

	return filePath, int64(len(content)), nil
}

// GetPath returns the file path for a source map
func (s *SourceMapStorage) GetPath(appID, release, buildID string) string {
	filename := sanitizePathSegment(release) + "-" + sanitizePathSegment(buildID) + ".map"
	return filepath.Join(s.baseDir, sanitizePathSegment(appID), filename)
}

// Get retrieves the source map file content
func (s *SourceMapStorage) Get(appID, release, buildID string) ([]byte, error) {
	filePath := s.GetPath(appID, release, buildID)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source map file: %w", err)
	}
	return content, nil
}

// GetByPath retrieves the source map file content by path
func (s *SourceMapStorage) GetByPath(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source map file: %w", err)
	}
	return content, nil
}

// Delete deletes a source map file
func (s *SourceMapStorage) Delete(filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete source map file: %w", err)
	}
	return nil
}

// CleanupOld removes source map files older than the specified days
func (s *SourceMapStorage) CleanupOld(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)

	return filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				slog.Info("[sourcemap] Failed to delete old file %s: %v\n", path, err)
			}
		}
		return nil
	})
}

// GetSourceMap retrieves a source map file by appId, release, and filename (buildID)
// The filename should be the buildID (without .map extension) or the full filename
func (s *SourceMapStorage) GetSourceMap(appID, release, filename string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Sanitize inputs
	appDir := filepath.Join(s.baseDir, sanitizePathSegment(appID))

	// If filename doesn't have .map extension, add it with release prefix
	ext := filepath.Ext(filename)
	var filePath string
	if ext == ".map" || ext == ".json" {
		// Full filename provided - try to match in the app directory
		entries, err := os.ReadDir(appDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read app directory: %w", err)
		}
		for _, entry := range entries {
			if entry.Name() == filename {
				filePath = filepath.Join(appDir, filename)
				break
			}
		}
		if filePath == "" {
			return nil, fmt.Errorf("source map file not found: %s", filename)
		}
	} else {
		// buildID provided - construct the filename
		filePath = s.GetPath(appID, release, filename)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source map file: %w", err)
	}
	return content, nil
}

// ListSourceMaps lists all source map filenames for an app and optional release
// If release is empty, returns all source maps for the app
func (s *SourceMapStorage) ListSourceMaps(appID, release string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	appDir := filepath.Join(s.baseDir, sanitizePathSegment(appID))

	entries, err := os.ReadDir(appDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read app directory: %w", err)
	}

	var filenames []string
	releasePrefix := sanitizePathSegment(release)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".map" && ext != ".json" {
			continue
		}

		// Filter by release if specified
		if release != "" {
			// Check if filename starts with "{release}-"
			if !strings.HasPrefix(name, releasePrefix+"-") {
				continue
			}
		}

		filenames = append(filenames, name)
	}

	return filenames, nil
}

func sanitizePathSegment(s string) string {
	// Remove any path separators and special characters
	result := ""
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' {
			result += string(c)
		} else {
			result += "_"
		}
	}
	if result == "" {
		result = "_"
	}
	return result
}

package storage

import (
	"time"
)

// SQLiteStore implements Store interface using SQLite database
type SQLiteStore struct {
	db          *DB
	userStorage *UserStorage
}

// NewSQLiteStore creates a new SQLite store
func NewSQLiteStore(cfg Config) (*SQLiteStore, error) {
	db, err := NewDB(cfg)
	if err != nil {
		return nil, err
	}
	return &SQLiteStore{
		db:          db,
		userStorage: NewUserStorage(db),
	}, nil
}

// Events returns the event store
func (s *SQLiteStore) Events() EventStore {
	return s.db
}

// Issues returns the issue store
func (s *SQLiteStore) Issues() IssueStore {
	return s.db
}

// Projects returns the project store
func (s *SQLiteStore) Projects() ProjectStore {
	return s.db
}

// Alerts returns the alert store
func (s *SQLiteStore) Alerts() AlertStore {
	return s.db
}

// Analytics returns the analytics store
func (s *SQLiteStore) Analytics() AnalyticsStore {
	return s.db
}

// System returns the system store with wrapper methods
func (s *SQLiteStore) System() SystemStore {
	return &systemStoreWrapper{db: s.db}
}

// systemStoreWrapper wraps DB to provide SystemStore interface methods
type systemStoreWrapper struct {
	db *DB
}

func (w *systemStoreWrapper) GetStorageStats() (*StorageStats, error) {
	return w.db.GetStorageStats()
}

func (w *systemStoreWrapper) GetRetentionPolicySimple() (int, error) {
	return w.db.GetRetentionPolicySimple()
}

func (w *systemStoreWrapper) SetRetentionPolicySimple(days int) error {
	return w.db.SetRetentionPolicySimple(days)
}

func (w *systemStoreWrapper) TriggerManualCleanup() error {
	return w.db.TriggerManualCleanup()
}

func (w *systemStoreWrapper) GetLastCleanupTime() int64 {
	return w.db.GetLastCleanupTime()
}

func (w *systemStoreWrapper) SetLastCleanupTime(timestamp int64) error {
	return w.db.SetLastCleanupTime(timestamp)
}

func (w *systemStoreWrapper) CleanupOldDataWithDays(days int) CleanupResult {
	return w.db.CleanupOldDataWithDays(days)
}

func (w *systemStoreWrapper) DeleteEventsBefore(before time.Time) (int64, error) {
	return w.db.DeleteEventsBefore(before)
}

func (w *systemStoreWrapper) DeleteRecordingsBefore(before time.Time) (int64, error) {
	return w.db.DeleteRecordingsBefore(before)
}

func (w *systemStoreWrapper) Ping() error {
	return w.db.Ping()
}

// Recordings returns the recording repository
func (s *SQLiteStore) Recordings() RecordingRepository {
	return s.db
}

// SourceMaps returns the source map repository
func (s *SQLiteStore) SourceMaps() SourceMapRepository {
	return s.db
}

// Users returns the user repository
func (s *SQLiteStore) Users() UserRepository {
	return s.userStorage
}

// AuditLogs returns the audit store
func (s *SQLiteStore) AuditLogs() AuditStore {
	return s.db
}

// PerformanceMetrics returns the performance store
func (s *SQLiteStore) PerformanceMetrics() PerformanceStore {
	return s.db
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

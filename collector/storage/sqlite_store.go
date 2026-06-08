package storage

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

// Events returns the event repository
func (s *SQLiteStore) Events() EventRepository {
	return s.db
}

// Alerts returns the alert repository
func (s *SQLiteStore) Alerts() AlertRepository {
	return s.db
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

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

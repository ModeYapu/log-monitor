package storage

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/logmonitor/collector/model"
)

// UserStorage handles user database operations
type UserStorage struct {
	db   *DB
	mu   sync.RWMutex
}

// NewUserStorage creates a new user storage
func NewUserStorage(db *DB) *UserStorage {
	return &UserStorage{db: db}
}

// EnsureUsersTable creates the users table if it doesn't exist
func (s *UserStorage) EnsureUsersTable() error {

	if s.db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		display_name TEXT DEFAULT '',
		role TEXT NOT NULL DEFAULT 'user',
		enabled INTEGER NOT NULL DEFAULT 1,
		last_login_at INTEGER DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
	`

	_, err := s.db.conn.Exec(schema)
	return err
}

// CreateUser creates a new user with hashed password
func (s *UserStorage) CreateUser(username, passwordHash, displayName, role string) (int64, error) {

	if s.db.closed.Load() {
		return 0, fmt.Errorf("database is closed")
	}

	createdAt, updatedAt := model.Timestamps()
	result, err := s.db.conn.Exec(`
		INSERT INTO users (username, password_hash, display_name, role, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
	`, username, passwordHash, displayName, role, createdAt, updatedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	return result.LastInsertId()
}

// GetUserByUsername retrieves a user by username
func (s *UserStorage) GetUserByUsername(username string) (*model.User, string, error) {

	if s.db.closed.Load() {
		return nil, "", fmt.Errorf("database is closed")
	}

	var user model.User
	var passwordHash string
	err := s.db.conn.QueryRow(`
		SELECT id, username, password_hash, display_name, role, enabled, last_login_at, created_at, updated_at
		FROM users WHERE username = ?
	`, username).Scan(
		&user.ID, &user.Username, &passwordHash, &user.DisplayName, &user.Role,
		&user.Enabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}

	return &user, passwordHash, nil
}

// GetUserByID retrieves a user by ID
func (s *UserStorage) GetUserByID(id int64) (*model.User, error) {

	if s.db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	var user model.User
	err := s.db.conn.QueryRow(`
		SELECT id, username, display_name, role, enabled, last_login_at, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Username, &user.DisplayName, &user.Role,
		&user.Enabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// ListUsers retrieves all users
func (s *UserStorage) ListUsers() ([]model.User, error) {

	if s.db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := s.db.conn.Query(`
		SELECT id, username, display_name, role, enabled, last_login_at, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var user model.User
		err := rows.Scan(
			&user.ID, &user.Username, &user.DisplayName, &user.Role,
			&user.Enabled, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// UpdateUser updates a user's display name, role, and enabled status
func (s *UserStorage) UpdateUser(id int64, displayName, role string, enabled bool) error {

	if s.db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, updatedAt := model.Timestamps()
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	_, err := s.db.conn.Exec(`
		UPDATE users SET display_name = ?, role = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, displayName, role, enabledInt, updatedAt, id)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// UpdatePassword updates a user's password
func (s *UserStorage) UpdatePassword(id int64, passwordHash string) error {

	if s.db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, updatedAt := model.Timestamps()
	_, err := s.db.conn.Exec(`
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?
	`, passwordHash, updatedAt, id)

	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// UpdateLastLogin updates the last login timestamp
func (s *UserStorage) UpdateLastLogin(id int64) error {

	if s.db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	now, _ := model.Timestamps()
	_, err := s.db.conn.Exec(`
		UPDATE users SET last_login_at = ? WHERE id = ?
	`, now, id)

	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// DeleteUser deletes a user by ID
func (s *UserStorage) DeleteUser(id int64) error {

	if s.db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := s.db.conn.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// CountUsers returns the total number of users
func (s *UserStorage) CountUsers() (int64, error) {

	if s.db.closed.Load() {
		return 0, fmt.Errorf("database is closed")
	}

	var count int64
	err := s.db.conn.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

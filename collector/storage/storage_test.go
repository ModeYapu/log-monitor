package storage

import (
	"testing"
)

// TestDB_Init tests database initialization
func TestDB_Init(t *testing.T) {
	cfg := Config{
		Path:          ":memory:",
		RetentionDays: 30,
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("Database is nil")
	}

	if db.conn == nil {
		t.Fatal("Database connection is nil")
	}
}

// TestDB_Close tests database closing
func TestDB_Close(t *testing.T) {
	cfg := Config{
		Path:          ":memory:",
		RetentionDays: 30,
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Verify database is closed
	if !db.closed {
		t.Error("Database should be marked as closed")
	}
}
package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// RunEmbedded runs migrations from embedded FS against the given database.
// If the database is fresh (no schema_migrations table), all migrations run.
// If migrations have been run before, only new ones execute.
func RunEmbedded(db *sql.DB, fs embed.FS, dir string) error {
	drv, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("migrate: create driver: %w", err)
	}

	src, err := iofs.New(fs, dir)
	if err != nil {
		return fmt.Errorf("migrate: create source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", drv)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate: up: %w", err)
	}

	version, dirty, _ := m.Version()
	slog.Info("Database migration complete", "version", version, "dirty", dirty)
	return nil
}

// Version returns the current migration version.
func Version(db *sql.DB, fs embed.FS, dir string) (uint, bool, error) {
	drv, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return 0, false, err
	}

	src, err := iofs.New(fs, dir)
	if err != nil {
		return 0, false, err
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", drv)
	if err != nil {
		return 0, false, err
	}

	return m.Version()
}

package migrate

import "embed"

// MigrationsFS embeds the SQL migration files.
// Usage: migrate.RunEmbedded(db, MigrationsFS, ".")
//
//go:embed all:sql
var MigrationsFS embed.FS

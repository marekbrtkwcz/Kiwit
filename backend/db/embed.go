package dbfiles

import "embed"

//go:embed migrations/sqlite/*.sql
var SQLiteMigrations embed.FS

//go:embed migrations/*.sql
var PostgresMigrations embed.FS

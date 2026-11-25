package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	dbfiles "github.com/kiwit/kiwit/db"
)

// OpenSQLite opens a SQLite database (pure Go, no CGO) and runs embedded migrations.
func OpenSQLite(ctx context.Context, dsn string) (*sql.DB, error) {
	path := strings.TrimPrefix(dsn, "sqlite://")
	if path == "" {
		path = "./kiwit.db"
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Recommended pragmas for concurrent web workloads
	if _, err := sqlDB.ExecContext(ctx, "PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("configure sqlite pragmas: %w", err)
	}

	goose.SetBaseFS(dbfiles.SQLiteMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, "migrations/sqlite"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("run sqlite migrations: %w", err)
	}

	return sqlDB, nil
}

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

	// Pragmas are set via the DSN so they apply to every connection the pool
	// opens (PRAGMA foreign_keys is per-connection in SQLite — setting it once
	// on *sql.DB only covers whichever connection happened to run the Exec).
	// busy_timeout avoids SQLITE_BUSY under concurrent writes.
	out := path
	if !strings.Contains(out, "_pragma=") {
		// append pragmas to the DSN
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		out += sep + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}
	sqlDB, err := sql.Open("sqlite", out)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite allows a single writer; one connection avoids SQLITE_BUSY and
	// makes the pragmas above deterministic.
	sqlDB.SetMaxOpenConns(1)

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

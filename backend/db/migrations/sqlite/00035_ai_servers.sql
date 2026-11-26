-- +goose Up
CREATE TABLE ai_servers (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT NOT NULL UNIQUE,
    provider      TEXT NOT NULL DEFAULT 'custom',
    base_url      TEXT NOT NULL,
    model         TEXT NOT NULL,
    api_key       TEXT NOT NULL DEFAULT '',
    temperature   REAL NOT NULL DEFAULT 0.3,
    max_tokens    INTEGER NOT NULL DEFAULT 2000,
    timeout_sec   INTEGER NOT NULL DEFAULT 60,
    retries       INTEGER NOT NULL DEFAULT 1,
    is_default    INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS ai_servers;

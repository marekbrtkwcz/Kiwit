-- +goose Up
ALTER TABLE test_plans ADD COLUMN parent_id INTEGER REFERENCES test_plans(id) ON DELETE SET NULL;

CREATE TABLE environments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    properties  TEXT NOT NULL DEFAULT '{}',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS environments;

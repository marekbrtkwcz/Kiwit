-- +goose Up
CREATE TABLE exploratory_sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id       INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    test_run_id      INTEGER REFERENCES test_runs(id) ON DELETE SET NULL,
    charter          TEXT NOT NULL,
    timebox_minutes  INTEGER NOT NULL DEFAULT 60,
    tester_id        INTEGER REFERENCES users(id) ON DELETE SET NULL,
    notes            TEXT NOT NULL DEFAULT '',
    findings         TEXT NOT NULL DEFAULT '[]',
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_exploratory_product ON exploratory_sessions(product_id);
CREATE INDEX idx_exploratory_run ON exploratory_sessions(test_run_id);

-- +goose Down
DROP TABLE IF EXISTS exploratory_sessions;

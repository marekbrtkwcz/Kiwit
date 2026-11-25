-- +goose Up
ALTER TABLE issue_trackers ADD COLUMN api_username TEXT NOT NULL DEFAULT '';
ALTER TABLE bugs ADD COLUMN last_synced_at DATETIME;

CREATE TABLE internal_bugs (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    product_id        INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    build_id          INTEGER REFERENCES builds(id) ON DELETE SET NULL,
    test_execution_id INTEGER REFERENCES test_executions(id) ON DELETE SET NULL,
    summary           TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'open',
    severity          TEXT NOT NULL DEFAULT 'medium',
    reporter_id       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    assignee_id       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_internal_bugs_product ON internal_bugs(product_id);
CREATE INDEX idx_internal_bugs_exec ON internal_bugs(test_execution_id);

-- +goose Down
DROP TABLE IF EXISTS internal_bugs;

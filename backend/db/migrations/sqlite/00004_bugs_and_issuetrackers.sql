-- +goose Up
CREATE TABLE issue_trackers (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    provider   TEXT NOT NULL, -- 'github' | 'gitlab' | 'jira'
    base_url   TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE bugs (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    test_execution_id INTEGER NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    tracker_id        INTEGER REFERENCES issue_trackers(id) ON DELETE SET NULL,
    external_key      TEXT NOT NULL,
    url               TEXT NOT NULL,
    title             TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'open',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_bugs_execution ON bugs(test_execution_id);

-- +goose Down
DROP TABLE IF EXISTS bugs;
DROP TABLE IF EXISTS issue_trackers;

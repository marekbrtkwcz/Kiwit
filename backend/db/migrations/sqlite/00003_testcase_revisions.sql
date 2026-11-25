-- +goose Up
CREATE TABLE test_case_revisions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    test_case_id   INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    version        INTEGER NOT NULL,
    summary        TEXT NOT NULL,
    preconditions  TEXT NOT NULL DEFAULT '',
    format         TEXT NOT NULL DEFAULT 'steps',
    gherkin_body   TEXT NOT NULL DEFAULT '',
    priority       TEXT NOT NULL DEFAULT 'medium',
    status         TEXT NOT NULL DEFAULT 'confirmed',
    steps_snapshot TEXT NOT NULL DEFAULT '[]',
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (test_case_id, version)
);

-- +goose Down
DROP TABLE IF EXISTS test_case_revisions;

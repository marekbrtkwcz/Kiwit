-- +goose Up
ALTER TABLE test_cases ADD COLUMN default_tester_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE test_cases ADD COLUMN reviewer_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE test_cases ADD COLUMN requirement TEXT NOT NULL DEFAULT '';
ALTER TABLE test_cases ADD COLUMN extra_link TEXT NOT NULL DEFAULT '';
ALTER TABLE test_cases ADD COLUMN notes TEXT NOT NULL DEFAULT '';

ALTER TABLE test_plans ADD COLUMN plan_type TEXT NOT NULL DEFAULT 'regression';
ALTER TABLE test_plans ADD COLUMN extra_link TEXT NOT NULL DEFAULT '';

ALTER TABLE test_runs ADD COLUMN environment_id INTEGER REFERENCES environments(id) ON DELETE SET NULL;
ALTER TABLE test_runs ADD COLUMN default_tester_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE test_runs ADD COLUMN planned_start DATETIME;
ALTER TABLE test_runs ADD COLUMN planned_stop DATETIME;

ALTER TABLE test_executions ADD COLUMN environment_id INTEGER REFERENCES environments(id) ON DELETE SET NULL;
ALTER TABLE test_executions ADD COLUMN case_text_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE test_executions ADD COLUMN started_at DATETIME;
ALTER TABLE test_executions ADD COLUMN stopped_at DATETIME;

CREATE TABLE execution_links (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    test_execution_id INTEGER NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    url               TEXT NOT NULL,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_execution_links_exec ON execution_links(test_execution_id);

-- +goose Down
DROP TABLE IF EXISTS execution_links;
